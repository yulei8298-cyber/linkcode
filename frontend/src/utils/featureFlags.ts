/**
 * Feature flag registry — single source of truth for public-settings-driven
 * feature switches used by the sidebar, routes, and views.
 *
 * ## Why this module exists
 *
 * `public settings` reach the frontend through two channels:
 *
 *   1. **SSR injection** — the backend embeds `window.__APP_CONFIG__` into the
 *      HTML. `main.ts` calls `appStore.initFromInjectedConfig()` synchronously
 *      before Vue mounts, so `cachedPublicSettings` is populated on first
 *      render.
 *   2. **Async API** — `App.vue` awaits `appStore.fetchPublicSettings()` on
 *      mount as a fallback (used when injection is missing or stale).
 *
 * If the SSR injection struct forgets to include a feature flag field — the
 * exact bug that hid the "可用渠道" menu after every refresh — the frontend
 * reads `undefined` until the async call resolves. An opt-in flag written as
 * `settings?.xxx_enabled === true` then evaluates to `false` and the menu
 * disappears. An opt-out flag written as `settings?.xxx_enabled !== false`
 * evaluates to `true` (menu stays) but will flicker off if the backend sends
 * `false`.
 *
 * This module hides that `undefined` handling behind two explicit modes.
 *
 * ## Modes
 *
 *   - **`opt-out`** (default enabled) — menu visible when settings unloaded,
 *     hidden only when the backend explicitly sends `false`. Use for features
 *     that ship enabled by default (Channel Monitor, Payment).
 *   - **`opt-in`**  (default disabled) — menu hidden when settings unloaded,
 *     visible only when the backend explicitly sends `true`. Use for features
 *     that ship disabled (Available Channels).
 *
 * For `opt-in` flags to render immediately on refresh, the backend **must**
 * inject the field through `PublicSettingsInjectionPayload`. A drift test in
 * `backend/internal/handler/dto/public_settings_injection_schema_test.go`
 * catches omissions.
 *
 * 注入是第一道保障，但它会因为「后端二进制比前端旧、结构里还没有新字段」
 * 「纯静态部署根本没有注入」「注入被中间层缓存成旧版本」等原因失效，
 * 每失效一次，opt-in 菜单就会在刷新时闪一下——上面那起"可用渠道"事故当时是靠
 * 补注入字段修的，没有解决"注入一失效就复发"这个根因。
 * 因此 `isFeatureFlagEnabled` 另外把上次解析成功的值记在 localStorage 里，
 * 在未知窗口期拿它兜底（见下方 FEATURE_FLAG_MEMORY_KEY 处的说明）。
 * 两层互补：注入让首次访问就正确，记忆让后续刷新不再受注入失效影响。
 *
 * ## Adding a new flag
 *
 *   1. Backend `service/domain_constants.go`  → `SettingKey<Name>Enabled`
 *   2. Backend `service/settings_view.go`      → `PublicSettings` + `SystemSettings`
 *   3. Backend `service/setting_service.go`    → `GetPublicSettings` / `UpdateSettings` /
 *                                                 `GetAllSettings` / `InitDefaultSettings` /
 *                                                 **`PublicSettingsInjectionPayload`**
 *                                                 (the drift test enforces this)
 *   4. Backend `handler/dto/settings.go`       → `PublicSettings` + `SystemSettings`
 *   5. Backend `handler/setting_handler.go`    → handler response
 *   6. Backend `handler/admin/setting_handler.go` → update request + audit diff
 *   7. Frontend `types/index.ts`               → `PublicSettings` typings
 *   8. Frontend `api/admin/settings.ts`        → admin DTO typings
 *   9. **Frontend `utils/featureFlags.ts` (this file)** → register via `defineFlag`
 *  10. Frontend `views/admin/SettingsView.vue` → Toggle UI + form defaults + save payload
 *  11. Frontend `components/layout/AppSidebar.vue` → attach via `makeSidebarFlag`
 *
 * ## Usage
 *
 * ```ts
 * import { FeatureFlags, makeSidebarFlag } from '@/utils/featureFlags'
 *
 * const flagAvailableChannels = makeSidebarFlag(FeatureFlags.availableChannels)
 * // ...
 * { path: '/available-channels', label: ..., featureFlag: flagAvailableChannels }
 * ```
 *
 * `isFeatureFlagEnabled(flag)` returns the resolved boolean (`true` = show).
 * `makeSidebarFlag(flag)` returns a `() => boolean | undefined` compatible with
 * `AppSidebar.NavItem.featureFlag`, where `false` hides the menu entry.
 */
