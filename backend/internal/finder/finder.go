package finder

import (
	"context"
	"strings"
)

// EmailFinder is the entry point to Pillar 1 (email extraction).
// Runtime implementation is a CascadeFinder that composes:
//   cache lookup → domain resolve → pattern generate → verify → Apollo fallback.
// Phase 2 uses StubFinder which returns a deterministic first.last@company.com.
type EmailFinder interface {
	FindEmail(ctx context.Context, req FindEmailRequest) (*Result, error)
}

type FindEmailRequest struct {
	Name        string
	Company     string
	LinkedInURL string
}

type Result struct {
	Email              string
	Source             string // "cache" | "pattern" | "apollo" | "stub"
	VerificationStatus string // "deliverable" | "risky" | "invalid" | "unknown"
	CompanyDomain      string
}

// DomainResolver — Phase 3 implementation (real). Interface lives here for
// callers to depend on ahead of time.
type DomainResolver interface {
	Resolve(ctx context.Context, companyName string) (string, error)
}

// PatternGenerator produces candidate emails.
type PatternGenerator interface {
	Generate(fullName, domain string) []string
}

// VerificationResult from a real EmailVerifier.
type VerificationResult struct {
	Status string // "deliverable" | "risky" | "invalid" | "unknown"
	Reason string
}

type EmailVerifier interface {
	Verify(ctx context.Context, email string) (VerificationResult, error)
}

// StubFinder is a placeholder used until Phase 3 lands the real cascade.
type StubFinder struct{}

func NewStubFinder() *StubFinder { return &StubFinder{} }

func (StubFinder) FindEmail(_ context.Context, req FindEmailRequest) (*Result, error) {
	first, last := splitName(req.Name)
	domain := heuristicDomain(req.Company)
	local := first
	if last != "" {
		local = first + "." + last
	}
	if local == "" {
		local = "recruiter"
	}
	return &Result{
		Email:              local + "@" + domain,
		Source:             "stub",
		VerificationStatus: "unknown",
		CompanyDomain:      domain,
	}, nil
}

func splitName(full string) (first, last string) {
	full = strings.TrimSpace(full)
	if full == "" {
		return "", ""
	}
	parts := strings.Fields(full)
	first = strings.ToLower(parts[0])
	if len(parts) > 1 {
		last = strings.ToLower(parts[len(parts)-1])
	}
	return
}

// heuristicDomain is a best-effort guess used by the stub. The real
// DomainResolver (Phase 3) will replace this with MX checks + Clearbit.
func heuristicDomain(company string) string {
	c := strings.ToLower(strings.TrimSpace(company))
	if c == "" {
		return "example.com"
	}
	for _, suffix := range []string{" inc", " inc.", " corp", " corporation", " ltd", " ltd.", " llc", " gmbh", ",", "."} {
		c = strings.TrimSuffix(c, suffix)
	}
	c = strings.ReplaceAll(c, " ", "")
	return c + ".com"
}
