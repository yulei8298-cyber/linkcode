<template>
  <!--
    双形态，沿用 ModelPlazaView 的既有模式：
      ?embedded=1 且已登录 → 套后台布局，用户从控制台侧边栏进来时留在控制台内；
      否则                  → 独立门户页，未登录访客直达（本功能的主场景）。
    未登录时即便带了 embedded=1 也降级为门户形态，否则转发出去的链接会渲染出
    一个没有登录态的空后台骨架。
  -->
  <component :is="isEmbedded ? AppLayout : PortalLayout">
    <!-- 门户形态的大标题。后台形态下不渲染：那句「首页 /」面包屑在控制台里
         指向站外，而页面标题已由后台布局的页头承担。 -->
    <section v-if="!isEmbedded" class="lc-page-head">
      <div class="lc-wrap lc-page-head-inner">
        <div class="lc-crumb"><RouterLink to="/home">首页</RouterLink> / 模型智力检测</div>
        <h1 class="lc-page-title">模型<span>智力检测</span></h1>
        <p class="lc-lead">{{ introText }}</p>
      </div>
    </section>

    <div class="lc-wrap" :class="{ 'lc-ic-embedded': isEmbedded }">
      <!-- 后台形态下补一张说明卡，替代上面那段门户大标题 -->
      <div v-if="isEmbedded" class="lc-card lc-ic-embedded-head">
        <h1>模型智力检测</h1>
        <p>{{ introText }}</p>
      </div>
      <!-- 汇总条：24h 通过率 + 上次/下次检测。数据未加载完时不显示，
           避免先渲染一组 0 再跳到真实值。 -->
      <div v-if="overview" class="lc-card lc-ic-hero">
        <div class="lc-ic-hero-stats">
          <div class="lc-ic-stat">
            <small>24 小时通过率</small>
            <b>{{ formatPassRate(summary.stats_24h.pass_rate, summary.stats_24h.has_data) }}</b>
          </div>
          <div class="lc-ic-stat">
            <small>受检分组</small>
            <b>{{ summary.total_groups }}</b>
          </div>
          <div class="lc-ic-stat">
            <small>状态正常</small>
            <b>{{ summary.normal_groups }}</b>
          </div>
          <div class="lc-ic-stat" :class="{ 'lc-ic-stat-alert': summary.degraded_groups > 0 }">
            <small>疑似降智</small>
            <b>{{ summary.degraded_groups }}</b>
          </div>
        </div>
        <div class="lc-ic-hero-meta">
          <span>第 {{ summary.latest_round_seq }} 轮 · 上次检测 {{ formatRelative(summary.last_checked_at) }}</span>
          <span>下次检测 {{ formatCountdown(summary.next_check_at) }}</span>
          <span>
            判定规则：连续 {{ overview.degraded_rule.fail_streak }} 次未通过判为疑似降智，
            连续 {{ overview.degraded_rule.recover_streak }} 次通过恢复正常
          </span>
        </div>
      </div>

      <!-- 图例。色块本身只是颜色，不写明含义读者无从复核 -->
      <div class="lc-ic-legend">
        <span v-for="item in legend" :key="item.label" class="lc-ic-legend-item">
          <i class="lc-ic-legend-dot" :class="item.cls"></i>{{ item.label }}
        </span>
        <button type="button" class="lc-button lc-button-small" :disabled="loading" @click="manualReload">
          <Icon name="refresh" size="sm" :class="{ 'animate-spin': loading }" />
          自动刷新 {{ countdown }}s
        </button>
      </div>

      <div v-if="loading && !overview" class="lc-ic-grid">
        <div v-for="i in 3" :key="i" class="lc-card lc-ic-card animate-pulse" style="height: 220px"></div>
      </div>
      <div v-else-if="unavailable" class="lc-card lc-empty">
        <h3>智力检测尚未开放</h3>
        <p>该功能当前未开启，或正在配置中。</p>
      </div>
      <div v-else-if="groups.length === 0" class="lc-card lc-empty">
        <h3>暂无受检分组</h3>
        <p>后台配置受检分组并录入题目后，检测结果会自动显示在这里。</p>
      </div>
      <div v-else class="lc-ic-grid">
        <IntelCheckGroupCard
          v-for="group in groups"
          :key="group.id"
          :group="group"
          :timeline-points="timelinePoints"
          @select="openDetail"
        />
      </div>
    </div>

    <IntelCheckResultDialog :show="showDetail" :result-id="detailResultId" @close="closeDetail" />
  </component>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { RouterLink, useRoute } from 'vue-router'
import PortalLayout from './components/PortalLayout.vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import IntelCheckGroupCard from './components/IntelCheckGroupCard.vue'
import IntelCheckResultDialog from './components/IntelCheckResultDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import { useAutoRefresh } from '@/composables/useAutoRefresh'
import { extractApiErrorMessage } from '@/utils/apiError'
import { getIntelCheckOverview, type IntelCheckOverview } from '@/api/intelCheck'
import { formatCountdown, formatPassRate, formatRelative } from './components/intelCheckFormat'

const DEFAULT_INTRO_TEXT =
  '我们会定期用与 Codex CLI 完全一致的请求方式，向下列分组各发一道逻辑题和一道绘图题，' +
  '并把每一次的原始回复与判定过程如实公开。点击任意色块可查看那一次的完整细节。'

const route = useRoute()
const appStore = useAppStore()
const authStore = useAuthStore()

