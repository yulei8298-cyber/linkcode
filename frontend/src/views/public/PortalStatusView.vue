<template>
  <PortalLayout>
    <section class="lc-page-head">
      <div class="lc-wrap lc-page-head-inner">
        <div class="lc-crumb"><RouterLink to="/home">首页</RouterLink> / 可用性检测</div>
        <h1 class="lc-page-title">渠道<span>可用性检测</span></h1>
        <p class="lc-lead">展示现有监控渠道的实时状态、延迟与可用率。数据自动刷新，无需登录即可查看。</p>
      </div>
    </section>

    <div class="lc-wrap">
      <div class="lc-toolbar">
        <div class="lc-segments" aria-label="可用率时间窗口">
          <button v-for="option in windows" :key="option.value" type="button" class="lc-segment" :class="{ active: currentWindow === option.value }" @click="changeWindow(option.value)">{{ option.label }}</button>
        </div>
        <button type="button" class="lc-button lc-button-small" :disabled="loading" @click="manualReload">
          <Icon name="refresh" size="sm" :class="{ 'animate-spin': loading }" />
          自动刷新 {{ countdown }}s
        </button>
      </div>

      <div v-if="loading && items.length === 0" class="lc-monitor-grid">
        <div v-for="i in 6" :key="i" class="lc-card lc-monitor-card animate-pulse"></div>
      </div>
      <div v-else-if="items.length === 0" class="lc-card lc-empty">
        <h3>暂无公开监控渠道</h3>
        <p>后台配置监控后，数据会自动显示在这里。</p>
      </div>
      <div v-else class="lc-monitor-grid">
        <button v-for="item in items" :key="item.id" type="button" class="lc-card lc-monitor-card" @click="openDetail(item)">
          <div class="lc-monitor-top">
            <span class="lc-provider">{{ providerCode(item.provider) }}</span>
            <div class="lc-monitor-name"><b>{{ item.name }}</b><div class="lc-monitor-meta">{{ providerLabel(item.provider) }} · {{ item.primary_model }}</div></div>
            <span class="lc-status" :class="statusClass(item.primary_status)">{{ statusLabel(item.primary_status) }}</span>
          </div>
          <div class="lc-metrics">
            <div class="lc-metric"><small>模型延迟</small><b>{{ formatLatency(item.primary_latency_ms) }}</b></div>
            <div class="lc-metric"><small>端点 Ping</small><b>{{ formatLatency(item.primary_ping_latency_ms) }}</b></div>
          </div>
          <div class="lc-availability"><small>可用率 · {{ currentWindowLabel }}<template v-if="item.extra_models?.length"> · 另含 {{ item.extra_models.length }} 个模型</template></small><b>{{ formatAvailability(resolveAvailability(item)) }}</b></div>
          <div class="lc-timeline" aria-hidden="true">
            <i v-for="(point, index) in normalizedTimeline(item)" :key="index" :class="timelineClass(point?.status)" :title="timelineTitle(point)"></i>
          </div>
        </button>
      </div>
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
import { useAutoRefresh } from '@/composables/useAutoRefresh'
import { extractApiErrorMessage } from '@/utils/apiError'
import { STATUS_DEGRADED } from '@/constants/channelMonitor'

type MonitorWindow = '7d' | '15d' | '30d'
const windows: { value: MonitorWindow; label: string }[] = [{ value: '7d', label: '7 天' }, { value: '15d', label: '15 天' }, { value: '30d', label: '30 天' }]
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
    try { detailCache[item.id] = await getPublicMonitorStatus(item.id) } catch { /* card keeps its 7-day fallback */ }
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
function providerCode(provider: string) { return ({ anthropic: 'AN', openai: 'OA', gemini: 'GM', grok: 'XA' } as Record<string,string>)[provider] || provider.slice(0, 2).toUpperCase() }
function providerLabel(provider: string) { return ({ anthropic: 'ANTHROPIC', openai: 'OPENAI', gemini: 'GEMINI', grok: 'xAI' } as Record<string,string>)[provider] || provider.toUpperCase() }
function formatLatency(value: number | null) { return value == null ? '--' : `${Math.round(value)} ms` }
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
function timelineTitle(point: MonitorTimelinePoint | null) {
  if (!point) return ''
  const checkedAt = new Date(point.checked_at)
  const checkedAtLabel = Number.isNaN(checkedAt.getTime()) ? point.checked_at : checkedAt.toLocaleString('zh-CN', { hour12: false })
  return `${checkedAtLabel} · ${statusLabel(point.status)} · ${formatLatency(point.latency_ms)}`
}

onMounted(() => { void reload(false); autoRefresh.setEnabled(true) })
onBeforeUnmount(() => abortController?.abort())
</script>
