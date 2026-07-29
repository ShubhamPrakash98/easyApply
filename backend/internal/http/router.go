package http

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shubham/oneapply/backend/internal/auth"
)

type Deps struct {
	DB   *pgxpool.Pool
	Auth *auth.Service
}

func NewRouter(deps Deps) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))

	r.Use(cors.Handler(cors.Options{
		AllowOriginFunc: func(r *http.Request, origin string) bool {
			// Dashboard (dev) + any Chrome extension origin.
			return origin == "http://localhost:5173" ||
				strings.HasPrefix(origin, "chrome-extension://")
		},
		AllowedMethods:   []string{"GET", "POST", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Authorization", "Content-Type"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	r.Get("/health", healthHandler(deps))

	r.Route("/api", func(r chi.Router) {
		r.Get("/health", healthHandler(deps))

		// Public auth routes (OAuth redirect + callback).
		r.Get("/auth/google", deps.Auth.StartLogin)
		r.Get("/auth/google/callback", deps.Auth.Callback)

		// Protected routes.
		r.Group(func(r chi.Router) {
			r.Use(deps.Auth.Middleware)
			r.Get("/auth/me", deps.Auth.Me)
			r.Post("/auth/logout", deps.Auth.Logout)
			// Phase 2+: /outreach, /contacts, /resumes, /analytics, /notifications
		})
	})

	return r
}

func healthHandler(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{"status": "ok", "time": time.Now().UTC()}
		if deps.DB != nil {
			if err := deps.DB.Ping(r.Context()); err != nil {
				resp["db"] = "down"
				resp["db_error"] = err.Error()
				writeJSON(w, http.StatusServiceUnavailable, resp)
				return
			}
			resp["db"] = "up"
		}
		writeJSON(w, http.StatusOK, resp)
	}
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