// 与 ModelPlazaView 同一判定：embedded=1 但未登录（例如链接被转发出去）
// 自动降级为门户形态，不去渲染一个没有登录态的后台骨架。
const isEmbedded = computed(() => route.query.embedded === '1' && authStore.isAuthenticated)

const overview = ref<IntelCheckOverview | null>(null)
const loading = ref(false)
/** 后端返回 404（功能未开启）时置位，与「开着但没有分组」区分开。 */
const unavailable = ref(false)
const showDetail = ref(false)
const detailResultId = ref<number | null>(null)
let abortController: AbortController | null = null

const summary = computed(() => overview.value!.summary)
const groups = computed(() => overview.value?.groups ?? [])
const timelinePoints = computed(() => overview.value?.timeline_points || 48)
const introText = computed(() => overview.value?.intro_text || DEFAULT_INTRO_TEXT)

const legend = [
  { label: '通过', cls: '' },
  { label: '未通过', cls: 'bad' },
  { label: '请求失败（不计入判定）', cls: 'degraded' },
  { label: '检测中', cls: 'running' },
  { label: '暂无数据', cls: 'unknown' },
]

const autoRefresh = useAutoRefresh({
  storageKey: 'portal-intel-check-auto-refresh',
  intervals: [30, 60, 120] as const,
  defaultInterval: 30,
  onRefresh: () => reload(true),
  // 详情弹窗打开时暂停轮询：卡片重渲染会打断正在播放的画作动画，
  // 而读者多半正是为了看动画才点开的。
  shouldPause: () => document.hidden || loading.value || showDetail.value,
})
const countdown = autoRefresh.countdown

async function reload(silent = false) {
  abortController?.abort()
  const controller = new AbortController()
  abortController = controller
  if (!silent) loading.value = true
  try {
    const data = await getIntelCheckOverview({ signal: controller.signal })
    if (!controller.signal.aborted) {
      overview.value = data
      unavailable.value = false
    }
  } catch (error: unknown) {
    const reason = error as { name?: string; code?: string; status?: number }
    if (reason.name === 'AbortError' || reason.code === 'ERR_CANCELED') return
    if (reason.status === 404) {
      // 功能未开启是正常状态而非故障，不弹错误提示——访客对此无能为力。
      unavailable.value = true
      overview.value = null
      return
    }
    // 静默轮询失败不打扰：页面上还留着上一轮的数据，弹窗只会盖住它。
    if (!silent) appStore.showError(extractApiErrorMessage(error, '加载智力检测数据失败'))
  } finally {
    if (abortController === controller) {
      loading.value = false
      countdown.value = 30
      abortController = null
    }
  }
}

async function manualReload() {
  await reload(false)
}

function openDetail(resultId: number) {
  if (resultId <= 0) return
  detailResultId.value = resultId
  showDetail.value = true
}

function closeDetail() {
  showDetail.value = false
  detailResultId.value = null
}

onMounted(() => {
  void reload(false)
  autoRefresh.setEnabled(true)
})

onBeforeUnmount(() => abortController?.abort())
</script>

<style scoped>
/* 后台形态：lc-wrap 自带门户页的最大宽度与左右留白，嵌进控制台内容区后
   会在已有的内边距里再缩一层，看着像没对齐。这里让它撑满。 */
.lc-ic-embedded {
  max-width: none;
  padding-left: 0;
  padding-right: 0;
}

.lc-ic-embedded-head {
  padding: 18px;
  margin-bottom: 14px;
}

.lc-ic-embedded-head h1 {
  margin: 0;
  font-size: 18px;
  font-weight: 800;
}

.lc-ic-embedded-head p {
  margin: 6px 0 0;
  font-size: 12px;
  line-height: 1.6;
  opacity: 0.72;
}

.lc-ic-hero {
  display: flex;
  flex-wrap: wrap;
  gap: 18px;
  justify-content: space-between;
  padding: 18px;
  margin-bottom: 14px;
}

.lc-ic-hero-stats {
  display: flex;
  flex-wrap: wrap;
  gap: 26px;
}

.lc-ic-stat {
  display: flex;
  flex-direction: column;
  gap: 3px;
}

.lc-ic-stat small {
  font-size: 12px;
  opacity: 0.7;
}

.lc-ic-stat b {
  font-size: 20px;
  font-variant-numeric: tabular-nums;
}

.lc-ic-stat-alert b {
  color: #ef4444;
}

.lc-ic-hero-meta {
  display: flex;
  flex-direction: column;
  gap: 4px;
  font-size: 12px;
  line-height: 1.6;
  opacity: 0.72;
}

.lc-ic-legend {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 14px;
  margin-bottom: 14px;
  font-size: 12px;
  opacity: 0.82;
}

.lc-ic-legend-item {
  display: inline-flex;
  align-items: center;
  gap: 5px;
}

.lc-ic-legend-dot {
  width: 11px;
  height: 11px;
  border-radius: 3px;
  background: #22c55e;
}

.lc-ic-legend-dot.bad {
  background: #ef4444;
}

.lc-ic-legend-dot.degraded {
  background: #f59e0b;
}

.lc-ic-legend-dot.running {
  background: #7dd3fc;
}

.lc-ic-legend-dot.unknown {
  background: rgba(148, 163, 184, 0.32);
}

.lc-ic-legend .lc-button {
  margin-left: auto;
}

.lc-ic-grid {
  display: grid;
  gap: 14px;
}
</style>