import { useAppStore } from '@/stores/app'
import type { PublicSettings } from '@/types'
import { DEFAULT_INTERVAL_SECONDS } from '@/constants/channelMonitor'

export type FeatureFlagMode = 'opt-in' | 'opt-out'

export interface FeatureFlagDefinition {
  /** Public-settings key used for lookup. */
  readonly key: keyof PublicSettings
  /** Resolution mode when the key is missing/undefined. */
  readonly mode: FeatureFlagMode
  /** Short human label for logs and debug tooling. */
  readonly label: string
}

function defineFlag<K extends keyof PublicSettings>(
  def: { key: K; mode: FeatureFlagMode; label: string },
): FeatureFlagDefinition {
  return def
}

/**
 * Registered feature flags. Add a new entry here when introducing a new
 * public-settings-driven switch; see the "Adding a new flag" checklist above.
 */
export const FeatureFlags = {
  channelMonitor: defineFlag({
    key: 'channel_monitor_enabled',
    mode: 'opt-out',
    label: 'Channel Monitor',
  }),
  availableChannels: defineFlag({
    key: 'available_channels_enabled',
    mode: 'opt-in',
    label: 'Available Channels',
  }),
  subscription: defineFlag({
    key: 'subscription_enabled',
    mode: 'opt-out',
    label: 'Subscription',
  }),
  modelPlaza: defineFlag({
    key: 'model_plaza_enabled',
    mode: 'opt-in',
    label: 'Model Plaza',
  }),
  pluginManagement: defineFlag({
    key: 'plugin_management_enabled',
    mode: 'opt-in',
    label: 'Plugin Management',
  }),
  payment: defineFlag({
    key: 'payment_enabled',
    mode: 'opt-out',
    label: 'Payment',
  }),
  riskControl: defineFlag({
    key: 'risk_control_enabled',
    mode: 'opt-in',
    label: 'Risk Control',
  }),
  affiliate: defineFlag({
    key: 'affiliate_enabled',
    mode: 'opt-in',
    label: 'Affiliate',
  }),
  // opt-in：后端 DefaultIntelCheckSettings.Enabled 为 false（配置完成前不自动开放）。
  // 若写成 opt-out，设置尚未加载的那一瞬间入口会先出现再消失，
  // 而这个开关默认关闭，闪现的恰好是大多数部署不该看到的东西。
  intelCheck: defineFlag({
    key: 'intel_check_enabled',
    mode: 'opt-in',
    label: 'Intel Check',
  }),
} as const

export type RegisteredFeatureFlag = keyof typeof FeatureFlags

/**
 * 上一次解析成功的开关值，跨刷新记在 localStorage 里。
 *
 * ## 为什么需要这一层
 *
 * SSR 注入（`window.__APP_CONFIG__`）本应消除"设置未加载"的窗口期，但它有几种
 * 失效方式：前端不由 Go 后端托管（纯静态部署 / dev server）、后端二进制比前端旧
 * 因而注入结构里还没有新字段、注入被中间层缓存成了旧版本。任何一种发生时，
 * opt-in 的菜单会在每次刷新时先消失一下再出现——本文件开头记的"可用渠道"事故
 * 就是这么来的，而那次是靠补注入字段修的，没有解决"注入失效就复发"这个根因。
 *
 * 记住上次的结果，是对未知窗口期远好于 mode 默认值的猜测：用户上次看到菜单，
 * 这次大概率还该看到。真值一旦到达就立即覆盖（通常在几百毫秒内），
 * 所以管理员关掉功能后最多短暂多显示一次，不会长期错。
 *
 * localStorage 不可用（隐私模式、被禁用）时整层静默退化为 mode 默认值。
 */
const FEATURE_FLAG_MEMORY_KEY = 'sub2api:feature-flag-memory'

/** 模块级缓存：isFeatureFlagEnabled 会在 computed 里被高频调用，不能每次都读存储解析 JSON。 */
let flagMemoryCache: Record<string, boolean> | null = null

function flagMemory(): Record<string, boolean> {
  if (flagMemoryCache) return flagMemoryCache
  flagMemoryCache = {}
  try {
    const raw = globalThis.localStorage?.getItem(FEATURE_FLAG_MEMORY_KEY)
    if (raw) {
      const parsed: unknown = JSON.parse(raw)
      if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) {
        for (const [key, value] of Object.entries(parsed as Record<string, unknown>)) {
          if (typeof value === 'boolean') flagMemoryCache[key] = value
        }
      }
    }
  } catch {
    // 存储不可用或内容损坏：当作没有记忆，退回 mode 默认值。
  }
  return flagMemoryCache
}

