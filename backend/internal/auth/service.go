package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"golang.org/x/oauth2"

	"github.com/shubham/oneapply/backend/internal/features"
	"github.com/shubham/oneapply/backend/internal/users"
)

const (
	SessionCookieName = "oneapply_session"
	stateCookieName   = "oneapply_oauth_state"
	stateCookieTTL    = 10 * time.Minute
	sessionCookieTTL  = 30 * 24 * time.Hour
)

type Service struct {
	oauth        *oauth2.Config
	users        *users.Repo
	jwt          *JWTSigner
	cipher       *Cipher
	dashboardURL string
	cookieSecure bool
}

type ServiceParams struct {
	OAuth        *oauth2.Config
	Users        *users.Repo
	JWT          *JWTSigner
	Cipher       *Cipher
	DashboardURL string
	CookieSecure bool
}

func NewService(p ServiceParams) *Service {
	return &Service{
		oauth:        p.OAuth,
		users:        p.Users,
		jwt:          p.JWT,
		cipher:       p.Cipher,
		dashboardURL: p.DashboardURL,
		cookieSecure: p.CookieSecure,
	}
}

// StartLogin generates a state, sets it in a short-lived cookie,
// and redirects the browser to Google's consent screen.
func (s *Service) StartLogin(w http.ResponseWriter, r *http.Request) {
	if s.oauth.ClientID == "" || s.oauth.ClientSecret == "" {
		http.Error(w, "google oauth not configured (set GOOGLE_CLIENT_ID and GOOGLE_CLIENT_SECRET)", http.StatusServiceUnavailable)
		return
	}
	state, err := randomState(24)
	if err != nil {
		http.Error(w, "state gen failed", http.StatusInternalServerError)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     stateCookieName,
		Value:    state,
		Path:     "/",
		HttpOnly: true,
		Secure:   s.cookieSecure,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(stateCookieTTL),
	})
	// AccessTypeOffline + prompt=consent guarantees Google returns a refresh_token,
	// even on re-consent. Without prompt=consent, Google only issues refresh_token
	// on the very first grant.
	url := s.oauth.AuthCodeURL(state,
		oauth2.AccessTypeOffline,
		oauth2.SetAuthURLParam("prompt", "consent"),
	)
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

// Callback exchanges the code, fetches userinfo, upserts the user,
// encrypts + stores the refresh token, sets a session cookie, and
// redirects to the dashboard.
func (s *Service) Callback(w http.ResponseWriter, r *http.Request) {
	stateCookie, err := r.Cookie(stateCookieName)
	if err != nil {
		http.Error(w, "missing state cookie", http.StatusBadRequest)
		return
	}
	// Clear the state cookie now that we've read it.
	http.SetCookie(w, &http.Cookie{
		Name:     stateCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   s.cookieSecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})

	if r.URL.Query().Get("state") != stateCookie.Value {
		http.Error(w, "state mismatch", http.StatusBadRequest)
		return
	}
	if errParam := r.URL.Query().Get("error"); errParam != "" {
		http.Redirect(w, r, s.dashboardURL+"/login?error="+errParam, http.StatusTemporaryRedirect)
		return
	}
	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "missing code", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	tok, err := s.oauth.Exchange(ctx, code)
	if err != nil {
		slog.Error("oauth code exchange failed", "err", err)
		http.Error(w, "code exchange: "+err.Error(), http.StatusBadGateway)
		return
	}
	info, err := FetchGoogleUserInfo(ctx, s.oauth, tok)
	if err != nil {
		slog.Error("google userinfo fetch failed", "err", err)
		http.Error(w, "userinfo: "+err.Error(), http.StatusBadGateway)
		return
	}
	if info.Sub == "" || info.Email == "" {
		slog.Error("google userinfo missing sub/email", "sub_empty", info.Sub == "", "email_empty", info.Email == "")
		http.Error(w, "userinfo missing sub/email", http.StatusBadGateway)
		return
	}

	var encRefresh []byte
	if tok.RefreshToken != "" {
		encRefresh, err = s.cipher.Encrypt([]byte(tok.RefreshToken))
		if err != nil {
			http.Error(w, "encrypt refresh: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}

	u, err := s.users.Upsert(ctx, users.UpsertParams{
		GoogleSub:            info.Sub,
		Email:                info.Email,
		Name:                 info.Name,
		GmailRefreshTokenEnc: encRefresh,
	})
	if err != nil {
		http.Error(w, "user upsert: "+err.Error(), http.StatusInternalServerError)
		return
	}

	sessionJWT, err := s.jwt.Sign(u.ID)
	if err != nil {
		http.Error(w, "jwt sign: "+err.Error(), http.StatusInternalServerError)
		return
	}
	s.setSessionCookie(w, sessionJWT)

	http.Redirect(w, r, s.dashboardURL+"/outreach", http.StatusTemporaryRedirect)
}

// Me returns the current user (requires the auth middleware to have populated context).
func (s *Service) Me(w http.ResponseWriter, r *http.Request) {
	userID, ok := UserIDFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	u, err := s.users.GetByID(r.Context(), userID)
	if err != nil {
		if errors.Is(err, users.ErrNotFound) {
			writeJSONError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id":                u.ID,
		"email":             u.Email,
		"name":              u.Name,
		"trial_ends_at":     u.TrialEndsAt,
		"gmail_connected":   len(u.GmailRefreshTokenEnc) > 0,
		"subscription_tier": u.SubscriptionTier,
		"features":          features.Snapshot(u),
	})
}

func (s *Service) Logout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   s.cookieSecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
	w.WriteHeader(http.StatusNoContent)
}

func (s *Service) setSessionCookie(w http.ResponseWriter, jwt string) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    jwt,
		Path:     "/",
		HttpOnly: true,
		Secure:   s.cookieSecure,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(sessionCookieTTL),
	})
}

func randomState(nBytes int) (string, error) {
	buf := make([]byte, nBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeJSONError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
