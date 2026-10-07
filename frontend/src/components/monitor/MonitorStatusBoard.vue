<template>
  <div class="st">
    <p v-if="items.length" class="st-summary" aria-live="polite">
      <span class="st-dot ok"></span>{{ counts.ok }} {{ t('monitorBoard.ok') }}
      <span class="st-dot warn"></span>{{ counts.warn }} {{ t('monitorBoard.degraded') }}
      <span class="st-dot bad"></span>{{ counts.bad }} {{ t('monitorBoard.bad') }}
      <span class="st-summary-note">{{ t('monitorBoard.total', { n: items.length }) }}</span>
    </p>

    <div v-if="loading && items.length === 0" class="st-grid">
      <div v-for="i in 4" :key="i" class="st-skeleton animate-pulse"></div>
    </div>
    <div v-else-if="items.length === 0" class="st-empty">
      <h3>{{ emptyTitle || t('monitorBoard.emptyTitle') }}</h3>
      <p>{{ t('monitorBoard.emptyText') }}</p>
    </div>

    <section v-for="group in groups" :key="group.provider" class="st-group">
      <h2><span class="st-dot" :class="group.level"></span>{{ providerLabel(group.provider) }}</h2>
      <div class="st-grid">
        <article v-for="item in group.items" :key="item.id" class="lc-monitor-card st-item">
          <div class="st-item-head">
            <span class="st-icon" :style="{ color: platformAccentColor(item.provider) }"><PlatformIcon :platform="item.provider" size="sm" /></span>
            <div class="st-name">
              <b>{{ item.name }}</b>
              <span>{{ formatMonitorModel(item.primary_model) }}</span>
            </div>
            <span class="lc-status" :class="statusClass(item.primary_status)">{{ statusLabel(item.primary_status) }}</span>
          </div>

          <div class="lc-timeline st-bar" role="img" :aria-label="timelineLabel(item)">
            <i
              v-for="(point, index) in normalizedTimeline(item)"
              :key="index"
              :class="statusClass(point?.status)"
              :title="timelineTitle(point)"
            ></i>
          </div>
          <div class="st-axis"><span>{{ oldestLabel(item) }}</span><span>{{ t('monitorBoard.now') }}</span></div>

          <dl class="st-metrics">
            <div :title="latencyHint(item)">
              <dt>{{ t('monitorBoard.firstToken') }}</dt>
              <dd>{{ formatLatency(item.primary_first_token_ms ?? item.primary_latency_ms) }}</dd>
            </div>
            <div><dt>{{ t('monitorBoard.ping') }}</dt><dd>{{ formatLatency(item.primary_ping_latency_ms) }}</dd></div>
            <div>
              <dt>{{ t('monitorBoard.availability', { window: windowLabel }) }}</dt>
              <dd>{{ formatAvailability(resolveAvailability(item)) }}</dd>
            </div>
            <button type="button" class="st-detail" @click="emit('detail', item)">{{ t('monitorBoard.detail') }}</button>
          </dl>

          <!-- 配额模式快照：服务端已按系统开关剥离，这里再按开关做一次纵深防御 -->
          <MonitorQuotaView v-if="quotaVisible(item)" class="st-quota" :snapshot="item.latest_quota" />

          <ul v-if="item.extra_models?.length" class="st-extra" :aria-label="t('monitorBoard.extraModels')">
            <li v-for="extra in item.extra_models" :key="extra.model" :title="statusLabel(extra.status)">
              <span class="st-dot" :class="dotClass(extra.status)"></span>{{ extra.model }}
            </li>
          </ul>
        </article>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
// 号池状态列表：按厂商分组，每个号池一条 72 段状态条 + 首字延迟 / Ping / 可用率。
// 公开可用性页与控制台渠道状态页共用；数据加载、时间窗切换与详情弹窗由页面负责。
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { MonitorTimelinePoint, UserMonitorDetail, UserMonitorView } from '@/api/channelMonitor'
import PlatformIcon from '@/components/common/PlatformIcon.vue'
import MonitorQuotaView from '@/components/common/MonitorQuotaView.vue'
import { STATUS_DEGRADED } from '@/constants/channelMonitor'
import { useChannelMonitorFormat } from '@/composables/useChannelMonitorFormat'
import { isChannelMonitorQuotaVisible } from '@/utils/featureFlags'
import { platformAccentColor } from '@/utils/platformColors'

type MonitorWindow = '7d' | '15d' | '30d'

const TIMELINE_POINTS = 72
const PROVIDER_ORDER = ['anthropic', 'openai', 'gemini', 'grok', 'deepseek', 'kimi', 'zhipu', 'minimax', 'antigravity', 'opencode_go']
const PROVIDER_LABELS: Record<string, string> = {
  anthropic: 'Claude', openai: 'GPT', gemini: 'Gemini', grok: 'Grok', deepseek: 'DeepSeek', kimi: 'Kimi',
  zhipu: 'GLM', minimax: 'MiniMax', antigravity: 'Antigravity', opencode_go: 'OpenCode',
}

