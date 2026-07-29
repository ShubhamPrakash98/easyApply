package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const sessionTTL = 30 * 24 * time.Hour

type JWTSigner struct {
	secret []byte
}

func NewJWTSigner(secret []byte) *JWTSigner {
	return &JWTSigner{secret: secret}
}

type Claims struct {
	UserID string `json:"sub"`
	jwt.RegisteredClaims
}

func (s *JWTSigner) Sign(userID string) (string, error) {
	now := time.Now()
	claims := &Claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(sessionTTL)),
			Issuer:    "oneapply",
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return tok.SignedString(s.secret)
}

func (s *JWTSigner) Verify(tokenStr string) (string, error) {
	tok, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return s.secret, nil
	})
	if err != nil {
		return "", err
	}
	claims, ok := tok.Claims.(*Claims)
	if !ok || !tok.Valid {
		return "", errors.New("invalid token")
	}
	return claims.UserID, nil
}
