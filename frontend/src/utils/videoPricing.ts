import type { VideoPricingSpec, VideoTierPrice } from '@/api/admin/modelBasePricing'

/** 视频计费分辨率档（与后端 VideoPricingSpec 的键一致），"*" 表示任意分辨率。 */
export const VIDEO_TIERS: Array<{ value: string; label: string }> = [
  { value: '480p', label: '480p' },
  { value: '720p', label: '720p' },
  { value: '1080p', label: '1080p' },
  { value: '4k', label: '4K' },
  { value: '*', label: '任意' },
]

const ORDER = ['480p', '720p', '1080p', '4k']

/** 与后端 VideoPricingSpec.tier 相同的取档规则：精确档 > 通配档 > 更高的最近档 > 最高档。 */
function pickTier(spec: VideoPricingSpec, resolution: string): VideoTierPrice | undefined {
  const prices = spec.prices ?? {}
  if (prices[resolution]) return prices[resolution]
  if (prices['*']) return prices['*']
  const start = Math.max(ORDER.indexOf(resolution), 0)
  for (let i = start; i < ORDER.length; i++) if (prices[ORDER[i]]) return prices[ORDER[i]]
  for (let i = ORDER.length - 1; i >= 0; i--) if (prices[ORDER[i]]) return prices[ORDER[i]]
  return undefined
}

/** 选择不小于时长的最短时长档；都更短时取通配档或最长档。 */
function pickPerVideo(perVideo: Record<string, number>, seconds: number): number | undefined {
  if (perVideo[String(seconds)] != null) return perVideo[String(seconds)]
  const keys = Object.keys(perVideo).map(Number).filter(Number.isFinite).sort((a, b) => a - b)
  const hit = keys.find(k => k >= seconds)
  if (hit != null) return perVideo[String(hit)]
  if (perVideo['*'] != null) return perVideo['*']
  return keys.length ? perVideo[String(keys[keys.length - 1])] : undefined
}

/** 估算单条视频的基础价格（未乘倍率）；按 token 计费或无法定价时返回 null。 */
export function estimateVideoCost(spec: VideoPricingSpec, resolution: string, seconds: number, withAudio = false): number | null {
  let duration = seconds
  if (spec.min_duration_seconds && duration < spec.min_duration_seconds) duration = spec.min_duration_seconds
  if (spec.max_duration_seconds && duration > spec.max_duration_seconds) duration = spec.max_duration_seconds
  const tier = pickTier(spec, resolution)
  if (!tier) return null
  if (spec.unit === 'per_second') {
    const unit = withAudio && tier.per_second_with_audio != null ? tier.per_second_with_audio : tier.per_second
    return unit == null ? null : unit * duration
  }
  if (spec.unit === 'per_video') {
    return pickPerVideo(tier.per_video ?? {}, duration) ?? null
  }
  return null
}

/** 计费规则的简短描述，例如「$0.085/秒 起」或「$0.28/条 起」。 */
export function describeVideoPricing(spec: VideoPricingSpec, multiplier = 1): string {
  if (spec.unit === 'token') {
    return spec.token_price_per_million != null ? `$${(spec.token_price_per_million * multiplier).toFixed(2)}/百万 token` : '按 token'
  }
  const values: number[] = []
  for (const tier of Object.values(spec.prices ?? {})) {
    if (spec.unit === 'per_second' && tier.per_second != null) values.push(tier.per_second)
    if (spec.unit === 'per_video') values.push(...Object.values(tier.per_video ?? {}))
  }
  if (!values.length) return ''
  const min = Math.min(...values) * multiplier
  return spec.unit === 'per_second' ? `$${formatPrice(min)}/秒 起` : `$${formatPrice(min)}/条 起`
}

function formatPrice(v: number): string {
  return v >= 1 ? v.toFixed(2) : v.toPrecision(3).replace(/0+$/, '').replace(/\.$/, '')
}
