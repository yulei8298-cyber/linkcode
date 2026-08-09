export interface PricingBenefitColumn {
  title: string
  tone: 'positive' | 'negative'
  items: string[]
}

export interface RechargeTierConfig {
  amount: number
  bonusPercent: number
  label: string
}

export interface PricingDisplayConfig {
  eyebrow: string
  title: string
  subtitle: string
  rechargeLabel: string
  yuanAmount: number
  yuanAmountMax: number | null
  usdAmount: number
  usdAmountMax: number | null
  creditUnitLabel: string
  exampleText: string
  highlightText: string
  activityLabel: string
  activityText: string
  recommendedAmountLabel: string
  recommendedAmounts: number[]
  rechargeTiers: RechargeTierConfig[]
  rechargeButtonText: string
  rechargeButtonUrl: string
  benefitsTitle: string
  benefitsDescription: string
  benefits: string[]
  channelTitle: string
  channelDescription: string
  channelDetailText: string
  channelGroupDescriptionTemplate: string
  channelEmptyText: string
  showPublicChannelGroups: boolean
}

export const defaultPricingDisplayConfig: PricingDisplayConfig = {
  eyebrow: 'PRICING',
  title: '按量计费 · 充多少用多少',
  subtitle: '已取消包月套餐，统一改为按 token 用量计费，余额永久有效，不同渠道按倍率扣费',
  rechargeLabel: '充值比例',
  yuanAmount: 0.8,
  yuanAmountMax: null,
  usdAmount: 1,
  usdAmountMax: null,
  creditUnitLabel: '额度',
  exampleText: '例如活动期间充值 ¥ 500 可获得 $688 额度',
  highlightText: '本店充值比例按实际活动配置生效，到账即时生效',
  activityLabel: '活动期间',
  activityText: '',
  recommendedAmountLabel: '推荐充值金额',
  recommendedAmounts: [20, 50, 100, 200],
  rechargeTiers: [],
  rechargeButtonText: '前往在线充值',
  rechargeButtonUrl: '/payment',
  benefitsTitle: '福利说明',
  benefitsDescription: '以下内容可在后台配置，用于说明余额、到账、售后或活动规则',
  benefits: ['余额永久有效', '充值后即时到账', '按实际 token 用量扣费', '公开渠道倍率透明展示'],
  channelTitle: '渠道倍率',
  channelDescription: '每次请求按所用渠道的倍率扣费，倍率会折算为每 1 美元官方价格所需额度',
  channelDetailText: '查看完整渠道详情',
  channelGroupDescriptionTemplate: '{multiplier} 倍率（相当于 {price} 元一刀）',
  channelEmptyText: '暂无公开渠道分组',
  showPublicChannelGroups: true,
}

function normalizeNumber(value: unknown, fallback: number): number {
  const numberValue = Number(value)
  return Number.isFinite(numberValue) ? numberValue : fallback
}

function normalizeString(value: unknown, fallback: string): string {
  return typeof value === 'string' ? value : fallback
}

function normalizeStringArray(value: unknown, fallback: string[]): string[] {
  if (!Array.isArray(value)) return fallback
  return value.map((item) => String(item).trim()).filter(Boolean)
}

function normalizeNumberArray(value: unknown, fallback: number[]): number[] {
  if (!Array.isArray(value)) return fallback
  const items = value
    .map((item) => Number(item))
    .filter((item) => Number.isFinite(item) && item > 0)
  return items.length > 0 ? items : fallback
}

function normalizeRechargeTiers(value: unknown): RechargeTierConfig[] {
  if (!Array.isArray(value)) return []
  return value
    .map((item) => {
      const source = item as Partial<RechargeTierConfig>
      const amount = Number(source.amount)
      const bonusPercent = Number(source.bonusPercent ?? 0)
      if (!Number.isFinite(amount) || amount <= 0 || !Number.isFinite(bonusPercent) || bonusPercent < 0) {
        return null
      }
      return {
        amount,
        bonusPercent,
        label: typeof source.label === 'string' ? source.label.trim() : '',
      }
    })
    .filter((item): item is RechargeTierConfig => item !== null)
}

