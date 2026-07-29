# OneApply — Implementation Plan

**Positioning:** Apollo.io for job seekers.
**MVP goal:** end-to-end flow across all 5 pillars, one real user (you), 3 outreaches/day.

---

## Guiding principles

- **Pillar-shaped phases.** Each phase delivers one pillar end-to-end (skeleton → real integration → surfaced in UI). No horizontal-then-vertical.
- **Interface-first.** Every phase starts by defining the Go interface. Stub implementation lands before the real one — so downstream work is never blocked on an external API.
- **Ship a vertical slice at the end of Phase 2.** By then you can log in, hit "Reach Out" on a LinkedIn profile, get a stubbed email + LLM draft, approve it, and see it in the dashboard. Every subsequent phase swaps a stub for a real integration.
- **No premature abstractions.** No SendGrid support, no OpenAI support, no team accounts. Interfaces exist for testability + one future swap, not five.

---

## Repo layout (monorepo, pnpm + Go workspaces)

```
oneapply/
├── backend/                     # Go
│   ├── cmd/api/main.go
│   ├── cmd/worker/main.go
│   ├── internal/
│   │   ├── http/                # Chi handlers, middleware
│   │   ├── auth/                # Google OAuth, JWT
│   │   ├── outreach/            # service + repo
│   │   ├── contacts/            # cascade orchestration
│   │   ├── finder/              # DomainResolver, PatternGenerator, Verifier, ApolloAdapter
│   │   ├── gmail/               # Gmail send + poll
│   │   ├── llm/                 # Claude client
│   │   ├── workers/             # gmail_poller, followup, verify_refresh
│   │   ├── notify/              # SSE hub + Chrome push
│   │   ├── db/                  # pgx pool, sqlc queries, migrations
│   │   └── config/
│   ├── migrations/              # golang-migrate SQL files
│   ├── go.mod
│   └── Dockerfile
├── extension/                   # Manifest V3
│   ├── manifest.json
│   ├── src/
│   │   ├── background.ts        # service worker: SSE listener, notifications
│   │   ├── content.ts           # LinkedIn DOM reader
│   │   ├── popup/               # React popup entry
│   │   └── options/             # Settings entry
│   └── vite.config.ts
├── dashboard/                   # Web SPA
│   ├── src/
│   │   ├── main.tsx
│   │   ├── routes/              # /outreach, /inbox, /analytics, /contacts, /resumes, /settings
│   │   ├── api/                 # generated from packages/api-client
│   │   └── components/
│   └── vite.config.ts
├── packages/
│   ├── ui/                      # shared React components (Button, Table, Toast, ...)
│   └── api-client/              # typed fetch client, shared by extension + dashboard
├── docs/
│   ├── BRAINSTORM.md
│   ├── PRD.html
│   ├── workflow.html
│   └── plan.md
├── pnpm-workspace.yaml
└── README.md
```

---

## Phase 0 — Scaffolding (Day 1)

**Goal:** repo boots, hello-world backend + extension + dashboard, one migration runs.

- [ ] Init monorepo. pnpm workspace + Go workspace.
- [ ] `backend/`: Chi router with `GET /health`. Postgres via Docker Compose. `golang-migrate` wired.
- [ ] `extension/`: MV3 manifest, popup renders "Hello". Content script injects a "Reach Out" button on `linkedin.com/in/*`.
- [ ] `dashboard/`: Vite + React + Tailwind. Empty routes rendered.
- [ ] `packages/ui`: one shared `Button` component consumed by both.
- [ ] `packages/api-client`: typed `apiFetch(path, opts)` reading base URL from env.
- [ ] First migration: `users` table only.
- [ ] Basic README with dev commands.

**Ship:** you can run `pnpm dev` (extension) + `pnpm dev` (dashboard) + `go run ./cmd/api` and see all three alive.

---

## Phase 1 — Auth (Day 2–3)

