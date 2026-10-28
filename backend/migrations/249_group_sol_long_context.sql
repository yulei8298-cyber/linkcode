-- LinkCode 二开：GPT-6.1 Sol 长上下文按分组配置。
-- sol_long_context_exempt：开启后该分组的 Sol 请求不向用户收取长上下文附加费（默认关闭，保持现有计费）。
-- sol_context_window：下发给 Codex 客户端的 Sol 上下文窗口，0 表示沿用上游/官方目录值。
ALTER TABLE groups ADD COLUMN IF NOT EXISTS sol_long_context_exempt BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE groups ADD COLUMN IF NOT EXISTS sol_context_window INTEGER NOT NULL DEFAULT 0;