const props = withDefaults(defineProps<{
  items: UserMonitorView[]
  window: MonitorWindow
  windowLabel: string
  detailCache: Record<number, UserMonitorDetail>
  loading?: boolean
  showQuota?: boolean
  emptyTitle?: string
}>(), {
  loading: false,
  showQuota: false,
  emptyTitle: '',
})

const emit = defineEmits<{ (e: 'detail', item: UserMonitorView): void }>()
const { t, locale } = useI18n()
// 纯配额模式的主模型是占位符 "quota"，展示为本地化的「配额」标签
const { formatMonitorModel } = useChannelMonitorFormat()

// 组标题的圆点取组内最差状态，一眼看出哪家有问题
const groups = computed(() => {
  const map = new Map<string, UserMonitorView[]>()
  for (const item of props.items) map.set(item.provider, [...(map.get(item.provider) ?? []), item])
  const rank = (p: string) => (PROVIDER_ORDER.indexOf(p) === -1 ? 99 : PROVIDER_ORDER.indexOf(p))
  return [...map.entries()]
    .sort((a, b) => rank(a[0]) - rank(b[0]))
    .map(([provider, list]) => ({ provider, items: list, level: worstLevel(list) }))
})

const counts = computed(() => {
  const levels = props.items.map(item => dotClass(item.primary_status))
  return {
    ok: levels.filter(level => level === 'ok').length,
    warn: levels.filter(level => level === 'warn').length,
    bad: levels.filter(level => level === 'bad').length,
  }
})

function isOperational(status?: string) { return status === 'operational' || status === 'success' }
function isDegraded(status?: string) { return status === STATUS_DEGRADED }
function isUnknown(status?: string) { return !status || status === 'unknown' }
function statusClass(status?: string) { return isOperational(status) ? '' : isDegraded(status) ? 'degraded' : isUnknown(status) ? 'unknown' : 'bad' }
function statusLabel(status?: string) {
  const key = isOperational(status) ? 'ok' : isDegraded(status) ? 'degraded' : isUnknown(status) ? 'unknown' : 'bad'
  return t(`monitorBoard.${key}`)
}
function dotClass(status?: string) { return isOperational(status) ? 'ok' : isDegraded(status) ? 'warn' : isUnknown(status) ? 'off' : 'bad' }
function worstLevel(list: UserMonitorView[]) {
  const levels = list.map(item => dotClass(item.primary_status))
  return levels.includes('bad') ? 'bad' : levels.includes('warn') ? 'warn' : levels.includes('ok') ? 'ok' : 'off'
}
function providerLabel(provider: string) { return PROVIDER_LABELS[provider] || provider }

function formatLatency(value?: number | null) {
  if (value == null) return '--'
  return value >= 1000 ? `${(value / 1000).toFixed(2)} s` : `${Math.round(value)} ms`
}
// 有真实调用时展示近 1 小时平均首字延迟，否则退回探测请求的整次对话耗时
function latencyHint(item: UserMonitorView) {
  return t(item.primary_first_token_ms != null ? 'monitorBoard.firstTokenRealHint' : 'monitorBoard.firstTokenProbeHint')
}
function formatAvailability(value: number | null) { return value == null ? '--' : `${Number(value).toFixed(2)}%` }
function resolveAvailability(item: UserMonitorView) {
  if (props.window === '7d') return item.availability_7d ?? null
  const primary = props.detailCache[item.id]?.models.find(model => model.model === item.primary_model)
  return props.window === '15d' ? primary?.availability_15d ?? null : primary?.availability_30d ?? null
}
function quotaVisible(item: UserMonitorView) {
  return props.showQuota && isChannelMonitorQuotaVisible() && !!item.latest_quota
}

function recentPoints(item: UserMonitorView) { return (item.timeline || []).slice(0, TIMELINE_POINTS) }
function normalizedTimeline(item: UserMonitorView): Array<MonitorTimelinePoint | null> {
  const points = recentPoints(item).reverse()
  const missing = Array.from({ length: TIMELINE_POINTS - points.length }, () => null)
  return [...missing, ...points]
}
function formatTime(value: string) {
  const date = new Date(value)
  const tag = locale.value === 'zh' ? 'zh-CN' : 'en-US'
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString(tag, { hour12: false })
}
function timelineTitle(point: MonitorTimelinePoint | null) {
  if (!point) return ''
  return `${formatTime(point.checked_at)} · ${statusLabel(point.status)} · ${formatLatency(point.latency_ms)}`
}
// 左端标签：最早一次探测距今多久
function oldestLabel(item: UserMonitorView) {
  const oldest = recentPoints(item).at(-1)
  if (!oldest) return '--'
  const minutes = Math.round((Date.now() - new Date(oldest.checked_at).getTime()) / 60000)
  if (!Number.isFinite(minutes) || minutes < 0) return formatTime(oldest.checked_at)
  if (minutes < 60) return t('monitorBoard.minutesAgo', { n: minutes })
  if (minutes < 60 * 48) return t('monitorBoard.hoursAgo', { n: Math.round(minutes / 60) })
  return t('monitorBoard.daysAgo', { n: Math.round(minutes / 1440) })
}
function timelineLabel(item: UserMonitorView) {
  const points = recentPoints(item)
  return t('monitorBoard.timelineLabel', { total: points.length, ok: points.filter(point => isOperational(point.status)).length })
}
</script>

