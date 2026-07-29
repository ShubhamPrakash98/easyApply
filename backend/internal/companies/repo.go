package companies

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Company struct {
	ID          string
	Name        string
	Domain      string
	ResolvedVia string
	CreatedAt   time.Time
}

var ErrNotFound = errors.New("company not found")

type Repo struct {
	db *pgxpool.Pool
}

func NewRepo(db *pgxpool.Pool) *Repo { return &Repo{db: db} }

// GetOrCreateByName finds a company by name (case-insensitive) or creates a
// new row. Domain and resolved_via are populated on create; if a row already
// exists they are left as-is.
func (r *Repo) GetOrCreateByName(ctx context.Context, name, domain, resolvedVia string) (*Company, error) {
	if existing, err := r.GetByName(ctx, name); err == nil {
		return existing, nil
	} else if !errors.Is(err, ErrNotFound) {
		return nil, err
	}
	const q = `
INSERT INTO companies (name, domain, resolved_via)
VALUES ($1, NULLIF($2, ''), NULLIF($3, ''))
ON CONFLICT ((LOWER(name))) DO UPDATE SET name = EXCLUDED.name
RETURNING id, name, COALESCE(domain, ''), COALESCE(resolved_via, ''), created_at
`
	row := r.db.QueryRow(ctx, q, name, domain, resolvedVia)
	return scan(row)
}

func (r *Repo) GetByName(ctx context.Context, name string) (*Company, error) {
	const q = `
SELECT id, name, COALESCE(domain, ''), COALESCE(resolved_via, ''), created_at
FROM companies WHERE LOWER(name) = LOWER($1)
`
	row := r.db.QueryRow(ctx, q, name)
	c, err := scan(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return c, err
}

func scan(row pgx.Row) (*Company, error) {
	var c Company
	if err := row.Scan(&c.ID, &c.Name, &c.Domain, &c.ResolvedVia, &c.CreatedAt); err != nil {
		return nil, err
	}
	return &c, nil
}
