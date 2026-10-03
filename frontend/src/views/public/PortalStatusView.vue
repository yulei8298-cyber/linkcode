<template>
  <PortalLayout>
    <div class="lc-wrap st">
      <header class="st-head">
        <div>
          <p class="st-crumb"><RouterLink to="/home">首页</RouterLink> / 可用性检测</p>
          <h1>可用性检测</h1>
          <p class="st-sub">定时用真实请求探测各号池，色块从左到右是最近 60 次结果。无需登录即可查看。</p>
        </div>
        <div class="st-tools">
          <div class="st-seg" role="group" aria-label="可用率时间窗口">
            <button
              v-for="option in windows"
              :key="option.value"
              type="button"
              :aria-pressed="currentWindow === option.value"
              @click="changeWindow(option.value)"
            >{{ option.label }}</button>
          </div>
          <button type="button" class="st-refresh" :disabled="loading" @click="manualReload">
            <Icon name="refresh" size="sm" :class="{ 'animate-spin': loading }" />
            {{ countdown }}s 后刷新
          </button>
        </div>
      </header>

      <p v-if="items.length" class="st-summary" aria-live="polite">
        <span class="st-dot ok"></span>{{ counts.ok }} 正常
        <span class="st-dot warn"></span>{{ counts.warn }} 降级
        <span class="st-dot bad"></span>{{ counts.bad }} 异常
        <span class="st-summary-note">共 {{ items.length }} 个号池</span>
      </p>

      <div v-if="loading && items.length === 0" class="st-grid">
        <div v-for="i in 4" :key="i" class="st-skeleton animate-pulse"></div>
      </div>
      <div v-else-if="items.length === 0" class="st-empty">
        <h3>暂无公开监控</h3>
        <p>后台配置监控后，数据会自动显示在这里。</p>
      </div>

      <section v-for="group in groups" :key="group.provider" class="st-group">
        <h2><span class="st-dot" :class="group.level"></span>{{ providerLabel(group.provider) }}</h2>
        <div class="st-grid">
          <article v-for="item in group.items" :key="item.id" class="lc-monitor-card st-item">
            <div class="st-item-head">
              <span class="st-icon"><PlatformIcon :platform="item.provider" size="sm" /></span>
              <div class="st-name">
                <b>{{ item.name }}</b>
                <span>{{ item.primary_model }}</span>
              </div>
              <span class="lc-status" :class="statusClass(item.primary_status)">{{ statusLabel(item.primary_status) }}</span>
            </div>

            <div class="lc-timeline st-bar" role="img" :aria-label="timelineLabel(item)">
              <i
                v-for="(point, index) in normalizedTimeline(item)"
                :key="index"
                :class="timelineClass(point?.status)"
                :title="timelineTitle(point)"
              ></i>
            </div>
            <div class="st-axis"><span>{{ oldestLabel(item) }}</span><span>现在</span></div>

            <dl class="st-metrics">
              <div :title="latencyHint(item)"><dt>首字延迟</dt><dd>{{ formatLatency(item.primary_first_token_ms ?? item.primary_latency_ms) }}</dd></div>
              <div><dt>端点 Ping</dt><dd>{{ formatLatency(item.primary_ping_latency_ms) }}</dd></div>
              <div><dt>可用率 · {{ currentWindowLabel }}</dt><dd>{{ formatAvailability(resolveAvailability(item)) }}</dd></div>
              <button type="button" class="st-detail" @click="openDetail(item)">详情</button>
            </dl>

            <ul v-if="item.extra_models?.length" class="st-extra" aria-label="同号池其他模型">
              <li v-for="extra in item.extra_models" :key="extra.model" :title="statusLabel(extra.status)">
                <span class="st-dot" :class="dotClass(extra.status)"></span>{{ extra.model }}
              </li>
            </ul>
          </article>
        </div>
      </section>
    </div>

    <PortalMonitorDetailDialog :show="showDetail" :monitor-id="detailTarget?.id ?? null" :title="detailTarget?.name || '渠道详情'" @close="closeDetail" />
  </PortalLayout>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import { RouterLink } from 'vue-router'
