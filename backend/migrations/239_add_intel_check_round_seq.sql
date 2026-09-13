-- Migration: 239_add_intel_check_round_seq
-- 为 intel_check_rounds.seq 补一个独立的 Postgres 序列。
--
-- 背景：238 号迁移把 seq 建成了普通的 BIGINT NOT NULL + 唯一索引，取值方式留给
-- 应用层决定。落到代码上只有两种选择，都不成立：
--
--   1) MAX(seq) + 1 —— 违背该列的设计意图。seq 是对外展示的轮次号（#1284），
--      238 的列注释明确写了「与主键解耦，清理旧数据后不回退」。而保留期清理会
--      物理删除过期轮次，一旦服务停机超过 retention_days 把表清空，MAX 退化为
--      NULL，轮次号就从 #1 重新开始，公开页上的累计检测次数凭空归零。
--   2) MAX(seq) + 1 配合行锁 —— 能解决并发，仍解决不了上面的回退问题，
--      且给每次轮次创建都加了一次全表聚合与锁等待。
--
-- 序列同时解决两个问题：取值不受表内现存行影响（清空表不影响 nextval），
-- 且 nextval 本身并发安全，不必依赖 leader lock 的正确性——leader lock 一旦
-- 因为超时续约失败而被两个实例同时持有，MAX+1 方案会直接撞唯一索引报错。
--
-- 这里不改 238 的建表语句：迁移文件一旦落库即受 SHA256 校验保护，任何编辑都会
-- 让已部署环境的迁移校验失败。

-- 序列的起点取当前已有的最大轮次号 + 1，保证升级时不与存量数据冲突。
-- 全新部署时表为空，COALESCE 回落到 1。
DO $$
DECLARE
    next_seq BIGINT;
BEGIN
    IF EXISTS (
        SELECT 1 FROM pg_class WHERE relkind = 'S' AND relname = 'intel_check_rounds_seq_seq'
    ) THEN
        RETURN;
    END IF;

    SELECT COALESCE(MAX(seq), 0) + 1 INTO next_seq FROM intel_check_rounds;
    EXECUTE format('CREATE SEQUENCE intel_check_rounds_seq_seq START WITH %s', next_seq);
END
$$;

-- 绑定所有权：intel_check_rounds 被 DROP 时序列一并回收，不留孤儿对象。
ALTER SEQUENCE intel_check_rounds_seq_seq OWNED BY intel_check_rounds.seq;

-- 设为列默认值。应用层仍显式取 nextval 后写入（ent 的 seq 字段非 Optional，
-- 必须在 Create 时给值），这里的 DEFAULT 是给手写 SQL 与运维直接 INSERT 兜底。
ALTER TABLE intel_check_rounds
    ALTER COLUMN seq SET DEFAULT nextval('intel_check_rounds_seq_seq');

COMMENT ON SEQUENCE intel_check_rounds_seq_seq IS
    '智力检测轮次号发号器；与主键解耦，保留期清理不影响其取值';
