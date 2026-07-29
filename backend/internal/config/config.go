package config

import (
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	Port        string
	DatabaseURL string

	JWTSecret          []byte
	TokenEncryptionKey [32]byte

	GoogleClientID     string
	GoogleClientSecret string
	GoogleRedirectURL  string

	DashboardURL string
	CookieSecure bool

	AnthropicAPIKey string
	ApolloAPIKey    string
}

func Load() (*Config, error) {
	cfg := &Config{
		Port:               getenv("PORT", "8080"),
		DatabaseURL:        os.Getenv("DATABASE_URL"),
		JWTSecret:          []byte(os.Getenv("JWT_SECRET")),
		GoogleClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		GoogleClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		GoogleRedirectURL:  getenv("GOOGLE_REDIRECT_URL", "http://localhost:8080/api/auth/google/callback"),
		DashboardURL:       getenv("DASHBOARD_URL", "http://localhost:5173"),
		AnthropicAPIKey:    os.Getenv("ANTHROPIC_API_KEY"),
		ApolloAPIKey:       os.Getenv("APOLLO_API_KEY"),
	}

	if cfg.DatabaseURL == "" {
		return nil, errors.New("DATABASE_URL is required")
	}
	if len(cfg.JWTSecret) < 16 {
		return nil, errors.New("JWT_SECRET must be at least 16 chars")
	}

	rawKey := os.Getenv("TOKEN_ENCRYPTION_KEY")
	if rawKey == "" {
		return nil, errors.New("TOKEN_ENCRYPTION_KEY is required (32 bytes hex, 64 chars)")
	}
	keyBytes, err := hex.DecodeString(rawKey)
	if err != nil {
		return nil, fmt.Errorf("TOKEN_ENCRYPTION_KEY hex decode: %w", err)
	}
	if len(keyBytes) != 32 {
		return nil, fmt.Errorf("TOKEN_ENCRYPTION_KEY must decode to 32 bytes, got %d", len(keyBytes))
	}
	copy(cfg.TokenEncryptionKey[:], keyBytes)

	if v := os.Getenv("COOKIE_SECURE"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return nil, fmt.Errorf("COOKIE_SECURE: %w", err)
		}
		cfg.CookieSecure = b
	}

	return cfg, nil
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
