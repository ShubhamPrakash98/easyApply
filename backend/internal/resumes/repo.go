package resumes

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repo struct {
	db *pgxpool.Pool
}

func NewRepo(db *pgxpool.Pool) *Repo { return &Repo{db: db} }

type CreateParams struct {
	UserID        string
	Label         string
	StorageURL    string
	ExtractedText string
}

func (r *Repo) Create(ctx context.Context, p CreateParams) (*Resume, error) {
	const q = `
INSERT INTO resumes (user_id, label, storage_url, extracted_text)
VALUES ($1, $2, $3, $4)
RETURNING id, user_id, label, storage_url, COALESCE(extracted_text, ''), created_at
`
	row := r.db.QueryRow(ctx, q, p.UserID, p.Label, p.StorageURL, p.ExtractedText)
	return scan(row)
}

func (r *Repo) ListForUser(ctx context.Context, userID string) ([]Resume, error) {
	const q = `
SELECT id, user_id, label, storage_url, COALESCE(extracted_text, ''), created_at
FROM resumes WHERE user_id = $1
ORDER BY created_at DESC
`
	rows, err := r.db.Query(ctx, q, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Resume
	for rows.Next() {
		r, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *r)
	}
	return out, rows.Err()
}

func (r *Repo) GetForUser(ctx context.Context, id, userID string) (*Resume, error) {
	const q = `
SELECT id, user_id, label, storage_url, COALESCE(extracted_text, ''), created_at
FROM resumes WHERE id = $1 AND user_id = $2
`
	row := r.db.QueryRow(ctx, q, id, userID)
	res, err := scan(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return res, err
}

// DeleteForUser removes the DB row. The caller is responsible for also
// deleting the underlying file (Storage.Delete).
func (r *Repo) DeleteForUser(ctx context.Context, id, userID string) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM resumes WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func scan(row pgx.Row) (*Resume, error) {
	var r Resume
	if err := row.Scan(&r.ID, &r.UserID, &r.Label, &r.StorageURL, &r.ExtractedText, &r.CreatedAt); err != nil {
		return nil, err
	}
	return &r, nil
}
