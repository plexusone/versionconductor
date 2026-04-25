package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/plexusone/versionconductor/internal/collector"
	"github.com/plexusone/versionconductor/internal/merger"
	"github.com/plexusone/versionconductor/internal/policy"
	"github.com/plexusone/versionconductor/internal/report"
	"github.com/plexusone/versionconductor/pkg/model"
)

var mergeCmd = &cobra.Command{
	Use:   "merge",
	Short: "Auto-merge approved dependency PRs",
	Long: `Merge dependency PRs that have been approved and pass all checks.

By default, this runs in dry-run mode. Use --execute to actually merge PRs.

Examples:
  # Dry-run: show what would be merged
  versionconductor merge --orgs myorg

  # Merge with balanced profile
  versionconductor merge --orgs myorg --profile balanced --execute

  # Merge only patch updates
  versionconductor merge --orgs myorg --update-type patch --execute

  # Use squash merge strategy
  versionconductor merge --orgs myorg --strategy squash --execute`,
	RunE: runMerge,
}

func init() {
	rootCmd.AddCommand(mergeCmd)

	mergeCmd.Flags().String("profile", "balanced", "Merge profile: aggressive, balanced, conservative")
	mergeCmd.Flags().String("strategy", "squash", "Merge strategy: merge, squash, rebase")
	mergeCmd.Flags().Bool("execute", false, "Actually merge PRs (default is dry-run)")
	mergeCmd.Flags().Bool("delete-branch", true, "Delete branch after merge")
	mergeCmd.Flags().Int("max-prs", 0, "Maximum number of PRs to merge (0 = no limit)")
	mergeCmd.Flags().Bool("wait-for-checks", false, "Wait for pending checks to complete")
	mergeCmd.Flags().Int("checks-timeout", 300, "Timeout in seconds for waiting on checks")
	mergeCmd.Flags().StringSlice("update-type", nil, "Filter by update type: major, minor, patch")
	mergeCmd.Flags().String("bot", "", "Filter by dependency bot: renovate, dependabot")

	_ = viper.BindPFlag("merge.profile", mergeCmd.Flags().Lookup("profile"))
	_ = viper.BindPFlag("merge.strategy", mergeCmd.Flags().Lookup("strategy"))
	_ = viper.BindPFlag("merge.execute", mergeCmd.Flags().Lookup("execute"))
	_ = viper.BindPFlag("merge.delete-branch", mergeCmd.Flags().Lookup("delete-branch"))
	_ = viper.BindPFlag("merge.max-prs", mergeCmd.Flags().Lookup("max-prs"))
	_ = viper.BindPFlag("merge.wait-for-checks", mergeCmd.Flags().Lookup("wait-for-checks"))
	_ = viper.BindPFlag("merge.checks-timeout", mergeCmd.Flags().Lookup("checks-timeout"))
	_ = viper.BindPFlag("merge.update-type", mergeCmd.Flags().Lookup("update-type"))
	_ = viper.BindPFlag("merge.bot", mergeCmd.Flags().Lookup("bot"))
}

