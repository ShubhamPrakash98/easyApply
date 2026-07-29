package users

import "time"

type User struct {
	ID                    string
	GoogleSub             string
	Email                 string
	Name                  string
	GmailRefreshTokenEnc  []byte
	TrialEndsAt           time.Time
	CreatedAt             time.Time
	UpdatedAt             time.Time
}
