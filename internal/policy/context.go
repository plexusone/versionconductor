package policy

import (
	"github.com/plexusone/versionconductor/pkg/model"
)

// ContextBuilder builds PolicyContext from PR and check information.
type ContextBuilder struct{}

// NewContextBuilder creates a new context builder.
func NewContextBuilder() *ContextBuilder {
	return &ContextBuilder{}
}

// Build creates a PolicyContext from a PR and its checks.
func (b *ContextBuilder) Build(pr *model.PullRequest, repo *model.Repo, checks []model.CheckRun) *model.PolicyContext {
	ctx := &model.PolicyContext{
		Repo:       b.buildRepoContext(repo),
		PR:         b.buildPRContext(pr),
		Dependency: b.buildDependencyContext(&pr.Dependency),
		CI:         b.buildCIContext(checks),
	}

	return ctx
}

// buildRepoContext builds the repository context.
func (b *ContextBuilder) buildRepoContext(repo *model.Repo) model.RepoContext {
	if repo == nil {
		return model.RepoContext{}
	}

	return model.RepoContext{
		Owner:    repo.Owner,
		Name:     repo.Name,
		FullName: repo.FullName,
		Private:  repo.Private,
		Archived: repo.Archived,
		Language: repo.Language,
		Topics:   repo.Topics,
	}
}

// buildPRContext builds the pull request context.
func (b *ContextBuilder) buildPRContext(pr *model.PullRequest) model.PRContext {
	if pr == nil {
		return model.PRContext{}
	}

	ageHours := pr.AgeHours()

	return model.PRContext{
		Number:       pr.Number,
		Title:        pr.Title,
		Author:       pr.Author,
		IsDependency: pr.IsDependency,
		DependBot:    string(pr.DependBot),
		AgeHours:     ageHours,
		AgeDays:      ageHours / 24,
		Mergeable:    pr.Mergeable,
		Draft:        pr.Draft,
		Labels:       pr.Labels,
		HasConflicts: pr.MergeableStr == "dirty",
	}
}

// buildDependencyContext builds the dependency context.
func (b *ContextBuilder) buildDependencyContext(dep *model.Dependency) model.DependencyContext {
	if dep == nil {
		return model.DependencyContext{}
	}

	return model.DependencyContext{
		Name:        dep.Name,
		Ecosystem:   dep.Ecosystem,
		FromVersion: dep.FromVersion,
		ToVersion:   dep.ToVersion,
		UpdateType:  string(dep.UpdateType),
		IsMajor:     dep.UpdateType == model.UpdateTypeMajor,
		IsMinor:     dep.UpdateType == model.UpdateTypeMinor,
		IsPatch:     dep.UpdateType == model.UpdateTypePatch,
	}
}

// buildCIContext builds the CI/check context.
func (b *ContextBuilder) buildCIContext(checks []model.CheckRun) model.CIContext {
	ctx := model.CIContext{
		PassedChecks:  []string{},
		FailedChecks:  []string{},
		PendingChecks: []string{},
	}

	if len(checks) == 0 {
		return ctx
	}

	allPassed := true
	anyFailed := false
	anyPending := false

	for _, c := range checks {
		switch {
		case c.Status != "completed":
			anyPending = true
			allPassed = false
			ctx.PendingChecks = append(ctx.PendingChecks, c.Name)
		case c.IsSuccess():
			ctx.PassedChecks = append(ctx.PassedChecks, c.Name)
		default:
			anyFailed = true
			allPassed = false
			ctx.FailedChecks = append(ctx.FailedChecks, c.Name)
		}
	}

	ctx.AllPassed = allPassed
	ctx.AnyFailed = anyFailed
	ctx.AnyPending = anyPending
	ctx.RequiredPassed = allPassed // Simplified; could check specific required checks

	return ctx
}
