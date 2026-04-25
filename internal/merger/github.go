package merger

import (
	"context"
	"fmt"

	"github.com/google/go-github/v84/github"
	"github.com/grokify/gogithub/auth"
	"github.com/grokify/gogithub/pr"
	"github.com/grokify/gogithub/repo"

	"github.com/plexusone/versionconductor/pkg/model"
)

// GitHubMerger implements Merger for GitHub.
type GitHubMerger struct {
	client *github.Client
}

// NewGitHubMerger creates a new GitHub merger.
func NewGitHubMerger(token string) *GitHubMerger {
	ctx := context.Background()
	client := auth.NewGitHubClient(ctx, token)
	return &GitHubMerger{
		client: client,
	}
}

// MergePR merges a pull request using the specified strategy.
func (m *GitHubMerger) MergePR(ctx context.Context, repoRef model.RepoRef, prNumber int, strategy MergeStrategy, commitMessage string) (*MergeInfo, error) {
	opts := &github.PullRequestOptions{
		MergeMethod: string(strategy),
	}

	if commitMessage != "" {
		opts.CommitTitle = commitMessage
	}

	result, err := pr.MergePR(ctx, m.client, repoRef.Owner, repoRef.Name, prNumber, commitMessage, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to merge PR: %w", err)
	}

	return &MergeInfo{
		SHA:     result.GetSHA(),
		Message: result.GetMessage(),
		Merged:  result.GetMerged(),
	}, nil
}

// ApprovePR adds an approval review to a pull request.
func (m *GitHubMerger) ApprovePR(ctx context.Context, repoRef model.RepoRef, prNumber int, body string) error {
	_, err := pr.ApprovePR(ctx, m.client, repoRef.Owner, repoRef.Name, prNumber, body)
	if err != nil {
		return fmt.Errorf("failed to approve PR: %w", err)
	}
	return nil
}

// IsMergeable checks if a PR can be merged.
func (m *GitHubMerger) IsMergeable(ctx context.Context, repoRef model.RepoRef, prNumber int) (bool, string, error) {
	state, err := pr.IsMergeable(ctx, m.client, repoRef.Owner, repoRef.Name, prNumber)
	if err != nil {
		return false, "", fmt.Errorf("failed to check mergeable: %w", err)
	}

	return state.Mergeable, state.Message, nil
}

// DeleteBranch deletes the PR's head branch after merge.
func (m *GitHubMerger) DeleteBranch(ctx context.Context, repoRef model.RepoRef, branch string) error {
	return repo.DeleteBranch(ctx, m.client, repoRef.Owner, repoRef.Name, branch)
}
