package collector

import (
	"context"
	"regexp"
	"strings"
	"time"

	"github.com/google/go-github/v84/github"
	"github.com/grokify/gogithub/auth"
	"github.com/grokify/gogithub/checks"
	"github.com/grokify/gogithub/pr"
	"github.com/grokify/gogithub/release"
	"github.com/grokify/gogithub/tag"

	"github.com/plexusone/versionconductor/pkg/model"
)

// GitHubCollector implements Collector for GitHub repositories.
type GitHubCollector struct {
	client *github.Client
}

// NewGitHubCollector creates a new GitHub collector.
func NewGitHubCollector(token string) *GitHubCollector {
	ctx := context.Background()
	client := auth.NewGitHubClient(ctx, token)
	return &GitHubCollector{
		client: client,
	}
}

// ListRepos returns repositories matching the filter criteria.
func (c *GitHubCollector) ListRepos(ctx context.Context, orgs []string, filter model.RepoFilter) ([]model.Repo, error) {
	var repos []model.Repo

	for _, org := range orgs {
		opt := &github.RepositoryListByOrgOptions{
			Type: "all",
			ListOptions: github.ListOptions{
				PerPage: 100,
			},
		}

		for {
			ghRepos, resp, err := c.client.Repositories.ListByOrg(ctx, org, opt)
			if err != nil {
				return nil, err
			}

			for _, r := range ghRepos {
				repo := convertRepo(r)

				// Apply filters
				if repo.Archived && !filter.IncludeArchived {
					continue
				}
				if repo.Private && !filter.IncludePrivate {
					continue
				}
				if r.GetFork() && !filter.IncludeForks {
					continue
				}
				if isExcluded(repo.FullName, filter.ExcludeRepos) {
					continue
				}

				repos = append(repos, repo)
			}

			if resp.NextPage == 0 {
				break
			}
			opt.Page = resp.NextPage
		}
	}

	return repos, nil
}

// ListDependencyPRs returns open dependency PRs for a repository.
func (c *GitHubCollector) ListDependencyPRs(ctx context.Context, repo model.RepoRef) ([]model.PullRequest, error) {
	var prs []model.PullRequest

	opts := &github.PullRequestListOptions{
		State: "open",
		ListOptions: github.ListOptions{
			PerPage: 100,
		},
	}

	ghPRs, err := pr.ListPRs(ctx, c.client, repo.Owner, repo.Name, opts)
	if err != nil {
		return nil, err
	}

	for _, ghPR := range ghPRs {
		mpr := convertPR(ghPR, repo)

		// Check if this is a dependency PR
		mpr.DependBot = model.DetectDependBot(mpr.Author)
		if mpr.DependBot != model.DependBotUnknown {
			mpr.IsDependency = true
			mpr.Dependency = parseDependencyFromTitle(mpr.Title)
			prs = append(prs, mpr)
		}
	}

	return prs, nil
}

// GetPRDetails returns detailed information about a specific PR.
func (c *GitHubCollector) GetPRDetails(ctx context.Context, repo model.RepoRef, prNumber int) (*model.PullRequest, error) {
	ghPR, err := pr.GetPR(ctx, c.client, repo.Owner, repo.Name, prNumber)
	if err != nil {
		return nil, err
	}

	mpr := convertPR(ghPR, repo)
	mpr.DependBot = model.DetectDependBot(mpr.Author)
	if mpr.DependBot != model.DependBotUnknown {
		mpr.IsDependency = true
		mpr.Dependency = parseDependencyFromTitle(mpr.Title)
	}

	// Get mergeable status
	if ghPR.Mergeable != nil {
		mpr.Mergeable = *ghPR.Mergeable
	}
	if ghPR.MergeableState != nil {
		mpr.MergeableStr = *ghPR.MergeableState
	}

	return &mpr, nil
}

// GetPRChecks returns the CI check runs for a PR.
func (c *GitHubCollector) GetPRChecks(ctx context.Context, repo model.RepoRef, prNumber int) ([]model.CheckRun, error) {
	ghChecks, err := checks.ListCheckRunsForPR(ctx, c.client, repo.Owner, repo.Name, prNumber)
	if err != nil {
		return nil, err
	}

	var result []model.CheckRun
	for _, cr := range ghChecks {
		result = append(result, model.CheckRun{
			Name:       cr.GetName(),
			Status:     cr.GetStatus(),
			Conclusion: cr.GetConclusion(),
		})
	}

	return result, nil
}

