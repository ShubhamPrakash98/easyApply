package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"github.com/shubham/oneapply/backend/internal/auth"
	"github.com/shubham/oneapply/backend/internal/companies"
	"github.com/shubham/oneapply/backend/internal/config"
	"github.com/shubham/oneapply/backend/internal/contacts"
	"github.com/shubham/oneapply/backend/internal/db"
	"github.com/shubham/oneapply/backend/internal/finder"
	"github.com/shubham/oneapply/backend/internal/gmail"
	httpapi "github.com/shubham/oneapply/backend/internal/http"
	"github.com/shubham/oneapply/backend/internal/llm"
	"github.com/shubham/oneapply/backend/internal/outreach"
	"github.com/shubham/oneapply/backend/internal/users"
)

func main() {
	_ = godotenv.Load("../.env", ".env")

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		slog.Error("config load failed", "err", err)
		os.Exit(1)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("db pool init failed", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	cipher, err := auth.NewCipher(cfg.TokenEncryptionKey)
	if err != nil {
		slog.Error("cipher init failed", "err", err)
		os.Exit(1)
	}

	userRepo := users.NewRepo(pool)
	oauthCfg := auth.NewGoogleOAuthConfig(cfg.GoogleClientID, cfg.GoogleClientSecret, cfg.GoogleRedirectURL)
	authSvc := auth.NewService(auth.ServiceParams{
		OAuth:        oauthCfg,
		Users:        userRepo,
		JWT:          auth.NewJWTSigner(cfg.JWTSecret),
		Cipher:       cipher,
		DashboardURL: cfg.DashboardURL,
		CookieSecure: cfg.CookieSecure,
	})

	if cfg.GoogleClientID == "" || cfg.GoogleClientSecret == "" {
		slog.Warn("google oauth not configured — /api/auth/google will return 503 until GOOGLE_CLIENT_ID/SECRET are set")
	}

	// Phase 2: outreach service with stubbed integrations.
	companyRepo := companies.NewRepo(pool)
	contactRepo := contacts.NewRepo(pool)
	outreachRepo := outreach.NewRepo(pool)
	outreachSvc := outreach.NewService(outreach.ServiceParams{
		Users:      userRepo,
		Companies:  companyRepo,
		Contacts:   contactRepo,
		Outreach:   outreachRepo,
		Finder:     finder.NewStubFinder(),
		LLM:        llm.NewStubLLM(),
		Sender:     gmail.NewStubSender(),
		DailyLimit: 3,
	})
	outreachHandler := httpapi.NewOutreachHandler(outreachSvc)

	router := httpapi.NewRouter(httpapi.Deps{
		DB:       pool,
		Auth:     authSvc,
		Outreach: outreachHandler,
	})

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		slog.Info("api listening", "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server crashed", "err", err)
			cancel()
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("graceful shutdown failed", "err", err)
	}
}
