package policy

import (
	"github.com/plexusone/versionconductor/pkg/model"
)

// Predefined merge profiles.
var (
	// ProfileAggressive merges all passing PRs immediately.
	ProfileAggressive = model.MergeProfile{
		Name:        "aggressive",
		Description: "Merge all passing dependency PRs immediately",

		MinAgeHours: 0,
		MaxAgeHours: 0,

		AutoMergePatch: true,
		AutoMergeMinor: true,
		AutoMergeMajor: true,

		RequireAllChecks:   true,
		AllowPendingChecks: false,

		MergeStrategy: "squash",
		DeleteBranch:  true,

		RequireApproval: false,
		MaxPRsPerRun:    0, // No limit
	}

	// ProfileBalanced waits 24h and only merges patch/minor updates.
	ProfileBalanced = model.MergeProfile{
		Name:        "balanced",
		Description: "Wait 24h, auto-merge patch and minor updates only",

		MinAgeHours: 24,
		MaxAgeHours: 0,

		AutoMergePatch: true,
		AutoMergeMinor: true,
		AutoMergeMajor: false,

		RequireAllChecks:   true,
		AllowPendingChecks: false,

		MergeStrategy: "squash",
		DeleteBranch:  true,

		RequireApproval: false,
		MaxPRsPerRun:    10,
	}

	// ProfileConservative requires manual review for all but patch updates.
	ProfileConservative = model.MergeProfile{
		Name:        "conservative",
		Description: "Auto-merge only patch updates after 48h, manual review for others",

		MinAgeHours: 48,
		MaxAgeHours: 0,

		AutoMergePatch: true,
		AutoMergeMinor: false,
		AutoMergeMajor: false,

		RequireAllChecks:   true,
		AllowPendingChecks: false,

		MergeStrategy: "squash",
		DeleteBranch:  true,

		RequireApproval: true,
		MaxPRsPerRun:    5,
	}
)

// GetProfile returns a merge profile by name.
func GetProfile(name string) *model.MergeProfile {
	switch name {
	case "aggressive":
		return &ProfileAggressive
	case "balanced":
		return &ProfileBalanced
	case "conservative":
		return &ProfileConservative
	default:
		return nil
	}
}

// ListProfiles returns all available profile names.
func ListProfiles() []string {
	return []string{"aggressive", "balanced", "conservative"}
}

// EvaluateProfile evaluates a PR against a merge profile.
// Returns true if the PR should be merged according to the profile.
func EvaluateProfile(profile *model.MergeProfile, pr *model.PullRequest, checks []model.CheckRun) (bool, string) {
	// Check age requirements
	ageHours := pr.AgeHours()
	if profile.MinAgeHours > 0 && ageHours < profile.MinAgeHours {
		return false, "PR is too young"
	}
	if profile.MaxAgeHours > 0 && ageHours > profile.MaxAgeHours {
		return false, "PR is too old"
	}

	// Check update type
	switch pr.Dependency.UpdateType {
	case model.UpdateTypeMajor:
		if !profile.AutoMergeMajor {
			return false, "major updates require manual review"
		}
	case model.UpdateTypeMinor:
		if !profile.AutoMergeMinor {
			return false, "minor updates require manual review"
		}
	case model.UpdateTypePatch:
		if !profile.AutoMergePatch {
			return false, "patch updates require manual review"
		}
	default:
		return false, "unknown update type"
	}

	// Check CI status
	if profile.RequireAllChecks {
		allPassed := true
		anyPending := false

		for _, c := range checks {
			if c.Status != "completed" {
				anyPending = true
				allPassed = false
			} else if !c.IsSuccess() {
				return false, "CI checks failed"
			}
		}

		if anyPending && !profile.AllowPendingChecks {
			return false, "CI checks still pending"
		}

		if !allPassed && !anyPending {
			return false, "CI checks failed"
		}
	}

	// Check mergeable status
	if !pr.Mergeable {
		return false, "PR is not mergeable"
	}

	if pr.Draft {
		return false, "PR is a draft"
	}

	return true, ""
}
