export interface PricingBenefitColumn {
  title: string
  tone: 'positive' | 'negative'
  items: string[]
}

export interface PricingDisplayConfig {
  eyebrow: string
  title: string
  subtitle: string
  rechargeLabel: string
  yuanAmount: number
  usdAmount: number
  creditUnitLabel: string
  exampleText: string
  highlightText: string
  recommendedAmountLabel: string
  recommendedAmounts: number[]
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
  usdAmount: 1,
  creditUnitLabel: '额度',
  exampleText: '例如充值 ¥ 8.00 即可获得 $10 额度',
  highlightText: '本店充值比例 0.8 ¥ : 1$，到账即时生效',
  recommendedAmountLabel: '推荐充值金额',
  recommendedAmounts: [10, 20, 50, 100, 200],
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
      usdAmount: normalizeNumber(parsed.usdAmount, defaultPricingDisplayConfig.usdAmount),
      creditUnitLabel: normalizeString(parsed.creditUnitLabel, defaultPricingDisplayConfig.creditUnitLabel),
      exampleText: normalizeString(parsed.exampleText, defaultPricingDisplayConfig.exampleText),
      highlightText: normalizeString(parsed.highlightText, defaultPricingDisplayConfig.highlightText),
      recommendedAmountLabel: normalizeString(
        parsed.recommendedAmountLabel,
        defaultPricingDisplayConfig.recommendedAmountLabel,
      ),
      recommendedAmounts: normalizeNumberArray(
        parsed.recommendedAmounts,
        defaultPricingDisplayConfig.recommendedAmounts,
      ),
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

export function stringifyDefaultPricingDisplayConfig(): string {
  return JSON.stringify(defaultPricingDisplayConfig, null, 2)
}
