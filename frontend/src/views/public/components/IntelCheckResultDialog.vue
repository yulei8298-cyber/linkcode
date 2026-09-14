<template>
  <BaseDialog :show="show" :title="dialogTitle" width="wide" @close="emit('close')">
    <div v-if="loading" class="ic-dlg-hint">加载中…</div>
    <div v-else-if="!detail" class="ic-dlg-hint">未能加载这次检测的详情，请稍后重试。</div>

    <div v-else class="ic-dlg">
      <!-- 结论 + 上下文。刻意不含上游地址、HTTP 状态码与任何凭据片段 -->
      <div class="ic-dlg-head">
        <span class="ic-dlg-pill" :class="statusPillClass">
          <i></i>{{ statusLabel(detail.status) }}
        </span>
        <span class="ic-dlg-note">{{ detail.status_note }}</span>
        <span class="ic-dlg-meta">
          <b>{{ detail.model }}</b> · 推理 {{ detail.reasoning_effort }} ·
          耗时 {{ formatLatency(detail.latency_ms) }} · {{ formatDateTime(detail.checked_at) }}
        </span>
      </div>

      <!-- 逻辑题：左右两栏，左边看题与判定，右边看模型怎么想的 -->
      <div v-if="detail.kind === 'logic'" class="ic-dlg-cols">
        <section class="ic-dlg-col">
          <h4>题目{{ detail.question_title ? ` · ${detail.question_title}` : '' }}</h4>
          <div class="ic-dlg-q">{{ detail.prompt_snapshot || '--' }}</div>

          <h4>答案判定</h4>
          <div class="ic-dlg-ans">
            <div class="exp">
              <b>期望答案</b>
              {{ detail.expected_answer || '--' }}
            </div>
            <div :class="answerMatched ? 'got' : 'miss'">
              <b>提取到的答案 · {{ matchModeLabel(detail.match_mode) }}</b>
              {{ detail.extracted_answer || '（未提取到）' }}
            </div>
          </div>
        </section>

        <section class="ic-dlg-col">
          <h4>模型原始回复</h4>
          <pre class="ic-dlg-pre tall">{{ detail.raw_reply || '--' }}</pre>
        </section>
      </div>

      <!-- 绘图题：预览 / 源码 / 原始回复 三页签 -->
      <section v-else class="ic-dlg-block">
        <h4>题目{{ detail.question_title ? ` · ${detail.question_title}` : '' }}</h4>
        <div class="ic-dlg-q">{{ detail.prompt_snapshot || '--' }}</div>

        <p v-if="typeof detail.judge_detail?.scope_note === 'string'" class="ic-dlg-note">{{ detail.judge_detail.scope_note }}</p>

        <div class="ic-dlg-tabs">
          <button
            v-for="tab in drawingTabs"
            :key="tab.value"
            type="button"
            :class="{ active: activeTab === tab.value }"
            @click="activeTab = tab.value"
          >
            {{ tab.label }}
          </button>
        </div>

        <!-- v-if 而非 v-show：iframe 若在隐藏容器里创建，产物脚本读到的
             document.hidden 为 true，那类「隐藏时不启动 rAF 循环」的省电写法
             就永远不会启动动画（且只在 visibilitychange 里重试，而该事件在
             iframe 内未必触发）。必须等真正可见时才挂载。 -->
        <SvgArtworkPreview
          v-if="activeTab === 'preview'"
          :html="detail.html_output"
          :height="360"
          :title="`${detail.target_name} 的绘图产物`"
        />
        <pre v-if="activeTab === 'source'" class="ic-dlg-pre tall">{{
          detail.html_output || '本次没有产出可展示的画作'
        }}</pre>
        <pre v-if="activeTab === 'raw'" class="ic-dlg-pre tall">{{ detail.raw_reply || '--' }}</pre>
      </section>

      <!-- 判定明细：门禁结论与评审得分。这是本页「可复核」的落点 -->
      <section v-if="judgeItems.length" class="ic-dlg-block">
        <h4>判定明细</h4>
        <div class="ic-dlg-kv">
          <div v-for="item in judgeItems" :key="item.label">
            <span>{{ item.label }}</span>
            <b>{{ item.value }}</b>
          </div>
        </div>
      </section>

      <!-- 门禁逐项：每项是否达标 + 实测值，阈值定得合不合理靠它自证 -->
      <section v-if="gateItems.length" class="ic-dlg-block">
        <h4>结构门禁逐项</h4>
        <div class="ic-dlg-gate">
          <div v-for="(item, index) in gateItems" :key="index" :class="item.pass ? 'ok' : 'bad'">
            <i>{{ item.pass ? '✓' : '✗' }}</i>
            <span>{{ item.item }}</span>
            <em>{{ item.detail }}</em>
          </div>
        </div>
      </section>

      <section v-if="reviewItems.length" class="ic-dlg-block">
        <h4>源码评审逐项</h4>
        <div class="ic-dlg-review">
          <div v-for="(item, index) in reviewItems" :key="index">
            <div class="row">
              <b>{{ item.item }}</b>
              <span>{{ item.score }} / {{ item.max_score }}</span>
            </div>
            <p v-if="item.comment">{{ item.comment }}</p>
          </div>
        </div>
      </section>
    </div>

    <template #footer>
      <div class="flex justify-end">
        <button type="button" class="btn btn-secondary" @click="emit('close')">关闭</button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import SvgArtworkPreview from './SvgArtworkPreview.vue'
