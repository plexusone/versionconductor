package report

import (
	"fmt"
	"strings"
	"time"

	"github.com/plexusone/versionconductor/pkg/model"
)

// MarkdownFormatter formats results as Markdown.
type MarkdownFormatter struct{}

// NewMarkdownFormatter creates a new Markdown formatter.
func NewMarkdownFormatter() *MarkdownFormatter {
	return &MarkdownFormatter{}
}

// FormatScanResult formats a scan result as Markdown.
func (f *MarkdownFormatter) FormatScanResult(result *model.ScanResult) (string, error) {
	var sb strings.Builder

	sb.WriteString("# Dependency PR Scan Results\n\n")
	sb.WriteString(fmt.Sprintf("**Scan Time:** %s\n\n", result.Timestamp.Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("**Organizations:** %s\n\n", strings.Join(result.Orgs, ", ")))
	sb.WriteString(fmt.Sprintf("**Repositories Scanned:** %d\n\n", result.ReposScanned))
	sb.WriteString(fmt.Sprintf("**PRs Found:** %d\n\n", result.PRsFound))

	if len(result.PRs) > 0 {
		sb.WriteString("## Open Dependency PRs\n\n")
		sb.WriteString("| Repository | PR | Title | Bot | Update | Age | Tests |\n")
		sb.WriteString("|------------|-----|-------|-----|--------|-----|-------|\n")

		for _, pr := range result.PRs {
			tests := "⏳"
			if pr.TestsPassed {
				tests = "✅"
			}

			sb.WriteString(fmt.Sprintf("| %s | [#%d](%s) | %s | %s | %s | %dh | %s |\n",
				pr.Repo.FullName(),
				pr.Number,
				pr.HTMLURL,
				truncate(pr.Title, 50),
				pr.DependBot,
				pr.Dependency.UpdateType,
				pr.AgeHours(),
				tests,
			))
		}
	}

	if len(result.Errors) > 0 {
		sb.WriteString("\n## Errors\n\n")
		for _, e := range result.Errors {
			sb.WriteString(fmt.Sprintf("- **%s:** %s\n", e.Repo, e.Message))
		}
	}

	return sb.String(), nil
}

// FormatMergeResult formats a merge result as Markdown.
func (f *MarkdownFormatter) FormatMergeResult(result *model.MergeResult) (string, error) {
	var sb strings.Builder

	if result.DryRun {
		sb.WriteString("# Merge Dry Run Results\n\n")
	} else {
		sb.WriteString("# Merge Results\n\n")
	}

	sb.WriteString(fmt.Sprintf("**Time:** %s\n\n", result.Timestamp.Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("**Merged:** %d | **Skipped:** %d | **Failed:** %d\n\n",
		result.MergedCount, result.SkippedCount, result.FailedCount))

	if len(result.Merged) > 0 {
		sb.WriteString("## Merged PRs\n\n")
		for _, m := range result.Merged {
			sb.WriteString(fmt.Sprintf("- [%s#%d](%s): %s\n",
				m.PR.Repo.FullName(), m.PR.Number, m.PR.HTMLURL, m.PR.Title))
		}
		sb.WriteString("\n")
	}

	if len(result.Skipped) > 0 {
		sb.WriteString("## Skipped PRs\n\n")
		for _, s := range result.Skipped {
			sb.WriteString(fmt.Sprintf("- [%s#%d](%s): %s - *%s*\n",
				s.PR.Repo.FullName(), s.PR.Number, s.PR.HTMLURL, s.PR.Title, s.Reason))
		}
		sb.WriteString("\n")
	}

	if len(result.Failed) > 0 {
		sb.WriteString("## Failed PRs\n\n")
		for _, f := range result.Failed {
			sb.WriteString(fmt.Sprintf("- [%s#%d](%s): %s - **%s**\n",
				f.PR.Repo.FullName(), f.PR.Number, f.PR.HTMLURL, f.PR.Title, f.Error))
		}
	}

	return sb.String(), nil
}

// FormatReviewResult formats a review result as Markdown.
func (f *MarkdownFormatter) FormatReviewResult(result *model.ReviewResult) (string, error) {
	var sb strings.Builder

	if result.DryRun {
		sb.WriteString("# Review Dry Run Results\n\n")
	} else {
		sb.WriteString("# Review Results\n\n")
	}

	sb.WriteString(fmt.Sprintf("**Time:** %s\n\n", result.Timestamp.Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("**Approved:** %d | **Denied:** %d\n\n",
		result.ApprovedCount, result.DeniedCount))

	if len(result.Approved) > 0 {
		sb.WriteString("## Approved PRs\n\n")
		for _, pr := range result.Approved {
			sb.WriteString(fmt.Sprintf("- [%s#%d](%s): %s\n",
				pr.Repo.FullName(), pr.Number, pr.HTMLURL, pr.Title))
		}
		sb.WriteString("\n")
	}

	if len(result.Denied) > 0 {
		sb.WriteString("## Denied PRs\n\n")
		for _, d := range result.Denied {
			sb.WriteString(fmt.Sprintf("- [%s#%d](%s): %s - *%s*\n",
				d.PR.Repo.FullName(), d.PR.Number, d.PR.HTMLURL, d.PR.Title, d.Reason))
		}
	}

	return sb.String(), nil
}

// FormatReleaseResult formats a release result as Markdown.
func (f *MarkdownFormatter) FormatReleaseResult(result *model.ReleaseResult) (string, error) {
	var sb strings.Builder

	if result.DryRun {
		sb.WriteString("# Release Dry Run Results\n\n")
	} else {
		sb.WriteString("# Release Results\n\n")
	}

	sb.WriteString(fmt.Sprintf("**Time:** %s\n\n", result.Timestamp.Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("**Created:** %d | **Skipped:** %d | **Failed:** %d\n\n",
		result.CreatedCount, result.SkippedCount, result.FailedCount))

	if len(result.Created) > 0 {
		sb.WriteString("## Created Releases\n\n")
		for _, r := range result.Created {
			sb.WriteString(fmt.Sprintf("- [%s %s](%s): %s → %s (%d PRs merged)\n",
				r.Repo.FullName(), r.Version, r.ReleaseURL,
				r.PreviousVersion, r.Version, r.PRsMerged))
		}
		sb.WriteString("\n")
	}

	if len(result.Skipped) > 0 {
		sb.WriteString("## Skipped Repositories\n\n")
		for _, s := range result.Skipped {
			sb.WriteString(fmt.Sprintf("- %s: *%s*\n", s.Repo.FullName(), s.Reason))
		}
		sb.WriteString("\n")
	}

	if len(result.Failed) > 0 {
		sb.WriteString("## Failed Releases\n\n")
		for _, f := range result.Failed {
			sb.WriteString(fmt.Sprintf("- %s: **%s**\n", f.Repo.FullName(), f.Error))
		}
	}

	return sb.String(), nil
}
