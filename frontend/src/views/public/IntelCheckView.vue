<template>
  <!--
    双形态，沿用 ModelPlazaView 的既有模式：
      ?embedded=1 且已登录 → 套后台布局，从控制台侧边栏进来时留在控制台内；
      否则                  → 独立门户页，未登录访客直达（本功能的主场景）。
    未登录时即便带了 embedded=1 也降级为门户形态，否则转发出去的链接会渲染出
    一个没有登录态的空后台骨架。

    页面样式完全自带（ic-* 前缀），不使用门户的 lc-* 类：那套类的卡片背景是
    深色霓虹（portal-neon.css），而本页按设计稿「方案A · 状态页式」是浅色白卡片。
    自带样式后两种外壳里都能正常渲染，深色门户下只换表面色、布局不变。
  -->
  <component :is="isEmbedded ? AppLayout : PortalLayout">
    <div class="ic-root" :class="{ 'ic-portal': !isEmbedded }">
      <div class="ic-topbar">
        <h1>
          <span class="ic-ico">◈</span>模型智力检测
          <span class="ic-subtitle">对承诺「不降智」的分组做定时能力检测</span>
        </h1>
        <div class="ic-live">
          <i></i>
          <span v-if="overview">更新于 {{ updatedAt }} · </span>每 {{ countdown }} 秒后自动刷新
        </div>
      </div>

      <!-- 说明卡 + 右侧大 KPI。数据未加载完时不显示，避免先渲染一组 0 再跳到真实值。 -->
      <section v-if="overview" class="ic-card ic-hero">
        <div class="ic-hero-main">
          <div class="ic-brain">◈</div>
          <div class="ic-hero-text">
            <h2>{{ overview.intro_title || '模型真的是满血在跑吗？' }}</h2>
            <p>{{ introText }}</p>
            <div class="ic-sum">
              <b><span class="ic-sdot ok"></span>{{ summary.normal_groups }} 智力正常</b>
              <b v-if="summary.degraded_groups > 0">
                <span class="ic-sdot fail"></span>{{ summary.degraded_groups }} 疑似降智
              </b>
              <b class="muted">
                <span class="ic-sdot none"></span>{{ summary.total_groups }} 个分组受检 · 第
                {{ summary.latest_round_seq }} 轮
              </b>
            </div>
          </div>
        </div>

        <div class="ic-kpi">
          <div class="ic-kpi-lab">逻辑题通过率 · 最近 24 小时</div>
          <div class="ic-kpi-val">{{ formatPassRate(logicRate, logicHasData) }}</div>
          <div class="ic-kpi-next">⏱ 下次检测 {{ formatCountdown(summary.next_check_at) }}</div>
          <div class="ic-kpi-bar">
            <i :style="{ width: `${Math.round((logicRate ?? 0) * 100)}%` }"></i>
          </div>
        </div>
      </section>

      <!-- 图例。色块本身只是颜色，不写明含义读者无从复核 -->
      <div class="ic-legend">
        <span v-for="item in legend" :key="item.label">
          <i class="ic-sq" :class="item.cls"></i>{{ item.label }}
        </span>
        <span class="ic-legend-note">
          每个色块代表一次检测 · 展示最近 {{ timelinePoints }} 次
        </span>
        <button type="button" class="ic-refresh" :disabled="loading" @click="manualReload">
          <Icon name="refresh" size="sm" :class="{ 'animate-spin': loading }" />
          立即刷新
        </button>
      </div>

      <div v-if="loading && !overview" class="ic-card ic-skeleton"></div>
      <div v-else-if="unavailable" class="ic-card ic-empty">
        <h3>智力检测尚未开放</h3>
        <p>该功能当前未开启，或正在配置中。</p>
      </div>
      <div v-else-if="groups.length === 0" class="ic-card ic-empty">
        <h3>暂无受检分组</h3>
        <p>后台配置受检分组并录入题目后，检测结果会自动显示在这里。</p>
      </div>
      <template v-else>
        <IntelCheckGroupCard
          v-for="group in groups"
          :key="group.id"
          :group="group"
          :timeline-points="timelinePoints"
          :rule="overview!.degraded_rule"
          @select="openDetail"
        />
      </template>
    </div>

    <IntelCheckResultDialog :show="showDetail" :result-id="detailResultId" @close="closeDetail" />
  </component>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
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
import { formatCountdown, formatPassRate } from './components/intelCheckFormat'

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
const updatedAt = computed(() => {
  const raw = overview.value?.generated_at
  if (!raw) return ''
  const date = new Date(raw)
  return Number.isNaN(date.getTime()) ? '' : date.toLocaleTimeString('zh-CN', { hour12: false })
})

/**
 * 顶部 KPI 只算逻辑题，不含绘图题。
 *
 * 绘图题的判定含源码评审打分，天然比逻辑题波动大（后端也正是因此不让它参与降智
 * 判定）。把它混进这个最显眼的数字里，会让「今天评审模型抽了一下」看起来像
 * 「模型降智了」。分子分母先各自累加再相除——逐组求平均会让样本少的分组被放大。
 */
