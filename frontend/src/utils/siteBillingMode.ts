import type { PublicSettings } from '@/types'
import { FeatureFlags, resolveFeatureFlag } from '@/utils/featureFlags'

/**
 * 站点计费模式（后台「站点类型」单选），由两个后端开关派生：
 * - `subscription_enabled`（opt-out，缺省视为开启）：用户端订阅面（侧边栏、购买页订阅 tab、顶栏徽章、/subscriptions）
 * - `payment_balance_disabled`（strict true）：支付配置里的 BALANCE_PAYMENT_DISABLED，关闭余额充值下单
 *
 * 两者都关闭即「仅套餐」：用户端不提供余额充值与订阅，购买入口只剩套餐商店
 * （套餐下单仍复用 /purchase 页面，但不再出现在侧边栏）。
 */
export type SiteBillingMode = 'recharge_and_subscription' | 'recharge_only' | 'subscription_only' | 'packages_only'

export const SITE_BILLING_MODES: readonly SiteBillingMode[] = [
  'recharge_and_subscription',
  'recharge_only',
  'subscription_only',
  'packages_only',
]

/** i18n 子键（admin.settings.features.siteBillingMode.options / hints）。 */
export const SITE_BILLING_MODE_I18N_KEYS: Record<SiteBillingMode, string> = {
  recharge_and_subscription: 'rechargeAndSubscription',
  recharge_only: 'rechargeOnly',
  subscription_only: 'subscriptionOnly',
  packages_only: 'packagesOnly',
}

export interface BillingModeSettings {
  subscription_enabled?: boolean
  payment_balance_disabled?: boolean
}

export function resolveSiteBillingMode(settings: BillingModeSettings | null | undefined): SiteBillingMode {
  const subscriptionEnabled = resolveFeatureFlag(
    { subscription_enabled: settings?.subscription_enabled } as Partial<PublicSettings>,
    FeatureFlags.subscription,
  )
  if (!subscriptionEnabled) return settings?.payment_balance_disabled === true ? 'packages_only' : 'recharge_only'
  if (settings?.payment_balance_disabled === true) return 'subscription_only'
  return 'recharge_and_subscription'
}

export function billingModeToSettings(mode: SiteBillingMode): Required<BillingModeSettings> {
  switch (mode) {
    case 'recharge_only':
      return { subscription_enabled: false, payment_balance_disabled: false }
    case 'subscription_only':
      return { subscription_enabled: true, payment_balance_disabled: true }
    case 'packages_only':
      return { subscription_enabled: false, payment_balance_disabled: true }
    default:
      return { subscription_enabled: true, payment_balance_disabled: false }
  }
}
