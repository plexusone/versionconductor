package policy

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/plexusone/versionconductor/pkg/model"
)

// LoadProfileFromFile loads a merge profile from a YAML file.
func LoadProfileFromFile(path string) (*model.MergeProfile, error) {
	cleanPath := filepath.Clean(path)
	data, err := os.ReadFile(cleanPath) // #nosec G304
	if err != nil {
		return nil, fmt.Errorf("failed to read profile file: %w", err)
	}

	var profile model.MergeProfile
	if err := yaml.Unmarshal(data, &profile); err != nil {
		return nil, fmt.Errorf("failed to parse profile file: %w", err)
	}

	return &profile, nil
}

// LoadProfileFromBytes loads a merge profile from YAML bytes.
func LoadProfileFromBytes(data []byte) (*model.MergeProfile, error) {
	var profile model.MergeProfile
	if err := yaml.Unmarshal(data, &profile); err != nil {
		return nil, fmt.Errorf("failed to parse profile: %w", err)
	}
	return &profile, nil
}

// SaveProfileToFile saves a merge profile to a YAML file.
func SaveProfileToFile(profile *model.MergeProfile, path string) error {
	data, err := yaml.Marshal(profile)
	if err != nil {
		return fmt.Errorf("failed to marshal profile: %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write profile file: %w", err)
	}

	return nil
}
