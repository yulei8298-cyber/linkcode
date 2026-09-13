<template>
  <BaseDialog
    :show="show"
    :title="mode === 'evaluate' ? t('admin.intelCheck.trial.evaluateTitle') : t('admin.intelCheck.trial.dryRunTitle')"
    width="extra-wide"
    @close="emit('close')"
  >
    <div v-if="!question" class="py-10 text-center text-sm text-gray-500">
      {{ t('admin.intelCheck.trial.noQuestion') }}
    </div>

    <div v-else class="space-y-5">
      <div class="rounded-lg bg-gray-50 p-3 dark:bg-dark-800">
        <div class="flex flex-wrap items-center gap-2">
          <span class="badge badge-neutral">{{ kindLabel(question.kind) }}</span>
          <span class="font-medium text-gray-900 dark:text-white">{{ question.title }}</span>
        </div>
        <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ question.prompt }}</p>
      </div>

      <template v-if="mode === 'evaluate'">
        <div>
          <label class="label">{{ t('admin.intelCheck.trial.sourceLabel') }}</label>
          <textarea
            v-model="source"
            rows="14"
            class="input font-mono text-xs"
            :placeholder="t('admin.intelCheck.trial.sourcePlaceholder')"
          ></textarea>
          <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
            {{ t('admin.intelCheck.trial.evaluateHint') }}
          </p>
        </div>
      </template>

      <template v-else>
        <div>
          <label class="label">{{ t('admin.intelCheck.trial.selectTarget') }}</label>
          <select v-model.number="targetId" class="input max-w-lg">
            <option :value="0">{{ t('admin.intelCheck.common.none') }}</option>
            <option v-for="target in targets" :key="target.id" :value="target.id">
              {{ target.name }} · {{ target.model }}
            </option>
          </select>
          <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
            {{ t('admin.intelCheck.trial.dryRunHint') }}
          </p>
        </div>
      </template>

      <div v-if="mode === 'evaluate' && question.kind !== 'drawing'" class="rounded-lg bg-amber-50 px-3 py-2 text-xs text-amber-700 dark:bg-amber-950/30 dark:text-amber-300">
        {{ t('admin.intelCheck.trial.onlyDrawing') }}
      </div>

      <div v-if="result" class="space-y-4 border-t border-gray-100 pt-4 dark:border-dark-700">
        <div class="flex flex-wrap items-center gap-3">
          <h3 class="text-sm font-semibold text-gray-900 dark:text-white">
            {{ t('admin.intelCheck.trial.result') }}
          </h3>
          <span class="inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium" :class="statusClass(result.status)">
            {{ statusLabel(result.status) }}
          </span>
          <span v-if="result.target_name" class="text-xs text-gray-500 dark:text-gray-400">
            {{ result.target_name }} · {{ result.model || '--' }}
          </span>
        </div>

        <div class="grid gap-3 text-xs sm:grid-cols-4">
          <div class="rounded-lg bg-gray-50 px-3 py-2 dark:bg-dark-800">
            <div class="text-gray-500 dark:text-gray-400">{{ t('admin.intelCheck.overview.resultColumns.latency') }}</div>
            <div class="mt-0.5 font-medium text-gray-900 dark:text-gray-100">{{ formatLatency(result.latency_ms) }}</div>
          </div>
          <div class="rounded-lg bg-gray-50 px-3 py-2 dark:bg-dark-800">
            <div class="text-gray-500 dark:text-gray-400">{{ t('admin.intelCheck.overview.resultColumns.tokens') }}</div>
            <div class="mt-0.5 font-medium text-gray-900 dark:text-gray-100">{{ tokenText }}</div>
          </div>
          <div class="rounded-lg bg-gray-50 px-3 py-2 dark:bg-dark-800 sm:col-span-2">
            <div class="text-gray-500 dark:text-gray-400">{{ t('admin.intelCheck.overview.resultColumns.error') }}</div>
            <div class="mt-0.5 break-all font-medium text-gray-900 dark:text-gray-100">{{ result.error_message || '--' }}</div>
          </div>
        </div>

        <div v-if="result.question_kind === 'logic'" class="grid gap-3 sm:grid-cols-2">
          <div>
            <h4 class="mb-1.5 text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.intelCheck.questions.form.expectedAnswer') }}</h4>
            <pre class="trial-pre">{{ question.expected_answer || '--' }}</pre>
          </div>
          <div>
            <h4 class="mb-1.5 text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.intelCheck.questions.form.matchMode') }}</h4>
            <pre class="trial-pre">{{ question.match_mode ? matchModeLabel(question.match_mode) : '--' }}
{{ result.extracted_answer || '--' }}</pre>
          </div>
        </div>

        <template v-else>
          <div class="mb-2 flex gap-1.5">
            <button
              v-for="tab in drawingTabs"
              :key="tab.value"
              type="button"
              class="rounded-lg px-2.5 py-1 text-xs transition-colors"
              :class="activeDrawingTab === tab.value ? 'bg-teal-600 text-white' : 'bg-gray-100 text-gray-600 hover:bg-gray-200 dark:bg-dark-800 dark:text-gray-300 dark:hover:bg-dark-700'"
              @click="activeDrawingTab = tab.value"
            >
              {{ tab.label }}
            </button>
          </div>
          <SvgArtworkPreview
            v-if="activeDrawingTab === 'preview'"
            :html="result.html_output"
            :height="360"
            :title="`${question.title} 的试跑产物`"
          />
          <pre v-else-if="activeDrawingTab === 'source'" class="trial-pre trial-pre-tall">{{ result.html_output || '--' }}</pre>
          <pre v-else class="trial-pre trial-pre-tall">{{ result.raw_reply || '--' }}</pre>
        </template>

        <section v-if="gateItems.length">
          <h4 class="mb-1.5 text-xs font-medium text-gray-500 dark:text-gray-400">
            {{ t('admin.intelCheck.trial.gateItems') }}
          </h4>
          <div class="space-y-1">
            <div
              v-for="(item, index) in gateItems"
              :key="index"
              class="flex items-start gap-2 rounded-lg px-2.5 py-1.5 text-xs"
              :class="item.pass ? 'bg-emerald-50 dark:bg-emerald-950/30' : 'bg-red-50 dark:bg-red-950/30'"
            >
              <span :class="item.pass ? 'text-emerald-600' : 'text-red-600'">{{ item.pass ? '✓' : '✗' }}</span>
              <span class="flex-1 text-gray-700 dark:text-gray-300">{{ item.item }}</span>
              <span class="text-gray-500 dark:text-gray-400">{{ item.detail }}</span>
            </div>
          </div>
        </section>

        <section v-if="reviewItems.length">
          <h4 class="mb-1.5 text-xs font-medium text-gray-500 dark:text-gray-400">
            {{ t('admin.intelCheck.trial.reviewItems') }}
          </h4>
          <div class="space-y-1">
            <div v-for="(item, index) in reviewItems" :key="index" class="rounded-lg bg-gray-50 px-2.5 py-1.5 text-xs dark:bg-dark-800">
              <div class="flex items-center justify-between gap-3">
                <span class="font-medium text-gray-800 dark:text-gray-200">{{ item.item }}</span>
                <span class="shrink-0 text-gray-600 dark:text-gray-300">{{ item.score }} / {{ item.max_score }}</span>
              </div>
              <p v-if="item.comment" class="mt-0.5 text-gray-500 dark:text-gray-400">{{ item.comment }}</p>
            </div>
          </div>
        </section>

        <div v-if="judgeReason" class="rounded-lg bg-gray-50 px-3 py-2 text-xs text-gray-600 dark:bg-dark-800 dark:text-gray-300">
          {{ judgeReason }}
        </div>
      </div>
    </div>

    <template #footer>
      <div class="flex justify-end gap-2">
        <button type="button" class="btn btn-secondary" @click="emit('close')">
          {{ t('common.close') }}
        </button>
        <button type="button" class="btn btn-primary" :disabled="running || !canRun" @click="run">
          {{ running ? t('common.processing') : mode === 'evaluate' ? t('admin.intelCheck.trial.run') : t('admin.intelCheck.trial.dryRun') }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type {
  IntelCheckQuestionListItem,
  IntelCheckTarget,
  IntelCheckTrialResult,
} from '@/api/admin/intelCheck'
import BaseDialog from '@/components/common/BaseDialog.vue'
import SvgArtworkPreview from '@/views/public/components/SvgArtworkPreview.vue'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import { formatLatency, kindLabel, matchModeLabel, statusLabel } from '@/views/public/components/intelCheckFormat'

const props = defineProps<{
  show: boolean
  question: IntelCheckQuestionListItem | null
  mode: 'evaluate' | 'dry-run'
}>()

const emit = defineEmits<{
  (e: 'close'): void
}>()

const { t } = useI18n()
const appStore = useAppStore()
const source = ref('')
const targetId = ref(0)
const targets = ref<IntelCheckTarget[]>([])
const result = ref<IntelCheckTrialResult | null>(null)
const running = ref(false)
const activeDrawingTab = ref<'preview' | 'source' | 'raw'>('preview')

const drawingTabs = [
  { value: 'preview' as const, label: t('admin.intelCheck.trial.preview') },
  { value: 'source' as const, label: t('admin.intelCheck.trial.source') },
  { value: 'raw' as const, label: t('admin.intelCheck.trial.rawReply') },
]

const canRun = computed(() => {
  if (!props.question) return false
  if (props.mode === 'evaluate') return props.question.kind === 'drawing' && source.value.trim() !== ''
  return targetId.value > 0
})

const tokenText = computed(() => {
  if (!result.value) return '--'
  const input = result.value.input_tokens ?? 0
  const output = result.value.output_tokens ?? 0
  if (result.value.input_tokens == null && result.value.output_tokens == null) return '--'
  return `${input} / ${output}`
})

function statusClass(status: string): string {
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

function rawJudgeValue(key: string): unknown {
  return result.value?.judge_detail?.[key]
}

interface GateItemView {
  item: string
  pass: boolean
  detail: string
}

const gateItems = computed<GateItemView[]>(() => {
  const raw = rawJudgeValue('gate_items')
  if (!Array.isArray(raw)) return []
  return raw.flatMap((entry) => {
    if (!entry || typeof entry !== 'object') return []
    const row = entry as Record<string, unknown>
    return [{
      item: typeof row.item === 'string' ? row.item : '',
      pass: row.pass === true,
      detail: typeof row.detail === 'string' ? row.detail : '',
    }]
  })
})

interface ReviewItemView {
  item: string
  score: number
  max_score: number
  comment: string
}

const reviewItems = computed<ReviewItemView[]>(() => {
  const raw = rawJudgeValue('review_items')
  if (!Array.isArray(raw)) return []
  return raw.flatMap((entry) => {
    if (!entry || typeof entry !== 'object') return []
    const row = entry as Record<string, unknown>
    return [{
      item: typeof row.item === 'string' ? row.item : '',
      score: typeof row.score === 'number' ? row.score : 0,
      max_score: typeof row.max_score === 'number' ? row.max_score : 0,
      comment: typeof row.comment === 'string' ? row.comment : '',
    }]
  })
})

const judgeReason = computed(() => {
  const reason = rawJudgeValue('reason')
  return typeof reason === 'string' ? reason : ''
})

async function loadTargets() {
  try {
    const response = await adminAPI.intelCheck.listTargets({ page: 1, page_size: 100 })
    targets.value = response.items ?? []
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('admin.intelCheck.common.loadError')))
  }
}

async function run() {
  if (!props.question || running.value || !canRun.value) return
  running.value = true
  try {
    result.value = props.mode === 'evaluate'
      ? await adminAPI.intelCheck.evaluateQuestion(props.question.id, source.value)
      : await adminAPI.intelCheck.dryRunQuestion(props.question.id, targetId.value)
    activeDrawingTab.value = 'preview'
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('admin.intelCheck.overview.runFailed')))
  } finally {
    running.value = false
  }
}

watch(
  () => [props.show, props.question?.id, props.mode] as const,
  ([show, _questionId, mode]) => {
    if (!show) return
    source.value = ''
    targetId.value = 0
    result.value = null
    activeDrawingTab.value = 'preview'
    if (mode === 'dry-run') void loadTargets()
  },
  { immediate: true },
)
</script>

<style scoped>
.trial-pre {
  max-height: 180px;
  overflow: auto;
  border-radius: 10px;
  background: rgb(249 250 251);
  padding: 10px 12px;
  font-size: 12px;
  line-height: 1.6;
  white-space: pre-wrap;
  word-break: break-word;
}

.trial-pre-tall {
  max-height: 360px;
}

:global(.dark) .trial-pre {
  background: rgb(30 41 59 / 0.6);
}
</style>
