package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/plexusone/versionconductor/internal/collector"
	"github.com/plexusone/versionconductor/internal/releaser"
	"github.com/plexusone/versionconductor/internal/report"
	"github.com/plexusone/versionconductor/pkg/model"
)

var releaseCmd = &cobra.Command{
	Use:   "release",
	Short: "Create maintenance releases for repositories with merged dependency PRs",
	Long: `Create maintenance releases (patch version bumps) for repositories that have
merged dependency PRs since the last release.

By default, this runs in dry-run mode. Use --execute to actually create releases.

Examples:
  # Dry-run: show what releases would be created
  versionconductor release --orgs myorg

  # Create releases
  versionconductor release --orgs myorg --execute

  # Only consider PRs merged since a specific date
  versionconductor release --orgs myorg --since 2025-01-01 --execute

  # Create draft releases for review
  versionconductor release --orgs myorg --draft --execute`,
	RunE: runRelease,
}

func init() {
	rootCmd.AddCommand(releaseCmd)

	releaseCmd.Flags().Bool("execute", false, "Actually create releases (default is dry-run)")
	releaseCmd.Flags().Bool("draft", false, "Create releases as drafts")
	releaseCmd.Flags().Bool("prerelease", false, "Mark releases as prereleases")
	releaseCmd.Flags().Bool("generate-notes", true, "Use GitHub's auto-generated release notes")
	releaseCmd.Flags().String("since", "", "Only consider PRs merged since this date (YYYY-MM-DD)")
	releaseCmd.Flags().Int("min-prs", 1, "Minimum number of merged PRs to trigger a release")
	releaseCmd.Flags().Int("max-releases", 0, "Maximum number of releases to create (0 = no limit)")
	releaseCmd.Flags().String("prefix", "v", "Version prefix (e.g., 'v' for v1.2.3)")

	_ = viper.BindPFlag("release.execute", releaseCmd.Flags().Lookup("execute"))
	_ = viper.BindPFlag("release.draft", releaseCmd.Flags().Lookup("draft"))
	_ = viper.BindPFlag("release.prerelease", releaseCmd.Flags().Lookup("prerelease"))
	_ = viper.BindPFlag("release.generate-notes", releaseCmd.Flags().Lookup("generate-notes"))
	_ = viper.BindPFlag("release.since", releaseCmd.Flags().Lookup("since"))
	_ = viper.BindPFlag("release.min-prs", releaseCmd.Flags().Lookup("min-prs"))
	_ = viper.BindPFlag("release.max-releases", releaseCmd.Flags().Lookup("max-releases"))
	_ = viper.BindPFlag("release.prefix", releaseCmd.Flags().Lookup("prefix"))
}