func runMerge(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	token := viper.GetString("token")
	if token == "" {
		return fmt.Errorf("GitHub token required. Set GITHUB_TOKEN or use --token flag")
	}

	orgs := viper.GetStringSlice("orgs")
	repos := viper.GetStringSlice("repos")

	if len(orgs) == 0 && len(repos) == 0 {
		return fmt.Errorf("at least one organization (--orgs) or repository (--repos) required")
	}

	execute := viper.GetBool("merge.execute")
	dryRun := !execute
	verbose := viper.GetBool("verbose")

	// Get merge profile
	profileName := viper.GetString("merge.profile")
	profile := policy.GetProfile(profileName)
	if profile == nil {
		return fmt.Errorf("unknown profile: %s", profileName)
	}

	// Override profile settings with flags
	if strategy := viper.GetString("merge.strategy"); strategy != "" {
		profile.MergeStrategy = strategy
	}
	profile.DeleteBranch = viper.GetBool("merge.delete-branch")
	if maxPRs := viper.GetInt("merge.max-prs"); maxPRs > 0 {
		profile.MaxPRsPerRun = maxPRs
	}

	// Create collector and merger
	coll := collector.NewGitHub(token)
	merg := merger.NewGitHub(token)

	// Build filters
	repoFilter := model.RepoFilter{
		IncludePrivate: true,
	}

	prFilter := model.PRFilter{
		State:       "open",
		MinAgeHours: profile.MinAgeHours,
	}

	if bot := viper.GetString("merge.bot"); bot != "" {
		prFilter.DependBot = model.DependBot(bot)
	}

	if updateTypes := viper.GetStringSlice("merge.update-type"); len(updateTypes) > 0 {
		for _, t := range updateTypes {
			prFilter.UpdateTypes = append(prFilter.UpdateTypes, model.UpdateType(t))
		}
	}

	// Collect repositories
	var allRepos []model.Repo

	if len(orgs) > 0 {
		if verbose {
			fmt.Fprintf(os.Stderr, "Scanning organizations: %v\n", orgs)
		}
		reposFromOrgs, err := coll.ListRepos(ctx, orgs, repoFilter)
		if err != nil {
			return fmt.Errorf("failed to list repositories: %w", err)
		}
		allRepos = append(allRepos, reposFromOrgs...)
	}

	for _, repoRef := range repos {
		ref := model.ParseRepoRef(repoRef)
		allRepos = append(allRepos, model.Repo{
			Owner:    ref.Owner,
			Name:     ref.Name,
			FullName: ref.FullName(),
		})
	}

	// Collect and evaluate PRs
	result := model.MergeResult{
		Timestamp: time.Now(),
		DryRun:    dryRun,
	}

	mergeCount := 0

	for _, repo := range allRepos {
		if profile.MaxPRsPerRun > 0 && mergeCount >= profile.MaxPRsPerRun {
			break
		}

		ref := model.RepoRef{Owner: repo.Owner, Name: repo.Name}

		prs, err := coll.ListDependencyPRs(ctx, ref)
		if err != nil {
			if verbose {
				fmt.Fprintf(os.Stderr, "Error listing PRs for %s: %v\n", repo.FullName, err)
			}
			continue
		}

		for _, pr := range prs {
			if profile.MaxPRsPerRun > 0 && mergeCount >= profile.MaxPRsPerRun {
				break
			}

			if !matchesPRFilter(pr, prFilter) {
				continue
			}

			// Get checks
			checks, err := coll.GetPRChecks(ctx, ref, pr.Number)
			if err != nil {
				if verbose {
					fmt.Fprintf(os.Stderr, "Error getting checks for %s#%d: %v\n", repo.FullName, pr.Number, err)
				}
				continue
			}

			pr.TestsPassed = collector.TestsPassed(checks)

			// Get mergeable status
			prDetails, err := coll.GetPRDetails(ctx, ref, pr.Number)
			if err == nil {
				pr.Mergeable = prDetails.Mergeable
				pr.MergeableStr = prDetails.MergeableStr
			}

			// Evaluate against profile
			shouldMerge, reason := policy.EvaluateProfile(profile, &pr, checks)

			if !shouldMerge {
				result.Skipped = append(result.Skipped, model.SkippedPR{
					PR:     pr,
					Reason: reason,
				})
				continue
			}

			// Merge the PR
			if dryRun {
				if verbose {
					fmt.Fprintf(os.Stderr, "Would merge %s#%d: %s\n", repo.FullName, pr.Number, pr.Title)
				}
				result.Merged = append(result.Merged, model.MergedPR{
					PR:       pr,
					MergedBy: "dry-run",
				})
				mergeCount++
			} else {
				if verbose {
					fmt.Fprintf(os.Stderr, "Merging %s#%d: %s\n", repo.FullName, pr.Number, pr.Title)
				}

				info, err := merg.MergePR(ctx, ref, pr.Number, merger.MergeStrategy(profile.MergeStrategy), "")
				if err != nil {
					result.Failed = append(result.Failed, model.FailedPR{
						PR:    pr,
						Error: err.Error(),
					})
					continue
				}

				result.Merged = append(result.Merged, model.MergedPR{
					PR:       pr,
					MergedBy: "versionconductor",
					SHA:      info.SHA,
				})
				mergeCount++
			}
		}
	}

	result.MergedCount = len(result.Merged)
	result.SkippedCount = len(result.Skipped)
	result.FailedCount = len(result.Failed)

	// Generate output
	format := viper.GetString("format")
	var formatter report.Formatter

	switch format {
	case "json":
		formatter = report.NewJSONFormatter()
	case "markdown", "md":
		formatter = report.NewMarkdownFormatter()
	default:
		formatter = report.NewTableFormatter()
	}

	output, err := formatter.FormatMergeResult(&result)
	if err != nil {
		return fmt.Errorf("failed to format output: %w", err)
	}

	fmt.Print(output)

	return nil
}