import { getIntelCheckResult, type IntelCheckResult } from '@/api/intelCheck'
import {
  formatDateTime,
  formatLatency,
  kindLabel,
  matchModeLabel,
  statusLabel,
} from './intelCheckFormat'

const props = defineProps<{
  show: boolean
  resultId: number | null
}>()

const emit = defineEmits<{
  (e: 'close'): void
}>()

type DrawingTab = 'preview' | 'source' | 'raw'
const drawingTabs: { value: DrawingTab; label: string }[] = [
  { value: 'preview', label: '预览' },
  { value: 'source', label: '源码' },
  { value: 'raw', label: '原始回复' },
]

const detail = ref<IntelCheckResult | null>(null)
const loading = ref(false)
const activeTab = ref<DrawingTab>('preview')
let abortController: AbortController | null = null

const dialogTitle = computed(() => {
  if (!detail.value) return '检测详情'
  return `${detail.value.target_name} · ${kindLabel(detail.value.kind)} · 第 ${detail.value.round_seq} 轮`
})

const statusPillClass = computed(() => {
  switch (detail.value?.status) {
    case 'pass':
      return 'ok'
    case 'fail':
      return 'bad'
    case 'request_error':
      return 'warn'
    default:
      return 'run'
  }
})

/** 答案框是否按「匹配」着色。以判定结果为准，不在前端重新比一次字符串。 */
const answerMatched = computed(() => detail.value?.judge_detail?.matched === true)

function detailValue(key: string): unknown {
  return detail.value?.judge_detail?.[key]
}

const judgeItems = computed(() => {
  const raw = detail.value?.judge_detail
  if (!raw) return []

  const items: { label: string; value: string }[] = []
  const reason = raw.reason
  if (typeof reason === 'string' && reason) items.push({ label: '判定说明', value: reason })

  if (detail.value?.kind === 'logic') return items

  if (raw.judge_method === 'structure_v2') {
    items.push({ label: '判定方式', value: '确定性结构验收 v2' })
    items.push({ label: '结构综合分', value: `${raw.structure_score} / 100（通过线 ${raw.structure_threshold}）` })
    items.push({ label: '标准样本数', value: String(raw.reference_count) })
    items.push({ label: '运动学验证', value: '未验证' })
  }

  const gatePass = raw.gate_pass
  if (typeof gatePass === 'boolean') {
    items.push({ label: '结构门禁', value: gatePass ? '通过' : '未通过' })
  }
  const score = raw.review_score
  const passScore = raw.pass_score
  if (typeof score === 'number') {
    const line = typeof passScore === 'number' ? `（及格线 ${passScore}）` : ''
    items.push({ label: '源码评审得分', value: `${score} 分${line}` })
  }
  const summary = raw.review_summary
  if (typeof summary === 'string' && summary) items.push({ label: '评审总评', value: summary })
  return items
})

interface GateItemView {
  item: string
  pass: boolean
  detail: string
}

const gateItems = computed<GateItemView[]>(() => {
  const raw = detailValue('gate_items')
  if (!Array.isArray(raw)) return []
  return raw.flatMap((entry) => {
    if (!entry || typeof entry !== 'object') return []
    const row = entry as Record<string, unknown>
    return [
      {
        item: typeof row.item === 'string' ? row.item : '',
        pass: row.pass === true,
        detail: typeof row.detail === 'string' ? row.detail : '',
      },
    ]
  })
})

interface ReviewItemView {
  item: string
  score: number
  max_score: number
  comment: string
}

const reviewItems = computed<ReviewItemView[]>(() => {
  const raw = detailValue('review_items')
  if (!Array.isArray(raw)) return []
  return raw.flatMap((entry) => {
    if (!entry || typeof entry !== 'object') return []
    const row = entry as Record<string, unknown>
    return [
      {
        item: typeof row.item === 'string' ? row.item : '',
        score: typeof row.score === 'number' ? row.score : 0,
        max_score: typeof row.max_score === 'number' ? row.max_score : 0,
        comment: typeof row.comment === 'string' ? row.comment : '',
      },
    ]
  })
})

