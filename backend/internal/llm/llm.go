package llm

import (
	"context"
	"fmt"
	"strings"
)

// LLMService produces email drafts (and later follow-ups + resume matches).
// Phase 2 uses StubLLM. Phase 4 lands the real Anthropic client.
type LLMService interface {
	DraftEmail(ctx context.Context, req DraftRequest) (*Draft, error)
}

type DraftRequest struct {
	RecruiterName    string
	RecruiterHeadline string
	Company          string
	JobDescription   string
	ResumeText       string // may be empty in Phase 2
	SenderName       string
}

type Draft struct {
	Subject string
	Body    string
}

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
(This is a stubbed draft. Real Claude-generated drafts land in Phase 4.)
`,
		orDefault(first, "there"),
		orDefault(req.RecruiterHeadline, "your work stood out"),
		role,
		orDefault(req.Company, "your team"),
		orDefault(req.SenderName, "Me"),
	))

	return &Draft{Subject: subject, Body: body}, nil
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