<style scoped>
.st {
  /* 色条刻意用高饱和状态色，异常与降级一眼可见；正文里的状态胶囊仍用全站克制的状态色 */
  --st-ok: #22c55e;
  --st-warn: #e69500;
  --st-bad: #dc2626;
}

.st-summary { display: flex; flex-wrap: wrap; align-items: center; gap: 6px 10px; padding: 12px 16px; border: 1px solid var(--lc-line); border-radius: 12px; background: var(--lc-surface); font-size: 13.5px; color: var(--lc-ink); }
.st-summary .st-dot { margin-left: 8px; }
.st-summary .st-dot:first-child { margin-left: 0; }
.st-summary-note { margin-left: auto; color: var(--lc-ink-3); font-size: 12.5px; }
.st-dot { display: inline-block; flex: none; width: 8px; height: 8px; border-radius: 50%; background: var(--lc-ink-3); }
.st-dot.ok { background: var(--st-ok); }
.st-dot.warn { background: var(--st-warn); }
.st-dot.bad { background: var(--st-bad); }

/*
 * 排布与尺寸对齐 Krill 状态页：两列（列距 32px、行距 16px），每项底部 16px；
 * 号池名 14px，名称到状态条 12px；状态条 72 格平分宽度、固定 20px 高、格距 1px，
 * 大屏只会让格子变宽，不会被拉高。
 */
.st-group { margin-top: 24px; padding-top: 20px; border-top: 1px solid var(--lc-line); }
.st-group:first-of-type { border-top: 0; padding-top: 0; }
.st-group h2 { display: flex; align-items: center; gap: 8px; margin-bottom: 16px; font-size: 14px; font-weight: 600; }
.st-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 16px 32px; }
.st-item { display: block; min-width: 0; min-height: 0; padding: 0 0 16px; text-align: left; }
.st-item-head { display: flex; align-items: center; gap: 8px; min-width: 0; }
.st-icon { display: grid; place-items: center; flex: none; width: 18px; height: 18px; color: var(--lc-ink); }
.st-name { display: flex; align-items: baseline; gap: 8px; flex: 1; min-width: 0; }
.st-name b { overflow: hidden; font-size: 14px; font-weight: 600; line-height: 20px; text-overflow: ellipsis; white-space: nowrap; }
.st-name span { overflow: hidden; color: var(--lc-ink-3); font: 12px var(--lc-font-mono); text-overflow: ellipsis; white-space: nowrap; }
.st-bar { display: flex; gap: 1px; margin-top: 12px; }
.st-bar i { flex: 1 1 0; min-width: 0; height: 20px; border-radius: 2px; opacity: 1; background: var(--st-ok); }
.st-bar i.degraded { background: var(--st-warn); }
.st-bar i.bad { background: var(--st-bad); }
.st-bar i.unknown { background: var(--lc-line-2); }
.st-axis { display: flex; justify-content: space-between; margin-top: 4px; color: var(--lc-ink-3); font-size: 10px; line-height: 15px; }
.st-metrics { display: flex; flex-wrap: wrap; align-items: baseline; gap: 4px 20px; margin-top: 10px; font-size: 12px; line-height: 16px; }
.st-metrics div { display: flex; align-items: baseline; gap: 6px; }
.st-metrics dt { color: var(--lc-ink-3); }
.st-metrics dd { color: var(--lc-ink); font-weight: 500; font-variant-numeric: tabular-nums; }
.st-detail { margin-left: auto; color: var(--lc-accent); font-size: 12px; font-weight: 500; }
.st-detail:hover { text-decoration: underline; }
.st-quota { margin-top: 10px; }
.st-extra { display: flex; flex-wrap: wrap; gap: 6px; margin-top: 10px; }
.st-extra li { display: inline-flex; align-items: center; gap: 6px; height: 26px; padding: 0 9px; border: 1px solid var(--lc-line); border-radius: 999px; color: var(--lc-ink-2); font: 12px var(--lc-font-mono); }
.st-extra .st-dot { width: 6px; height: 6px; }
.st-skeleton { height: 97px; border-radius: 12px; background: var(--lc-surface-2); }
.st-empty { padding: 48px; border: 1px dashed var(--lc-line-2); border-radius: 12px; text-align: center; color: var(--lc-ink-2); }
.st-empty h3 { color: var(--lc-ink); font-size: 16px; font-weight: 600; }
.st-empty p { margin-top: 6px; font-size: 14px; }

@media (max-width: 900px) {
  .st-grid { grid-template-columns: minmax(0, 1fr); gap: 28px; }
}

@media (max-width: 640px) {
  .st-bar i { height: 18px; }
  .st-summary-note { margin-left: 0; flex-basis: 100%; }
}
</style>
