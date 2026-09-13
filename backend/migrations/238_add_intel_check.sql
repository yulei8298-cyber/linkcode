-- Migration: 238_add_intel_check
-- 模型智力检测：周期性对「承诺不降智」的分组下发固定题目，公开展示判定结果。
--
-- 表结构说明：
--   - intel_check_targets   受检分组配置（一行 = 一个对外承诺不降智的分组）
--   - intel_check_questions 题库（逻辑题带标准答案；绘图题带参考稿与判定规则）
--   - intel_check_rounds    检测轮次（一次调度对所有启用分组跑一遍题目）
--   - intel_check_results   检测明细（每轮 × 每分组 × 每题一行）
--
-- 设计要点：
--   - api_key_encrypted 存 AES-256-GCM 密文（base64），由 service 层加解密；
--     base_url / api_key 仅管理端可见，公开接口永不返回。
--   - Ent 的 field.Enum 在本库映射为 VARCHAR + CHECK 约束（与 channel_monitors
--     的 provider/status 一致），不使用 PG 原生 enum type，便于后续扩容枚举值。
--   - rounds.trigger_source 不叫 trigger：TRIGGER 是 Postgres 保留字。
--   - rounds 的两个 question_id 用 ON DELETE SET NULL：题目被删后历史轮次仍可查，
--     只是不再能还原题面（题面已快照在 results.prompt_snapshot 里）。
--   - results 上 (round_id, target_id, kind) 唯一：发起请求时先落 running 行，
--     拿到结果后原地 UPDATE。缺这个约束，一次重试就会让时间线多出一个色块。
--   - results 单独的 (checked_at) 索引服务保留期清理的 DELETE；
--     (target_id, kind, checked_at DESC) 服务公开页的时间线查询。
--   - 本次新增的四张表都不引入 deleted_at：127 号迁移已明确日志类表不做软删除
--     （无恢复需求，软删只会让行和索引只增不减），过期数据由清理任务物理删除。

-- 1) 受检分组
CREATE TABLE IF NOT EXISTS intel_check_targets (
    id                BIGSERIAL    PRIMARY KEY,
    name              VARCHAR(100) NOT NULL,
    description       VARCHAR(500) NOT NULL DEFAULT '',
    base_url          VARCHAR(500) NOT NULL,
    api_key_encrypted TEXT         NOT NULL,
    api_mode          VARCHAR(32)  NOT NULL DEFAULT 'responses',
    model             VARCHAR(200) NOT NULL,
    reasoning_effort  VARCHAR(32)  NOT NULL DEFAULT 'medium',
    rate_label        VARCHAR(20)  NOT NULL DEFAULT '',
    enabled           BOOLEAN      NOT NULL DEFAULT TRUE,
    sort_order        INT          NOT NULL DEFAULT 0,
    created_by        BIGINT       NOT NULL,
    created_at        TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT intel_check_targets_api_mode_check
        CHECK (api_mode IN ('responses', 'chat_completions'))
);

CREATE INDEX IF NOT EXISTS idx_intel_check_targets_enabled_sort
    ON intel_check_targets (enabled, sort_order);

COMMENT ON COLUMN intel_check_targets.base_url IS
    '上游地址，仅管理端可见，公开接口永不返回';
COMMENT ON COLUMN intel_check_targets.rate_label IS
    '纯展示用的倍率文案（如 x0.3），不参与任何计费逻辑';

-- 2) 题库
CREATE TABLE IF NOT EXISTS intel_check_questions (
    id                BIGSERIAL    PRIMARY KEY,
    kind              VARCHAR(20)  NOT NULL,
    title             VARCHAR(100) NOT NULL,
    prompt            TEXT         NOT NULL,
    expected_answer   VARCHAR(500) NOT NULL DEFAULT '',
    match_mode        VARCHAR(32)  NOT NULL DEFAULT 'exact',
    reference_html    TEXT         NOT NULL DEFAULT '',
    reference_metrics JSONB,
    drawing_rules     JSONB,
    review_rubric     TEXT         NOT NULL DEFAULT '',
    enabled           BOOLEAN      NOT NULL DEFAULT TRUE,
    created_at        TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT intel_check_questions_kind_check
        CHECK (kind IN ('logic', 'drawing')),
    CONSTRAINT intel_check_questions_match_mode_check
        CHECK (match_mode IN ('exact', 'numeric', 'contains', 'regex'))
);

