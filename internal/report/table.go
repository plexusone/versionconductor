package report

import (
	"fmt"
	"strings"
	"time"

	"github.com/plexusone/versionconductor/pkg/model"
)

// TableFormatter formats results as text tables.
type TableFormatter struct{}

// NewTableFormatter creates a new table formatter.
func NewTableFormatter() *TableFormatter {
	return &TableFormatter{}
}

// FormatScanResult formats a scan result as a text table.
func (f *TableFormatter) FormatScanResult(result *model.ScanResult) (string, error) {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Dependency PR Scan Results (%s)\n", result.Timestamp.Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("Organizations: %s\n", strings.Join(result.Orgs, ", ")))
	sb.WriteString(fmt.Sprintf("Repositories: %d | PRs Found: %d\n", result.ReposScanned, result.PRsFound))
	sb.WriteString(strings.Repeat("-", 100) + "\n")

	if len(result.PRs) == 0 {
		sb.WriteString("No dependency PRs found.\n")
		return sb.String(), nil
	}

	// Table header
	sb.WriteString(fmt.Sprintf("%-35s %-6s %-35s %-10s %-8s %-6s %-5s\n",
		"REPOSITORY", "PR", "TITLE", "BOT", "UPDATE", "AGE", "CI"))
	sb.WriteString(strings.Repeat("-", 100) + "\n")

	for _, pr := range result.PRs {
		ci := "⏳"
		if pr.TestsPassed {
			ci = "✅"
		}

		sb.WriteString(fmt.Sprintf("%-35s #%-5d %-35s %-10s %-8s %4dh %-5s\n",
			truncate(pr.Repo.FullName(), 35),
			pr.Number,
			truncate(pr.Title, 35),
			pr.DependBot,
			pr.Dependency.UpdateType,
			pr.AgeHours(),
			ci,
		))
	}

	if len(result.Errors) > 0 {
		sb.WriteString("\nErrors:\n")
		for _, e := range result.Errors {
			sb.WriteString(fmt.Sprintf("  %s: %s\n", e.Repo, e.Message))
		}
	}

	return sb.String(), nil
}

