/**
 * 格式化缓存 token 数量（1K/1M 缩写）
 */
export function formatCacheTokens(tokens: number): string {
  if (tokens >= 1000000) return `${(tokens / 1000000).toFixed(1)}M`
  if (tokens >= 1000) return `${(tokens / 1000).toFixed(1)}K`
  return tokens.toLocaleString()
}

/**
 * 自适应精度格式化倍率：保留至多 4 位小数并去掉末尾多余的 0，
 * 但至少保留 2 位小数（0.035 -> "0.035"，0.3 -> "0.30"，1 -> "1.00"）
 */
export function formatMultiplier(val: number): string {
  if (val < 0.0001) return val.toPrecision(2)
  return val.toFixed(4).replace(/(\.\d{2}\d*?)0+$/, '$1')
}

export interface TokenBreakdown {
  input_tokens?: number | null
  output_tokens?: number | null
  cache_creation_tokens?: number | null
  cache_read_tokens?: number | null
}

/**
 * 缓存率 = 缓存读取 Token ÷ 总 Token（输入 + 输出 + 缓存创建 + 缓存读取，与使用记录里的「总计」同口径）。
 * 返回 0~1；总数为 0（没有任何 Token）时返回 null，由展示层显示 "-"。
 */
export function calcCacheRate(cacheReadTokens: number, totalTokens: number): number | null {
  if (!(totalTokens > 0)) return null
  return Math.min(1, Math.max(0, (cacheReadTokens || 0) / totalTokens))
}

/** 单条使用记录的缓存率。 */
export function calcRowCacheRate(row: TokenBreakdown): number | null {
  const cacheRead = row.cache_read_tokens || 0
  const total = (row.input_tokens || 0) + (row.output_tokens || 0) + (row.cache_creation_tokens || 0) + cacheRead
  return calcCacheRate(cacheRead, total)
}

/** 缓存率展示：固定一位小数（37.5%），无数据显示 "-"。 */
export function formatCacheRate(rate: number | null): string {
  return rate === null ? '-' : `${(rate * 100).toFixed(1)}%`
}
