package report

import "github.com/plexusone/versionconductor/pkg/model"

// Formatter defines the interface for formatting results.
type Formatter interface {
	// FormatScanResult formats a scan result.
	FormatScanResult(result *model.ScanResult) (string, error)

	// FormatMergeResult formats a merge result.
	FormatMergeResult(result *model.MergeResult) (string, error)

	// FormatReviewResult formats a review result.
	FormatReviewResult(result *model.ReviewResult) (string, error)

	// FormatReleaseResult formats a release result.
	FormatReleaseResult(result *model.ReleaseResult) (string, error)
}
