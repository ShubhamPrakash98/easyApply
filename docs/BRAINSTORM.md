# OneApply - Brainstorming & Pre-PRD Document

**Date:** May 13, 2026
**Last Updated:** July 29, 2026
**Status:** Decisions Finalized — Ready for Technical PRD

**Positioning:** *Apollo.io, but for job seekers instead of sales reps.*

---

## 0. The Five Pillars

Every feature in the MVP must serve one of these five pillars. Anything else is out of scope.

1. **Email extraction from profile** — Given a LinkedIn profile, get the recruiter's email.
   - Step A: Check our PostgreSQL cache (`contacts` table).
   - Step B: If miss → generate candidate emails from company domain patterns (`first.last@company.com`, `first@company.com`, `flast@company.com`, etc.) and verify each with an SMTP/verifier API.
   - Step C: If no verified pattern → fall back to Apollo API.
   - Step D: Cache the winning email + source (`cache | pattern | apollo`) in `contacts`.
2. **Automated email drafting with JD context → user approval → send** — LLM drafts a personalized email using the recruiter's profile + user's pasted JD + selected resume. The draft is **shown to the user for approval/edit** before Gmail sends it. No silent sending.
3. **Email tracking** — Every outreach is logged in PostgreSQL and surfaced in the **web dashboard** (see Pillar 0.b) with status: `pending_approval | sent | replied | followed_up | no_response`.
4. **Follow-up mailing on no response** — Background worker sends up to 3 AI-generated follow-ups every 3 days on the same Gmail thread. Stops immediately on reply. Marks `no_response` after the 3rd.
5. **Notification system on response** — Gmail polling worker detects replies → updates status → notifies the user via (a) browser notification, (b) extension badge count, (c) dashboard inbox.

### 0.b Surfaces — Where the pillars are exposed

The five pillars are delivered via two UI surfaces built from **one shared React codebase**:

- **Chrome extension** (Manifest V3, React) — the *capture + approve + notify* surface. Runs on LinkedIn profile pages. Popup shows: profile info, JD paste box, resume picker, draft preview + approve/edit/send, recent outreach (last 5), reply badge.
- **Web dashboard** (React SPA at `dashboard.oneapply.com`) — the *monitor + configure* surface. This is the "Apollo dashboard, but for job seekers". Contains:
  - **Outreach table** — sortable/filterable by company, status, date, resume used. Search.
  - **Reply inbox** — all recruiter replies threaded, with the original email as context.
  - **Analytics** — sent count, reply rate, no-response rate, top responding companies, sends over time.
  - **Contacts** — every recruiter we've cached, with email source (`cache/pattern/apollo`) + verification status.
  - **Resume manager** — upload/label/delete resume variants.
  - **Settings** — Gmail connection, rate limit, follow-up cadence, subscription/quota, notification preferences.

**Google Sheets is dropped from MVP scope.** The dashboard is the single source of truth for tracking.

---

## 1. Problem Statement

Job seekers (especially in tech) spend hours manually:
- Searching for recruiters on LinkedIn and other platforms
- Using tools like Apollo.io to extract recruiter email addresses
- Writing and sending templated cold emails to each recruiter
- Waiting for responses with no tracking system
- Losing track of outreach status across multiple recruiters and companies

This process is repetitive, time-consuming, and error-prone. There is no single tool that unifies recruiter outreach, resume tailoring, and response tracking.

---

## 2. Proposed Solution

A Chrome extension that automates the recruiter outreach workflow:

1. User visits a recruiter's LinkedIn profile
2. Clicks "Reach Out" via the extension
3. Extension reads visible profile data, sends it to backend
4. Backend checks local DB for recruiter email → if not found, calls Apollo API → caches result in DB
5. AI drafts a personalized email based on recruiter context + user's JD (pasted manually)
6. AI selects the best resume variant from user's uploaded set
7. Email is sent via the user's connected Gmail
8. Outreach is logged to a Google Sheet (MVP tracking) with status, company, date, response
9. User gets notified on all replies (positive-only filtering comes later)

---

## 3. Competitive Landscape

### 3.1 Job Application Automation Tools

