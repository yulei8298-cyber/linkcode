<template>
  <PortalLayout>
    <div class="lc-wrap ps">
      <header class="ps-head">
        <div>
          <p class="ps-crumb"><RouterLink to="/home">首页</RouterLink> / 可用性检测</p>
          <h1>可用性检测</h1>
          <p class="ps-sub">定时用真实请求探测各号池，色块从左到右是最近 60 次结果。无需登录即可查看。</p>
        </div>
        <div class="ps-tools">
          <div class="ps-seg" role="group" aria-label="可用率时间窗口">
            <button
              v-for="option in windows"
              :key="option.value"
              type="button"
              :aria-pressed="currentWindow === option.value"
              @click="changeWindow(option.value)"
            >{{ option.label }}</button>
          </div>
          <button type="button" class="ps-refresh" :disabled="loading" @click="manualReload">
            <Icon name="refresh" size="sm" :class="{ 'animate-spin': loading }" />
            {{ countdown }}s 后刷新
          </button>
        </div>
      </header>

      <MonitorStatusBoard
        class="ps-board"
        :items="items"
        :window="currentWindow"
        :window-label="currentWindowLabel"
        :detail-cache="detailCache"
        :loading="loading"
        empty-title="暂无公开监控"
        @detail="openDetail"
      />
    </div>

    <PortalMonitorDetailDialog :show="showDetail" :monitor-id="detailTarget?.id ?? null" :title="detailTarget?.name || '渠道详情'" @close="closeDetail" />
  </PortalLayout>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import { RouterLink } from 'vue-router'
import { useAppStore } from '@/stores/app'
import { getPublicMonitors, getPublicMonitorStatus } from '@/api/public'
import type { UserMonitorDetail, UserMonitorView } from '@/api/channelMonitor'
import PortalLayout from './components/PortalLayout.vue'
import PortalMonitorDetailDialog from './components/PortalMonitorDetailDialog.vue'
import MonitorStatusBoard from '@/components/monitor/MonitorStatusBoard.vue'
import Icon from '@/components/icons/Icon.vue'
import { useAutoRefresh } from '@/composables/useAutoRefresh'
import { extractApiErrorMessage } from '@/utils/apiError'

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
    try { detailCache[item.id] = await getPublicMonitorStatus(item.id) } catch { /* 拉不到详情时保留 7 天数据 */ }
  }))
}

async function changeWindow(value: MonitorWindow) { currentWindow.value = value; await loadDetails() }
async function manualReload() { await reload(false); await loadDetails() }
function openDetail(item: UserMonitorView) { detailTarget.value = item; showDetail.value = true }
function closeDetail() { showDetail.value = false; detailTarget.value = null }

onMounted(() => { void reload(false); autoRefresh.setEnabled(true) })
onBeforeUnmount(() => abortController?.abort())
</script>

<style scoped>
.ps { padding: 48px 0 72px; }
.ps-head { display: flex; flex-wrap: wrap; align-items: flex-end; justify-content: space-between; gap: 20px; }
.ps-crumb { color: var(--lc-ink-3); font: 12.5px var(--lc-font-mono); }
.ps-crumb a:hover { color: var(--lc-ink); }
.ps-head h1 { margin-top: 10px; font-size: clamp(28px, 3.4vw, 40px); font-weight: 700; letter-spacing: -0.02em; }
.ps-sub { margin-top: 8px; max-width: 40em; color: var(--lc-ink-2); font-size: 15px; }
.ps-tools { display: flex; flex-wrap: wrap; align-items: center; gap: 8px; }
.ps-seg { display: inline-flex; gap: 2px; padding: 3px; border: 1px solid var(--lc-line); border-radius: 8px; background: var(--lc-bg-2); }
.ps-seg button { height: 28px; padding: 0 12px; border-radius: 6px; color: var(--lc-ink-2); font-size: 13px; }
.ps-seg button[aria-pressed='true'] { background: var(--lc-surface-3); color: var(--lc-ink); box-shadow: inset 0 1px 0 var(--lc-hi); }
.ps-refresh { display: inline-flex; align-items: center; gap: 6px; height: 36px; padding: 0 12px; border: 1px solid var(--lc-line-2); border-radius: 8px; background: var(--lc-surface); color: var(--lc-ink-2); font: 12.5px var(--lc-font-mono); }
.ps-refresh:hover { color: var(--lc-ink); }
.ps-board { margin-top: 28px; }

@media (max-width: 640px) {
  .ps { padding: 32px 0 56px; }
}
</style>
