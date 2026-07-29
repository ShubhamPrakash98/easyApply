# OneApply

Apollo.io, but for job seekers. Chrome extension + web dashboard that finds recruiter emails, drafts personalized outreach, sends via Gmail, and tracks replies.

See `docs/BRAINSTORM.md`, `docs/PRD.html`, `docs/workflow.html`, and `docs/plan.md`.

## Prerequisites

- Node 20+ and pnpm 9+
- Go 1.23+
- Docker (for Postgres) or a local Postgres 15+
- `golang-migrate` CLI: `brew install golang-migrate`

## First-time setup

```bash
cp .env.example .env
pnpm install
(cd backend && go mod tidy)
docker compose up -d postgres
make migrate-up   # or: migrate -database "$DATABASE_URL" -path backend/migrations up
```

## Dev

Three terminals:

```bash
# Terminal 1 — backend
cd backend && go run ./cmd/api

# Terminal 2 — dashboard
pnpm dev:dashboard   # http://localhost:5173

# Terminal 3 — extension
pnpm dev:extension   # builds to extension/dist, load unpacked in chrome://extensions
```

## Layout

```
backend/     Go API + workers
extension/   Chrome MV3 (React popup + content script)
dashboard/   React SPA (web app)
packages/
  ui/          shared React components
  api-client/  typed fetch client
docs/        design docs
migrations/   in backend/migrations
```

## Loading the extension

1. `pnpm build:extension`
2. Open `chrome://extensions`, enable Developer mode
3. Click "Load unpacked" → pick `extension/dist`