import { useAppStore } from '@/stores/app'
import { getPublicMonitors, getPublicMonitorStatus } from '@/api/public'
import type { MonitorTimelinePoint, UserMonitorDetail, UserMonitorView } from '@/api/channelMonitor'
import PortalLayout from './components/PortalLayout.vue'
import PortalMonitorDetailDialog from './components/PortalMonitorDetailDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import PlatformIcon from '@/components/common/PlatformIcon.vue'
import { useAutoRefresh } from '@/composables/useAutoRefresh'
import { extractApiErrorMessage } from '@/utils/apiError'
import { STATUS_DEGRADED } from '@/constants/channelMonitor'

type MonitorWindow = '7d' | '15d' | '30d'
const windows: { value: MonitorWindow; label: string }[] = [{ value: '7d', label: '7 天' }, { value: '15d', label: '15 天' }, { value: '30d', label: '30 天' }]
const PROVIDER_ORDER = ['anthropic', 'openai', 'gemini', 'grok', 'deepseek', 'kimi', 'zhipu', 'minimax', 'antigravity', 'opencode_go']
const PROVIDER_LABELS: Record<string, string> = {
  anthropic: 'Claude', openai: 'GPT', gemini: 'Gemini', grok: 'Grok', deepseek: 'DeepSeek', kimi: 'Kimi',
  zhipu: 'GLM', minimax: 'MiniMax', antigravity: 'Antigravity', opencode_go: 'OpenCode',
}

const appStore = useAppStore()
const items = ref<UserMonitorView[]>([])
const loading = ref(false)
const currentWindow = ref<MonitorWindow>('7d')
const detailCache = reactive<Record<number, UserMonitorDetail>>({})
const showDetail = ref(false)
const detailTarget = ref<UserMonitorView | null>(null)
let abortController: AbortController | null = null

const currentWindowLabel = computed(() => windows.find(item => item.value === currentWindow.value)?.label || '7 天')
const autoRefresh = useAutoRefresh({ storageKey: 'portal-status-auto-refresh', intervals: [30, 60, 120] as const, defaultInterval: 30, onRefresh: () => reload(true), shouldPause: () => document.hidden || loading.value })
const countdown = autoRefresh.countdown

// 按厂商分组；组标题的圆点取组内最差状态，一眼看出哪家有问题
const groups = computed(() => {
  const map = new Map<string, UserMonitorView[]>()
  for (const item of items.value) map.set(item.provider, [...(map.get(item.provider) ?? []), item])
  const rank = (p: string) => (PROVIDER_ORDER.indexOf(p) === -1 ? 99 : PROVIDER_ORDER.indexOf(p))
  return [...map.entries()]
    .sort((a, b) => rank(a[0]) - rank(b[0]))
    .map(([provider, list]) => ({ provider, items: list, level: worstLevel(list) }))
})
const counts = computed(() => {
  const levels = items.value.map(item => dotClass(item.primary_status))
  return {
    ok: levels.filter(level => level === 'ok').length,
    warn: levels.filter(level => level === 'warn').length,
    bad: levels.filter(level => level === 'bad').length,
  }
})

async function reload(silent = false) {
  abortController?.abort()
  const controller = new AbortController()
  abortController = controller
  if (!silent) loading.value = true
  try {
    const response = await getPublicMonitors({ signal: controller.signal })
    if (!controller.signal.aborted) items.value = response.items || []
  } catch (error: unknown) {
    const reason = error as { name?: string; code?: string }
    if (reason.name !== 'AbortError' && reason.code !== 'ERR_CANCELED') appStore.showError(extractApiErrorMessage(error, '加载监控数据失败'))
  } finally {
    if (abortController === controller) {
      loading.value = false
      countdown.value = 30
      abortController = null
    }
  }
}

async function loadDetails() {
  if (currentWindow.value === '7d') return
  await Promise.all(items.value.map(async item => {
    if (detailCache[item.id]) return
    try { detailCache[item.id] = await getPublicMonitorStatus(item.id) } catch { /* 拉不到详情时保留 7 天数据 */ }
  }))
}

