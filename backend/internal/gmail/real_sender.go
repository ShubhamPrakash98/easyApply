package gmail

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"

	"golang.org/x/oauth2"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

// TokenSource resolves a user's Gmail-scoped access token. RealSender pulls
// one per Send by asking this collaborator (implemented in cmd/api/main.go).
// The concrete impl decrypts users.gmail_refresh_token_enc via auth.Cipher
// and hands golang.org/x/oauth2 a refreshing TokenSource.
type TokenSource interface {
	TokenSource(ctx context.Context, userID string) (oauth2.TokenSource, string, error)
	// returns (tokenSource, senderEmail, error)
}

// RealSender uses the Gmail API. Every call creates a short-lived
// gmail.Service bound to the user's TokenSource so refreshes work
// transparently.
type RealSender struct {
	Tokens TokenSource
}

func NewRealSender(t TokenSource) *RealSender {
	return &RealSender{Tokens: t}
}

var (
	ErrNoGmailToken = errors.New("user has no Gmail refresh token; sign in again")
)

func (s *RealSender) Send(ctx context.Context, userID string, e Email) (string, error) {
	svc, senderEmail, err := s.buildService(ctx, userID)
	if err != nil {
		return "", err
	}
	from := senderEmail
	if e.FromDisplayName != "" {
		from = fmt.Sprintf("%q <%s>", e.FromDisplayName, senderEmail)
	}
	raw := encodeRFC822(from, e.To, e.Subject, e.Body, "")
	msg := &gmail.Message{Raw: raw}
	sent, err := svc.Users.Messages.Send("me", msg).Context(ctx).Do()
	if err != nil {
		return "", fmt.Errorf("gmail send: %w", err)
	}
	return sent.ThreadId, nil
}

func (s *RealSender) SendReply(ctx context.Context, userID string, threadID string, e Email) error {
	svc, senderEmail, err := s.buildService(ctx, userID)
	if err != nil {
		return err
	}
	from := senderEmail
	if e.FromDisplayName != "" {
		from = fmt.Sprintf("%q <%s>", e.FromDisplayName, senderEmail)
	}
	// Gmail will thread the reply automatically when we pass ThreadId and
	// keep the subject with a "Re: " prefix (or unchanged if already prefixed).
	subject := e.Subject
	if !hasReplyPrefix(subject) {
		subject = "Re: " + subject
	}
	raw := encodeRFC822(from, e.To, subject, e.Body, "")
	msg := &gmail.Message{
		Raw:      raw,
		ThreadId: threadID,
	}
	if _, err := svc.Users.Messages.Send("me", msg).Context(ctx).Do(); err != nil {
		return fmt.Errorf("gmail send reply: %w", err)
	}
	return nil
}

func (s *RealSender) buildService(ctx context.Context, userID string) (*gmail.Service, string, error) {
	ts, senderEmail, err := s.Tokens.TokenSource(ctx, userID)
	if err != nil {
		return nil, "", err
	}
	svc, err := gmail.NewService(ctx, option.WithTokenSource(ts))
	if err != nil {
		return nil, "", fmt.Errorf("gmail service: %w", err)
	}
	return svc, senderEmail, nil
}

func encodeRFC822(from, to, subject, body, extraHeaders string) string {
	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=\"UTF-8\"\r\n%s\r\n%s",
		from, to, subject, extraHeaders, body)
	return base64.URLEncoding.EncodeToString([]byte(msg))
}

func hasReplyPrefix(subject string) bool {
	if len(subject) < 3 {
		return false
	}
	return subject[:3] == "Re:" || subject[:3] == "RE:" || subject[:3] == "re:"
}