func runRelease(cmd *cobra.Command, args []string) error {
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

	execute := viper.GetBool("release.execute")
	dryRun := !execute
	verbose := viper.GetBool("verbose")

	// Parse since date if provided
	var sinceDate *time.Time
	if since := viper.GetString("release.since"); since != "" {
		t, err := time.Parse("2006-01-02", since)
		if err != nil {
			return fmt.Errorf("invalid --since date format, use YYYY-MM-DD: %w", err)
		}
		sinceDate = &t
	}

	minPRs := viper.GetInt("release.min-prs")
	maxReleases := viper.GetInt("release.max-releases")

	// Create collector and releaser
	coll := collector.NewGitHub(token)
	rel := releaser.NewGitHub(token)

	// Build filters
	repoFilter := model.RepoFilter{
		IncludePrivate: true,
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

	// Process repositories
	result := model.ReleaseResult{
		Timestamp: time.Now(),
		DryRun:    dryRun,
	}

	releaseCount := 0

	for _, repo := range allRepos {
		if maxReleases > 0 && releaseCount >= maxReleases {
			break
		}

		ref := model.RepoRef{Owner: repo.Owner, Name: repo.Name}

		if verbose {
			fmt.Fprintf(os.Stderr, "Checking %s...\n", repo.FullName)
		}

		// Get latest tag
		latestTag, err := rel.GetLatestTag(ctx, ref)
		if err != nil {
			result.Skipped = append(result.Skipped, model.SkippedRelease{
				Repo:   ref,
				Reason: "no existing semver tags",
			})
			continue
		}

		// Get merged PRs since last tag
		mergedPRs, err := coll.GetMergedPRsSinceTag(ctx, ref, latestTag)
		if err != nil {
			result.Failed = append(result.Failed, model.FailedRelease{
				Repo:  ref,
				Error: fmt.Sprintf("failed to get merged PRs: %v", err),
			})
			continue
		}

		// Filter by date if specified
		if sinceDate != nil {
			var filtered []model.PullRequest
			for _, pr := range mergedPRs {
				if pr.MergedAt != nil && pr.MergedAt.After(*sinceDate) {
					filtered = append(filtered, pr)
				}
			}
			mergedPRs = filtered
		}

		// Filter to only dependency PRs
		var dependencyPRs []model.PullRequest
		for _, pr := range mergedPRs {
			if pr.IsDependency {
				dependencyPRs = append(dependencyPRs, pr)
			}
		}

		if len(dependencyPRs) < minPRs {
			result.Skipped = append(result.Skipped, model.SkippedRelease{
				Repo:   ref,
				Reason: fmt.Sprintf("only %d dependency PRs merged (minimum: %d)", len(dependencyPRs), minPRs),
			})
			continue
		}

		// Calculate next version
		nextVersion, err := releaser.NextPatchVersion(latestTag)
		if err != nil {
			result.Failed = append(result.Failed, model.FailedRelease{
				Repo:  ref,
				Error: fmt.Sprintf("failed to bump version: %v", err),
			})
			continue
		}

		// Create release
		if dryRun {
			if verbose {
				fmt.Fprintf(os.Stderr, "Would create release %s for %s (%d PRs)\n",
					nextVersion, repo.FullName, len(dependencyPRs))
			}
			result.Created = append(result.Created, model.CreatedRelease{
				Repo:            ref,
				Version:         nextVersion,
				PreviousVersion: latestTag,
				PRsMerged:       len(dependencyPRs),
			})
			releaseCount++
		} else {
			if verbose {
				fmt.Fprintf(os.Stderr, "Creating release %s for %s (%d PRs)\n",
					nextVersion, repo.FullName, len(dependencyPRs))
			}

			req := &model.ReleaseRequest{
				Repo:          ref,
				TagName:       nextVersion,
				Name:          nextVersion,
				Body:          generateReleaseBody(dependencyPRs),
				Draft:         viper.GetBool("release.draft"),
				Prerelease:    viper.GetBool("release.prerelease"),
				GenerateNotes: viper.GetBool("release.generate-notes"),
			}

			release, err := rel.CreateRelease(ctx, req)
			if err != nil {
				result.Failed = append(result.Failed, model.FailedRelease{
					Repo:  ref,
					Error: err.Error(),
				})
				continue
			}

			result.Created = append(result.Created, model.CreatedRelease{
				Repo:            ref,
				Version:         nextVersion,
				PreviousVersion: latestTag,
				ReleaseURL:      release.HTMLURL,
				PRsMerged:       len(dependencyPRs),
			})
			releaseCount++
		}
	}

	result.CreatedCount = len(result.Created)
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

	output, err := formatter.FormatReleaseResult(&result)
	if err != nil {
		return fmt.Errorf("failed to format output: %w", err)
	}

	fmt.Print(output)

	return nil
}

// generateReleaseBody creates a release body from merged PRs.
func generateReleaseBody(prs []model.PullRequest) string {
	if len(prs) == 0 {
		return "Maintenance release with dependency updates."
	}

	body := "## Dependency Updates\n\n"
	for _, pr := range prs {
		body += fmt.Sprintf("- %s (#%d)\n", pr.Title, pr.Number)
	}
	body += "\n---\n*This release was created automatically by VersionConductor.*"

	return body
}
