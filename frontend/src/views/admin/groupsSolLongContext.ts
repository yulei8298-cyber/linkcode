// GPT-6.1 Sol 长上下文分组策略的表单辅助（LinkCode 二开）。
// 范围与后端 service.SolContextWindowMin / SolContextWindowMax 一致，越界由后端拒绝。
export const SOL_CONTEXT_WINDOW_MIN = 16000;
export const SOL_CONTEXT_WINDOW_MAX = 872000;

// 留空或非法输入按 0（沿用上游默认值）提交；小数取整。
export const normalizeSolContextWindow = (
  value: number | string | null | undefined,
): number => {
  if (value === null || value === undefined || value === "") {
    return 0;
  }
  const parsed = Number(value);
  return Number.isFinite(parsed) && parsed > 0 ? Math.round(parsed) : 0;
};
