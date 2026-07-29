package outreach

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Status string

const (
	StatusPendingApproval Status = "pending_approval"
	StatusSent            Status = "sent"
	StatusReplied         Status = "replied"
	StatusFollowedUp      Status = "followed_up"
	StatusNoResponse      Status = "no_response"
	StatusCancelled       Status = "cancelled"
)

type Outreach struct {
	ID                string
	UserID            string
	ContactID         string
	ResumeID          *string
	JobDescription    string
	EmailSubject      string
	EmailBody         string
	GmailThreadID     *string
	Status            Status
	SentAt            *time.Time
	RepliedAt         *time.Time
	FollowUpCount     int
	LastFollowedUpAt  *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

var ErrNotFound = errors.New("outreach not found")

type Repo struct {
	db *pgxpool.Pool
}

func NewRepo(db *pgxpool.Pool) *Repo { return &Repo{db: db} }

type CreateParams struct {
	UserID         string
	ContactID      string
	ResumeID       *string
	JobDescription string
	EmailSubject   string
	EmailBody      string
}

func (r *Repo) Create(ctx context.Context, p CreateParams) (*Outreach, error) {
	const q = `
INSERT INTO outreach (user_id, contact_id, resume_id, job_description, email_subject, email_body)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, user_id, contact_id, resume_id, job_description, email_subject, email_body,
          gmail_thread_id, status, sent_at, replied_at, follow_up_count, last_followed_up_at,
          created_at, updated_at
`
	row := r.db.QueryRow(ctx, q, p.UserID, p.ContactID, p.ResumeID, p.JobDescription, p.EmailSubject, p.EmailBody)
	return scan(row)
}

func (r *Repo) GetByIDForUser(ctx context.Context, id, userID string) (*Outreach, error) {
	const q = `
SELECT id, user_id, contact_id, resume_id, job_description, email_subject, email_body,
       gmail_thread_id, status, sent_at, replied_at, follow_up_count, last_followed_up_at,
       created_at, updated_at
FROM outreach WHERE id = $1 AND user_id = $2
`
	row := r.db.QueryRow(ctx, q, id, userID)
	o, err := scan(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return o, err
}

type ListFilter struct {
	Status string
	Limit  int
	Offset int
}

type ListRow struct {
	Outreach
	RecruiterName string
	RecruiterURL  string
	CompanyName   string
}

func (r *Repo) ListForUser(ctx context.Context, userID string, f ListFilter) ([]ListRow, error) {
	if f.Limit <= 0 || f.Limit > 100 {
		f.Limit = 25
	}
	const q = `
SELECT o.id, o.user_id, o.contact_id, o.resume_id, o.job_description, o.email_subject, o.email_body,
       o.gmail_thread_id, o.status, o.sent_at, o.replied_at, o.follow_up_count, o.last_followed_up_at,
       o.created_at, o.updated_at,
       c.name, COALESCE(c.linkedin_url, ''), COALESCE(co.name, '')
FROM outreach o
JOIN contacts c ON c.id = o.contact_id
LEFT JOIN companies co ON co.id = c.company_id
WHERE o.user_id = $1
  AND ($2 = '' OR o.status = $2)
ORDER BY o.created_at DESC
LIMIT $3 OFFSET $4
`
	rows, err := r.db.Query(ctx, q, userID, f.Status, f.Limit, f.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ListRow
	for rows.Next() {
		var row ListRow
		err := rows.Scan(
			&row.ID, &row.UserID, &row.ContactID, &row.ResumeID, &row.JobDescription,
			&row.EmailSubject, &row.EmailBody, &row.GmailThreadID, &row.Status,
			&row.SentAt, &row.RepliedAt, &row.FollowUpCount, &row.LastFollowedUpAt,
			&row.CreatedAt, &row.UpdatedAt,
			&row.RecruiterName, &row.RecruiterURL, &row.CompanyName,
		)
		if err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r *Repo) MarkSent(ctx context.Context, id, userID, threadID string, finalSubject, finalBody string) (*Outreach, error) {
	const q = `
UPDATE outreach SET
  status          = 'sent',
  gmail_thread_id = $3,
  email_subject   = $4,
  email_body      = $5,
  sent_at         = NOW(),
  updated_at      = NOW()
WHERE id = $1 AND user_id = $2 AND status = 'pending_approval'
RETURNING id, user_id, contact_id, resume_id, job_description, email_subject, email_body,
          gmail_thread_id, status, sent_at, replied_at, follow_up_count, last_followed_up_at,
          created_at, updated_at
`
	row := r.db.QueryRow(ctx, q, id, userID, threadID, finalSubject, finalBody)
	o, err := scan(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return o, err
}

func (r *Repo) MarkCancelled(ctx context.Context, id, userID string) error {
	const q = `
UPDATE outreach SET status = 'cancelled', updated_at = NOW()
WHERE id = $1 AND user_id = $2 AND status = 'pending_approval'
`
	tag, err := r.db.Exec(ctx, q, id, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// CountSentToday reports how many outreach rows for a user have status IN
// (sent, replied, followed_up, no_response) with sent_at today (UTC).
// Used to enforce the daily rate limit.
func (r *Repo) CountSentToday(ctx context.Context, userID string) (int, error) {
	const q = `
SELECT COUNT(*) FROM outreach
WHERE user_id = $1
  AND sent_at IS NOT NULL
  AND sent_at >= date_trunc('day', NOW() AT TIME ZONE 'UTC')
`
	var n int
	if err := r.db.QueryRow(ctx, q, userID).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

func scan(row pgx.Row) (*Outreach, error) {
	var o Outreach
	err := row.Scan(
		&o.ID, &o.UserID, &o.ContactID, &o.ResumeID, &o.JobDescription,
		&o.EmailSubject, &o.EmailBody, &o.GmailThreadID, &o.Status,
		&o.SentAt, &o.RepliedAt, &o.FollowUpCount, &o.LastFollowedUpAt,
		&o.CreatedAt, &o.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &o, nil
}
