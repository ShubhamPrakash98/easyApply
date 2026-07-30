ALTER TABLE users
  ADD COLUMN subscription_tier TEXT NOT NULL DEFAULT 'free'
    CHECK (subscription_tier IN ('free', 'premium'));

CREATE INDEX users_subscription_tier_idx ON users (subscription_tier);
