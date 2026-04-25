package collector

import (
	"context"

	"github.com/plexusone/versionconductor/pkg/model"
)

// Collector defines the interface for collecting repository and PR information.
type Collector interface {
	// ListRepos returns repositories matching the filter criteria.
	ListRepos(ctx context.Context, orgs []string, filter model.RepoFilter) ([]model.Repo, error)

	// ListDependencyPRs returns open dependency PRs for a repository.
	ListDependencyPRs(ctx context.Context, repo model.RepoRef) ([]model.PullRequest, error)

	// GetPRDetails returns detailed information about a specific PR.
	GetPRDetails(ctx context.Context, repo model.RepoRef, prNumber int) (*model.PullRequest, error)

	// GetPRChecks returns the CI check runs for a PR.
	GetPRChecks(ctx context.Context, repo model.RepoRef, prNumber int) ([]model.CheckRun, error)

	// GetLatestRelease returns the most recent release for a repository.
	GetLatestRelease(ctx context.Context, repo model.RepoRef) (*model.Release, error)

	// ListTags returns all tags for a repository.
	ListTags(ctx context.Context, repo model.RepoRef) ([]model.Tag, error)

	// GetMergedPRsSinceTag returns PRs merged since the given tag.
	GetMergedPRsSinceTag(ctx context.Context, repo model.RepoRef, tagName string) ([]model.PullRequest, error)
}

// NewGitHub creates a new GitHub collector with the given token.
func NewGitHub(token string) Collector {
	return NewGitHubCollector(token)
}