**Goal:** users can log in with Google. Extension and dashboard share the session. Gmail scope is pre-requested (so we don't need a second consent screen later).

- [ ] `POST /api/auth/google` → OAuth redirect with scopes: `openid email profile https://www.googleapis.com/auth/gmail.send https://www.googleapis.com/auth/gmail.readonly`.
- [ ] `/api/auth/google/callback` → exchange code, encrypt refresh_token, store user, issue JWT.
- [ ] `GET /api/auth/me` → returns user + quota.
- [ ] Dashboard: `/login` page with "Sign in with Google" button, protected routes redirect if no session.
- [ ] Extension: "Connect Google" button in popup → opens dashboard OAuth in a new tab → dashboard writes JWT to `chrome.storage` via a redirect back to the extension.
- [ ] `authMiddleware` on all `/api/*` routes.

**Ship:** log in from dashboard, popup shows "Signed in as you@…".

---

## Phase 2 — Vertical slice (Day 4–6) — Pillars 1+2+3 stubbed end-to-end

**Goal:** the full flow works with stubs. This is the "prove it hangs together" milestone. No real Apollo, no real Gmail send, no real LLM — just deterministic stubs.

- [ ] Migrations: `companies`, `contacts`, `resumes`, `outreach`, `replies`, `notifications`, `config`.
- [ ] Interfaces defined in Go: `EmailFinder`, `DomainResolver`, `PatternGenerator`, `EmailVerifier`, `EmailSender`, `LLMService`, `OutreachRepo`, `Notifier`.
- [ ] Stub implementations: `stub.EmailFinder` returns `first.last@company.com`, `stub.EmailSender` writes to console + returns fake thread ID, `stub.LLMService` returns a canned draft.
- [ ] `POST /api/outreach/draft`: runs cascade (stubbed), calls LLM (stubbed), creates outreach row with `status=pending_approval`, returns draft.
- [ ] `POST /api/outreach/:id/approve`: sends via `EmailSender` (stubbed), flips to `sent`.
- [ ] `GET /api/outreach` with basic pagination.
- [ ] Extension popup: full flow — click "Reach Out" → JD paste → draft view → approve → toast.
- [ ] Dashboard `/outreach`: TanStack Table with the sent row visible.

**Ship:** one full loop end-to-end with stubs. Everything after this is swapping stubs for real integrations.

---

## Phase 3 — Pillar 1 real: Email extraction cascade (Day 7–9)

**Goal:** find real emails via cache → patterns+verify → Apollo.

- [ ] `finder/domain_resolver.go`: heuristic (`normalize company name → lowercase, strip Inc/Corp/Ltd, .com`), fallback to a Clearbit-style lookup. Cache in `companies`.
- [ ] `finder/pattern_generator.go`: 8 formats. Split name → first/last, handle single-name, middle initials.
- [ ] `finder/smtp_verifier.go`: MX lookup, TCP dial port 25, HELO/MAIL FROM/RCPT TO, parse response code. Timeout budget: 3s per candidate. Concurrency: verify 3 candidates in parallel, stop on first deliverable. Handle Google Workspace / catch-all → `risky`.
- [ ] `finder/apollo_adapter.go`: Apollo API client. Rate limit + retry with backoff.
- [ ] `finder/cascade.go`: orchestrator — cache → domain → patterns → verify → Apollo → verify → error.
- [ ] Wire `CascadeFinder` in `main.go`. Swap stub.
- [ ] Contact detail page in dashboard shows `source` + `verification_status` badges.

**Ship:** hit "Reach Out" on a real LinkedIn profile → get a real (verified) email.

---

## Phase 4 — Pillar 2 real: LLM drafting + Gmail send (Day 10–12)

**Goal:** real personalized drafts sent from user's Gmail.

- [ ] `llm/claude.go`: Anthropic SDK client. Structured output (tool use) constrained to `{subject, body}`. Prompt caching on the system prompt.
- [ ] Prompt: recruiter context + JD + resume text + user profile. Explicit "no fluff", "concrete reason to email this person", "under 150 words".
- [ ] `llm/resume_matcher.go`: given JD + all user resumes → pick best. Simple embedding cosine OR just prompt the LLM.
- [ ] Resume upload endpoint + PDF text extraction (`ledongthuc/pdf` for MVP).
- [ ] `gmail/sender.go`: send using user's refresh_token via Gmail API. Store `gmail_thread_id`.
- [ ] Extension popup draft view: editable subject + body, confidence badge (from cascade), resume attachment preview.
- [ ] Rate-limit middleware: 3 approves/day per user (configurable).

**Ship:** approve draft → real email lands in recruiter's inbox from your Gmail.

---

## Phase 5 — Pillar 3 real: Dashboard depth (Day 13–15)

**Goal:** the dashboard actually looks like Apollo.

- [ ] `/outreach` table: server-side filters (status, company, date range), search, sort, pagination. Row click → drawer with full email + resume link + thread.
- [ ] `/contacts`: cached recruiter list with source/verification chips, last outreach date, "re-verify" button.
- [ ] `/resumes`: upload/label/delete, shown with usage counts.
- [ ] `/settings`: Gmail connection status, rate limit slider, follow-up cadence, notification prefs, sign out.
- [ ] `/analytics`: sent/replied/reply-rate cards, sends-over-time line chart (Recharts), top-5 responding companies table.
- [ ] Extension "Recent outreach" panel: last 5 rows, click opens dashboard.

**Ship:** dashboard is a genuinely useful tool even without follow-ups/notifications.

---

## Phase 6 — Pillar 5 real: Reply detection + notifications (Day 16–18)

**Goal:** you learn about replies within 5 minutes without checking Gmail.

- [ ] `workers/gmail_poller.go`: every 5 min, for each user with pending outreach, list messages on tracked `thread_id`s, insert new inbound messages into `replies`, update outreach `status=replied`, insert `notifications`, publish on SSE hub.
- [ ] `notify/sse_hub.go`: per-user channels, `GET /api/stream` handler with heartbeat.
- [ ] Dashboard subscribes to SSE, invalidates TanStack Query caches on reply event, shows toast.
- [ ] Extension service worker subscribes to SSE (or polls `/api/notifications` every 60s as a fallback since MV3 SWs are short-lived), fires `chrome.notifications.create` and updates badge count.
- [ ] Dashboard `/inbox`: thread view (original + replies), "mark as read".

**Ship:** send an email to yourself, reply from another account, get a browser notification within 5 min.

---

## Phase 7 — Pillar 4 real: Follow-up worker (Day 19–20)

**Goal:** silence gets 3 nudges automatically.

- [ ] `workers/followup.go`: hourly cron. SELECT outreach WHERE status IN (sent, followed_up) AND last_contact_at < now() - '3 days' AND follow_up_count < 3.
- [ ] For each: re-check Gmail thread first (belt + suspenders vs. the poller). If reply → mark replied, skip.
- [ ] `llm/followup_prompt.go`: distinct prompt (references original, escalates gently, follow-up number 1/2/3 shifts tone).
- [ ] `gmail/reply_in_thread.go`: send with `In-Reply-To` + `References` headers on same thread.
- [ ] Update outreach: `follow_up_count++`, `last_followed_up_at`, `status=followed_up`. If count hits 3 → `no_response`.
- [ ] Dashboard settings: cadence + max count sliders.

**Ship:** an unanswered email triggers 3 follow-ups at 3/6/9 days, then goes quiet.

---

## Phase 8 — Hardening (Day 21–24)

- [ ] Encrypted refresh tokens (AES-GCM with key from env).
- [ ] Errors surfaced to user via toast, structured logging (slog) on backend.
- [ ] Sentry or bugsnag on all three surfaces.
- [ ] Rate limits enforced at middleware + at LLM/Apollo adapters (belt + suspenders).
- [ ] Cancel-pending cron: outreach in `pending_approval` for > 24h → `cancelled`.
- [ ] Verification refresh cron: nightly re-verify contacts older than 90 days.
- [ ] Basic e2e test with Playwright: login → capture → draft → approve → sent row visible.
- [ ] CAN-SPAM footer template (opt-out line, mailing address) auto-appended by sender.
- [ ] Deploy: Railway or Fly. Postgres on same. Dashboard served same-origin under `/dashboard/*`.

**Ship:** it's yours to use full-time.

---

## Non-goals for MVP (defer)

- Stripe / payments
- Team workspaces
- Own email finder (replace Apollo)
- Sentiment analysis on replies
- Sequences / multi-step campaigns
- Multi-platform beyond LinkedIn
- Auto resume generation
- Chrome Web Store submission (sideload while iterating)

---

## Open questions to resolve before Phase 3

1. **SMTP verification from a cheap VPS gets its IP blocked by big providers.** Do we start with SMTP verifier and add ZeroBounce/Hunter as a fallback in Phase 3, or use a paid verifier from day one? (My recommendation: start SMTP + ZeroBounce fallback; a $10/mo credit pack covers ~1500 verifications.)
2. **Chrome Web Store vs sideload.** MV3 requires a paid dev account and review turnaround. Ship as sideload-only for MVP; publish after Phase 8.
3. **Where to host resumes.** S3, Cloudflare R2, or Postgres BYTEA? Suggest R2 for cost + no egress fees.

---

## Success criteria for MVP done

- You (Shubham) use OneApply for your own outreach for one full week.
- 15+ real emails sent through the system.
- ≥1 reply lands in your dashboard inbox with notification.
- ≥1 follow-up sent automatically.
- Dashboard analytics show reply rate.
- No manual DB pokes needed for a normal week of use.
