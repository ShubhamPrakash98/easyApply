package finder

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"
)

// ContactCache reads previously-cached emails. The write side lives in
// internal/contacts (the outreach service calls Upsert after we return).
type ContactCache interface {
	LookupByLinkedInURL(ctx context.Context, linkedinURL string) (*CachedContact, error)
}

type CachedContact struct {
	Email              string
	Source             string
	VerificationStatus string
	CompanyDomain      string
}

// CascadeFinder is the runtime EmailFinder: cache → domain → patterns →
// verify → Apollo. Each stage is behind an interface so we can swap
// implementations without touching this orchestrator.
type CascadeFinder struct {
	Cache    ContactCache
	Domains  DomainResolver
	Patterns PatternGenerator
	Verifier EmailVerifier
	Apollo   EmailFinder

	// PerCandidateTimeout caps a single verify call.
	PerCandidateTimeout time.Duration
	// MaxConcurrentVerify caps parallel verifier calls per FindEmail invocation.
	MaxConcurrentVerify int
}

func NewCascadeFinder(cache ContactCache, domains DomainResolver, patterns PatternGenerator, verifier EmailVerifier, apollo EmailFinder) *CascadeFinder {
	return &CascadeFinder{
		Cache:               cache,
		Domains:             domains,
		Patterns:            patterns,
		Verifier:            verifier,
		Apollo:              apollo,
		PerCandidateTimeout: 4 * time.Second,
		MaxConcurrentVerify: 3,
	}
}

func (c *CascadeFinder) FindEmail(ctx context.Context, req FindEmailRequest) (*Result, error) {
	// Stage A — cache
	if c.Cache != nil && req.LinkedInURL != "" {
		cached, err := c.Cache.LookupByLinkedInURL(ctx, req.LinkedInURL)
		if err == nil && cached != nil && cached.Email != "" && cached.VerificationStatus != "invalid" {
			slog.Info("finder cache hit", "linkedin_url", req.LinkedInURL, "email", cached.Email)
			return &Result{
				Email:              cached.Email,
				Source:             "cache",
				VerificationStatus: cached.VerificationStatus,
				CompanyDomain:      cached.CompanyDomain,
			}, nil
		}
	}

	// Stage B — domain
	var domain string
	if c.Domains != nil && req.Company != "" {
		if d, err := c.Domains.Resolve(ctx, req.Company); err == nil {
			domain = d
		} else {
			slog.Debug("domain resolve failed", "company", req.Company, "err", err)
		}
	}

	// Stage C + D — patterns + verify
	if domain != "" && c.Patterns != nil && c.Verifier != nil {
		candidates := c.Patterns.Generate(req.Name, domain)
		if len(candidates) > 0 {
			slog.Info("finder trying patterns", "count", len(candidates), "domain", domain)
			if res := c.verifyFirstDeliverable(ctx, candidates); res != nil {
				res.CompanyDomain = domain
				return res, nil
			}
		}
	}

	// Stage E — Apollo fallback
	if c.Apollo != nil {
		res, err := c.Apollo.FindEmail(ctx, req)
		if err == nil && res != nil && res.Email != "" {
			return res, nil
		}
		if err != nil && !errors.Is(err, ErrEmailNotFound) {
			slog.Warn("apollo lookup errored", "err", err)
		}
	}

	return nil, nil // caller treats nil result as email_not_found
}

// verifyFirstDeliverable runs verifier calls in parallel (up to
// MaxConcurrentVerify at a time) and returns as soon as one comes back
// "deliverable" — cancelling the rest. If none are deliverable, returns
// the first "risky" result if any, else nil.
func (c *CascadeFinder) verifyFirstDeliverable(ctx context.Context, candidates []string) *Result {
	if c.MaxConcurrentVerify <= 0 {
		c.MaxConcurrentVerify = 3
	}

	type outcome struct {
		email  string
		result VerificationResult
	}

	convoCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	sem := make(chan struct{}, c.MaxConcurrentVerify)
	resCh := make(chan outcome, len(candidates))
	var wg sync.WaitGroup

	for _, email := range candidates {
		wg.Add(1)
		go func(email string) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-convoCtx.Done():
				return
			}
			if convoCtx.Err() != nil {
				return
			}
			cctx, ccancel := context.WithTimeout(convoCtx, c.PerCandidateTimeout)
			defer ccancel()
			res, _ := c.Verifier.Verify(cctx, email)
			resCh <- outcome{email: email, result: res}
		}(email)
	}

	go func() { wg.Wait(); close(resCh) }()

	var firstRisky *outcome
	for o := range resCh {
		switch o.result.Status {
		case "deliverable":
			cancel()
			// Drain remaining goroutines so we don't leak.
			go func() {
				for range resCh {
				}
			}()
			return &Result{
				Email:              o.email,
				Source:             "pattern",
				VerificationStatus: "deliverable",
			}
		case "risky":
			if firstRisky == nil {
				dup := o
				firstRisky = &dup
			}
		}
	}
	if firstRisky != nil {
		return &Result{
			Email:              firstRisky.email,
			Source:             "pattern",
			VerificationStatus: "risky",
		}
	}
	return nil
}

