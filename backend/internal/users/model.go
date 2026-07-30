package users

import "time"

type User struct {
	ID                   string
	GoogleSub            string
	Email                string
	Name                 string
	GmailRefreshTokenEnc []byte
	TrialEndsAt          time.Time
	SubscriptionTier     string // "free" | "premium" — see internal/features
	CreatedAt            time.Time
	UpdatedAt            time.Time
}