| Tool | What It Does | Model | Gap |
|------|-------------|-------|-----|
| **Simplify** | Auto-fills job applications via Chrome extension | Free + Premium | No outreach/cold email; purely form-filling |
| **LazyApply** | Bulk-applies to jobs on LinkedIn/Indeed/ZipRecruiter | Subscription (~$50-100/mo) | Spray-and-pray; no recruiter outreach |
| **Sonara** | AI agent that finds and applies to jobs autonomously | Subscription | Limited user control; no direct recruiter contact |
| **JobRight.ai** | AI job matching + resume tailoring | Freemium | No cold outreach; focused on matching only |
| **Hirect** | Chat-based hiring connecting candidates to hiring managers | Free | Limited to its own ecosystem; no LinkedIn integration |

### 3.2 Email Extraction Tools

| Tool | Accuracy | Model | Notes |
|------|----------|-------|-------|
| **Apollo.io** | ~70-85% | Freemium (50 credits/mo free) | Largest B2B database; Chrome extension |
| **Hunter.io** | ~60-80% | Freemium | Email finder/verifier by domain |
| **Lusha** | ~70-85% | Credit-based | Direct-dial + email enrichment |
| **Snov.io** | ~65-80% | Freemium | Email finder + drip campaigns |
| **RocketReach** | ~70-85% | Subscription | Professional contact info |

**Key insight:** Accuracy ranges 60-85%. ~15-40% of extracted emails may bounce, which damages sender reputation.

### 3.3 Job Tracking Tools

| Tool | What It Does | Gap |
|------|-------------|-----|
| **Teal** | Job tracking + resume builder | No outreach automation |
| **Huntr** | Job tracking + some tailoring | No outreach automation |

### 3.4 Cold Email / Outreach Tools (Sales-Focused)

| Tool | What It Does | Gap |
|------|-------------|-----|
| **Lemlist** | Cold email sequences + personalization | Not designed for job seekers |
| **Instantly** | Cold email at scale + warm-up | Sales-focused, not job-seeker-friendly |
| **Woodpecker** | B2B cold email automation | No resume matching, no recruiter context |

### 3.5 Market Whitespace

**Nobody combines:** find recruiter email + tailor resume + send personalized cold email + track responses — in one tool for job seekers.

Current workaround users cobble together: Apollo (emails) + Mailmeteor (outreach) + Teal (tracking) + ChatGPT (resume tailoring). This is fragmented and painful.

---

## 4. Identified Gaps & Risks

### 4.1 Critical Risks

#### Risk 1: LinkedIn TOS Violation
- LinkedIn TOS (Section 8.2) **explicitly prohibits** scraping, automated access, and data extraction
- LinkedIn actively detects automation: rate-limiting, CAPTCHAs, account restrictions/bans
- **hiQ v. LinkedIn (2022):** Court ruled public profile scraping isn't a CFAA violation, but LinkedIn can still enforce TOS contractually
- **Impact:** User's LinkedIn account gets restricted/banned *while job hunting* — catastrophic for the user
- **Mitigation:** Extension only reads visible page data (name, headline, company) — no automated LinkedIn actions. Email enrichment is handled by Apollo API, not LinkedIn scraping.

#### Risk 2: Email Deliverability & Spam
- Gmail personal accounts: **500 emails/day limit**
- Google Workspace accounts: **2,000 emails/day limit**
- Sending templated cold emails at scale triggers spam filters fast
- If Gmail flags the account, the user's *personal email* gets burned
- New accounts sending cold email get flagged quickly — need 2-4 week warm-up
- **Mitigation:** Hard cap at 3 sends/day (MVP), personalize heavily via LLM, SPF/DKIM/DMARC setup, keep bounce rate under 5%

#### Risk 3: Email Tracking & Privacy Laws
- **CAN-SPAM Act (US):** Requires unsubscribe mechanism, physical address, no deceptive headers
- **GDPR (EU):** Cold emailing without consent carries regulatory risk
- **CCPA (California):** Similar consent requirements
- **Impact:** Adding tracking pixels and auto-sending at scale could classify this as commercial email
- **Mitigation:** Include opt-out mechanism, proper disclosures. Research whether job-seeker outreach qualifies as "transactional" vs "commercial" email.

#### Risk 4: Email Extraction Accuracy
- Apollo API has 70-85% accuracy
- 15-40% bounce rate destroys sender reputation
- **Mitigation:** Email verification step before sending (use a verification API), handle bounces gracefully, cache verified emails in DB to avoid re-verification

