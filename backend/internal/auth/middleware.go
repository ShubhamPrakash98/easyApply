package auth

import (
	"context"
	"net/http"
)

type ctxKey int

const userIDKey ctxKey = 1

func UserIDFromContext(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(userIDKey).(string)
	return v, ok
}

// Middleware requires a valid session cookie. On failure it returns 401
// and does not call the next handler.
func (s *Service) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID, err := s.readSessionUserID(r)
		if err != nil {
			writeJSONError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		ctx := context.WithValue(r.Context(), userIDKey, userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// Optional attaches a user_id to context if the cookie is valid, but does
// not require it. Use for endpoints where anon and authed both make sense.
func (s *Service) Optional(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if userID, err := s.readSessionUserID(r); err == nil {
			r = r.WithContext(context.WithValue(r.Context(), userIDKey, userID))
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Service) readSessionUserID(r *http.Request) (string, error) {
	c, err := r.Cookie(SessionCookieName)
	if err != nil {
		return "", err
	}
	return s.jwt.Verify(c.Value)
}
