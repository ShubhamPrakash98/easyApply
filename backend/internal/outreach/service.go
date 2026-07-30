package outreach

import (
	"context"
	"errors"
	"fmt"

	"github.com/shubham/oneapply/backend/internal/companies"
	"github.com/shubham/oneapply/backend/internal/contacts"
	"github.com/shubham/oneapply/backend/internal/features"
	"github.com/shubham/oneapply/backend/internal/finder"
	"github.com/shubham/oneapply/backend/internal/gmail"
	"github.com/shubham/oneapply/backend/internal/llm"
	"github.com/shubham/oneapply/backend/internal/resumes"
	"github.com/shubham/oneapply/backend/internal/users"
)

var (
	ErrRateLimited   = errors.New("daily rate limit exceeded")
	ErrEmailNotFound = errors.New("could not find recruiter email")
	ErrEmptyDraft    = errors.New("subject and body are required")
)

// Service orchestrates the 5 pillars. Phase 4 wires the real finder + LLM +
// Gmail sender behind the same interfaces.
type Service struct {
	users     *users.Repo
	companies *companies.Repo
	contacts  *contacts.Repo
	outreach  *Repo
	resumes   *resumes.Repo
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
	Resumes    *resumes.Repo
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
		resumes:    p.Resumes,
		finder:     p.Finder,
		llm:        p.LLM,
		sender:     p.Sender,
		dailyLimit: p.DailyLimit,
	}
}

type DraftInput struct {
	UserID            string
	RecruiterName     string
	RecruiterHeadline string
	Company           string
	LinkedInURL       string
	JobDescription    string
	// ResumeID: if empty, the LLM picks the best match among the user's
	// resumes. If none exist, the draft proceeds with no resume context.
	ResumeID string
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

	// Upsert company + contact. On a cache hit (source=cache) we skip the
	// upsert so we don't overwrite the original source label (pattern/apollo/…).
	var companyID string
	if in.Company != "" {
		co, err := s.companies.GetOrCreateByName(ctx, in.Company, fRes.CompanyDomain, "heuristic")
		if err != nil {
			return nil, fmt.Errorf("company upsert: %w", err)
		}
		companyID = co.ID
	}

	var contact *contacts.Contact
	if fRes.Source == "cache" {
		contact, err = s.contacts.GetByLinkedInURL(ctx, in.LinkedInURL)
		if err != nil {
			return nil, fmt.Errorf("contact cache reload: %w", err)
		}
	} else {
		contact, err = s.contacts.UpsertByLinkedInURL(ctx, contacts.UpsertParams{
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
	}

	// Load user (need name/email + subscription tier for feature checks).
	u, err := s.users.GetByID(ctx, in.UserID)
	if err != nil {
		return nil, fmt.Errorf("load user: %w", err)
	}
	aiDraft := features.IsEnabled(u, features.AIDraftEmail)
	aiMatch := features.IsEnabled(u, features.AIResumeMatch)

	// Pick the resume:
	//   explicit ID  → use it
	//   else, one resume → auto-attach
	//   else, many resumes → LLM match (premium only), else first
	//   else, no resumes → skip
	var chosenResume *resumes.Resume
	if s.resumes != nil {
		if in.ResumeID != "" {
			r, err := s.resumes.GetForUser(ctx, in.ResumeID, in.UserID)
			if err != nil {
				return nil, fmt.Errorf("load resume: %w", err)
			}
			chosenResume = r
		} else {
			list, err := s.resumes.ListForUser(ctx, in.UserID)
			if err != nil {
				return nil, fmt.Errorf("list resumes: %w", err)
			}
			switch {
			case len(list) == 1:
				chosenResume = &list[0]
			case len(list) > 1 && aiMatch:
				cands := make([]llm.ResumeCandidate, 0, len(list))
				for _, r := range list {
					cands = append(cands, llm.ResumeCandidate{
						ID: r.ID, Label: r.Label, Text: truncate(r.ExtractedText, 2000),
					})
				}
				match, err := s.llm.MatchResume(ctx, llm.MatchResumeRequest{
					JobDescription: in.JobDescription,
					Resumes:        cands,
				})
				if err == nil && match != nil {
					for i, r := range list {
						if r.ID == match.ResumeID {
							chosenResume = &list[i]
							break
						}
					}
				}
			case len(list) > 1:
				// Free tier: default to the most recent (list is DESC).
				chosenResume = &list[0]
			}
		}
	}

	// LLM draft — premium only. Free users get an empty draft to write themselves.
	var draft *llm.Draft
	if aiDraft {
		draftReq := llm.DraftRequest{
			RecruiterName:     in.RecruiterName,
			RecruiterHeadline: in.RecruiterHeadline,
			Company:           in.Company,
			JobDescription:    in.JobDescription,
			SenderName:        firstNonEmpty(u.Name, u.Email),
			SenderEmail:       u.Email,
		}
		if chosenResume != nil {
			draftReq.ResumeText = chosenResume.ExtractedText
			draftReq.ResumeLabel = chosenResume.Label
		}
		draft, err = s.llm.DraftEmail(ctx, draftReq)
		if err != nil {
			return nil, fmt.Errorf("llm draft: %w", err)
		}
	} else {
		draft = &llm.Draft{Subject: "", Body: ""}
	}

	// Create outreach row (pending approval).
	var resumeIDPtr *string
	if chosenResume != nil {
		resumeIDPtr = &chosenResume.ID
	}
	o, err := s.outreach.Create(ctx, CreateParams{
		UserID:         in.UserID,
		ContactID:      contact.ID,
		ResumeID:       resumeIDPtr,
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
	if subject == "" || body == "" {
		return nil, ErrEmptyDraft
	}

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

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