async function load(id: number) {
  abortController?.abort()
  const controller = new AbortController()
  abortController = controller

  detail.value = null
  loading.value = true
  try {
    const result = await getIntelCheckResult(id, { signal: controller.signal })
    if (!controller.signal.aborted) detail.value = result
  } catch {
    // 用户快速连点不同色块时旧请求会被取消，那不是错误，不应清掉刚到的新数据。
  } finally {
    if (abortController === controller) {
      loading.value = false
      abortController = null
    }
  }
}

watch(
  () => [props.show, props.resultId] as const,
  ([show, id]) => {
    if (!show) {
      abortController?.abort()
      abortController = null
      detail.value = null
      return
    }
    // 每次打开都回到预览页签：上一次停在「源码」不该影响下一次打开的第一印象。
    activeTab.value = 'preview'
    if (id != null) void load(id)
  },
  { immediate: true },
)
</script>

<style scoped>
/*
 * 配色与主页面同一套路：变量定义在根节点上，深色环境整体替换。
 *
 * 两处深色环境都要覆盖（门户 .lc-shell / 后台 .dark），漏一处就会出现
 * 「浅色框砸在深色弹窗里」。凡是自定义背景的块，背景与文字色必须成对定死——
 * 只设背景、让文字色向上继承，在浅色环境下测不出问题，一换主题就瞎。
 */
.ic-dlg {
  --d-text: #0f172a;
  --d-soft: #334155;
  --d-muted: #64748b;
  --d-line: #e2e8f0;
  --d-surface: #f8fafc;
  --d-ok-bg: #ecfdf5;
  --d-ok-text: #065f46;
  --d-ok-line: #a7f3d0;
  --d-bad-bg: #fef2f2;
  --d-bad-text: #b91c1c;
  --d-bad-line: #fecaca;
  --d-warn-bg: #fffbeb;
  --d-warn-text: #b45309;
  --d-accent: #0d9488;

  color: var(--d-text);
}

.lc-shell .ic-dlg,
.dark .ic-dlg {
  --d-text: #f1f5f9;
  --d-soft: #cbd5e1;
  --d-muted: #94a3b8;
  --d-line: rgba(148, 163, 184, 0.22);
  --d-surface: rgba(15, 23, 42, 0.55);
  --d-ok-bg: rgba(16, 185, 129, 0.14);
  --d-ok-text: #6ee7b7;
  --d-ok-line: rgba(16, 185, 129, 0.35);
  --d-bad-bg: rgba(239, 68, 68, 0.14);
  --d-bad-text: #fca5a5;
  --d-bad-line: rgba(239, 68, 68, 0.35);
  --d-warn-bg: rgba(245, 158, 11, 0.14);
  --d-warn-text: #fbbf24;
  --d-accent: #2dd4bf;
}

.ic-dlg-hint {
  padding: 40px 0;
  text-align: center;
  font-size: 13px;
  color: #64748b;
}

/* ---------- 结论行 ---------- */

.ic-dlg-head {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 10px;
  padding-bottom: 14px;
  margin-bottom: 16px;
  border-bottom: 1px solid var(--d-line);
}

.ic-dlg-pill {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 4px 11px;
  border-radius: 99px;
  font-size: 12px;
  font-weight: 700;
}

.ic-dlg-pill i {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: currentColor;
}

.ic-dlg-pill.ok {
  background: var(--d-ok-bg);
  color: var(--d-ok-text);
  outline: 1px solid var(--d-ok-line);
}

.ic-dlg-pill.bad {
  background: var(--d-bad-bg);
  color: var(--d-bad-text);
  outline: 1px solid var(--d-bad-line);
}

.ic-dlg-pill.warn {
  background: var(--d-warn-bg);
  color: var(--d-warn-text);
}

.ic-dlg-pill.run {
  background: var(--d-surface);
  color: var(--d-muted);
}

.ic-dlg-note {
  font-size: 12px;
  color: var(--d-muted);
}

.ic-dlg-meta {
  margin-left: auto;
  font-size: 12px;
  color: var(--d-muted);
  font-family: ui-monospace, Menlo, Consolas, monospace;
}

.ic-dlg-meta b {
  color: var(--d-soft);
  font-weight: 600;
}

/* ---------- 两栏 ---------- */

.ic-dlg-cols {
  display: grid;
  gap: 20px;
}

/* 两栏在窄屏折叠成上下：左右并排本是为了「题目与回复对照着看」，
   折叠后顺序仍是题目在前，阅读逻辑不变。 */
@media (min-width: 860px) {
  .ic-dlg-cols {
    grid-template-columns: 1fr 1fr;
  }

  .ic-dlg-cols .ic-dlg-col + .ic-dlg-col {
    padding-left: 20px;
    border-left: 1px solid var(--d-line);
  }
}

.ic-dlg h4 {
  margin: 0 0 8px;
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.06em;
  text-transform: uppercase;
  color: var(--d-muted);
}