function rememberFlag(key: string, value: boolean): void {
  const memory = flagMemory()
  // 值没变就不写：本函数在 computed 求值路径上，无谓的写入会拖慢侧边栏渲染。
  if (memory[key] === value) return
  memory[key] = value
  try {
    globalThis.localStorage?.setItem(FEATURE_FLAG_MEMORY_KEY, JSON.stringify(memory))
  } catch {
    // 写不进去不影响本次渲染，内存里的那份仍然生效。
  }
}

/**
 * Read the current value of a flag, honoring the mode's fallback.
 * `true`  → the feature is enabled (menu/route should render).
 * `false` → the feature is disabled (menu/route should hide).
 *
 * 优先级：已加载的真值 > 上次记住的值 > mode 声明的默认值。
 */
export function isFeatureFlagEnabled(flag: FeatureFlagDefinition): boolean {
  const appStore = useAppStore()
  return resolveFeatureFlag(appStore.cachedPublicSettings, flag)
}

/** Resolve a flag from an already-available settings object without Pinia access. */
export function resolveFeatureFlag(
  settings: Partial<PublicSettings> | null | undefined,
  flag: FeatureFlagDefinition,
): boolean {
  const raw = settings?.[flag.key] as boolean | undefined
  if (typeof raw === 'boolean') {
    rememberFlag(flag.key, raw)
    return raw
  }

  // 设置尚未加载：先用上次记住的值，避免菜单在每次刷新时闪一下。
  const remembered = flagMemory()[flag.key]
  if (typeof remembered === 'boolean') return remembered

  // 从未成功加载过（首次访问）→ 回落到 flag 声明的模式：
  //   opt-out → 默认可见，opt-in → 默认隐藏。
  return flag.mode === 'opt-out'
}

/**
 * Sidebar NavItem.featureFlag accepts a getter that returns
 * `false` to hide. Keeping the same contract lets callers swap in
 * registry-backed flags without changing AppSidebar's filter logic.
 */
export function makeSidebarFlag(flag: FeatureFlagDefinition): () => boolean {
  return () => isFeatureFlagEnabled(flag)
}

/** True when channel monitor feature flag is enabled. */
export function isChannelMonitorRouteEnabled(): boolean {
  return isFeatureFlagEnabled(FeatureFlags.channelMonitor)
}

export type ChannelMonitorMode = 'v1' | 'v2'

/** Exclusive channel-monitor implementation. Invalid/missing → v1 (opt-in to v2). */
export function getChannelMonitorMode(): ChannelMonitorMode {
  const appStore = useAppStore()
  const mode = appStore.cachedPublicSettings?.channel_monitor_mode
  return mode === 'v2' ? 'v2' : 'v1'
}

export function isChannelMonitorV1Mode(): boolean {
  return isChannelMonitorRouteEnabled() && getChannelMonitorMode() === 'v1'
}

export function isChannelMonitorV2Mode(): boolean {
  return isChannelMonitorRouteEnabled() && getChannelMonitorMode() === 'v2'
}

export function getChannelMonitorRefreshIntervalSeconds(): number {
  const appStore = useAppStore()
  const configured = appStore.cachedPublicSettings?.channel_monitor_default_interval_seconds
  return configured && configured > 0 ? configured : DEFAULT_INTERVAL_SECONDS
}

/** Hide RPM/TPM on user-facing monitor (scale privacy). Admin always shows full metrics. */
export function isChannelMonitorThroughputHidden(): boolean {
  const appStore = useAppStore()
  return Boolean(appStore.cachedPublicSettings?.channel_monitor_hide_throughput)
}

/**
 * Show quota/balance snapshots on the user-facing monitor page
 * (channel_monitor_show_quota, default off). The backend strips
 * latest_quota server-side when the switch is off; this flag is
 * defense-in-depth only. Admin views always show quota.
 */
export function isChannelMonitorQuotaVisible(): boolean {
  const appStore = useAppStore()
  return appStore.cachedPublicSettings?.channel_monitor_show_quota === true
}

/** Hide the user ranking tab on user-facing monitor v2. Admin always keeps it. */
export function isChannelMonitorUserRankingHidden(): boolean {
  const appStore = useAppStore()
  return Boolean(appStore.cachedPublicSettings?.channel_monitor_hide_user_ranking)
}
