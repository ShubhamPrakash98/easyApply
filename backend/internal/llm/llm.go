package llm

import (
	"context"
	"fmt"
	"strings"
)

// LLMService is the abstraction the outreach service depends on. Phase 4
// swaps StubLLM for ClaudeLLM. Both must satisfy this interface.
type LLMService interface {
	DraftEmail(ctx context.Context, req DraftRequest) (*Draft, error)
	MatchResume(ctx context.Context, req MatchResumeRequest) (*ResumeMatch, error)
}

// ---- Draft ----

type DraftRequest struct {
	RecruiterName     string
	RecruiterHeadline string
	Company           string
	JobDescription    string
	ResumeText        string // extracted PDF text of the chosen resume; may be empty
	ResumeLabel       string // e.g. "backend-go"
	SenderName        string
	SenderEmail       string
}

type Draft struct {
	Subject string
	Body    string
}

// ---- Resume match ----

type ResumeCandidate struct {
	ID    string
	Label string
	Text  string // first ~2000 chars is plenty for matching
}

type MatchResumeRequest struct {
	JobDescription string
	Resumes        []ResumeCandidate
}

type ResumeMatch struct {
	ResumeID  string
	Rationale string
}

// ---- Stub used in Phase 2/3 (fallback / tests) ----

type StubLLM struct{}

func NewStubLLM() *StubLLM { return &StubLLM{} }

func (StubLLM) DraftEmail(_ context.Context, req DraftRequest) (*Draft, error) {
	role := firstLine(req.JobDescription, 60)
	if role == "" {
		role = "the role you're hiring for"
	}
	subject := fmt.Sprintf("Interested in %s at %s", role, orDefault(req.Company, "your team"))
	first := firstName(req.RecruiterName)
	body := strings.TrimSpace(fmt.Sprintf(`Hi %s,

I came across your profile and wanted to reach out — %s.

I'd love to learn more about %s at %s and share my background. Would you have a few minutes for a quick chat this or next week?

Thanks,
%s

--
(Stubbed draft. Real Claude drafts land in Phase 4.)
`,
		orDefault(first, "there"),
		orDefault(req.RecruiterHeadline, "your work stood out"),
		role,
		orDefault(req.Company, "your team"),
		orDefault(req.SenderName, "Me"),
	))
	return &Draft{Subject: subject, Body: body}, nil
}

func (StubLLM) MatchResume(_ context.Context, req MatchResumeRequest) (*ResumeMatch, error) {
	if len(req.Resumes) == 0 {
		return nil, nil
	}
	return &ResumeMatch{ResumeID: req.Resumes[0].ID, Rationale: "stub picked first"}, nil
}

func firstName(full string) string {
	full = strings.TrimSpace(full)
	if full == "" {
		return ""
	}
	return strings.Split(full, " ")[0]
}

func orDefault(s, def string) string {
	if strings.TrimSpace(s) == "" {
		return def
	}
	return s
}

func firstLine(s string, max int) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if i := strings.IndexAny(s, "\n\r."); i > 0 {
		s = s[:i]
	}
	s = strings.TrimSpace(s)
	if len(s) > max {
		s = s[:max]
	}
	return s
}
