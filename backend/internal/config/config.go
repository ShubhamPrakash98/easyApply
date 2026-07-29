package config

import (
	"errors"
	"os"
)

type Config struct {
	Port        string
	DatabaseURL string

	JWTSecret           string
	TokenEncryptionKey  string

	GoogleClientID     string
	GoogleClientSecret string
	GoogleRedirectURL  string

	AnthropicAPIKey string
	ApolloAPIKey    string
}

func Load() (*Config, error) {
	cfg := &Config{
		Port:               getenv("PORT", "8080"),
		DatabaseURL:        os.Getenv("DATABASE_URL"),
		JWTSecret:          os.Getenv("JWT_SECRET"),
		TokenEncryptionKey: os.Getenv("TOKEN_ENCRYPTION_KEY"),
		GoogleClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		GoogleClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		GoogleRedirectURL:  os.Getenv("GOOGLE_REDIRECT_URL"),
		AnthropicAPIKey:    os.Getenv("ANTHROPIC_API_KEY"),
		ApolloAPIKey:       os.Getenv("APOLLO_API_KEY"),
	}
	if cfg.DatabaseURL == "" {
		return nil, errors.New("DATABASE_URL is required")
	}
	return cfg, nil
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