---

## 5. Decisions Made

### Product Decisions

| Question | Decision | Notes |
|----------|----------|-------|
| **Primary user** | Any job seeker | Keep UI simple, no jargon |
| **Core value prop** | Sending *better*, more personalized emails | LLM quality matters — invest in good prompts |
| **MVP recruiter discovery** | Manual — user visits recruiter profile | No automated recruiter finding in v1 |
| **Business model** | Subscription-based | 3 requests/day, 3-day free trial. Configurable via DB. |
| **Dashboard (MVP)** | Web dashboard (React SPA) — Apollo-style monitoring | Outreach table, reply inbox, analytics, contacts, resumes, settings. Single React codebase shared with the extension popup. Google Sheets is not in MVP. |
| **Action label** | "Reach Out" not "Apply Job" | Cold email ≠ application. Be transparent. |
| **Notifications (MVP)** | Notify on ALL replies | Positive-only sentiment filtering comes in v2 |
| **Follow-ups (MVP)** | Auto follow-ups: 3 follow-ups every 3 days, then mark as no_response | AI-generated, sent on same Gmail thread |
| **Send flow** | Draft → user approves/edits → send | No silent sending. User always sees the draft before Gmail fires. Approval is a required step. |
| **Notifications on reply** | Browser push notification via Chrome extension + extension badge count | Extension listens for server-sent events (SSE) or polls `/api/notifications`. Reply preview shown inline. |
| **Platform (MVP)** | LinkedIn only | Multi-platform based on demand |
| **JD source (MVP)** | User pastes JD manually | Auto-extract from LinkedIn job posting as a bonus if user is on that page |
| **Resume handling (MVP)** | User uploads 2-3 variants tagged by role type. AI recommends which to attach. | No auto-generation in v1 |

### Technical Decisions

| Question | Decision | Notes |
|----------|----------|-------|
| **Backend** | Go (Golang) | Fast, cheap to host, goroutines for background work |
| **Database** | PostgreSQL (via pgx) | JSONB for flexible data, rock solid |
| **Chrome Extension** | Manifest V3 + vanilla JS or React | Reads visible LinkedIn page data |
| **Email enrichment** | Cascade: DB → Pattern+Verify → Apollo | Pattern generation is free; Apollo credits are expensive. Try patterns first. |
| **Email verifier** | SMTP handshake for MVP (MX lookup + RCPT probe), ZeroBounce/Hunter as swap-in | Behind `EmailVerifier` interface. Cache verification result with 90-day TTL. |
| **Domain resolution** | Company name → domain via heuristic (`Acme Corp → acme.com`) with Clearbit lookup as fallback | Behind `DomainResolver` interface. Cache in `companies` table. |
| **Frontend framework** | React + Vite + TypeScript, TailwindCSS | Shared codebase → two builds: extension bundle + web SPA. |
| **Dashboard hosting** | Static SPA served by Go backend (`/dashboard/*` route) OR Vercel — TBD | Same-origin with API simplifies auth; Vercel gives free CDN. Decide during Phase 4. |
| **Tracking (MVP)** | PostgreSQL + web dashboard | Google Sheets removed from MVP. |
| **Email sending** | Gmail API (OAuth 2.0) | Read + send access for tracking |
| **LLM** | Claude API or OpenAI API | Email drafts + resume matching |
| **Hosting** | Single VPS (Railway/Fly.io/DO) | Go binary + PostgreSQL, $5-15/mo |

---

## 6. Architecture

### 6.1 Layered Architecture Principle

All external service integrations MUST be behind interfaces so they can be swapped without touching business logic.

