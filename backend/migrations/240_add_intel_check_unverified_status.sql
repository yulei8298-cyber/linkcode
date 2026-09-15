-- 绘图产物缺少可测量动作证据时，不能继续显示为绿色通过。
-- unverified 既不是模型失败，也不是链路故障，不进入通过率分母或降智连续计数。
ALTER TABLE intel_check_results
    DROP CONSTRAINT IF EXISTS intel_check_results_status_check;

ALTER TABLE intel_check_results
    ADD CONSTRAINT intel_check_results_status_check
    CHECK (status IN ('pass', 'fail', 'request_error', 'running', 'unverified'));

COMMENT ON COLUMN intel_check_results.status IS
    'pass = 通过；fail = 未通过；request_error = 上游或验收器故障；running = 检测中；unverified = 产物存在但动作证据不足';
