package gmail

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
)

// EmailSender is Pillar 2's send surface (and Pillar 5's polling surface via
// GetReplies which arrives in Phase 6). Phase 2 uses StubSender.
type EmailSender interface {
	Send(ctx context.Context, userID string, email Email) (threadID string, err error)
	SendReply(ctx context.Context, userID string, threadID string, email Email) error
}

type Email struct {
	To      string
	Subject string
	Body    string
	// ReplyTo, Cc, Attachments — added in Phase 4.
}

type StubSender struct{}

func NewStubSender() *StubSender { return &StubSender{} }

func (StubSender) Send(_ context.Context, userID string, e Email) (string, error) {
	threadID := "stub-" + randomHex(8)
	slog.Info("stub email send",
		"user_id", userID,
		"to", e.To,
		"subject", e.Subject,
		"body_bytes", len(e.Body),
		"thread_id", threadID,
	)
	return threadID, nil
}

func (s StubSender) SendReply(ctx context.Context, userID string, threadID string, e Email) error {
	slog.Info("stub email reply",
		"user_id", userID,
		"thread_id", threadID,
		"to", e.To,
		"subject", e.Subject,
	)
	return nil
}

func randomHex(n int) string {
	buf := make([]byte, n)
	_, _ = rand.Read(buf)
	return hex.EncodeToString(buf)
}
