package finder

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net"
	"net/smtp"
	"net/textproto"
	"strings"
	"time"
)

// SMTPVerifier probes candidate emails via a live SMTP conversation:
//   MX lookup → TCP dial :25 → HELO → MAIL FROM → RCPT TO → parse response.
//
// Caveats:
//   - Many ISPs and cloud providers block outbound port 25. If dial fails
//     consistently, you're behind such a network and need a paid verifier
//     (ZeroBounce/Hunter) — the EmailVerifier interface lets you swap.
//   - Google Workspace and Outlook often accept ALL RCPTs (catch-all) even
//     for non-existent addresses. We probe a randomized junk address first
//     and mark the domain risky if it accepts both.
//   - 4xx responses (greylist / temporary) are "unknown".
type SMTPVerifier struct {
	HelloName    string        // domain we announce in HELO/EHLO
	MailFrom     string        // MAIL FROM: address
	DialTimeout  time.Duration
	TotalTimeout time.Duration // whole conversation budget per email
}

func NewSMTPVerifier(helloName, mailFrom string) *SMTPVerifier {
	if helloName == "" {
		helloName = "oneapply.local"
	}
	if mailFrom == "" {
		mailFrom = "postmaster@" + helloName
	}
	return &SMTPVerifier{
		HelloName:    helloName,
		MailFrom:     mailFrom,
		DialTimeout:  3 * time.Second,
		TotalTimeout: 8 * time.Second,
	}
}

func (v *SMTPVerifier) Verify(ctx context.Context, email string) (VerificationResult, error) {
	domain := domainOf(email)
	if domain == "" {
		return VerificationResult{Status: "invalid", Reason: "malformed email"}, nil
	}

	mxHosts, err := lookupMX(ctx, domain, v.DialTimeout)
	if err != nil {
		return VerificationResult{Status: "unknown", Reason: "mx lookup: " + err.Error()}, nil
	}
	if len(mxHosts) == 0 {
		return VerificationResult{Status: "invalid", Reason: "no mx"}, nil
	}

	convoCtx, cancel := context.WithTimeout(ctx, v.TotalTimeout)
	defer cancel()

	var lastErr error
	for _, host := range mxHosts {
		res, err := v.probe(convoCtx, host, email)
		if err == nil {
			return res, nil
		}
		lastErr = err
		if convoCtx.Err() != nil {
			break
		}
	}
	reason := "smtp conversation failed"
	if lastErr != nil {
		reason = lastErr.Error()
	}
	return VerificationResult{Status: "unknown", Reason: reason}, nil
}

func (v *SMTPVerifier) probe(ctx context.Context, mxHost, email string) (VerificationResult, error) {
	dialer := net.Dialer{Timeout: v.DialTimeout}
	conn, err := dialer.DialContext(ctx, "tcp", mxHost+":25")
	if err != nil {
		return VerificationResult{}, err
	}
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}
	client, err := smtp.NewClient(conn, mxHost)
	if err != nil {
		_ = conn.Close()
		return VerificationResult{}, err
	}
	defer func() {
		_ = client.Quit()
		_ = conn.Close()
	}()

	if err := client.Hello(v.HelloName); err != nil {
		return VerificationResult{}, err
	}
	if err := client.Mail(v.MailFrom); err != nil {
		return VerificationResult{}, err
	}

	// Catch-all probe: RCPT a definitely-nonexistent local part first.
	fake := "zzz-" + randToken() + "@" + domainOf(email)
	fakeStatus := statusFromRCPT(client.Rcpt(fake))

	// Reset the transaction before the real RCPT (some servers require it).
	if err := client.Reset(); err == nil {
		_ = client.Mail(v.MailFrom)
	}
	realStatus := statusFromRCPT(client.Rcpt(email))

	switch realStatus {
	case "deliverable":
		if fakeStatus == "deliverable" {
			return VerificationResult{Status: "risky", Reason: "catch-all server"}, nil
		}
		return VerificationResult{Status: "deliverable"}, nil
	case "invalid":
		return VerificationResult{Status: "invalid"}, nil
	default:
		return VerificationResult{Status: "unknown", Reason: "server ambiguous"}, nil
	}
}

func statusFromRCPT(err error) string {
	if err == nil {
		return "deliverable"
	}
	var protoErr *textproto.Error
	if asErr, ok := err.(*textproto.Error); ok {
		protoErr = asErr
	}
	if protoErr != nil {
		switch {
		case protoErr.Code >= 500:
			return "invalid"
		case protoErr.Code >= 400:
			return "unknown"
		}
	}
	// Fallback string scan for weirdly-wrapped errors.
	msg := err.Error()
	switch {
	case strings.Contains(msg, "550"), strings.Contains(msg, "551"),
		strings.Contains(msg, "553"), strings.Contains(msg, "554"):
		return "invalid"
	case strings.Contains(msg, "421"), strings.Contains(msg, "450"),
		strings.Contains(msg, "451"), strings.Contains(msg, "452"):
		return "unknown"
	}
	return "unknown"
}

func domainOf(email string) string {
	i := strings.LastIndex(email, "@")
	if i < 0 {
		return ""
	}
	return email[i+1:]
}

func lookupMX(ctx context.Context, domain string, timeout time.Duration) ([]string, error) {
	resolver := &net.Resolver{}
	c, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	mxs, err := resolver.LookupMX(c, domain)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(mxs))
	for _, mx := range mxs {
		host := strings.TrimSuffix(mx.Host, ".")
		if host != "" {
			out = append(out, host)
		}
	}
	return out, nil
}

func randToken() string {
	buf := make([]byte, 6)
	_, _ = rand.Read(buf)
	return hex.EncodeToString(buf)
}