// GetLatestRelease returns the most recent release for a repository.
func (c *GitHubCollector) GetLatestRelease(ctx context.Context, repo model.RepoRef) (*model.Release, error) {
	ghRelease, err := release.GetLatestRelease(ctx, c.client, repo.Owner, repo.Name)
	if err != nil {
		// Check for 404 (no releases)
		if strings.Contains(err.Error(), "404") {
			return nil, nil
		}
		return nil, err
	}

	return &model.Release{
		ID:          ghRelease.GetID(),
		TagName:     ghRelease.GetTagName(),
		Name:        ghRelease.GetName(),
		Body:        ghRelease.GetBody(),
		Draft:       ghRelease.GetDraft(),
		Prerelease:  ghRelease.GetPrerelease(),
		CreatedAt:   ghRelease.GetCreatedAt().Time,
		PublishedAt: ghRelease.GetPublishedAt().Time,
		HTMLURL:     ghRelease.GetHTMLURL(),
		Repo:        repo,
	}, nil
}

// ListTags returns all tags for a repository.
func (c *GitHubCollector) ListTags(ctx context.Context, repo model.RepoRef) ([]model.Tag, error) {
	ghTags, err := tag.ListTags(ctx, c.client, repo.Owner, repo.Name)
	if err != nil {
		return nil, err
	}

	var tags []model.Tag
	for _, t := range ghTags {
		tags = append(tags, model.Tag{
			Name: t.GetName(),
			SHA:  t.GetCommit().GetSHA(),
			Repo: repo,
		})
	}

	return tags, nil
}

// GetMergedPRsSinceTag returns PRs merged since the given tag.
func (c *GitHubCollector) GetMergedPRsSinceTag(ctx context.Context, repo model.RepoRef, tagName string) ([]model.PullRequest, error) {
	// Get the tag's commit date
	tagSHA, err := tag.GetTagSHA(ctx, c.client, repo.Owner, repo.Name, tagName)
	if err != nil {
		return nil, err
	}

	commit, _, err := c.client.Git.GetCommit(ctx, repo.Owner, repo.Name, tagSHA)
	if err != nil {
		return nil, err
	}

	since := commit.GetCommitter().GetDate().Time

	var prs []model.PullRequest

	opts := &github.PullRequestListOptions{
		State:     "closed",
		Sort:      "updated",
		Direction: "desc",
		ListOptions: github.ListOptions{
			PerPage: 100,
		},
	}

	for {
		ghPRs, resp, err := c.client.PullRequests.List(ctx, repo.Owner, repo.Name, opts)
		if err != nil {
			return nil, err
		}

		foundOlder := false
		for _, ghPR := range ghPRs {
			if ghPR.MergedAt == nil {
				continue
			}
			mergedAt := ghPR.GetMergedAt().Time
			if mergedAt.Before(since) {
				foundOlder = true
				continue
			}

			mpr := convertPR(ghPR, repo)
			mpr.DependBot = model.DetectDependBot(mpr.Author)
			if mpr.DependBot != model.DependBotUnknown {
				mpr.IsDependency = true
				mpr.Dependency = parseDependencyFromTitle(mpr.Title)
			}
			prs = append(prs, mpr)
		}

		if resp.NextPage == 0 || foundOlder {
			break
		}
		opts.Page = resp.NextPage
	}

	return prs, nil
}

// convertRepo converts a GitHub repository to our model.
func convertRepo(r *github.Repository) model.Repo {
	var topics []string
	if r.Topics != nil {
		topics = r.Topics
	}

	return model.Repo{
		Owner:         r.GetOwner().GetLogin(),
		Name:          r.GetName(),
		FullName:      r.GetFullName(),
		Description:   r.GetDescription(),
		DefaultBranch: r.GetDefaultBranch(),
		Private:       r.GetPrivate(),
		Archived:      r.GetArchived(),
		Language:      r.GetLanguage(),
		Topics:        topics,
		UpdatedAt:     r.GetUpdatedAt().Time,
		HTMLURL:       r.GetHTMLURL(),
	}
}

// convertPR converts a GitHub pull request to our model.
func convertPR(ghPR *github.PullRequest, repo model.RepoRef) model.PullRequest {
	var labels []string
	for _, l := range ghPR.Labels {
		labels = append(labels, l.GetName())
	}

	mpr := model.PullRequest{
		Number:    ghPR.GetNumber(),
		Title:     ghPR.GetTitle(),
		Body:      ghPR.GetBody(),
		State:     ghPR.GetState(),
		Author:    ghPR.GetUser().GetLogin(),
		HTMLURL:   ghPR.GetHTMLURL(),
		Draft:     ghPR.GetDraft(),
		Labels:    labels,
		CreatedAt: ghPR.GetCreatedAt().Time,
		UpdatedAt: ghPR.GetUpdatedAt().Time,
		Repo:      repo,
	}

	if ghPR.MergedAt != nil {
		t := ghPR.GetMergedAt().Time
		mpr.MergedAt = &t
	}

	return mpr
}

