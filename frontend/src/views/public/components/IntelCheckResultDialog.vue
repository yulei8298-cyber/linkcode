<template>
  <BaseDialog :show="show" :title="dialogTitle" width="wide" @close="emit('close')">
    <div v-if="loading" class="py-10 text-center text-sm text-gray-500">加载中…</div>
    <div v-else-if="!detail" class="py-10 text-center text-sm text-gray-500">
      未能加载这次检测的详情，请稍后重试。
    </div>

    <div v-else class="space-y-4">
      <!-- 判定结论 -->
      <div class="flex flex-wrap items-center gap-2">
        <span
          class="inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium"
          :class="statusBadgeClass(detail.status)"
        >
          {{ statusLabel(detail.status) }}
        </span>
        <span class="text-xs text-gray-500 dark:text-gray-400">{{ detail.status_note }}</span>
      </div>

      <!-- 本次检测的上下文。刻意不含上游地址、HTTP 状态码与任何凭据片段 -->
      <dl class="grid grid-cols-2 gap-x-4 gap-y-2 text-sm sm:grid-cols-4">
        <div v-for="item in metaItems" :key="item.label">
          <dt class="text-xs text-gray-500 dark:text-gray-400">{{ item.label }}</dt>
          <dd class="mt-0.5 break-all font-medium text-gray-900 dark:text-gray-100">
            {{ item.value }}
          </dd>
        </div>
      </dl>

      <!-- 题面 -->
      <section>
        <h4 class="mb-1.5 text-xs font-medium text-gray-500 dark:text-gray-400">
          题目{{ detail.question_title ? ` · ${detail.question_title}` : '' }}
        </h4>
        <pre class="lc-ic-pre">{{ detail.prompt_snapshot || '--' }}</pre>
      </section>

      <!-- 逻辑题：期望 / 提取 左右两栏，让读者自己核对判定 -->
      <section v-if="detail.kind === 'logic'" class="grid gap-3 sm:grid-cols-2">
        <div>
          <h4 class="mb-1.5 text-xs font-medium text-gray-500 dark:text-gray-400">
            标准答案（{{ matchModeLabel(detail.match_mode) }}）
          </h4>
          <pre class="lc-ic-pre">{{ detail.expected_answer || '--' }}</pre>
        </div>
        <div>
          <h4 class="mb-1.5 text-xs font-medium text-gray-500 dark:text-gray-400">提取到的答案</h4>
          <pre class="lc-ic-pre">{{ detail.extracted_answer || '--' }}</pre>
        </div>
      </section>

      <!-- 绘图题：预览 / 源码 / 原始回复 三页签 -->
      <section v-else>
        <div class="mb-2 flex gap-1.5">
          <button
            v-for="tab in drawingTabs"
            :key="tab.value"
            type="button"
            class="rounded-lg px-2.5 py-1 text-xs transition-colors"
            :class="
              activeTab === tab.value
                ? 'bg-teal-600 text-white'
                : 'bg-gray-100 text-gray-600 hover:bg-gray-200 dark:bg-dark-800 dark:text-gray-300 dark:hover:bg-dark-700'
            "
            @click="activeTab = tab.value"
          >
            {{ tab.label }}
          </button>
        </div>

        <SvgArtworkPreview
          v-show="activeTab === 'preview'"
          :html="detail.html_output"
          :height="360"
          :title="`${detail.target_name} 的绘图产物`"
        />
        <pre v-if="activeTab === 'source'" class="lc-ic-pre lc-ic-pre-tall">{{
          detail.html_output || '本次没有产出可展示的画作'
        }}</pre>
        <pre v-if="activeTab === 'raw'" class="lc-ic-pre lc-ic-pre-tall">{{
          detail.raw_reply || '--'
        }}</pre>
      </section>

      <!-- 逻辑题的原始回复单独列出；绘图题的已在上面的页签里 -->
      <section v-if="detail.kind === 'logic'">
        <h4 class="mb-1.5 text-xs font-medium text-gray-500 dark:text-gray-400">模型原始回复</h4>
        <pre class="lc-ic-pre lc-ic-pre-tall">{{ detail.raw_reply || '--' }}</pre>
      </section>

      <!-- 判定明细：门禁逐项结果与评审得分。它是这张页面「可复核」的落点 -->
      <section v-if="judgeItems.length">
        <h4 class="mb-1.5 text-xs font-medium text-gray-500 dark:text-gray-400">判定明细</h4>
        <div class="space-y-1.5">
          <div
            v-for="item in judgeItems"
            :key="item.label"
            class="flex items-start justify-between gap-3 rounded-lg bg-gray-50 px-2.5 py-1.5 text-xs dark:bg-dark-800"
          >
            <span class="text-gray-500 dark:text-gray-400">{{ item.label }}</span>
            <span class="text-right font-medium text-gray-900 dark:text-gray-100">
              {{ item.value }}
            </span>
          </div>
        </div>
      </section>

      <!-- 门禁逐项：每项是否达标 + 实测值，阈值定得合不合理靠它自证 -->
      <section v-if="gateItems.length">
        <h4 class="mb-1.5 text-xs font-medium text-gray-500 dark:text-gray-400">结构门禁逐项</h4>
        <div class="space-y-1">
          <div
            v-for="(item, index) in gateItems"
            :key="index"
            class="flex items-start gap-2 rounded-lg px-2.5 py-1.5 text-xs"
            :class="item.pass ? 'bg-emerald-50 dark:bg-emerald-950/30' : 'bg-red-50 dark:bg-red-950/30'"
          >
            <span :class="item.pass ? 'text-emerald-600' : 'text-red-600'">
              {{ item.pass ? '✓' : '✗' }}
            </span>
            <span class="flex-1 text-gray-700 dark:text-gray-300">{{ item.item }}</span>
            <span class="text-gray-500 dark:text-gray-400">{{ item.detail }}</span>
          </div>
        </div>
      </section>

      <!-- 源码评审逐项得分 -->
      <section v-if="reviewItems.length">
        <h4 class="mb-1.5 text-xs font-medium text-gray-500 dark:text-gray-400">源码评审逐项</h4>
        <div class="space-y-1">
          <div
            v-for="(item, index) in reviewItems"
            :key="index"
            class="rounded-lg bg-gray-50 px-2.5 py-1.5 text-xs dark:bg-dark-800"
          >
            <div class="flex items-center justify-between gap-3">
              <span class="font-medium text-gray-800 dark:text-gray-200">{{ item.item }}</span>
              <span class="shrink-0 text-gray-600 dark:text-gray-300">
                {{ item.score }} / {{ item.max_score }}
              </span>
            </div>
            <p v-if="item.comment" class="mt-0.5 text-gray-500 dark:text-gray-400">
              {{ item.comment }}
            </p>
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
const loadError = ref('')
const activeTab = ref<DrawingTab>('preview')
let abortController: AbortController | null = null

