package report

import (
	"bytes"
	"encoding/csv"
	"fmt"

	"github.com/plexusone/versionconductor/pkg/model"
)

// CSVFormatter formats results as CSV.
type CSVFormatter struct{}

// NewCSVFormatter creates a new CSV formatter.
func NewCSVFormatter() *CSVFormatter {
	return &CSVFormatter{}
}

// FormatScanResult formats a scan result as CSV.
func (f *CSVFormatter) FormatScanResult(result *model.ScanResult) (string, error) {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)

	// Header
	header := []string{"Repository", "PR Number", "Title", "Bot", "Dependency", "From", "To", "Update Type", "Age (hours)", "Tests Passed", "URL"}
	if err := w.Write(header); err != nil {
		return "", err
	}

	// Data rows
	for _, pr := range result.PRs {
		testsPassed := "false"
		if pr.TestsPassed {
			testsPassed = "true"
		}

		row := []string{
			pr.Repo.FullName(),
			fmt.Sprintf("%d", pr.Number),
			pr.Title,
			string(pr.DependBot),
			pr.Dependency.Name,
			pr.Dependency.FromVersion,
			pr.Dependency.ToVersion,
			string(pr.Dependency.UpdateType),
			fmt.Sprintf("%d", pr.AgeHours()),
			testsPassed,
			pr.HTMLURL,
		}

		if err := w.Write(row); err != nil {
			return "", err
		}
	}

	w.Flush()
	return buf.String(), w.Error()
}

// FormatMergeResult formats a merge result as CSV.
func (f *CSVFormatter) FormatMergeResult(result *model.MergeResult) (string, error) {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)

	// Header
	header := []string{"Repository", "PR Number", "Title", "Status", "Details", "URL"}
	if err := w.Write(header); err != nil {
		return "", err
	}

	// Merged PRs
	for _, m := range result.Merged {
		row := []string{
			m.PR.Repo.FullName(),
			fmt.Sprintf("%d", m.PR.Number),
			m.PR.Title,
			"merged",
			m.SHA,
			m.PR.HTMLURL,
		}
		if err := w.Write(row); err != nil {
			return "", err
		}
	}

	// Skipped PRs
	for _, s := range result.Skipped {
		row := []string{
			s.PR.Repo.FullName(),
			fmt.Sprintf("%d", s.PR.Number),
			s.PR.Title,
			"skipped",
			s.Reason,
			s.PR.HTMLURL,
		}
		if err := w.Write(row); err != nil {
			return "", err
		}
	}

	// Failed PRs
	for _, fail := range result.Failed {
		row := []string{
			fail.PR.Repo.FullName(),
			fmt.Sprintf("%d", fail.PR.Number),
			fail.PR.Title,
			"failed",
			fail.Error,
			fail.PR.HTMLURL,
		}
		if err := w.Write(row); err != nil {
			return "", err
		}
	}

	w.Flush()
	return buf.String(), w.Error()
}

// FormatReviewResult formats a review result as CSV.
func (f *CSVFormatter) FormatReviewResult(result *model.ReviewResult) (string, error) {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)

	// Header
	header := []string{"Repository", "PR Number", "Title", "Status", "Reason", "URL"}
	if err := w.Write(header); err != nil {
		return "", err
	}

	// Approved PRs
	for _, pr := range result.Approved {
		row := []string{
			pr.Repo.FullName(),
			fmt.Sprintf("%d", pr.Number),
			pr.Title,
			"approved",
			"",
			pr.HTMLURL,
		}
		if err := w.Write(row); err != nil {
			return "", err
		}
	}

	// Denied PRs
	for _, d := range result.Denied {
		row := []string{
			d.PR.Repo.FullName(),
			fmt.Sprintf("%d", d.PR.Number),
			d.PR.Title,
			"denied",
			d.Reason,
			d.PR.HTMLURL,
		}
		if err := w.Write(row); err != nil {
			return "", err
		}
	}

	w.Flush()
	return buf.String(), w.Error()
}

// FormatReleaseResult formats a release result as CSV.
func (f *CSVFormatter) FormatReleaseResult(result *model.ReleaseResult) (string, error) {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)

	// Header
	header := []string{"Repository", "Status", "Previous Version", "New Version", "PRs Merged", "Details", "URL"}
	if err := w.Write(header); err != nil {
		return "", err
	}

	// Created releases
	for _, r := range result.Created {
		row := []string{
			r.Repo.FullName(),
			"created",
			r.PreviousVersion,
			r.Version,
			fmt.Sprintf("%d", r.PRsMerged),
			"",
			r.ReleaseURL,
		}
		if err := w.Write(row); err != nil {
			return "", err
		}
	}

	// Skipped releases
	for _, s := range result.Skipped {
		row := []string{
			s.Repo.FullName(),
			"skipped",
			"",
			"",
			"",
			s.Reason,
			"",
		}
		if err := w.Write(row); err != nil {
			return "", err
		}
	}

	// Failed releases
	for _, fail := range result.Failed {
		row := []string{
			fail.Repo.FullName(),
			"failed",
			"",
			"",
			"",
			fail.Error,
			"",
		}
		if err := w.Write(row); err != nil {
			return "", err
		}
	}

	w.Flush()
	return buf.String(), w.Error()
}
