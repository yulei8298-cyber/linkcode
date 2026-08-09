-- Preserve the normalized input fingerprint for moderation and upstream cyber-policy findings.
-- The corresponding input_excerpt remains capped and redacted by the service layer.
ALTER TABLE content_moderation_logs
    ADD COLUMN IF NOT EXISTS input_hash VARCHAR(64) NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_content_moderation_logs_input_hash
    ON content_moderation_logs(input_hash)
    WHERE input_hash <> '';