.ic-dlg-col h4:not(:first-child) {
  margin-top: 16px;
}

.ic-dlg-block {
  margin-top: 18px;
}

.ic-dlg-q {
  border-radius: 12px;
  background: var(--d-surface);
  color: var(--d-soft);
  padding: 12px 14px;
  font-size: 13px;
  line-height: 1.7;
  white-space: pre-wrap;
  word-break: break-word;
}

/* ---------- 答案对照 ---------- */

.ic-dlg-ans {
  display: flex;
  gap: 10px;
}

.ic-dlg-ans > div {
  flex: 1;
  min-width: 0;
  border-radius: 12px;
  padding: 10px 12px;
  font-size: 13px;
  word-break: break-word;
}

.ic-dlg-ans b {
  display: block;
  margin-bottom: 4px;
  font-size: 11px;
  font-weight: 600;
}

.ic-dlg-ans .exp {
  background: var(--d-surface);
  color: var(--d-soft);
}

.ic-dlg-ans .exp b {
  color: var(--d-muted);
}

/* 提取到的答案按判定结果着色：绿=匹配、红=不匹配。
   这一格是整个弹窗里最该被一眼看到的——读者核对判定就靠它。 */
.ic-dlg-ans .got {
  background: var(--d-ok-bg);
  color: var(--d-ok-text);
  outline: 1px solid var(--d-ok-line);
}

.ic-dlg-ans .miss {
  background: var(--d-bad-bg);
  color: var(--d-bad-text);
  outline: 1px solid var(--d-bad-line);
}

.ic-dlg-ans .got b,
.ic-dlg-ans .miss b {
  color: inherit;
  opacity: 0.85;
}

/* ---------- 代码块 ---------- */

.ic-dlg-pre {
  max-height: 200px;
  overflow: auto;
  margin: 0;
  border-radius: 12px;
  background: var(--d-surface);
  color: var(--d-soft);
  padding: 12px 14px;
  font-family: ui-monospace, Menlo, Consolas, monospace;
  font-size: 12px;
  line-height: 1.6;
  /* 模型回复里常有超长单行（压缩过的 SVG path），不换行会撑出横向滚动条 */
  white-space: pre-wrap;
  word-break: break-word;
}

.ic-dlg-pre.tall {
  max-height: 320px;
}

/* ---------- 页签 ---------- */

.ic-dlg-tabs {
  display: flex;
  gap: 6px;
  margin: 14px 0 10px;
}

.ic-dlg-tabs button {
  border: 1px solid var(--d-line);
  border-radius: 8px;
  background: transparent;
  color: var(--d-muted);
  padding: 4px 12px;
  font-size: 12px;
  cursor: pointer;
  transition: all 0.12s ease;
}

.ic-dlg-tabs button:hover {
  color: var(--d-accent);
}

.ic-dlg-tabs button.active {
  background: var(--d-accent);
  border-color: var(--d-accent);
  color: #fff;
}

/* ---------- 判定明细 ---------- */

.ic-dlg-kv > div {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  padding: 8px 12px;
  border-radius: 10px;
  background: var(--d-surface);
  font-size: 12px;
}

.ic-dlg-kv > div + div {
  margin-top: 6px;
}

.ic-dlg-kv span {
  color: var(--d-muted);
  flex-shrink: 0;
}

.ic-dlg-kv b {
  color: var(--d-text);
  font-weight: 600;
  text-align: right;
}

.ic-dlg-gate > div {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  padding: 7px 12px;
  border-radius: 10px;
  font-size: 12px;
}

.ic-dlg-gate > div + div {
  margin-top: 4px;
}

.ic-dlg-gate .ok {
  background: var(--d-ok-bg);
}

.ic-dlg-gate .bad {
  background: var(--d-bad-bg);
}

.ic-dlg-gate i {
  font-style: normal;
  font-weight: 700;
}

.ic-dlg-gate .ok i {
  color: var(--d-ok-text);
}

.ic-dlg-gate .bad i {
  color: var(--d-bad-text);
}

.ic-dlg-gate span {
  flex: 1;
  color: var(--d-soft);
}

.ic-dlg-gate em {
  font-style: normal;
  color: var(--d-muted);
  text-align: right;
}

.ic-dlg-review > div {
  padding: 8px 12px;
  border-radius: 10px;
  background: var(--d-surface);
  font-size: 12px;
}

.ic-dlg-review > div + div {
  margin-top: 4px;
}

.ic-dlg-review .row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.ic-dlg-review b {
  color: var(--d-text);
  font-weight: 600;
}

.ic-dlg-review .row span {
  flex-shrink: 0;
  color: var(--d-soft);
  font-variant-numeric: tabular-nums;
}

.ic-dlg-review p {
  margin: 3px 0 0;
  color: var(--d-muted);
  line-height: 1.6;
}
</style>
