package report

import (
	"encoding/json"

	"github.com/plexusone/versionconductor/pkg/model"
)

// JSONFormatter formats results as JSON.
type JSONFormatter struct {
	Indent bool
}

// NewJSONFormatter creates a new JSON formatter.
func NewJSONFormatter() *JSONFormatter {
	return &JSONFormatter{Indent: true}
}

// FormatScanResult formats a scan result as JSON.
func (f *JSONFormatter) FormatScanResult(result *model.ScanResult) (string, error) {
	return f.marshal(result)
}

// FormatMergeResult formats a merge result as JSON.
func (f *JSONFormatter) FormatMergeResult(result *model.MergeResult) (string, error) {
	return f.marshal(result)
}

// FormatReviewResult formats a review result as JSON.
func (f *JSONFormatter) FormatReviewResult(result *model.ReviewResult) (string, error) {
	return f.marshal(result)
}

// FormatReleaseResult formats a release result as JSON.
func (f *JSONFormatter) FormatReleaseResult(result *model.ReleaseResult) (string, error) {
	return f.marshal(result)
}

func (f *JSONFormatter) marshal(v any) (string, error) {
	var data []byte
	var err error

	if f.Indent {
		data, err = json.MarshalIndent(v, "", "  ")
	} else {
		data, err = json.Marshal(v)
	}

	if err != nil {
		return "", err
	}

	return string(data), nil
}