// parseDependencyFromTitle extracts dependency information from a PR title.
func parseDependencyFromTitle(title string) model.Dependency {
	dep := model.Dependency{}

	// Try to extract version numbers
	versionRe := regexp.MustCompile(`v?(\d+\.\d+\.\d+)`)
	versions := versionRe.FindAllString(title, 2)

	if len(versions) >= 2 {
		dep.FromVersion = versions[0]
		dep.ToVersion = versions[1]
		dep.UpdateType = determineUpdateType(dep.FromVersion, dep.ToVersion)
	} else if len(versions) == 1 {
		dep.ToVersion = versions[0]
	}

	// Try to extract dependency name
	patterns := []string{
		`(?:update|bump|upgrade)\s+(?:dependency\s+)?(\S+)`,
		`deps(?:\([^)]+\))?:\s*(?:update|bump|upgrade)\s+(\S+)`,
		`(\S+)\s+from\s+v?\d`,
	}

	lower := strings.ToLower(title)
	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		if matches := re.FindStringSubmatch(lower); len(matches) > 1 {
			dep.Name = matches[1]
			break
		}
	}

	// Detect ecosystem from dependency name
	dep.Ecosystem = detectEcosystem(dep.Name)

	return dep
}

// determineUpdateType determines the semantic version update type.
func determineUpdateType(from, to string) model.UpdateType {
	fromParts := parseVersion(from)
	toParts := parseVersion(to)

	if len(fromParts) < 3 || len(toParts) < 3 {
		return model.UpdateTypeUnknown
	}

	if toParts[0] > fromParts[0] {
		return model.UpdateTypeMajor
	}
	if toParts[1] > fromParts[1] {
		return model.UpdateTypeMinor
	}
	if toParts[2] > fromParts[2] {
		return model.UpdateTypePatch
	}

	return model.UpdateTypeUnknown
}

// parseVersion parses a version string into numeric parts.
func parseVersion(v string) []int {
	// Remove leading 'v'
	v = strings.TrimPrefix(v, "v")

	parts := strings.Split(v, ".")
	result := make([]int, len(parts))

	for i, p := range parts {
		// Parse only numeric prefix
		var num int
		for _, ch := range p {
			if ch >= '0' && ch <= '9' {
				num = num*10 + int(ch-'0')
			} else {
				break
			}
		}
		result[i] = num
	}

	return result
}

// detectEcosystem attempts to detect the package ecosystem from the dependency name.
func detectEcosystem(name string) string {
	switch {
	case strings.HasPrefix(name, "github.com/"):
		return "go"
	case strings.HasPrefix(name, "golang.org/"):
		return "go"
	case strings.HasPrefix(name, "@"):
		return "npm"
	case strings.Contains(name, "/") && !strings.Contains(name, "."):
		return "npm"
	default:
		return ""
	}
}

// isExcluded checks if a repo is in the exclude list.
func isExcluded(fullName string, excludeList []string) bool {
	for _, ex := range excludeList {
		if fullName == ex {
			return true
		}
	}
	return false
}

// TestsPassed checks if all check runs passed.
func TestsPassed(checkRuns []model.CheckRun) bool {
	if len(checkRuns) == 0 {
		return false
	}

	for _, c := range checkRuns {
		if !c.IsSuccess() {
			return false
		}
	}
	return true
}

// WaitForChecks polls until all checks complete or timeout.
func (c *GitHubCollector) WaitForChecks(ctx context.Context, repo model.RepoRef, prNumber int, timeout time.Duration) ([]model.CheckRun, error) {
	// Get PR to get head SHA
	ghPR, err := pr.GetPR(ctx, c.client, repo.Owner, repo.Name, prNumber)
	if err != nil {
		return nil, err
	}

	sha := ghPR.GetHead().GetSHA()
	pollInterval := 30 * time.Second

	ghChecks, _, err := checks.WaitForChecks(ctx, c.client, repo.Owner, repo.Name, sha, timeout, pollInterval)
	if err != nil {
		return nil, err
	}

	var result []model.CheckRun
	for _, cr := range ghChecks {
		result = append(result, model.CheckRun{
			Name:       cr.GetName(),
			Status:     cr.GetStatus(),
			Conclusion: cr.GetConclusion(),
		})
	}

	return result, nil
}