CREATE INDEX IF NOT EXISTS idx_intel_check_questions_kind_enabled
    ON intel_check_questions (kind, enabled);

COMMENT ON COLUMN intel_check_questions.reference_html IS
    '满血模型产出的参考稿原文，用于计算参考指标并作为评审对照样本';
COMMENT ON COLUMN intel_check_questions.reference_metrics IS
    '上传参考稿时服务端自动计算的结构指标快照';
COMMENT ON COLUMN intel_check_questions.drawing_rules IS
    '绘图题第一层结构门禁配置：min_ratio / required_keywords / max_bytes';

-- 3) 检测轮次
CREATE TABLE IF NOT EXISTS intel_check_rounds (
    id                  BIGSERIAL   PRIMARY KEY,
    seq                 BIGINT      NOT NULL,
    started_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    finished_at         TIMESTAMPTZ,
    logic_question_id   BIGINT      REFERENCES intel_check_questions(id) ON DELETE SET NULL,
    drawing_question_id BIGINT      REFERENCES intel_check_questions(id) ON DELETE SET NULL,
    trigger_source      VARCHAR(16) NOT NULL DEFAULT 'cron',
    CONSTRAINT intel_check_rounds_trigger_source_check
        CHECK (trigger_source IN ('cron', 'manual'))
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_intel_check_rounds_seq_unique
    ON intel_check_rounds (seq);
CREATE INDEX IF NOT EXISTS idx_intel_check_rounds_started_at
    ON intel_check_rounds (started_at);

COMMENT ON COLUMN intel_check_rounds.seq IS
    '对外展示的轮次号，与主键解耦，清理旧数据后不回退';
COMMENT ON COLUMN intel_check_rounds.trigger_source IS
    'cron = 定时调度；manual = 管理端手动触发。列名不用 trigger（PG 保留字）';

-- 4) 检测明细
CREATE TABLE IF NOT EXISTS intel_check_results (
    id                BIGSERIAL    PRIMARY KEY,
    round_id          BIGINT       NOT NULL REFERENCES intel_check_rounds(id) ON DELETE CASCADE,
    target_id         BIGINT       NOT NULL REFERENCES intel_check_targets(id) ON DELETE CASCADE,
    kind              VARCHAR(20)  NOT NULL,
    status            VARCHAR(20)  NOT NULL,
    latency_ms        INT,
    prompt_snapshot   TEXT         NOT NULL DEFAULT '',
    raw_reply         TEXT         NOT NULL DEFAULT '',
    extracted_answer  VARCHAR(500) NOT NULL DEFAULT '',
    html_output       TEXT         NOT NULL DEFAULT '',
    judge_detail      JSONB,
    error_message     VARCHAR(500) NOT NULL DEFAULT '',
    input_tokens      INT,
    output_tokens     INT,
    checked_at        TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT intel_check_results_kind_check
        CHECK (kind IN ('logic', 'drawing')),
    CONSTRAINT intel_check_results_status_check
        CHECK (status IN ('pass', 'fail', 'request_error', 'running'))
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_intel_check_results_round_target_kind_unique
    ON intel_check_results (round_id, target_id, kind);
CREATE INDEX IF NOT EXISTS idx_intel_check_results_target_kind_checked
    ON intel_check_results (target_id, kind, checked_at DESC);
CREATE INDEX IF NOT EXISTS idx_intel_check_results_round
    ON intel_check_results (round_id);
CREATE INDEX IF NOT EXISTS idx_intel_check_results_checked_at
    ON intel_check_results (checked_at);

COMMENT ON COLUMN intel_check_results.status IS
    'pass = 通过；fail = 未通过；request_error = 上游或网络故障（不计入降智连续计数）；running = 检测中';
COMMENT ON COLUMN intel_check_results.prompt_snapshot IS
    '本轮实际下发的题面快照，题目后续被改动也能还原当次上下文';
COMMENT ON COLUMN intel_check_results.judge_detail IS
    '逐项判定明细：结构门禁各项结果 + 源码评审得分与理由';