const dialogTitle = computed(() => {
  if (!detail.value) return '检测详情'
  return `${detail.value.target_name} · ${kindLabel(detail.value.kind)} · 第 ${detail.value.round_seq} 轮`
})

const metaItems = computed(() => {
  const value = detail.value
  if (!value) return []
  return [
    { label: '模型', value: value.model || '--' },
    { label: '推理等级', value: value.reasoning_effort || '--' },
    { label: '耗时', value: formatLatency(value.latency_ms) },
    { label: '检测时刻', value: formatDateTime(value.checked_at) },
  ]
})

/** judge_detail 是后端直接透出的 map，按已知键挑出可读项。 */
function detailValue(key: string): unknown {
  return detail.value?.judge_detail?.[key]
}

const judgeItems = computed(() => {
  const raw = detail.value?.judge_detail
  if (!raw) return []

  const items: { label: string; value: string }[] = []
  const reason = raw.reason
  if (typeof reason === 'string' && reason) items.push({ label: '判定说明', value: reason })

  if (detail.value?.kind === 'logic') {
    const matched = raw.matched
    if (typeof matched === 'boolean') {
      items.push({ label: '是否匹配', value: matched ? '匹配' : '不匹配' })
    }
    return items
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

function statusBadgeClass(status: string): string {
  switch (status) {
    case 'pass':
      return 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-300'
    case 'fail':
      return 'bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-300'
    case 'request_error':
      return 'bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-300'
    default:
      return 'bg-sky-100 text-sky-700 dark:bg-sky-900/40 dark:text-sky-300'
  }
}

async function load(id: number) {
  abortController?.abort()
  const controller = new AbortController()
  abortController = controller

  detail.value = null
  loadError.value = ''
  loading.value = true
  try {
    const result = await getIntelCheckResult(id, { signal: controller.signal })
    if (!controller.signal.aborted) detail.value = result
  } catch (error: unknown) {
    const reason = error as { name?: string; code?: string }
    // 用户快速连点不同色块时旧请求会被取消，那不是错误，不应清掉刚到的新数据
    if (reason.name !== 'AbortError' && reason.code !== 'ERR_CANCELED') {
      loadError.value = '加载失败'
    }
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
    // 每次打开都回到预览页签：上一次停在「源码」不该影响下一次打开的第一印象
    activeTab.value = 'preview'
    if (id != null) void load(id)
  },
  { immediate: true },
)
</script>

<style scoped>
/*
 * 背景与文字颜色必须成对写死。
 *
 * 之前只设了背景、让文字颜色向上继承，结果在深色主题下继承到白色，
 * 白字压在浅色底上——题面和源码整块看不见。这类"只定一半"的配色在浅色环境里
 * 测不出问题，一换主题就瞎。凡是自定义背景的块，颜色都在同一处定死。
 */
.lc-ic-pre {
  max-height: 180px;
  overflow: auto;
  border-radius: 10px;
  background: #f8fafc;
  color: #334155;
  padding: 10px 12px;
  font-family: ui-monospace, Menlo, Consolas, monospace;
  font-size: 12px;
  line-height: 1.6;
  /* 模型回复里常有超长的单行（压缩过的 SVG path），不换行会把弹窗撑出横向滚动条 */
  white-space: pre-wrap;
  word-break: break-word;
}

.lc-ic-pre-tall {
  max-height: 320px;
}

:global(.dark) .lc-ic-pre {
  background: rgba(15, 23, 42, 0.55);
  color: #cbd5e1;
}
</style>