```
┌──────────────────────┐         ┌──────────────────────┐
│  Chrome Extension    │         │   Web Dashboard      │
│  (Manifest V3, React)│         │   (React SPA)        │
│                      │         │                      │
│  - LinkedIn scrape   │         │  - Outreach table    │
│  - JD paste          │         │  - Reply inbox       │
│  - Draft approve/edit│         │  - Analytics         │
│  - Recent + badge    │         │  - Contacts          │
│                      │         │  - Resumes           │
│                      │         │  - Settings          │
└──────────┬───────────┘         └──────────┬───────────┘
           │  REST + SSE                    │  REST + SSE
           │  (Google OAuth JWT)            │  (Google OAuth JWT)
           └──────────────┬─────────────────┘
                          ▼
      ┌────────────────────────────────────────┐
      │            Go Backend (Chi)            │
      │                                        │
      │  Handlers → Services → Interfaces      │
      │                          │             │
      │  ┌─────────────────┐    │             │
      │  │ Workers         │    │             │
      │  │  - Gmail poll   │    │             │
      │  │  - Follow-ups   │    │             │
      │  │  - Verify cache │    │             │
      │  └─────────────────┘    │             │
      └──────────────────────────┼─────────────┘
                                 │
    ┌──────────┬─────────────┬──┴────────┬───────────┐
    ▼          ▼             ▼           ▼           ▼
 ┌──────┐ ┌─────────┐ ┌───────────┐ ┌────────┐ ┌────────┐
 │Cache │ │Domain   │ │Pattern    │ │Verifier│ │Apollo  │
 │(pg)  │ │Resolver │ │Generator  │ │(SMTP)  │ │(fallbk)│
 └──┬───┘ └────┬────┘ └─────┬─────┘ └───┬────┘ └───┬────┘
    │          │            │           │          │
    └──────────┴──────┬─────┴───────────┴──────────┘
                     ▼
              ┌─────────────┐    ┌─────────┐    ┌─────────┐
              │ PostgreSQL  │    │ Gmail   │    │ Claude  │
              │             │    │ API     │    │ / OpenAI│
              │ users       │    └─────────┘    └─────────┘
              │ contacts    │
              │ companies   │
              │ resumes     │
              │ outreach    │
              │ replies     │
              │ config      │
              └─────────────┘
```

### 6.2 Email Finder — DB → Pattern+Verify → Apollo Cascade

Apollo credits are expensive and rate-limited. Pattern generation + SMTP verification is essentially free. So we try patterns first and only fall back to Apollo when patterns fail.

```
Backend receives: { name, company, linkedin_url }
        │
        ▼
┌─ STEP A: Check PostgreSQL contacts table ─┐
│   SELECT email FROM contacts               │
│   WHERE linkedin_url = ? OR                │
│   (name = ? AND company = ?)               │
└────────────┬───────────────────────────────┘
             │
      ┌──────┴──────┐
      │  Found?     │
      └─── YES ─────▶ Return cached email  [source='cache']
             │
             NO
             │
             ▼
┌─ STEP B: Resolve company → domain ─────────┐
│   - Company name → domain (Clearbit-style   │
│     lookup OR heuristic from company name)  │
│   - Cache domain in companies table         │
└────────────┬───────────────────────────────┘
             │
             ▼
┌─ STEP C: Generate candidate emails ────────┐
│   From "Jane Smith" @ acme.com generate:    │
│   - jane.smith@acme.com                     │
│   - jsmith@acme.com                         │
│   - jane@acme.com                           │
│   - smith.jane@acme.com                     │
│   - j.smith@acme.com                        │
│   - jane_smith@acme.com                     │
└────────────┬───────────────────────────────┘
             │
             ▼
┌─ STEP D: Verify each via EmailVerifier ────┐
│   SMTP handshake / ZeroBounce / Hunter      │
│   verifier. Stop at first "deliverable".    │
└────────────┬───────────────────────────────┘
             │
      ┌──────┴──────┐
      │ Verified?   │
      └─── YES ─────▶ Cache + return  [source='pattern']
             │
             NO
             │
             ▼
┌─ STEP E: Apollo fallback ──────────────────┐
│   Call Apollo API with name + company.      │
│   Verify the returned email too.            │
└────────────┬───────────────────────────────┘
             │
      ┌──────┴──────┐
      │ Found?      │
      └─── YES ─────▶ Cache + return  [source='apollo']
             │
             NO
             ▼
      Return error: email_not_found
```

**Contacts table gains `source` (`cache | pattern | apollo`), `verification_status` (`deliverable | risky | unverified`), and `verified_at`.** This lets us prefer high-confidence emails and re-verify old ones on a schedule.

### 6.3 Swappable Layer Design (Go Interfaces)

