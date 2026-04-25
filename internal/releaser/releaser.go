package releaser

import (
	"context"

	"github.com/plexusone/versionconductor/pkg/model"
)

// Releaser defines the interface for creating releases.
type Releaser interface {
	// CreateRelease creates a new release for a repository.
	CreateRelease(ctx context.Context, req *model.ReleaseRequest) (*model.Release, error)

	// CreateTag creates a new tag for a repository.
	CreateTag(ctx context.Context, repo model.RepoRef, tagName, sha, message string) error

	// GetLatestTag returns the most recent semver tag.
	GetLatestTag(ctx context.Context, repo model.RepoRef) (string, error)

	// GetTagSHA returns the SHA for a given tag.
	GetTagSHA(ctx context.Context, repo model.RepoRef, tagName string) (string, error)

	// GetDefaultBranchSHA returns the SHA of the default branch HEAD.
	GetDefaultBranchSHA(ctx context.Context, repo model.RepoRef, branch string) (string, error)
}

// Options configures release behavior.
type Options struct {
	Prefix        string // Version prefix, e.g., "v"
	GenerateNotes bool   // Use GitHub's auto-generated release notes
	Draft         bool   // Create as draft
	Prerelease    bool   // Mark as prerelease
	IncludeBody   bool   // Include changelog in body
}

// DefaultOptions returns sensible default release options.
func DefaultOptions() Options {
	return Options{
		Prefix:        "v",
		GenerateNotes: true,
		Draft:         false,
		Prerelease:    false,
		IncludeBody:   true,
	}
}

// NewGitHub creates a new GitHub releaser with the given token.
func NewGitHub(token string) Releaser {
	return NewGitHubReleaser(token)
}
