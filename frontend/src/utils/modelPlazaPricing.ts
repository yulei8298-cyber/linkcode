import type { ModelPlazaGroup, PlazaModel } from '@/api/modelPlaza'
import { formatScaled } from '@/utils/pricing'

export function plazaGroupRate(group: ModelPlazaGroup): number {
  return group.user_rate_multiplier ?? group.rate_multiplier
}

export function plazaProvider(platform: string): string {
  return ({ openai: 'OpenAI', anthropic: 'Anthropic', claude: 'Anthropic',
    gemini: 'Google', google: 'Google', antigravity: 'Antigravity',
    grok: 'xAI', xai: 'xAI', deepseek: 'DeepSeek', composite: 'Composite' } as Record<string, string>)[platform] || platform
}

/** Use the billing schedule, before the group rate, just like the detail table.
 * Missing quotes stay unknown; zero prices and zero personal rates remain valid.
 */
export function plazaCardPricing(model: PlazaModel, group: ModelPlazaGroup) {
  const pricing = model.pricing
  const mode = pricing?.billing_mode ?? 'token'
  const token = mode === 'token'
  const rate = mode === 'image' && group.image_rate_independent
    ? group.image_rate_multiplier ?? 1 : plazaGroupRate(group)
  const intervals = (pricing?.intervals ?? [])
    .filter(interval => token || interval.per_request_price != null)
    .sort((a, b) => a.min_tokens - b.min_tokens)
  const officialIntervals = [...(model.official_pricing?.intervals ?? [])].sort((a, b) => a.min_tokens - b.min_tokens)
  const base = intervals[0] ?? pricing
  const official = officialIntervals[0] ?? model.official_pricing
  const fields = token ? ['input_price', 'output_price'] as const : ['per_request_price'] as const
  const metrics = fields.map(field => {
    const value = base?.[field] ?? null
    const reference = field === 'per_request_price' ? null : official?.[field] ?? null
    const isOfficial = value != null && reference != null && Math.abs(value - reference) <= Math.max(Math.abs(reference) * 1e-9, 1e-15)
    const scale = token ? 1_000_000 : 1
    return {
      field, price: formatScaled(value == null ? null : value * rate, scale, 2),
      base: formatScaled(value, scale, 2), official: formatScaled(reference, scale, 2),
      isOfficial, available: value != null
    }
  })
  return { mode, rate, metrics, tiered: intervals.length > 1, token }
}

export function sortPlazaModels(models: PlazaModel[]): PlazaModel[] {
  return [...models].sort((a, b) => {
    const tokenA = (a.pricing?.billing_mode ?? 'token') === 'token'
    const tokenB = (b.pricing?.billing_mode ?? 'token') === 'token'
    if (tokenA !== tokenB) return tokenA ? -1 : 1
    const priceA = a.official_pricing?.output_price
    const priceB = b.official_pricing?.output_price
    if (priceA != null && priceB != null && priceA !== priceB) return priceB - priceA
    if (priceA != null && priceB == null) return -1
    if (priceA == null && priceB != null) return 1
    return b.name.localeCompare(a.name)
  })
}
