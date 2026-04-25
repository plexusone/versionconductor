package releaser

import (
	"context"
	"fmt"

	"github.com/google/go-github/v84/github"
	"github.com/grokify/gogithub/auth"
	"github.com/grokify/gogithub/release"
	"github.com/grokify/gogithub/tag"

	"github.com/plexusone/versionconductor/pkg/model"
)

// GitHubReleaser implements Releaser for GitHub.
type GitHubReleaser struct {
	client *github.Client
}

// NewGitHubReleaser creates a new GitHub releaser.
func NewGitHubReleaser(token string) *GitHubReleaser {
	ctx := context.Background()
	client := auth.NewGitHubClient(ctx, token)
	return &GitHubReleaser{
		client: client,
	}
}

// CreateRelease creates a new release for a repository.
func (r *GitHubReleaser) CreateRelease(ctx context.Context, req *model.ReleaseRequest) (*model.Release, error) {
	ghRelease := &github.RepositoryRelease{
		TagName:              github.Ptr(req.TagName),
		Name:                 github.Ptr(req.Name),
		Body:                 github.Ptr(req.Body),
		Draft:                github.Ptr(req.Draft),
		Prerelease:           github.Ptr(req.Prerelease),
		GenerateReleaseNotes: github.Ptr(req.GenerateNotes),
	}

	if req.TargetCommitish != "" {
		ghRelease.TargetCommitish = github.Ptr(req.TargetCommitish)
	}

	created, err := release.CreateRelease(ctx, r.client, req.Repo.Owner, req.Repo.Name, ghRelease)
	if err != nil {
		return nil, fmt.Errorf("failed to create release: %w", err)
	}

	return &model.Release{
		ID:          created.GetID(),
		TagName:     created.GetTagName(),
		Name:        created.GetName(),
		Body:        created.GetBody(),
		Draft:       created.GetDraft(),
		Prerelease:  created.GetPrerelease(),
		CreatedAt:   created.GetCreatedAt().Time,
		PublishedAt: created.GetPublishedAt().Time,
		HTMLURL:     created.GetHTMLURL(),
		Repo:        req.Repo,
	}, nil
}

// CreateTag creates a new tag for a repository.
func (r *GitHubReleaser) CreateTag(ctx context.Context, repo model.RepoRef, tagName, sha, message string) error {
	return tag.CreateTag(ctx, r.client, repo.Owner, repo.Name, tagName, sha, message)
}

// GetLatestTag returns the most recent semver tag.
func (r *GitHubReleaser) GetLatestTag(ctx context.Context, repo model.RepoRef) (string, error) {
	tagNames, err := tag.GetTagNames(ctx, r.client, repo.Owner, repo.Name)
	if err != nil {
		return "", fmt.Errorf("failed to list tags: %w", err)
	}

	latest := FindLatestVersion(tagNames)
	if latest == "" {
		return "", fmt.Errorf("no semver tags found")
	}

	return latest, nil
}

// GetTagSHA returns the SHA for a given tag.
func (r *GitHubReleaser) GetTagSHA(ctx context.Context, repo model.RepoRef, tagName string) (string, error) {
	return tag.GetTagSHA(ctx, r.client, repo.Owner, repo.Name, tagName)
}

// GetDefaultBranchSHA returns the SHA of the default branch HEAD.
func (r *GitHubReleaser) GetDefaultBranchSHA(ctx context.Context, repo model.RepoRef, branch string) (string, error) {
	ref, _, err := r.client.Git.GetRef(ctx, repo.Owner, repo.Name, "heads/"+branch)
	if err != nil {
		return "", fmt.Errorf("failed to get branch ref: %w", err)
	}

	return ref.GetObject().GetSHA(), nil
}