const logicTotals = computed(() => {
  return groups.value.reduce(
    (acc, group) => {
      acc.pass += group.logic_stats_24h?.pass ?? 0
      acc.fail += group.logic_stats_24h?.fail ?? 0
      return acc
    },
    { pass: 0, fail: 0 },
  )
})
const logicHasData = computed(() => logicTotals.value.pass + logicTotals.value.fail > 0)
const logicRate = computed(() => {
  const { pass, fail } = logicTotals.value
  return pass + fail > 0 ? pass / (pass + fail) : null
})

const legend = [
  { label: '通过', cls: 'ok' },
  { label: '未通过', cls: 'bad' },
  { label: '请求失败', cls: 'degraded' },
  { label: '未验证', cls: 'unverified' },
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
/* 设计稿「方案A · 状态页式」的配色。定义在 ic-root 上而不是 :root，
   避免污染全局，也让同一套变量能在门户深色壳里被整体替换。 */
.ic-root {
  --ic-accent: #0d9488;
  --ic-accent-strong: #0f766e;
  --ic-surface: #fff;
  --ic-text: #0f172a;
  --ic-text-soft: #334155;
  --ic-muted: #64748b;
  --ic-faint: #94a3b8;
  --ic-line: #e2e8f0;
  --ic-line-soft: rgba(15, 23, 42, 0.05);
  --ic-chip-bg: #f1f5f9;
  --ic-chip-text: #334155;
  --ic-subtle: #f8fafc;

  --ic-ok: #10b981;
  --ic-fail: #ef4444;
  --ic-req: #f59e0b;
  --ic-run: #7dd3fc;
  --ic-unverified: #64748b;
  --ic-none: #e2e8f0;

  --ic-ok-bg: #ecfdf5;
  --ic-ok-text: #047857;
  --ic-ok-line: #a7f3d0;
  --ic-fail-bg: #fef2f2;
  --ic-fail-text: #b91c1c;
  --ic-fail-line: #fecaca;
  --ic-warn-text: #b45309;
  --ic-bad-outline: rgba(239, 68, 68, 0.35);
  --ic-bad-surface: linear-gradient(180deg, #fff, #fff8f8);

  color: var(--ic-text);
  padding: 24px 0 40px;
}

/* 深色环境有两处，必须一起覆盖，漏掉任何一处都会出现「深色字压深色底」：
     .lc-shell —— 门户的霓虹主题（portal-neon.css）
     .dark     —— 后台控制台的深色模式（类挂在 html 上）
   顶栏标题位于卡片之外、直接压在页面背景上，是最先暴露问题的地方。
   布局与信息结构完全不变，只换表面色与文字色。 */
.lc-shell .ic-root,
.dark .ic-root {
  --ic-surface: rgba(19, 19, 22, 0.94);
  --ic-text: #f8fafc;
  --ic-text-soft: #e2e8f0;
  --ic-muted: #94a3b8;
  --ic-faint: #64748b;
  --ic-line: rgba(148, 163, 184, 0.22);
  --ic-line-soft: rgba(148, 163, 184, 0.18);
  --ic-chip-bg: rgba(148, 163, 184, 0.16);
  --ic-chip-text: #cbd5e1;
  --ic-subtle: rgba(148, 163, 184, 0.1);
  --ic-accent: #2dd4bf;
  --ic-accent-strong: #5eead4;
  --ic-none: rgba(148, 163, 184, 0.25);
  --ic-unverified: #94a3b8;
  --ic-ok-bg: rgba(16, 185, 129, 0.14);
  --ic-ok-text: #6ee7b7;
  --ic-ok-line: rgba(16, 185, 129, 0.35);
  --ic-fail-bg: rgba(239, 68, 68, 0.14);
  --ic-fail-text: #fca5a5;
  --ic-fail-line: rgba(239, 68, 68, 0.35);
  --ic-warn-text: #fbbf24;
  --ic-bad-surface: linear-gradient(180deg, rgba(19, 19, 22, 0.94), rgba(60, 20, 20, 0.5));
}

/* 门户形态给页面留出左右留白；后台形态由 AppLayout 的内容区负责。 */
.ic-root.ic-portal {
  max-width: 1560px;
  margin: 0 auto;
  padding: 24px 24px 48px;
}

.ic-card {
  background: var(--ic-surface);
  border-radius: 24px;
  box-shadow:
    0 1px 3px rgba(0, 0, 0, 0.04),
    0 1px 2px rgba(0, 0, 0, 0.06);
  outline: 1px solid var(--ic-line-soft);
}

/* ---------- 顶栏 ---------- */

.ic-topbar {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 18px;
  font-size: 13px;
  color: var(--ic-muted);
}

.ic-topbar h1 {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 10px;
  margin: 0;
  font-size: 22px;
  font-weight: 900;
  color: var(--ic-text);
}

.ic-ico {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 34px;
  height: 34px;
  border-radius: 12px;
  background: var(--ic-ok-bg);
  color: var(--ic-accent);
  font-size: 18px;
}

.ic-subtitle {
  font-size: 13px;
  font-weight: 400;
  color: var(--ic-muted);
}

.ic-live {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.ic-live i {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--ic-ok);
  box-shadow: 0 0 0 4px rgba(16, 185, 129, 0.15);
}

/* ---------- 说明卡 + KPI ---------- */

.ic-hero {
  display: grid;
  gap: 24px;
  padding: 26px 30px;
  margin-bottom: 18px;
}

@media (min-width: 900px) {
  .ic-hero {
    grid-template-columns: minmax(0, 1fr) 320px;
  }
}

.ic-hero-main {
  display: flex;
  gap: 20px;
  min-width: 0;
}

.ic-brain {
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  width: 56px;
  height: 56px;
  border-radius: 16px;
  background: linear-gradient(135deg, #14b8a6, #0d9488);
  color: #fff;
  font-size: 26px;
  box-shadow: 0 8px 24px rgba(20, 184, 166, 0.3);
}

.ic-hero-text {
  min-width: 0;
}

.ic-hero-text h2 {
  margin: 0 0 8px;
  font-size: 20px;
  font-weight: 800;
}

.ic-hero-text p {
  margin: 0;
  max-width: 820px;
  font-size: 14px;
  line-height: 1.7;
  color: var(--ic-muted);
}

.ic-sum {
  display: flex;
  flex-wrap: wrap;
  gap: 18px;
  margin-top: 14px;
  font-size: 13px;
}

.ic-sum b {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-weight: 600;
}

.ic-sum b.muted {
  color: var(--ic-muted);
  font-weight: 400;
}

.ic-sdot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
}

.ic-sdot.ok {
  background: var(--ic-ok);
}

.ic-sdot.fail {
  background: var(--ic-fail);
}

.ic-sdot.none {
  background: var(--ic-faint);
}

.ic-kpi {
  display: flex;
  flex-direction: column;
  justify-content: center;
  padding-left: 24px;
  border-left: 1px solid var(--ic-line);
}

@media (max-width: 899px) {
  .ic-kpi {
    padding-left: 0;
    padding-top: 18px;
    border-left: 0;
    border-top: 1px solid var(--ic-line);
  }
}

.ic-kpi-lab {
  font-size: 12px;
  color: var(--ic-muted);
  text-align: right;
}

.ic-kpi-val {
  font-size: 52px;
  font-weight: 900;
  line-height: 1.1;
  letter-spacing: -1px;
  color: var(--ic-accent);
  text-align: right;
  font-variant-numeric: tabular-nums;
}

.ic-kpi-next {
  margin-top: 8px;
  font-size: 13px;
  color: var(--ic-muted);
  text-align: right;
}

.ic-kpi-bar {
  margin-top: 8px;
  height: 5px;
  border-radius: 99px;
  background: var(--ic-chip-bg);
  overflow: hidden;
}

.ic-kpi-bar i {
  display: block;
  height: 100%;
  border-radius: 99px;
  background: linear-gradient(90deg, #14b8a6, #2dd4bf);
  transition: width 0.4s ease;
}

/* ---------- 图例 ---------- */

.ic-legend {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 18px;
  margin: 0 0 16px 6px;
  font-size: 13px;
  color: var(--ic-muted);
}

.ic-legend > span {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.ic-sq {
  width: 12px;
  height: 12px;
  border-radius: 3px;
  background: var(--ic-ok);
}

.ic-sq.bad {
  background: var(--ic-fail);
}

.ic-sq.degraded {
  background: var(--ic-req);
}

.ic-sq.running {
  background: var(--ic-run);
}

.ic-sq.unverified {
  background: var(--ic-unverified);
}

.ic-sq.unknown {
  background: var(--ic-none);
}

.ic-legend-note {
  color: var(--ic-faint);
}

.ic-refresh {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  margin-left: auto;
  padding: 5px 12px;
  border: 1px solid var(--ic-line);
  border-radius: 99px;
  background: var(--ic-surface);
  color: var(--ic-muted);
  font-size: 12px;
  cursor: pointer;
}

.ic-refresh:hover:not(:disabled) {
  color: var(--ic-accent);
  border-color: var(--ic-accent);
}

.ic-refresh:disabled {
  cursor: default;
  opacity: 0.6;
}

/* ---------- 空态 / 骨架 ---------- */

.ic-empty {
  padding: 48px 24px;
  text-align: center;
}

.ic-empty h3 {
  margin: 0 0 6px;
  font-size: 16px;
  font-weight: 700;
}

.ic-empty p {
  margin: 0;
  font-size: 13px;
  color: var(--ic-muted);
}

.ic-skeleton {
  height: 280px;
  animation: ic-fade 1.4s ease-in-out infinite;
}

@keyframes ic-fade {
  50% {
    opacity: 0.55;
  }
}
</style>
