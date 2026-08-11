-- Migration: 标记历史 Responses input_tokens 辅助请求错误
-- Purpose: 这类请求是本地可处理的 token 统计探针；旧版本曾将上游 404 记录为普通错误，
--          需要补标后才能从运维健康度和渠道监控中排除。

UPDATE ops_error_logs
SET is_count_tokens = TRUE
WHERE COALESCE(is_count_tokens, FALSE) = FALSE
  AND RTRIM(COALESCE(request_path, ''), '/') LIKE '%/responses/input_tokens';
