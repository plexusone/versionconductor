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

var reviewCmd = &cobra.Command{
	Use:   "review",
	Short: "Auto-review dependency PRs based on policy",
	Long: `Automatically add approval reviews to dependency PRs that meet policy criteria.

By default, this runs in dry-run mode. Use --execute to actually add reviews.

Examples:
  # Dry-run: show what would be approved
  versionconductor review --orgs myorg

  # Approve PRs using balanced profile
  versionconductor review --orgs myorg --profile balanced --execute

  # Only approve patch updates
  versionconductor review --orgs myorg --update-type patch --execute`,
	RunE: runReview,
}

func init() {
	rootCmd.AddCommand(reviewCmd)

	reviewCmd.Flags().String("profile", "balanced", "Review profile: aggressive, balanced, conservative")
	reviewCmd.Flags().Bool("execute", false, "Actually add reviews (default is dry-run)")
	reviewCmd.Flags().StringSlice("update-type", nil, "Filter by update type: major, minor, patch")
	reviewCmd.Flags().String("bot", "", "Filter by dependency bot: renovate, dependabot")
	reviewCmd.Flags().String("review-body", "", "Custom review body message")

	_ = viper.BindPFlag("review.profile", reviewCmd.Flags().Lookup("profile"))
	_ = viper.BindPFlag("review.execute", reviewCmd.Flags().Lookup("execute"))
	_ = viper.BindPFlag("review.update-type", reviewCmd.Flags().Lookup("update-type"))
	_ = viper.BindPFlag("review.bot", reviewCmd.Flags().Lookup("bot"))
	_ = viper.BindPFlag("review.review-body", reviewCmd.Flags().Lookup("review-body"))
}

func runReview(cmd *cobra.Command, args []string) error {
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

	execute := viper.GetBool("review.execute")
	dryRun := !execute
	verbose := viper.GetBool("verbose")

	// Get review profile
	profileName := viper.GetString("review.profile")
	profile := policy.GetProfile(profileName)
	if profile == nil {
		return fmt.Errorf("unknown profile: %s", profileName)
	}

	// Create collector and merger (for reviews)
	coll := collector.NewGitHub(token)
	merg := merger.NewGitHub(token)

	// Build filters
	repoFilter := model.RepoFilter{
		IncludePrivate: true,
	}

	prFilter := model.PRFilter{
		State: "open",
	}

	if bot := viper.GetString("review.bot"); bot != "" {
		prFilter.DependBot = model.DependBot(bot)
	}

	if updateTypes := viper.GetStringSlice("review.update-type"); len(updateTypes) > 0 {
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

	// Process PRs
	result := model.ReviewResult{
		Timestamp: time.Now(),
		DryRun:    dryRun,
	}

	reviewBody := viper.GetString("review.review-body")
	if reviewBody == "" {
		reviewBody = "Automatically approved by VersionConductor. All CI checks have passed."
	}

	for _, repo := range allRepos {
		ref := model.RepoRef{Owner: repo.Owner, Name: repo.Name}

		prs, err := coll.ListDependencyPRs(ctx, ref)
		if err != nil {
			if verbose {
				fmt.Fprintf(os.Stderr, "Error listing PRs for %s: %v\n", repo.FullName, err)
			}
			continue
		}

		for _, pr := range prs {
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

			// Evaluate for review approval
			shouldApprove, reason := evaluateForReview(profile, &pr, checks)

			if !shouldApprove {
				result.Denied = append(result.Denied, model.DeniedPR{
					PR:     pr,
					Reason: reason,
				})
				continue
			}

			// Approve the PR
			if dryRun {
				if verbose {
					fmt.Fprintf(os.Stderr, "Would approve %s#%d: %s\n", repo.FullName, pr.Number, pr.Title)
				}
				result.Approved = append(result.Approved, pr)
			} else {
				if verbose {
					fmt.Fprintf(os.Stderr, "Approving %s#%d: %s\n", repo.FullName, pr.Number, pr.Title)
				}

				err := merg.ApprovePR(ctx, ref, pr.Number, reviewBody)
				if err != nil {
					result.Denied = append(result.Denied, model.DeniedPR{
						PR:     pr,
						Reason: fmt.Sprintf("failed to approve: %v", err),
					})
					continue
				}

				result.Approved = append(result.Approved, pr)
			}
		}
	}

	result.ApprovedCount = len(result.Approved)
	result.DeniedCount = len(result.Denied)

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

	output, err := formatter.FormatReviewResult(&result)
	if err != nil {
		return fmt.Errorf("failed to format output: %w", err)
	}

	fmt.Print(output)

	return nil
}

// evaluateForReview determines if a PR should receive an approval review.
func evaluateForReview(profile *model.MergeProfile, pr *model.PullRequest, checks []model.CheckRun) (bool, string) {
	// Check if tests pass
	if profile.RequireAllChecks {
		if !pr.TestsPassed {
			return false, "CI checks not passed"
		}
		// Verify all checks completed successfully
		for _, check := range checks {
			if check.Status != "completed" || check.Conclusion != "success" {
				return false, "not all CI checks passed: " + check.Name
			}
		}
	}

	// Check update type eligibility
	switch pr.Dependency.UpdateType {
	case model.UpdateTypeMajor:
		if !profile.AutoMergeMajor {
			return false, "major updates require manual review"
		}
	case model.UpdateTypeMinor:
		if !profile.AutoMergeMinor {
			return false, "minor updates require manual review"
		}
	case model.UpdateTypePatch:
		if !profile.AutoMergePatch {
			return false, "patch updates require manual review"
		}
	}

	// Check if PR is in a reviewable state
	if pr.Draft {
		return false, "PR is a draft"
	}

	return true, ""
}