```go
// EmailFinder orchestrates the cascade: cache → pattern+verify → Apollo.
type EmailFinder interface {
    FindEmail(ctx context.Context, req FindEmailRequest) (*Contact, error)
}

// CascadeFinder is the composite implementation used at runtime.
type CascadeFinder struct {
    cache      ContactCache
    domains    DomainResolver     // company name → domain
    patterns   PatternGenerator   // {name, domain} → []candidate emails
    verifier   EmailVerifier      // candidate → deliverable? risky? invalid?
    apollo     EmailFinder        // last-resort external lookup
}

// PatternGenerator produces candidate emails from a name + domain.
type PatternGenerator interface {
    Generate(name PersonName, domain string) []string
}

// EmailVerifier validates a single candidate address.
type EmailVerifier interface {
    Verify(ctx context.Context, email string) (VerificationResult, error)
}

// VerificationResult { Status: deliverable|risky|invalid|unknown; Reason: string }

// DomainResolver maps a company name to its primary email domain.
type DomainResolver interface {
    Resolve(ctx context.Context, companyName string) (domain string, err error)
}

// ApolloFinder is one implementation of EmailFinder, used as the last stage.
type ApolloFinder struct { apiKey string; client *http.Client }

// EmailSender — Gmail today, SendGrid tomorrow.
type EmailSender interface {
    Send(ctx context.Context, email Email) (threadID string, err error)
    SendReply(ctx context.Context, threadID string, email Email) error
    GetReplies(ctx context.Context, threadID string) ([]Reply, error)
}

// LLMService — Claude today, OpenAI tomorrow.
type LLMService interface {
    DraftEmail(ctx context.Context, req DraftRequest) (*EmailDraft, error)
    DraftFollowUp(ctx context.Context, req FollowUpRequest) (*EmailDraft, error)
    MatchResume(ctx context.Context, jd string, resumes []Resume) (*ResumeMatch, error)
}

// OutreachRepo — persistence for tracking (Pillar 3). Backs the dashboard.
type OutreachRepo interface {
    Create(ctx context.Context, entry OutreachEntry) (id string, err error)
    UpdateStatus(ctx context.Context, id string, status string) error
    RecordReply(ctx context.Context, id string, reply Reply) error
    List(ctx context.Context, filter OutreachFilter) ([]OutreachEntry, error)
    Analytics(ctx context.Context, userID string, window TimeWindow) (Metrics, error)
}

// Notifier — browser push today, email digest tomorrow.
type Notifier interface {
    NotifyReply(ctx context.Context, userID string, reply Reply) error
}
```

**Key principle:** every external dependency is behind an interface. To swap Apollo for our own finder, we replace one field in `CascadeFinder`. To swap the verifier from SMTP to ZeroBounce, we replace `EmailVerifier`. Zero changes to business logic.

---

## 7. Dashboard — Apollo-Style Monitoring UI

The dashboard is where users **live**. It's the primary surface for pillars 3 (tracking) and 5 (notifications), and the configuration surface for pillars 2 (approval settings) and 4 (follow-up cadence).

### 7.1 Screens

| Screen | Purpose | Key elements |
|--------|---------|--------------|
| **Outreach** | Full history table | Sortable columns: date, recruiter, company, role, status, resume, reply-at. Filters: status, date range, company. Search. Row click → detail drawer with full email + thread. |
| **Inbox** | All recruiter replies | Threaded conversation view. Original email + reply(ies). "Mark as read", "Snooze", "Move to no-response". |
| **Analytics** | Aggregate metrics | Sent, replied, reply rate %, no-response %, follow-ups sent, top responding companies, sends-per-day chart. |
| **Contacts** | Cached recruiters | Recruiter name, company, email, source (`cache/pattern/apollo`), verification status, last outreach. |
| **Resumes** | Resume variants | Upload, label (e.g., "Backend Go", "Frontend React"), delete. Shows which JDs matched which resume. |
| **Settings** | Config | Gmail connection status, rate limit, follow-up cadence, subscription tier + quota used, notification prefs, sign out. |

### 7.2 Auth Model

- Users authenticate via **Google OAuth** (same flow as Gmail connection — one consent screen covers both dashboard login and email sending scope).
- Dashboard and extension share the same backend session (JWT in HTTP-only cookie for the web app; JWT in extension storage for the extension). Same user record.

### 7.3 Tech

