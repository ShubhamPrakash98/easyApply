# backend

Go API for OneApply. Chi router, pgx pool, layered services behind interfaces.

## Run locally

```bash
docker compose up -d postgres       # from repo root
go mod tidy
migrate -database "$DATABASE_URL" -path migrations up
go run ./cmd/api
curl localhost:8080/health
```

## Layout

- `cmd/api` — HTTP server
- `cmd/worker` — background workers (Gmail poll, follow-ups) — populated in Phase 6+
- `internal/config` — env config loader
- `internal/db` — pgx pool
- `internal/http` — Chi router + handlers
- `internal/auth` — Google OAuth + JWT (Phase 1)
- `internal/outreach` — outreach service + repo (Phase 2)
- `internal/contacts` — contact repo (Phase 2)
- `internal/finder` — email cascade: domain resolver, patterns, verifier, Apollo (Phase 3)
- `internal/llm` — Claude client (Phase 4)
- `internal/gmail` — Gmail send + poll (Phase 4/6)
- `internal/workers` — background jobs (Phase 6/7)
- `internal/notify` — SSE hub + push (Phase 6)
- `migrations/` — golang-migrate SQL
