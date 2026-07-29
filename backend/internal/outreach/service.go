package outreach

import (
	"context"
	"errors"
	"fmt"

	"github.com/shubham/oneapply/backend/internal/companies"
	"github.com/shubham/oneapply/backend/internal/contacts"
	"github.com/shubham/oneapply/backend/internal/finder"
	"github.com/shubham/oneapply/backend/internal/gmail"
	"github.com/shubham/oneapply/backend/internal/llm"
	"github.com/shubham/oneapply/backend/internal/users"
)

var (
	ErrRateLimited   = errors.New("daily rate limit exceeded")
	ErrEmailNotFound = errors.New("could not find recruiter email")
)

// Service orchestrates the 5 pillars. Phase 2 wires stubs; Phase 3+ swap them.
type Service struct {
	users     *users.Repo
	companies *companies.Repo
	contacts  *contacts.Repo
	outreach  *Repo
	finder    finder.EmailFinder
	llm       llm.LLMService
	sender    gmail.EmailSender

	dailyLimit int
}

type ServiceParams struct {
	Users      *users.Repo
	Companies  *companies.Repo
	Contacts   *contacts.Repo
	Outreach   *Repo
	Finder     finder.EmailFinder
	LLM        llm.LLMService
	Sender     gmail.EmailSender
	DailyLimit int
}

func NewService(p ServiceParams) *Service {
	if p.DailyLimit <= 0 {
		p.DailyLimit = 3
	}
	return &Service{
		users:      p.Users,
		companies:  p.Companies,
		contacts:   p.Contacts,
		outreach:   p.Outreach,
		finder:     p.Finder,
		llm:        p.LLM,
		sender:     p.Sender,
		dailyLimit: p.DailyLimit,
	}
}

type DraftInput struct {
	UserID           string
	RecruiterName    string
	RecruiterHeadline string
	Company          string
	LinkedInURL      string
	JobDescription   string
}

type DraftResult struct {
	Outreach *Outreach
	Contact  *contacts.Contact
	Company  string
}

// Draft is the entry point for the extension "Reach Out" flow.
// 1) rate-limit check
// 2) run the finder cascade (Phase 2: stub returns first.last@heuristic.com)
// 3) upsert company + contact
// 4) LLM draft
// 5) create outreach row with status=pending_approval
// The email is NOT sent here — Approve() does that.
func (s *Service) Draft(ctx context.Context, in DraftInput) (*DraftResult, error) {
	// Rate limit.
	sentToday, err := s.outreach.CountSentToday(ctx, in.UserID)
	if err != nil {
		return nil, fmt.Errorf("rate limit check: %w", err)
	}
	if sentToday >= s.dailyLimit {
		return nil, ErrRateLimited
	}

	// Find email.
	fRes, err := s.finder.FindEmail(ctx, finder.FindEmailRequest{
		Name:        in.RecruiterName,
		Company:     in.Company,
		LinkedInURL: in.LinkedInURL,
	})
	if err != nil {
		return nil, fmt.Errorf("find email: %w", err)
	}
	if fRes == nil || fRes.Email == "" {
		return nil, ErrEmailNotFound
	}

	// Upsert company + contact.
	var companyID string
	if in.Company != "" {
		co, err := s.companies.GetOrCreateByName(ctx, in.Company, fRes.CompanyDomain, "heuristic")
		if err != nil {
			return nil, fmt.Errorf("company upsert: %w", err)
		}
		companyID = co.ID
	}

	contact, err := s.contacts.UpsertByLinkedInURL(ctx, contacts.UpsertParams{
		Name:               in.RecruiterName,
		CompanyID:          companyID,
		Email:              fRes.Email,
		LinkedInURL:        in.LinkedInURL,
		Source:             fRes.Source,
		VerificationStatus: fRes.VerificationStatus,
	})
	if err != nil {
		return nil, fmt.Errorf("contact upsert: %w", err)
	}

	// Load user (need name for LLM).
	u, err := s.users.GetByID(ctx, in.UserID)
	if err != nil {
		return nil, fmt.Errorf("load user: %w", err)
	}

	// LLM draft.
	draft, err := s.llm.DraftEmail(ctx, llm.DraftRequest{
		RecruiterName:     in.RecruiterName,
		RecruiterHeadline: in.RecruiterHeadline,
		Company:           in.Company,
		JobDescription:    in.JobDescription,
		SenderName:        firstNonEmpty(u.Name, u.Email),
	})
	if err != nil {
		return nil, fmt.Errorf("llm draft: %w", err)
	}

	// Create outreach row (pending approval).
	o, err := s.outreach.Create(ctx, CreateParams{
		UserID:         in.UserID,
		ContactID:      contact.ID,
		JobDescription: in.JobDescription,
		EmailSubject:   draft.Subject,
		EmailBody:      draft.Body,
	})
	if err != nil {
		return nil, fmt.Errorf("outreach create: %w", err)
	}

	return &DraftResult{
		Outreach: o,
		Contact:  contact,
		Company:  in.Company,
	}, nil
}

type ApproveInput struct {
	UserID       string
	OutreachID   string
	FinalSubject string // if empty, keep existing draft
	FinalBody    string // if empty, keep existing draft
}

// Approve sends the (possibly edited) draft via EmailSender and flips
// the row to status=sent.
func (s *Service) Approve(ctx context.Context, in ApproveInput) (*Outreach, error) {
	o, err := s.outreach.GetByIDForUser(ctx, in.OutreachID, in.UserID)
	if err != nil {
		return nil, err
	}
	if o.Status != StatusPendingApproval {
		return nil, fmt.Errorf("outreach status is %q, cannot approve", o.Status)
	}

	subject := firstNonEmpty(in.FinalSubject, o.EmailSubject)
	body := firstNonEmpty(in.FinalBody, o.EmailBody)

	c, err := s.contacts.GetByID(ctx, o.ContactID)
	if err != nil {
		return nil, fmt.Errorf("load contact: %w", err)
	}
	if c.Email == "" {
		return nil, ErrEmailNotFound
	}

	threadID, err := s.sender.Send(ctx, in.UserID, gmail.Email{
		To:      c.Email,
		Subject: subject,
		Body:    body,
	})
	if err != nil {
		return nil, fmt.Errorf("send: %w", err)
	}

	sent, err := s.outreach.MarkSent(ctx, o.ID, in.UserID, threadID, subject, body)
	if err != nil {
		return nil, fmt.Errorf("mark sent: %w", err)
	}
	return sent, nil
}

// Cancel drops a pending draft.
func (s *Service) Cancel(ctx context.Context, userID, id string) error {
	return s.outreach.MarkCancelled(ctx, id, userID)
}

// ListForUser is a thin passthrough for the dashboard/extension list views.
func (s *Service) ListForUser(ctx context.Context, userID string, f ListFilter) ([]ListRow, error) {
	return s.outreach.ListForUser(ctx, userID, f)
}

// GetDetail returns the outreach + associated contact for the drawer view.
type Detail struct {
	Outreach *Outreach
	Contact  *contacts.Contact
}

func (s *Service) GetDetail(ctx context.Context, userID, id string) (*Detail, error) {
	o, err := s.outreach.GetByIDForUser(ctx, id, userID)
	if err != nil {
		return nil, err
	}
	c, err := s.contacts.GetByID(ctx, o.ContactID)
	if err != nil {
		return nil, err
	}
	return &Detail{Outreach: o, Contact: c}, nil
}

func firstNonEmpty(vs ...string) string {
	for _, v := range vs {
		if v != "" {
			return v
		}
	}
	return ""
}
