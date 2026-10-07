-- 企业尊享：管理员对单个用户的手动覆盖。没有记录 = 按累计消费自动判定。
-- mode: on 强制开通 / off 强制关闭。
CREATE TABLE IF NOT EXISTS user_enterprise_overrides (
    user_id    BIGINT      PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    mode       VARCHAR(8)  NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT user_enterprise_overrides_mode_check CHECK (mode IN ('on', 'off'))
);

COMMENT ON TABLE user_enterprise_overrides IS '企业尊享手动覆盖：on 强制开通，off 强制关闭；无记录按累计消费自动判定';
