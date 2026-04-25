package merger

import (
	"context"

	"github.com/plexusone/versionconductor/pkg/model"
)

// Merger defines the interface for merging pull requests.
type Merger interface {
	// MergePR merges a pull request using the specified strategy.
	MergePR(ctx context.Context, repo model.RepoRef, prNumber int, strategy MergeStrategy, commitMessage string) (*MergeInfo, error)

	// ApprovePR adds an approval review to a pull request.
	ApprovePR(ctx context.Context, repo model.RepoRef, prNumber int, body string) error

	// IsMergeable checks if a PR can be merged.
	IsMergeable(ctx context.Context, repo model.RepoRef, prNumber int) (bool, string, error)

	// DeleteBranch deletes the PR's head branch after merge.
	DeleteBranch(ctx context.Context, repo model.RepoRef, branch string) error
}

// MergeStrategy defines how to merge a PR.
type MergeStrategy string

const (
	MergeStrategyMerge  MergeStrategy = "merge"
	MergeStrategySquash MergeStrategy = "squash"
	MergeStrategyRebase MergeStrategy = "rebase"
)

// MergeInfo contains information about a successful merge.
type MergeInfo struct {
	SHA     string `json:"sha"`
	Message string `json:"message"`
	Merged  bool   `json:"merged"`
}

// Options configures merge behavior.
type Options struct {
	Strategy      MergeStrategy
	DeleteBranch  bool
	CommitMessage string
	WaitForChecks bool
	ChecksTimeout int // seconds
}

// DefaultOptions returns sensible default merge options.
func DefaultOptions() Options {
	return Options{
		Strategy:      MergeStrategySquash,
		DeleteBranch:  true,
		WaitForChecks: false,
		ChecksTimeout: 300,
	}
}

// NewGitHub creates a new GitHub merger with the given token.
func NewGitHub(token string) Merger {
	return NewGitHubMerger(token)
}
