-- Phase 2 tables: everything the outreach flow touches.

CREATE TABLE companies (
    id           UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name         TEXT NOT NULL,
    domain       TEXT,
    resolved_via TEXT CHECK (resolved_via IN ('heuristic', 'clearbit', 'manual')),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX companies_name_lower_idx ON companies (LOWER(name));
CREATE INDEX companies_domain_idx ON companies (domain);

CREATE TABLE contacts (
    id                  UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name                TEXT NOT NULL,
    company_id          UUID REFERENCES companies(id) ON DELETE SET NULL,
    email               CITEXT,
    linkedin_url        TEXT,
    source              TEXT NOT NULL CHECK (source IN ('cache', 'pattern', 'apollo', 'manual', 'stub')),
    verification_status TEXT NOT NULL DEFAULT 'unknown' CHECK (verification_status IN ('deliverable', 'risky', 'invalid', 'unknown')),
    verified_at         TIMESTAMPTZ,
    fetched_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX contacts_linkedin_url_uidx ON contacts (linkedin_url) WHERE linkedin_url IS NOT NULL;
CREATE INDEX contacts_name_company_idx ON contacts (LOWER(name), company_id);
CREATE INDEX contacts_email_idx ON contacts (email);

CREATE TABLE resumes (
    id             UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id        UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    label          TEXT NOT NULL,
    storage_url    TEXT NOT NULL,
    extracted_text TEXT,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX resumes_user_id_idx ON resumes (user_id);

CREATE TABLE outreach (
    id                   UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id              UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    contact_id           UUID NOT NULL REFERENCES contacts(id) ON DELETE RESTRICT,
    resume_id            UUID REFERENCES resumes(id) ON DELETE SET NULL,
    job_description      TEXT NOT NULL,
    email_subject        TEXT NOT NULL,
    email_body           TEXT NOT NULL,
    gmail_thread_id      TEXT,
    status               TEXT NOT NULL DEFAULT 'pending_approval'
        CHECK (status IN ('pending_approval', 'sent', 'replied', 'followed_up', 'no_response', 'cancelled')),
    sent_at              TIMESTAMPTZ,
    replied_at           TIMESTAMPTZ,
    follow_up_count      INTEGER NOT NULL DEFAULT 0,
    last_followed_up_at  TIMESTAMPTZ,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX outreach_user_id_created_at_idx ON outreach (user_id, created_at DESC);
CREATE INDEX outreach_status_idx ON outreach (status);
CREATE INDEX outreach_user_status_idx ON outreach (user_id, status);
CREATE INDEX outreach_thread_id_idx ON outreach (gmail_thread_id) WHERE gmail_thread_id IS NOT NULL;

CREATE TABLE replies (
    id                UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    outreach_id       UUID NOT NULL REFERENCES outreach(id) ON DELETE CASCADE,
    gmail_message_id  TEXT NOT NULL,
    from_email        CITEXT NOT NULL,
    snippet           TEXT,
    body              TEXT,
    received_at       TIMESTAMPTZ NOT NULL,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX replies_gmail_message_id_uidx ON replies (gmail_message_id);
CREATE INDEX replies_outreach_id_idx ON replies (outreach_id);

CREATE TABLE notifications (
    id         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type       TEXT NOT NULL CHECK (type IN ('reply', 'quota', 'other')),
    payload    JSONB NOT NULL DEFAULT '{}'::jsonb,
    read_at    TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX notifications_user_unread_idx ON notifications (user_id, created_at DESC) WHERE read_at IS NULL;

CREATE TABLE config (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);
INSERT INTO config (key, value) VALUES
    ('max_daily_requests', '3'),
    ('trial_days', '3'),
    ('follow_up_interval_days', '3'),
    ('max_follow_ups', '3');
