package finder

import (
	"context"
	"errors"
	"net"
	"strings"
	"time"
)

// HeuristicDomainResolver is our Phase 3 implementation: normalize the
// company name (drop suffixes, lowercase, strip punctuation, remove spaces)
// and append `.com`. Optionally validate that the domain has MX records —
// if it doesn't, we return an error so the caller can fall through to Apollo.
type HeuristicDomainResolver struct {
	CheckMX bool
	Timeout time.Duration
}

func NewHeuristicDomainResolver() *HeuristicDomainResolver {
	return &HeuristicDomainResolver{
		CheckMX: true,
		Timeout: 2 * time.Second,
	}
}

var ErrDomainUnresolvable = errors.New("could not resolve domain")

// junkSuffixes are stripped from the tail of a company name before we
// squash it into a domain guess.
var junkSuffixes = []string{
	" inc.", " inc",
	" corp.", " corp", " corporation",
	" co.", " co",
	" ltd.", " ltd",
	" llc.", " llc",
	" plc.", " plc",
	" gmbh",
	" s.a.", " sa", " s.a", " sarl",
	" the", // trailing "The" is rare but seen
}

func (r *HeuristicDomainResolver) Resolve(ctx context.Context, companyName string) (string, error) {
	guess := guessDomain(companyName)
	if guess == "" {
		return "", ErrDomainUnresolvable
	}
	if !r.CheckMX {
		return guess, nil
	}
	if hasMX(ctx, guess, r.Timeout) {
		return guess, nil
	}
	// Try .io as a fallback for tech companies where the .com is squatted.
	base := strings.TrimSuffix(guess, ".com")
	for _, tld := range []string{".io", ".ai", ".co"} {
		alt := base + tld
		if hasMX(ctx, alt, r.Timeout) {
			return alt, nil
		}
	}
	return "", ErrDomainUnresolvable
}

// guessDomain applies the normalization heuristics.
func guessDomain(name string) string {
	n := strings.ToLower(strings.TrimSpace(name))
	if n == "" {
		return ""
	}
	for _, s := range junkSuffixes {
		n = strings.TrimSuffix(n, s)
	}
	// Drop punctuation.
	n = strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z':
			return r
		case r >= '0' && r <= '9':
			return r
		case r == ' ' || r == '-':
			return r
		default:
			return -1
		}
	}, n)
	n = strings.ReplaceAll(n, " ", "")
	n = strings.ReplaceAll(n, "-", "")
	if n == "" {
		return ""
	}
	return n + ".com"
}

func hasMX(ctx context.Context, domain string, timeout time.Duration) bool {
	resolver := &net.Resolver{}
	c, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	mx, err := resolver.LookupMX(c, domain)
	if err != nil {
		return false
	}
	return len(mx) > 0
}
