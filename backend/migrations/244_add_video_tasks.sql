-- 视频生成平台：异步任务账务源 + 旧 Seedance 账号迁移。
--
-- video_tasks 是视频任务的状态与账务源：网关受理后落库，后台轮询器按 next_poll_at
-- 查询上游，终态成功时按实际时长 / token 结算并写 usage_logs（request_id 幂等），
-- 失败与取消不计费。用户是否轮询都不影响扣费。

CREATE TABLE IF NOT EXISTS video_tasks (
    id BIGSERIAL PRIMARY KEY,
    task_id VARCHAR(64) NOT NULL UNIQUE,
    user_id BIGINT NOT NULL,
    api_key_id BIGINT NOT NULL,
    group_id BIGINT,
    account_id BIGINT NOT NULL,
    protocol VARCHAR(32) NOT NULL,
    source VARCHAR(16) NOT NULL DEFAULT 'unified',
    model VARCHAR(128) NOT NULL,
    upstream_model VARCHAR(256) NOT NULL DEFAULT '',
    upstream_task_id VARCHAR(512) NOT NULL,
    upstream_meta VARCHAR(256) NOT NULL DEFAULT '',
    status VARCHAR(16) NOT NULL DEFAULT 'queued',
    progress INTEGER NOT NULL DEFAULT 0,
    resolution VARCHAR(16) NOT NULL DEFAULT '',
    aspect_ratio VARCHAR(16) NOT NULL DEFAULT '',
    duration_seconds INTEGER NOT NULL DEFAULT 0,
    actual_duration_seconds DECIMAL(10,2),
    video_count INTEGER NOT NULL DEFAULT 1,
    generate_audio BOOLEAN NOT NULL DEFAULT FALSE,
    estimated_cost DECIMAL(20,10) NOT NULL DEFAULT 0,
    actual_cost DECIMAL(20,10),
    completion_tokens BIGINT NOT NULL DEFAULT 0,
    video_url TEXT NOT NULL DEFAULT '',
    file_id VARCHAR(256) NOT NULL DEFAULT '',
    error_code VARCHAR(128) NOT NULL DEFAULT '',
    error_message TEXT NOT NULL DEFAULT '',
    poll_count INTEGER NOT NULL DEFAULT 0,
    next_poll_at TIMESTAMPTZ,
    billing_status VARCHAR(16) NOT NULL DEFAULT 'pending',
    billing_attempts INTEGER NOT NULL DEFAULT 0,
    billing_error TEXT NOT NULL DEFAULT '',
    inbound_endpoint VARCHAR(128) NOT NULL DEFAULT '',
    client_ip VARCHAR(64) NOT NULL DEFAULT '',
    user_agent VARCHAR(512) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    finished_at TIMESTAMPTZ,
    billed_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS video_tasks_account_upstream_uq ON video_tasks (account_id, upstream_task_id);
CREATE INDEX IF NOT EXISTS video_tasks_user_created_idx ON video_tasks (user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS video_tasks_protocol_upstream_idx ON video_tasks (protocol, upstream_task_id);
CREATE INDEX IF NOT EXISTS video_tasks_status_created_idx ON video_tasks (status, created_at DESC);
CREATE INDEX IF NOT EXISTS video_tasks_poll_due_idx ON video_tasks (next_poll_at)
    WHERE status IN ('queued', 'in_progress');
CREATE INDEX IF NOT EXISTS video_tasks_billing_pending_idx ON video_tasks (updated_at)
    WHERE status = 'completed' AND billing_status = 'pending';
CREATE INDEX IF NOT EXISTS video_tasks_file_id_idx ON video_tasks (protocol, file_id)
    WHERE file_id <> '';

-- 旧版 Seedance 挂在「OpenAI API Key 账号 + seedance 端点能力」上，统一迁到视频平台
-- 的方舟协议；账号原有分组绑定保持不变，视频端点按账号平台调度，因此原分组的 Key
-- 仍可调用 Seedance。
UPDATE accounts
SET platform = 'video',
    credentials = (credentials - 'openai_capabilities') || jsonb_build_object('video_protocol', 'ark'),
    updated_at = NOW()
WHERE platform = 'openai'
  AND type = 'apikey'
  AND deleted_at IS NULL
  AND credentials ? 'openai_capabilities'
  AND (
      (jsonb_typeof(credentials -> 'openai_capabilities') = 'array'
          AND credentials -> 'openai_capabilities' ? 'seedance')
      OR (jsonb_typeof(credentials -> 'openai_capabilities') = 'object'
          AND COALESCE(credentials -> 'openai_capabilities' ->> 'seedance', 'false') = 'true')
  );
