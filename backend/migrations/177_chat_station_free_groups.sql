-- Migration: 177_chat_station_free_groups
-- Add field-driven chat-station/free-group configuration and a unique
-- per-user, per-group, per-calendar-day free-usage ledger.

ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS is_hidden BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS is_free BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS daily_free_limit_usd DECIMAL(20, 10),
    ADD COLUMN IF NOT EXISTS chat_station_only BOOLEAN NOT NULL DEFAULT FALSE;

-- Convert the legacy group only when every undeleted subscription on it is a
-- known LobeHub auto-assignment. If manual or paid subscriptions exist, keep
-- the group in subscription mode so their authorization remains unchanged.
DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM groups AS g
        WHERE g.name = 'OpenAI-chat'
          AND g.platform = 'openai'
          AND g.subscription_type = 'subscription'
          AND g.deleted_at IS NULL
          AND NOT EXISTS (
              SELECT 1
              FROM user_subscriptions AS us
              WHERE us.group_id = g.id
                AND us.deleted_at IS NULL
                AND NOT (
                    us.assigned_by IS NULL
                    AND us.notes = 'auto assigned by LobeHub SSO'
                )
          )
          AND (g.daily_limit_usd IS NULL OR g.daily_limit_usd <= 0)
    ) THEN
        RAISE EXCEPTION 'cannot migrate OpenAI-chat free group without a positive daily_limit_usd';
    END IF;
END $$;

-- Preserve the legacy daily quota, then move the group away from subscription
-- billing. Weekly/monthly subscription windows no longer apply to free usage.
UPDATE groups AS g
SET is_hidden = TRUE,
    is_free = TRUE,
    daily_free_limit_usd = daily_limit_usd,
    chat_station_only = TRUE,
    subscription_type = 'standard',
    daily_limit_usd = NULL,
    weekly_limit_usd = NULL,
    monthly_limit_usd = NULL,
    peak_rate_enabled = FALSE,
    peak_start = '',
    peak_end = '',
    peak_rate_multiplier = 1.0,
    updated_at = NOW()
WHERE g.name = 'OpenAI-chat'
  AND g.platform = 'openai'
  AND g.subscription_type = 'subscription'
  AND g.deleted_at IS NULL
  AND NOT EXISTS (
      SELECT 1
      FROM user_subscriptions AS us
      WHERE us.group_id = g.id
        AND us.deleted_at IS NULL
        AND NOT (
            us.assigned_by IS NULL
            AND us.notes = 'auto assigned by LobeHub SSO'
        )
  );

-- Only retire subscriptions created by the legacy LobeHub SSO path. Manual or
-- paid subscriptions, including subscriptions for every other group, remain.
UPDATE user_subscriptions AS us
SET status = 'expired',
    deleted_at = NOW(),
    updated_at = NOW()
FROM groups AS g
WHERE us.group_id = g.id
  AND g.name = 'OpenAI-chat'
  AND g.platform = 'openai'
  AND us.assigned_by IS NULL
  AND us.notes = 'auto assigned by LobeHub SSO'
  AND us.deleted_at IS NULL;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'groups_daily_free_config_check'
          AND conrelid = 'groups'::regclass
    ) THEN
        ALTER TABLE groups
            ADD CONSTRAINT groups_daily_free_config_check
            CHECK (
                (is_free = FALSE AND daily_free_limit_usd IS NULL)
                OR
                (is_free = TRUE
                 AND subscription_type = 'standard'
                 AND daily_free_limit_usd > 0)
            );
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_groups_chat_station_free_candidates
    ON groups(platform, sort_order, id)
    WHERE deleted_at IS NULL
      AND status = 'active'
      AND subscription_type = 'standard'
      AND is_free = TRUE
      AND chat_station_only = TRUE;

-- Serialize the first-request bootstrap without exposing a general hidden-key
-- creation API. A concurrent retry reuses the row that won this unique insert.
CREATE UNIQUE INDEX IF NOT EXISTS api_keys_chat_station_auto_unique_active
    ON api_keys(user_id, group_id)
    WHERE deleted_at IS NULL
      AND status = 'active'
      AND name = 'LobeHub Chat Station';

CREATE TABLE IF NOT EXISTS daily_free_usages (
    id          BIGSERIAL PRIMARY KEY,
    user_id     BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    group_id    BIGINT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    usage_date  DATE NOT NULL,
    usage_usd   DECIMAL(20, 10) NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT daily_free_usages_usage_nonnegative_check CHECK (usage_usd >= 0),
    CONSTRAINT daily_free_usages_user_group_date_key UNIQUE (user_id, group_id, usage_date)
);

CREATE INDEX IF NOT EXISTS idx_daily_free_usages_group_date
    ON daily_free_usages(group_id, usage_date);

CREATE INDEX IF NOT EXISTS idx_daily_free_usages_user_date
    ON daily_free_usages(user_id, usage_date);