async function changeWindow(value: MonitorWindow) { currentWindow.value = value; await loadDetails() }
async function manualReload() { await reload(false); await loadDetails() }
function openDetail(item: UserMonitorView) { detailTarget.value = item; showDetail.value = true }
function closeDetail() { showDetail.value = false; detailTarget.value = null }
function isOperational(status: string) { return status === 'operational' || status === 'success' }
function isDegraded(status?: string) { return status === STATUS_DEGRADED }
function isUnknown(status?: string) { return !status || status === 'unknown' }
function statusClass(status?: string) { return isOperational(status || '') ? '' : isDegraded(status) ? 'degraded' : isUnknown(status) ? 'unknown' : 'bad' }
function statusLabel(status: string) { return isOperational(status) ? '正常' : isDegraded(status) ? '降级' : isUnknown(status) ? '未知' : '异常' }
function dotClass(status?: string) { return isOperational(status || '') ? 'ok' : isDegraded(status) ? 'warn' : isUnknown(status) ? 'off' : 'bad' }
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
function hasRealFirstToken(item: UserMonitorView) { return item.primary_first_token_ms != null }
function latencyHint(item: UserMonitorView) {
  return hasRealFirstToken(item) ? '近 1 小时用户真实调用的平均首字耗时' : '近期没有用户调用，显示最近一次探测的整次对话耗时'
}
function formatAvailability(value: number | null) { return value == null ? '--' : `${Number(value).toFixed(2)}%` }
function resolveAvailability(item: UserMonitorView) {
  if (currentWindow.value === '7d') return item.availability_7d ?? null
  const primary = detailCache[item.id]?.models.find(model => model.model === item.primary_model)
  return currentWindow.value === '15d' ? primary?.availability_15d ?? null : primary?.availability_30d ?? null
}
function normalizedTimeline(item: UserMonitorView): Array<MonitorTimelinePoint | null> {
  const points = (item.timeline || []).slice(0, 60).reverse()
  const missing = Array.from({ length: 60 - points.length }, () => null)
  return [...missing, ...points]
}
function timelineClass(status?: string) { return statusClass(status) }
function formatTime(value: string) {
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString('zh-CN', { hour12: false })
}
function timelineTitle(point: MonitorTimelinePoint | null) {
  if (!point) return ''
  return `${formatTime(point.checked_at)} · ${statusLabel(point.status)} · ${formatLatency(point.latency_ms)}`
}
// 左端标签：最早一次探测距今多久
function oldestLabel(item: UserMonitorView) {
  const oldest = (item.timeline || []).slice(0, 60).at(-1)
  if (!oldest) return '--'
  const minutes = Math.round((Date.now() - new Date(oldest.checked_at).getTime()) / 60000)
  if (!Number.isFinite(minutes) || minutes < 0) return formatTime(oldest.checked_at)
  if (minutes < 60) return `${minutes} 分钟前`
  if (minutes < 60 * 48) return `${Math.round(minutes / 60)} 小时前`
  return `${Math.round(minutes / 1440)} 天前`
}
function timelineLabel(item: UserMonitorView) {
  const points = (item.timeline || []).slice(0, 60)
  const ok = points.filter(point => isOperational(point.status)).length
  return `最近 ${points.length} 次探测中 ${ok} 次正常`
}

onMounted(() => { void reload(false); autoRefresh.setEnabled(true) })
onBeforeUnmount(() => abortController?.abort())
</script>

<style scoped>
.st {
  /* 色条用更明快的状态色，便于远看分辨；正文状态胶囊仍用全站状态色 */
  --st-ok: #34b36a;
  --st-warn: #e5a23a;
  --st-bad: #e0584b;
  padding: 48px 0 72px;
}
:global(.dark) .st {
  --st-ok: #3fbf74;
  --st-warn: #e8a948;
  --st-bad: #ec6a5d;
}
.st-head { display: flex; flex-wrap: wrap; align-items: flex-end; justify-content: space-between; gap: 20px; }
.st-crumb { color: var(--lc-ink-3); font: 12.5px var(--lc-font-mono); }
.st-crumb a:hover { color: var(--lc-ink); }
.st-head h1 { margin-top: 10px; font-size: clamp(28px, 3.4vw, 40px); font-weight: 700; letter-spacing: -0.02em; }
.st-sub { margin-top: 8px; max-width: 40em; color: var(--lc-ink-2); font-size: 15px; }
.st-tools { display: flex; flex-wrap: wrap; align-items: center; gap: 8px; }
.st-seg { display: inline-flex; gap: 2px; padding: 3px; border: 1px solid var(--lc-line); border-radius: 8px; background: var(--lc-bg-2); }
.st-seg button { height: 28px; padding: 0 12px; border-radius: 6px; color: var(--lc-ink-2); font-size: 13px; }
.st-seg button[aria-pressed='true'] { background: var(--lc-surface-3); color: var(--lc-ink); box-shadow: inset 0 1px 0 var(--lc-hi); }
.st-refresh { display: inline-flex; align-items: center; gap: 6px; height: 36px; padding: 0 12px; border: 1px solid var(--lc-line-2); border-radius: 8px; background: var(--lc-surface); color: var(--lc-ink-2); font: 12.5px var(--lc-font-mono); }
.st-refresh:hover { color: var(--lc-ink); }

