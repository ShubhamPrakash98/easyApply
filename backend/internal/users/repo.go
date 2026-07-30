package users

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("user not found")

type Repo struct {
	db *pgxpool.Pool
}

func NewRepo(db *pgxpool.Pool) *Repo {
	return &Repo{db: db}
}

// UpsertParams captures the fields that can change on re-login.
type UpsertParams struct {
	GoogleSub            string
	Email                string
	Name                 string
	GmailRefreshTokenEnc []byte // may be empty on subsequent logins (Google only issues refresh tokens on first consent)
}

// Upsert inserts or updates a user by google_sub. Returns the resulting row.
// If GmailRefreshTokenEnc is nil the existing value is kept (Google only issues
// refresh_tokens on first grant unless prompt=consent is forced).
func (r *Repo) Upsert(ctx context.Context, p UpsertParams) (*User, error) {
	const q = `
INSERT INTO users (google_sub, email, name, gmail_refresh_token_enc)
VALUES ($1, $2, $3, $4)
ON CONFLICT (google_sub) DO UPDATE SET
  email      = EXCLUDED.email,
  name       = EXCLUDED.name,
  gmail_refresh_token_enc = COALESCE(EXCLUDED.gmail_refresh_token_enc, users.gmail_refresh_token_enc),
  updated_at = NOW()
RETURNING id, google_sub, email, name, gmail_refresh_token_enc, trial_ends_at, subscription_tier, created_at, updated_at
`
	var refresh any = nil
	if len(p.GmailRefreshTokenEnc) > 0 {
		refresh = p.GmailRefreshTokenEnc
	}
	row := r.db.QueryRow(ctx, q, p.GoogleSub, p.Email, p.Name, refresh)
	return scanUser(row)
}

func (r *Repo) GetByID(ctx context.Context, id string) (*User, error) {
	const q = `
SELECT id, google_sub, email, name, gmail_refresh_token_enc, trial_ends_at, subscription_tier, created_at, updated_at
FROM users WHERE id = $1
`
	row := r.db.QueryRow(ctx, q, id)
	u, err := scanUser(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return u, err
}

func scanUser(row pgx.Row) (*User, error) {
	var u User
	err := row.Scan(
		&u.ID,
		&u.GoogleSub,
		&u.Email,
		&u.Name,
		&u.GmailRefreshTokenEnc,
		&u.TrialEndsAt,
		&u.SubscriptionTier,
		&u.CreatedAt,
		&u.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &u, nil
}
