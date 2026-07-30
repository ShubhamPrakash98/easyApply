// Package features gates paid capabilities behind a user's subscription tier.
// Every AI-powered thing on OneApply lives behind a flag so we can ship the
// free surface (email finder + tracking + follow-ups + notifications)
// without giving away the drafting/matching costs.
package features

import "github.com/shubham/oneapply/backend/internal/users"

type Feature string

const (
	// AIDraftEmail — Claude writes the outreach body using the JD. When
	// off, the extension shows an empty editable form and the user writes
	// their own email.
	AIDraftEmail Feature = "ai_draft_email"

	// AIResumeMatch — Claude picks the best-matching resume from the user's
	// uploaded set. When off, we use the explicitly-picked resume, or the
	// first uploaded resume, or none.
	AIResumeMatch Feature = "ai_resume_match"

	// AIFollowUp — Claude drafts the follow-up body (Phase 7). Off →
	// user won't get automated follow-ups at all.
	AIFollowUp Feature = "ai_followup"
)

// All lists every feature. Handy for the /api/auth/me response.
func All() []Feature {
	return []Feature{AIDraftEmail, AIResumeMatch, AIFollowUp}
}

// IsEnabled returns whether the feature is on for the given user.
// Tier rules — kept in one place so they're easy to shift as the plan matrix evolves.
func IsEnabled(u *users.User, f Feature) bool {
	if u == nil {
		return false
	}
	switch f {
	case AIDraftEmail, AIResumeMatch, AIFollowUp:
		return u.SubscriptionTier == "premium"
	}
	return false
}

// Snapshot returns the full feature map for a user (used in /api/auth/me).
func Snapshot(u *users.User) map[string]bool {
	out := make(map[string]bool, len(All()))
	for _, f := range All() {
		out[string(f)] = IsEnabled(u, f)
	}
	return out
}
