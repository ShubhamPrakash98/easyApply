DROP INDEX IF EXISTS users_subscription_tier_idx;
ALTER TABLE users DROP COLUMN IF EXISTS subscription_tier;