// FormatMergeResult formats a merge result as a text table.
func (f *TableFormatter) FormatMergeResult(result *model.MergeResult) (string, error) {
	var sb strings.Builder

	if result.DryRun {
		sb.WriteString("Merge Dry Run Results")
	} else {
		sb.WriteString("Merge Results")
	}
	sb.WriteString(fmt.Sprintf(" (%s)\n", result.Timestamp.Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("Merged: %d | Skipped: %d | Failed: %d\n",
		result.MergedCount, result.SkippedCount, result.FailedCount))
	sb.WriteString(strings.Repeat("-", 80) + "\n")

	if len(result.Merged) > 0 {
		sb.WriteString("\nMerged:\n")
		for _, m := range result.Merged {
			sb.WriteString(fmt.Sprintf("  ✅ %s#%d: %s\n",
				m.PR.Repo.FullName(), m.PR.Number, truncate(m.PR.Title, 50)))
		}
	}

	if len(result.Skipped) > 0 {
		sb.WriteString("\nSkipped:\n")
		for _, s := range result.Skipped {
			sb.WriteString(fmt.Sprintf("  ⏭️  %s#%d: %s (%s)\n",
				s.PR.Repo.FullName(), s.PR.Number, truncate(s.PR.Title, 40), s.Reason))
		}
	}

	if len(result.Failed) > 0 {
		sb.WriteString("\nFailed:\n")
		for _, fail := range result.Failed {
			sb.WriteString(fmt.Sprintf("  ❌ %s#%d: %s (%s)\n",
				fail.PR.Repo.FullName(), fail.PR.Number, truncate(fail.PR.Title, 40), fail.Error))
		}
	}

	return sb.String(), nil
}

// FormatReviewResult formats a review result as a text table.
func (f *TableFormatter) FormatReviewResult(result *model.ReviewResult) (string, error) {
	var sb strings.Builder

	if result.DryRun {
		sb.WriteString("Review Dry Run Results")
	} else {
		sb.WriteString("Review Results")
	}
	sb.WriteString(fmt.Sprintf(" (%s)\n", result.Timestamp.Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("Approved: %d | Denied: %d\n",
		result.ApprovedCount, result.DeniedCount))
	sb.WriteString(strings.Repeat("-", 80) + "\n")

	if len(result.Approved) > 0 {
		sb.WriteString("\nApproved:\n")
		for _, pr := range result.Approved {
			sb.WriteString(fmt.Sprintf("  ✅ %s#%d: %s\n",
				pr.Repo.FullName(), pr.Number, truncate(pr.Title, 50)))
		}
	}

	if len(result.Denied) > 0 {
		sb.WriteString("\nDenied:\n")
		for _, d := range result.Denied {
			sb.WriteString(fmt.Sprintf("  ❌ %s#%d: %s (%s)\n",
				d.PR.Repo.FullName(), d.PR.Number, truncate(d.PR.Title, 40), d.Reason))
		}
	}

	return sb.String(), nil
}

// FormatReleaseResult formats a release result as a text table.
func (f *TableFormatter) FormatReleaseResult(result *model.ReleaseResult) (string, error) {
	var sb strings.Builder

	if result.DryRun {
		sb.WriteString("Release Dry Run Results")
	} else {
		sb.WriteString("Release Results")
	}
	sb.WriteString(fmt.Sprintf(" (%s)\n", result.Timestamp.Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("Created: %d | Skipped: %d | Failed: %d\n",
		result.CreatedCount, result.SkippedCount, result.FailedCount))
	sb.WriteString(strings.Repeat("-", 80) + "\n")

	if len(result.Created) > 0 {
		sb.WriteString("\nCreated:\n")
		for _, r := range result.Created {
			sb.WriteString(fmt.Sprintf("  ✅ %s: %s → %s (%d PRs)\n",
				r.Repo.FullName(), r.PreviousVersion, r.Version, r.PRsMerged))
		}
	}

	if len(result.Skipped) > 0 {
		sb.WriteString("\nSkipped:\n")
		for _, s := range result.Skipped {
			sb.WriteString(fmt.Sprintf("  ⏭️  %s: %s\n", s.Repo.FullName(), s.Reason))
		}
	}

	if len(result.Failed) > 0 {
		sb.WriteString("\nFailed:\n")
		for _, fail := range result.Failed {
			sb.WriteString(fmt.Sprintf("  ❌ %s: %s\n", fail.Repo.FullName(), fail.Error))
		}
	}

	return sb.String(), nil
}

// truncate shortens a string to maxLen, adding "..." if truncated.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// Table represents a simple text table for output.
type Table struct {
	Headers []string
	Rows    []TableRow
}

// TableRow represents a row in a table.
type TableRow struct {
	Cells []string
}

// Render renders the table as a string.
func (t *Table) Render() string {
	if len(t.Headers) == 0 || len(t.Rows) == 0 {
		return ""
	}

	// Calculate column widths
	widths := make([]int, len(t.Headers))
	for i, h := range t.Headers {
		widths[i] = len(h)
	}
	for _, row := range t.Rows {
		for i, cell := range row.Cells {
			if i < len(widths) && len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}

	var sb strings.Builder

	// Render header
	for i, h := range t.Headers {
		sb.WriteString(fmt.Sprintf("%-*s  ", widths[i], h))
	}
	sb.WriteString("\n")

	// Render separator
	for _, w := range widths {
		sb.WriteString(strings.Repeat("-", w) + "  ")
	}
	sb.WriteString("\n")

	// Render rows
	for _, row := range t.Rows {
		for i, cell := range row.Cells {
			if i < len(widths) {
				sb.WriteString(fmt.Sprintf("%-*s  ", widths[i], cell))
			}
		}
		sb.WriteString("\n")
	}

	return sb.String()
}
