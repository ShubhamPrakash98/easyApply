CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "citext";

CREATE TABLE users (
    id                       UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    google_sub               TEXT UNIQUE NOT NULL,
    email                    CITEXT UNIQUE NOT NULL,
    name                     TEXT,
    gmail_refresh_token_enc  BYTEA,
    trial_ends_at            TIMESTAMPTZ NOT NULL DEFAULT (NOW() + INTERVAL '3 days'),
    created_at               TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at               TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX users_email_idx ON users (email);
