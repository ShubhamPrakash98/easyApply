package contacts

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Contact struct {
	ID                 string
	Name               string
	CompanyID          string
	Email              string
	LinkedInURL        string
	Source             string
	VerificationStatus string
	VerifiedAt         *time.Time
	FetchedAt          time.Time
	CreatedAt          time.Time
}

var ErrNotFound = errors.New("contact not found")

type Repo struct {
	db *pgxpool.Pool
}

func NewRepo(db *pgxpool.Pool) *Repo { return &Repo{db: db} }

type UpsertParams struct {
	Name               string
	CompanyID          string
	Email              string
	LinkedInURL        string
	Source             string
	VerificationStatus string
}

// UpsertByLinkedInURL either creates a new contact or updates the existing
// row keyed by linkedin_url (unique). Verification data is refreshed on hit.
func (r *Repo) UpsertByLinkedInURL(ctx context.Context, p UpsertParams) (*Contact, error) {
	const q = `
INSERT INTO contacts (name, company_id, email, linkedin_url, source, verification_status, verified_at, fetched_at)
VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())
ON CONFLICT (linkedin_url) WHERE linkedin_url IS NOT NULL DO UPDATE SET
  name                = EXCLUDED.name,
  company_id          = EXCLUDED.company_id,
  email               = EXCLUDED.email,
  source              = EXCLUDED.source,
  verification_status = EXCLUDED.verification_status,
  verified_at         = NOW(),
  fetched_at          = NOW()
RETURNING id, name, COALESCE(company_id::text, ''), COALESCE(email, ''), COALESCE(linkedin_url, ''),
          source, verification_status, verified_at, fetched_at, created_at
`
	var companyID any
	if p.CompanyID != "" {
		companyID = p.CompanyID
	}
	row := r.db.QueryRow(ctx, q,
		p.Name,
		companyID,
		nullIfEmpty(p.Email),
		nullIfEmpty(p.LinkedInURL),
		p.Source,
		p.VerificationStatus,
	)
	return scan(row)
}

func (r *Repo) GetByID(ctx context.Context, id string) (*Contact, error) {
	const q = `
SELECT id, name, COALESCE(company_id::text, ''), COALESCE(email, ''), COALESCE(linkedin_url, ''),
       source, verification_status, verified_at, fetched_at, created_at
FROM contacts WHERE id = $1
`
	row := r.db.QueryRow(ctx, q, id)
	c, err := scan(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return c, err
}

// GetByLinkedInURL returns the cached contact for a linkedin_url or
// ErrNotFound if we haven't seen this recruiter before. Used by the
// CascadeFinder's Stage A (cache lookup) via an adapter in main.go.
func (r *Repo) GetByLinkedInURL(ctx context.Context, url string) (*Contact, error) {
	if url == "" {
		return nil, ErrNotFound
	}
	const q = `
SELECT id, name, COALESCE(company_id::text, ''), COALESCE(email, ''), COALESCE(linkedin_url, ''),
       source, verification_status, verified_at, fetched_at, created_at
FROM contacts WHERE linkedin_url = $1
`
	row := r.db.QueryRow(ctx, q, url)
	c, err := scan(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return c, err
}

func scan(row pgx.Row) (*Contact, error) {
	var c Contact
	err := row.Scan(
		&c.ID,
		&c.Name,
		&c.CompanyID,
		&c.Email,
		&c.LinkedInURL,
		&c.Source,
		&c.VerificationStatus,
		&c.VerifiedAt,
		&c.FetchedAt,
		&c.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}
