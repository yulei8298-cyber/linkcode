-- Migration: 241_add_packages
-- 套餐（周卡 / 月卡）：绑定普通（余额）分组的预付额度，扣费时优先于余额。
--
-- 表结构说明：
--   - package_plans          套餐配置（每个分组 × 周期 × 档位 一个）
--   - user_packages          用户已购套餐（一次购买一行，可叠加）
--   - user_package_freezes   冻结记录（审计与 7 天上限核算）
--   - package_freeze_days    可冻结日期（官方节假日同步 + 后台手动添加）
--   - users.package_concurrency  用户级套餐并发，余额请求仍用 users.concurrency
--
-- 设计要点：
--   - 额度 quota_usd / used_usd 与余额同口径（分组倍率折算后的实际扣费金额），
--     精度对齐 users.balance 的 NUMERIC(20,8)。
--   - user_packages 快照 name / cycle / tier / quota_usd：后台改名或调额不影响已购套餐。
--   - user_packages.order_id 唯一：支付回调重放时发货幂等；管理员赠送时为空。
--   - 冻结期间 expires_at 不变，解冻时按冻结时长顺延；frozen_seconds_total 只累计
--     已结束的冻结段，进行中的一段由 frozen_at 推算。
--   - (user_id, group_id, status, expires_at) 服务鉴权与扣费的「按到期顺序取可用套餐」；
--     (status, expires_at) 服务过期任务；frozen 的部分索引服务自动解冻任务。
--   - 冻结记录与可冻结日期都不做软删除。

-- 1) 套餐配置
CREATE TABLE IF NOT EXISTS package_plans (
    id            BIGSERIAL     PRIMARY KEY,
    group_id      BIGINT        NOT NULL,
    name          VARCHAR(50)   NOT NULL,
    cycle         VARCHAR(10)   NOT NULL,
    tier          SMALLINT      NOT NULL,
    price         DECIMAL(20,2) NOT NULL,
    quota_usd     DECIMAL(20,8) NOT NULL,
    validity_days INT           NOT NULL,
    for_sale      BOOLEAN       NOT NULL DEFAULT TRUE,
    created_at    TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    CONSTRAINT package_plans_cycle_check CHECK (cycle IN ('week', 'month')),
    CONSTRAINT package_plans_tier_check CHECK (tier IN (1, 2)),
    CONSTRAINT package_plans_price_check CHECK (price > 0),
    CONSTRAINT package_plans_quota_check CHECK (quota_usd > 0)
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_package_plans_group_cycle_tier
    ON package_plans (group_id, cycle, tier);

-- 2) 用户套餐
CREATE TABLE IF NOT EXISTS user_packages (
    id                   BIGSERIAL     PRIMARY KEY,
    user_id              BIGINT        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    group_id             BIGINT        NOT NULL,
    plan_id              BIGINT        NOT NULL,
    order_id             BIGINT,
    name                 VARCHAR(50)   NOT NULL,
    cycle                VARCHAR(10)   NOT NULL,
    tier                 SMALLINT      NOT NULL,
    quota_usd            DECIMAL(20,8) NOT NULL,
    used_usd             DECIMAL(20,8) NOT NULL DEFAULT 0,
    starts_at            TIMESTAMPTZ   NOT NULL,
    expires_at           TIMESTAMPTZ   NOT NULL,
    status               VARCHAR(16)   NOT NULL DEFAULT 'active',
    frozen_at            TIMESTAMPTZ,
    frozen_seconds_total BIGINT        NOT NULL DEFAULT 0,
    created_at           TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_at           TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    CONSTRAINT user_packages_status_check
        CHECK (status IN ('active', 'frozen', 'exhausted', 'expired', 'voided'))
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_user_packages_order_id
    ON user_packages (order_id) WHERE order_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_user_packages_user_group_status_expires
    ON user_packages (user_id, group_id, status, expires_at);
CREATE INDEX IF NOT EXISTS idx_user_packages_status_expires
    ON user_packages (status, expires_at);
CREATE INDEX IF NOT EXISTS idx_user_packages_frozen
    ON user_packages (frozen_at) WHERE status = 'frozen';

-- 3) 冻结记录
CREATE TABLE IF NOT EXISTS user_package_freezes (
    id               BIGSERIAL   PRIMARY KEY,
    package_id       BIGINT      NOT NULL REFERENCES user_packages(id) ON DELETE CASCADE,
    user_id          BIGINT      NOT NULL,
    frozen_at        TIMESTAMPTZ NOT NULL,
    unfrozen_at      TIMESTAMPTZ,
    unfreeze_reason  VARCHAR(16) NOT NULL DEFAULT '',
    duration_seconds BIGINT      NOT NULL DEFAULT 0,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_user_package_freezes_package
    ON user_package_freezes (package_id, frozen_at DESC);

-- 4) 可冻结日期
CREATE TABLE IF NOT EXISTS package_freeze_days (
    id         BIGSERIAL   PRIMARY KEY,
    day        DATE        NOT NULL,
    name       VARCHAR(50) NOT NULL DEFAULT '',
    kind       VARCHAR(8)  NOT NULL,
    source     VARCHAR(8)  NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT package_freeze_days_kind_check CHECK (kind IN ('off', 'work')),
    CONSTRAINT package_freeze_days_source_check CHECK (source IN ('auto', 'manual'))
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_package_freeze_days_day_source
    ON package_freeze_days (day, source);

-- 5) 用户级套餐并发
ALTER TABLE users ADD COLUMN IF NOT EXISTS package_concurrency INT NOT NULL DEFAULT 5;

COMMENT ON COLUMN users.package_concurrency IS
    '套餐请求的并发上限，对该用户所有套餐合计生效；余额请求仍用 concurrency';
COMMENT ON COLUMN user_packages.frozen_seconds_total IS
    '已结束冻结段的累计秒数，用于单张套餐累计冻结上限';