export function parsePricingDisplayConfig(raw: unknown): PricingDisplayConfig {
  if (typeof raw !== 'string' || raw.trim() === '') {
    return { ...defaultPricingDisplayConfig }
  }

  try {
    const parsed = JSON.parse(raw) as Partial<PricingDisplayConfig>
    return {
      eyebrow: normalizeString(parsed.eyebrow, defaultPricingDisplayConfig.eyebrow),
      title: normalizeString(parsed.title, defaultPricingDisplayConfig.title),
      subtitle: normalizeString(parsed.subtitle, defaultPricingDisplayConfig.subtitle),
      rechargeLabel: normalizeString(parsed.rechargeLabel, defaultPricingDisplayConfig.rechargeLabel),
      yuanAmount: normalizeNumber(parsed.yuanAmount, defaultPricingDisplayConfig.yuanAmount),
      yuanAmountMax: parsed.yuanAmountMax == null ? null : normalizeNumber(parsed.yuanAmountMax, defaultPricingDisplayConfig.yuanAmountMax ?? defaultPricingDisplayConfig.yuanAmount),
      usdAmount: normalizeNumber(parsed.usdAmount, defaultPricingDisplayConfig.usdAmount),
      usdAmountMax: parsed.usdAmountMax == null ? null : normalizeNumber(parsed.usdAmountMax, defaultPricingDisplayConfig.usdAmountMax ?? defaultPricingDisplayConfig.usdAmount),
      creditUnitLabel: normalizeString(parsed.creditUnitLabel, defaultPricingDisplayConfig.creditUnitLabel),
      exampleText: normalizeString(parsed.exampleText, defaultPricingDisplayConfig.exampleText),
      highlightText: normalizeString(parsed.highlightText, defaultPricingDisplayConfig.highlightText),
      activityLabel: normalizeString(parsed.activityLabel, defaultPricingDisplayConfig.activityLabel),
      activityText: normalizeString(parsed.activityText, defaultPricingDisplayConfig.activityText),
      recommendedAmountLabel: normalizeString(
        parsed.recommendedAmountLabel,
        defaultPricingDisplayConfig.recommendedAmountLabel,
      ),
      recommendedAmounts: normalizeNumberArray(
        parsed.recommendedAmounts,
        defaultPricingDisplayConfig.recommendedAmounts,
      ),
      rechargeTiers: normalizeRechargeTiers(parsed.rechargeTiers),
      rechargeButtonText: normalizeString(parsed.rechargeButtonText, defaultPricingDisplayConfig.rechargeButtonText),
      rechargeButtonUrl: normalizeString(parsed.rechargeButtonUrl, defaultPricingDisplayConfig.rechargeButtonUrl),
      benefitsTitle: normalizeString(parsed.benefitsTitle, defaultPricingDisplayConfig.benefitsTitle),
      benefitsDescription: normalizeString(
        parsed.benefitsDescription,
        defaultPricingDisplayConfig.benefitsDescription,
      ),
      benefits: normalizeStringArray(parsed.benefits, defaultPricingDisplayConfig.benefits),
      channelTitle: normalizeString(parsed.channelTitle, defaultPricingDisplayConfig.channelTitle),
      channelDescription: normalizeString(
        parsed.channelDescription,
        defaultPricingDisplayConfig.channelDescription,
      ),
      channelDetailText: normalizeString(parsed.channelDetailText, defaultPricingDisplayConfig.channelDetailText),
      channelGroupDescriptionTemplate: normalizeString(
        parsed.channelGroupDescriptionTemplate,
        defaultPricingDisplayConfig.channelGroupDescriptionTemplate,
      ),
      channelEmptyText: normalizeString(parsed.channelEmptyText, defaultPricingDisplayConfig.channelEmptyText),
      showPublicChannelGroups:
        typeof parsed.showPublicChannelGroups === 'boolean'
          ? parsed.showPublicChannelGroups
          : defaultPricingDisplayConfig.showPublicChannelGroups,
    }
  } catch {
    return { ...defaultPricingDisplayConfig }
  }
}

export function findRechargeBenefit(benefits: string[], amount: number): string | null {
  for (const benefit of benefits) {
    const matches = benefit.matchAll(/(?:[¥￥]\s*|充值\s*)(\d+(?:\.\d+)?)(?:\s*元)?/gi)
    for (const match of matches) {
      if (Number(match[1]) === amount) return benefit
    }
  }
  return null
}

export function extractRechargeBenefitAmounts(benefits: string[]): number[] {
  const amounts = new Set<number>()
  for (const benefit of benefits) {
    for (const match of benefit.matchAll(/(?:[¥￥]\s*|充值\s*)(\d+(?:\.\d+)?)(?:\s*元)?/gi)) {
      const amount = Number(match[1])
      if (Number.isFinite(amount) && amount > 0) amounts.add(amount)
    }
  }
  return [...amounts].sort((a, b) => a - b)
}

export function stringifyDefaultPricingDisplayConfig(): string {
  return JSON.stringify(defaultPricingDisplayConfig, null, 2)
}
