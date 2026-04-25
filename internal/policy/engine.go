package policy

import (
	"context"

	"github.com/plexusone/versionconductor/pkg/model"
)

// Engine evaluates policies for PR merge decisions.
// This is a simplified implementation that uses merge profiles.
// Cedar policy support can be added in a future version.
type Engine struct {
	profile *model.MergeProfile
}

// NewEngine creates a new policy engine with the given profile name.
func NewEngine(profileName string) (*Engine, error) {
	profile := GetProfile(profileName)
	if profile == nil {
		profile = &ProfileBalanced
	}
	return &Engine{profile: profile}, nil
}

// NewEngineWithProfile creates a new policy engine with the given profile.
func NewEngineWithProfile(profile *model.MergeProfile) *Engine {
	return &Engine{profile: profile}
}

// Evaluate evaluates the policy for the given action and context.
func (e *Engine) Evaluate(ctx context.Context, action model.PolicyAction, pr *model.PullRequest, checks []model.CheckRun) (*model.PolicyDecision, error) {
	result := &model.PolicyDecision{
		Action: string(action),
	}

	switch action {
	case model.PolicyActionMerge:
		allowed, reason := EvaluateProfile(e.profile, pr, checks)
		result.Allowed = allowed
		if !allowed {
			result.Reasons = []string{reason}
		}
	case model.PolicyActionReview:
		// For review, we're more permissive - just need passing tests
		if pr.TestsPassed && !pr.Draft {
			result.Allowed = true
		} else {
			result.Allowed = false
			if !pr.TestsPassed {
				result.Reasons = append(result.Reasons, "CI checks not passed")
			}
			if pr.Draft {
				result.Reasons = append(result.Reasons, "PR is a draft")
			}
		}
	case model.PolicyActionRelease:
		// Release is allowed if there are merged PRs
		result.Allowed = true
	default:
		result.Allowed = false
		result.Reasons = []string{"unknown action"}
	}

	return result, nil
}

// CanMerge evaluates whether a PR can be auto-merged.
func (e *Engine) CanMerge(ctx context.Context, pr *model.PullRequest, checks []model.CheckRun) (*model.PolicyDecision, error) {
	return e.Evaluate(ctx, model.PolicyActionMerge, pr, checks)
}

// CanReview evaluates whether a PR can be auto-reviewed.
func (e *Engine) CanReview(ctx context.Context, pr *model.PullRequest, checks []model.CheckRun) (*model.PolicyDecision, error) {
	return e.Evaluate(ctx, model.PolicyActionReview, pr, checks)
}

// CanRelease evaluates whether a release can be created.
func (e *Engine) CanRelease(ctx context.Context) (*model.PolicyDecision, error) {
	return &model.PolicyDecision{
		Allowed: true,
		Action:  string(model.PolicyActionRelease),
	}, nil
}
