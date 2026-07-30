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
	"github.com/shubham/oneapply/backend/internal/resumes"
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

	companyRepo := companies.NewRepo(pool)
	contactRepo := contacts.NewRepo(pool)
	outreachRepo := outreach.NewRepo(pool)
	resumeRepo := resumes.NewRepo(pool)
	resumeStorage := resumes.NewLocalStorage("data/resumes")

	// Phase 3: real email cascade.
	emailFinder := finder.NewCascadeFinder(
		contactCacheAdapter{repo: contactRepo},
		finder.NewHeuristicDomainResolver(),
		finder.NewDefaultPatternGenerator(),
		finder.NewSMTPVerifier("oneapply.local", "postmaster@oneapply.local"),
		finder.NewApolloFinder(cfg.ApolloAPIKey),
	)
	if cfg.ApolloAPIKey == "" {
		slog.Warn("APOLLO_API_KEY not set — cascade will end at SMTP verification, no Apollo fallback")
	}

	// Phase 4: real LLM + real Gmail sender.
	var llmSvc llm.LLMService = llm.NewStubLLM()
	if cfg.AnthropicAPIKey != "" {
		llmSvc = llm.NewClaudeLLM(cfg.AnthropicAPIKey)
	} else {
		slog.Warn("ANTHROPIC_API_KEY not set — drafts will use the stubbed LLM")
	}

	tokenProvider := &auth.GmailTokenProvider{OAuth: oauthCfg, Users: userRepo, Cipher: cipher}
	var sender gmail.EmailSender = gmail.NewRealSender(tokenProvider)
	if cfg.GoogleClientID == "" || cfg.GoogleClientSecret == "" {
		slog.Warn("gmail sender falling back to stub — Google OAuth not configured")
		sender = gmail.NewStubSender()
	}

	outreachSvc := outreach.NewService(outreach.ServiceParams{
		Users:      userRepo,
		Companies:  companyRepo,
		Contacts:   contactRepo,
		Outreach:   outreachRepo,
		Resumes:    resumeRepo,
		Finder:     emailFinder,
		LLM:        llmSvc,
		Sender:     sender,
		DailyLimit: 3,
	})
	outreachHandler := httpapi.NewOutreachHandler(outreachSvc)
	resumeHandler := httpapi.NewResumeHandler(resumeRepo, resumeStorage)

	router := httpapi.NewRouter(httpapi.Deps{
		DB:       pool,
		Auth:     authSvc,
		Outreach: outreachHandler,
		Resumes:  resumeHandler,
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

// contactCacheAdapter bridges *contacts.Repo to finder.ContactCache so the
// finder package doesn't need to import contacts (keeps deps one-directional).
type contactCacheAdapter struct{ repo *contacts.Repo }

func (a contactCacheAdapter) LookupByLinkedInURL(ctx context.Context, url string) (*finder.CachedContact, error) {
	c, err := a.repo.GetByLinkedInURL(ctx, url)
	if err != nil {
		if errors.Is(err, contacts.ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &finder.CachedContact{
		Email:              c.Email,
		Source:             c.Source,
		VerificationStatus: c.VerificationStatus,
	}, nil
}
