package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/plexusone/versionconductor/internal/collector"
	"github.com/plexusone/versionconductor/internal/report"
	"github.com/plexusone/versionconductor/pkg/model"
)

var scanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Scan for dependency PRs across repositories",
	Long: `Scan GitHub organizations and repositories for open dependency PRs
from Renovate and Dependabot.

This is a read-only operation that lists all open dependency PRs with their
status, age, and update type.

Examples:
  # Scan an organization
  versionconductor scan --orgs myorg

  # Scan specific repositories
  versionconductor scan --repos owner/repo1,owner/repo2

  # Filter by dependency bot
  versionconductor scan --orgs myorg --bot renovate

  # Filter by update type
  versionconductor scan --orgs myorg --update-type patch,minor

  # Output as JSON
  versionconductor scan --orgs myorg --format json`,
	RunE: runScan,
}

func init() {
	rootCmd.AddCommand(scanCmd)

	scanCmd.Flags().String("bot", "", "Filter by dependency bot: renovate, dependabot")
	scanCmd.Flags().StringSlice("update-type", nil, "Filter by update type: major, minor, patch")
	scanCmd.Flags().Int("min-age", 0, "Minimum PR age in hours")
	scanCmd.Flags().Int("max-age", 0, "Maximum PR age in hours")
	scanCmd.Flags().Bool("include-archived", false, "Include archived repositories")
	scanCmd.Flags().Bool("include-private", true, "Include private repositories")
	scanCmd.Flags().String("output", "", "Output file (default: stdout)")

	_ = viper.BindPFlag("scan.bot", scanCmd.Flags().Lookup("bot"))
	_ = viper.BindPFlag("scan.update-type", scanCmd.Flags().Lookup("update-type"))
	_ = viper.BindPFlag("scan.min-age", scanCmd.Flags().Lookup("min-age"))
	_ = viper.BindPFlag("scan.max-age", scanCmd.Flags().Lookup("max-age"))
	_ = viper.BindPFlag("scan.include-archived", scanCmd.Flags().Lookup("include-archived"))
	_ = viper.BindPFlag("scan.include-private", scanCmd.Flags().Lookup("include-private"))
	_ = viper.BindPFlag("scan.output", scanCmd.Flags().Lookup("output"))
}

func runScan(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Get configuration
	token := viper.GetString("token")
	if token == "" {
		return fmt.Errorf("GitHub token required. Set GITHUB_TOKEN or use --token flag")
	}

	orgs := viper.GetStringSlice("orgs")
	repos := viper.GetStringSlice("repos")

	if len(orgs) == 0 && len(repos) == 0 {
		return fmt.Errorf("at least one organization (--orgs) or repository (--repos) required")
	}

	verbose := viper.GetBool("verbose")

	// Create collector
	coll := collector.NewGitHub(token)

	// Build filters
	repoFilter := model.RepoFilter{
		IncludeArchived: viper.GetBool("scan.include-archived"),
		IncludePrivate:  viper.GetBool("scan.include-private"),
	}

	prFilter := model.PRFilter{
		State:       "open",
		MinAgeHours: viper.GetInt("scan.min-age"),
		MaxAgeHours: viper.GetInt("scan.max-age"),
	}

	if bot := viper.GetString("scan.bot"); bot != "" {
		prFilter.DependBot = model.DependBot(bot)
	}

	if updateTypes := viper.GetStringSlice("scan.update-type"); len(updateTypes) > 0 {
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

	// Add specific repos
	for _, repoRef := range repos {
		ref := model.ParseRepoRef(repoRef)
		allRepos = append(allRepos, model.Repo{
			Owner:    ref.Owner,
			Name:     ref.Name,
			FullName: ref.FullName(),
		})
	}

	if verbose {
		fmt.Fprintf(os.Stderr, "Found %d repositories to scan\n", len(allRepos))
	}

	// Scan for dependency PRs
	result := model.ScanResult{
		Timestamp:    time.Now(),
		Orgs:         orgs,
		ReposScanned: len(allRepos),
	}

	for _, repo := range allRepos {
		if verbose {
			fmt.Fprintf(os.Stderr, "Scanning %s...\n", repo.FullName)
		}

		ref := model.RepoRef{Owner: repo.Owner, Name: repo.Name}
		prs, err := coll.ListDependencyPRs(ctx, ref)
		if err != nil {
			result.Errors = append(result.Errors, model.ScanError{
				Repo:    repo.FullName,
				Message: err.Error(),
			})
			continue
		}

		// Apply PR filters
		for _, pr := range prs {
			if !matchesPRFilter(pr, prFilter) {
				continue
			}

			// Get check status
			checks, err := coll.GetPRChecks(ctx, ref, pr.Number)
			if err == nil {
				pr.TestsPassed = collector.TestsPassed(checks)
			}

			result.PRs = append(result.PRs, pr)
		}
	}

	result.PRsFound = len(result.PRs)

	// Generate output
	format := viper.GetString("format")
	var formatter report.Formatter

	switch format {
	case "json":
		formatter = report.NewJSONFormatter()
	case "markdown", "md":
		formatter = report.NewMarkdownFormatter()
	case "csv":
		formatter = report.NewCSVFormatter()
	default:
		formatter = report.NewTableFormatter()
	}

	output, err := formatter.FormatScanResult(&result)
	if err != nil {
		return fmt.Errorf("failed to format output: %w", err)
	}

	// Write output
	outputFile := viper.GetString("scan.output")
	if outputFile != "" {
		if err := os.WriteFile(outputFile, []byte(output), 0600); err != nil {
			return fmt.Errorf("failed to write output file: %w", err)
		}
		fmt.Fprintf(os.Stderr, "Output written to %s\n", outputFile)
	} else {
		fmt.Print(output)
	}

	return nil
}

// matchesPRFilter checks if a PR matches the filter criteria.
func matchesPRFilter(pr model.PullRequest, filter model.PRFilter) bool {
	// Check bot filter
	if filter.DependBot != "" && pr.DependBot != filter.DependBot {
		return false
	}

	// Check update type filter
	if len(filter.UpdateTypes) > 0 {
		matched := false
		for _, t := range filter.UpdateTypes {
			if pr.Dependency.UpdateType == t {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	// Check age filters
	ageHours := pr.AgeHours()
	if filter.MinAgeHours > 0 && ageHours < filter.MinAgeHours {
		return false
	}
	if filter.MaxAgeHours > 0 && ageHours > filter.MaxAgeHours {
		return false
	}

	return true
}
