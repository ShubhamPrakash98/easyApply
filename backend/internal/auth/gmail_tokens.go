package auth

import (
	"context"
	"errors"
	"time"

	"golang.org/x/oauth2"

	"github.com/shubham/oneapply/backend/internal/users"
)

// GmailTokenProvider satisfies gmail.TokenSource without importing that
// package (structural interface match). It loads the user's encrypted
// refresh_token, decrypts it, and hands back an auto-refreshing token
// source bound to our Google OAuth client.
type GmailTokenProvider struct {
	OAuth  *oauth2.Config
	Users  *users.Repo
	Cipher *Cipher
}

var ErrNoGmailToken = errors.New("user has no Gmail refresh token; sign in again")

func (p *GmailTokenProvider) TokenSource(ctx context.Context, userID string) (oauth2.TokenSource, string, error) {
	u, err := p.Users.GetByID(ctx, userID)
	if err != nil {
		return nil, "", err
	}
	if len(u.GmailRefreshTokenEnc) == 0 {
		return nil, "", ErrNoGmailToken
	}
	refreshBytes, err := p.Cipher.Decrypt(u.GmailRefreshTokenEnc)
	if err != nil {
		return nil, "", err
	}
	tok := &oauth2.Token{
		RefreshToken: string(refreshBytes),
		Expiry:       time.Now().Add(-time.Hour), // force a refresh on first use
	}
	return p.OAuth.TokenSource(ctx, tok), u.Email, nil
}