- **React + Vite + TypeScript**, **TailwindCSS**, **TanStack Query** for server state, **TanStack Table** for the outreach grid.
- Shared component library between extension popup and web app (`packages/ui`).
- Realtime reply notifications via **Server-Sent Events** from Go backend (simpler than WebSockets for one-way push).

---

## 8. MVP Scope — What We're Building

### In Scope (MVP) — grouped by pillar

**Pillar 1 — Email extraction cascade**
- Chrome extension (Manifest V3) reads LinkedIn recruiter profiles (name, headline, company, linkedin_url)
- `contacts` table cache
- Domain resolver (company → domain)
- Pattern generator (6+ common formats)
- SMTP-based email verifier (MX + RCPT probe)
- Apollo API fallback
- Cache with source + verification status

**Pillar 2 — Draft → Approve → Send**
- LLM integration (Claude API primary)
- Resume selector (user uploads 2-3 variants, LLM picks best per JD)
- Draft preview in extension popup with edit + approve + send
- Gmail API (OAuth 2.0) for sending

**Pillar 3 — Tracking + Dashboard**
- PostgreSQL: users, contacts, companies, resumes, outreach, config
- Web dashboard (React SPA): outreach table, inbox, analytics, contacts, resumes, settings
- Extension popup: recent outreach summary
- Google OAuth login for dashboard

**Pillar 4 — Follow-ups**
- Background worker (goroutine, hourly tick)
- Up to 3 AI-generated follow-ups every 3 days
- Same Gmail thread
- Auto-stop on reply detection

**Pillar 5 — Reply notifications**
- Gmail polling worker (every 5 min)
- Update outreach status on reply
- Browser push notification via extension
- Extension badge count
- SSE stream to dashboard inbox

**Global**
- Rate limiting: 3 requests/day (configurable via `config` table)
- 3-day free trial
- Manual JD input
- LinkedIn only

### Out of Scope (v2+)
- Google Sheets export
- Automated recruiter discovery
- Own email extraction system (replaces Apollo)
- Sentiment classification on replies
- Smart notifications (positive-only filtering)
- Multi-platform support (Indeed, Glassdoor, etc.)
- Resume auto-generation
- Email warm-up features
- Team/multi-tenant workspaces
- Payment integration (Stripe) — trial-only in MVP

---

## 9. Competitive Landscape Summary

### 9.1 Our Differentiation

| Feature | OneApply (MVP) | Simplify | LazyApply | Teal | Apollo |
|---------|--------------|----------|-----------|------|--------|
| Cold email to recruiters | Yes | No | No | No | No (data only) |
| AI-personalized emails | Yes | No | No | No | No |
| Resume matching to JD | Yes | No | No | Partial | No |
| Outreach tracking dashboard | Yes (Apollo-style) | No | Basic | Yes | Yes (sales) |
| Reply monitoring + notifications | Yes | No | No | No | No |
| Auto follow-ups | Yes (3× / 3-day) | No | No | No | Yes (sales) |
| Approval gate before send | Yes | N/A | No | N/A | N/A |
| Rate-limited (anti-spam) | Yes (3/day) | N/A | No | N/A | N/A |

---

## 10. Next Steps

- [x] Resolve product decisions
- [x] Resolve technical decisions
- [x] Align on MVP scope
- [x] Define the five pillars
- [x] Decide dashboard is in MVP (replaces Google Sheets)
- [x] Choose tech stack (Go + PostgreSQL + React + Gmail API + Apollo fallback)
- [ ] Write formal Technical PRD (API specs, DB schema, endpoints) — see PRD.html
- [ ] Design database schema (typed, with indexes + FKs)
- [ ] Set up monorepo: `backend/`, `extension/`, `dashboard/`, `packages/ui`, `packages/api-client`
- [ ] Wireframes for extension popup + dashboard screens
- [ ] Apollo API integration behind `EmailFinder` interface
- [ ] SMTP verifier + pattern generator + domain resolver
- [ ] Gmail API OAuth flow
- [ ] Claude API integration behind `LLMService` interface
- [ ] Background workers (Gmail poll, follow-up, verifier refresh)
- [ ] Deploy: Go binary + Postgres on Railway/Fly, dashboard SPA on same-origin or Vercel

---

*This document captures all brainstorming and decisions. Next step: formal Technical PRD with implementation details.*