.st-summary { display: flex; flex-wrap: wrap; align-items: center; gap: 6px 10px; margin-top: 28px; padding: 12px 16px; border: 1px solid var(--lc-line); border-radius: 12px; background: var(--lc-surface); font-size: 13.5px; color: var(--lc-ink); }
.st-summary .st-dot { margin-left: 8px; }
.st-summary .st-dot:first-child { margin-left: 0; }
.st-summary-note { margin-left: auto; color: var(--lc-ink-3); font-size: 12.5px; }
.st-dot { display: inline-block; flex: none; width: 8px; height: 8px; border-radius: 50%; background: var(--lc-ink-3); }
.st-dot.ok { background: var(--lc-ok); }
.st-dot.warn { background: var(--lc-warn); }
.st-dot.bad { background: var(--lc-bad); }

.st-group { margin-top: 32px; padding-top: 28px; border-top: 1px solid var(--lc-line); }
.st-group:first-of-type { border-top: 0; padding-top: 0; }
.st-group h2 { display: flex; align-items: center; gap: 10px; margin-bottom: 20px; font-size: 17px; font-weight: 600; }
.st-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 32px 48px; }
.st-item { display: grid; gap: 10px; min-width: 0; min-height: 0; padding: 0; text-align: left; }
.st-item-head { display: flex; align-items: center; gap: 10px; min-width: 0; }
.st-icon { display: grid; place-items: center; flex: none; width: 28px; height: 28px; border-radius: 7px; background: var(--lc-pm-bg); color: var(--lc-ink); }
.st-name { display: flex; align-items: baseline; gap: 10px; flex: 1; min-width: 0; }
.st-name b { overflow: hidden; font-size: 15.5px; font-weight: 600; text-overflow: ellipsis; white-space: nowrap; }
.st-name span { overflow: hidden; color: var(--lc-ink-3); font: 12.5px var(--lc-font-mono); text-overflow: ellipsis; white-space: nowrap; }
.st-bar { margin-top: 2px; gap: 2px; }
.st-bar i { height: 26px; border-radius: 3px; opacity: 1; background: var(--st-ok); }
.st-bar i.degraded { background: var(--st-warn); }
.st-bar i.bad { background: var(--st-bad); }
.st-bar i.unknown { background: var(--lc-line-2); }
.st-axis { display: flex; justify-content: space-between; margin-top: -4px; color: var(--lc-ink-3); font-size: 12px; }
.st-metrics { display: flex; flex-wrap: wrap; align-items: baseline; gap: 6px 24px; font-size: 13px; }
.st-metrics div { display: flex; align-items: baseline; gap: 8px; }
.st-metrics dt { color: var(--lc-ink-3); }
.st-metrics dd { color: var(--lc-ink); font-family: var(--lc-font-mono); font-weight: 600; font-variant-numeric: tabular-nums; }
.st-detail { margin-left: auto; color: var(--lc-accent); font-size: 13px; font-weight: 500; }
.st-detail:hover { text-decoration: underline; }
.st-extra { display: flex; flex-wrap: wrap; gap: 6px; }
.st-extra li { display: inline-flex; align-items: center; gap: 6px; height: 26px; padding: 0 9px; border: 1px solid var(--lc-line); border-radius: 999px; color: var(--lc-ink-2); font: 12px var(--lc-font-mono); }
.st-extra .st-dot { width: 6px; height: 6px; }
.st-skeleton { height: 150px; border-radius: 12px; background: var(--lc-surface-2); }
.st-empty { margin-top: 32px; padding: 48px; border: 1px dashed var(--lc-line-2); border-radius: 12px; text-align: center; color: var(--lc-ink-2); }
.st-empty h3 { color: var(--lc-ink); font-size: 16px; font-weight: 600; }
.st-empty p { margin-top: 6px; font-size: 14px; }

@media (max-width: 900px) {
  .st-grid { grid-template-columns: minmax(0, 1fr); gap: 28px; }
}

@media (max-width: 640px) {
  .st { padding: 32px 0 56px; }
  .st-bar i { height: 22px; border-radius: 2px; }
  .st-bar { gap: 1px; }
  .st-summary-note { margin-left: 0; flex-basis: 100%; }
}
</style>
