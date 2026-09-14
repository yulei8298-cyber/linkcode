<template>
  <BaseDialog
    :show="show"
    :title="question ? t('admin.intelCheck.questions.edit') : t('admin.intelCheck.questions.create')"
    width="wide"
    @close="emit('close')"
  >
    <div v-if="detailLoading" class="py-10 text-center text-sm text-gray-500">
      {{ t('common.loading') }}
    </div>

    <div v-else class="space-y-5">
      <div class="grid gap-4 sm:grid-cols-2">
        <div>
          <label class="label">{{ t('admin.intelCheck.questions.form.kind') }}</label>
          <select v-model="form.kind" class="input" @change="handleKindChange">
            <option value="logic">{{ t('admin.intelCheck.common.logic') }}</option>
            <option value="drawing">{{ t('admin.intelCheck.common.drawing') }}</option>
          </select>
          <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
            {{ t('admin.intelCheck.questions.form.kindHint') }}
          </p>
        </div>

        <div>
          <label class="label">{{ t('admin.intelCheck.questions.form.title') }}</label>
          <input
            v-model="form.title"
            type="text"
            class="input"
            maxlength="100"
            :placeholder="t('admin.intelCheck.questions.form.titlePlaceholder')"
          />
        </div>

        <div class="sm:col-span-2">
          <label class="label">{{ t('admin.intelCheck.questions.form.prompt') }}</label>
          <textarea
            v-model="form.prompt"
            rows="6"
            class="input"
            :placeholder="t('admin.intelCheck.questions.form.promptPlaceholder')"
          ></textarea>
        </div>
      </div>

      <section v-if="form.kind === 'logic'" class="space-y-4 border-t border-gray-100 pt-4 dark:border-dark-700">
        <div class="grid gap-4 sm:grid-cols-2">
          <div>
            <label class="label">{{ t('admin.intelCheck.questions.form.expectedAnswer') }}</label>
            <input v-model="form.expected_answer" type="text" class="input" maxlength="500" />
            <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
              {{ t('admin.intelCheck.questions.form.expectedAnswerHint') }}
            </p>
          </div>
          <div>
            <label class="label">{{ t('admin.intelCheck.questions.form.matchMode') }}</label>
            <select v-model="form.match_mode" class="input">
              <option v-for="mode in matchModes" :key="mode" :value="mode">
                {{ t(`admin.intelCheck.matchMode.${mode}`) }}
              </option>
            </select>
          </div>
        </div>
      </section>

      <section v-else class="space-y-4 border-t border-gray-100 pt-4 dark:border-dark-700">
        <div>
          <label class="label" for="intel-check-standard-files">{{ t('admin.intelCheck.questions.form.standardFiles') }}</label>
          <input id="intel-check-standard-files" type="file" accept=".html,.htm,.svg" multiple :disabled="importing" @change="importStandards" />
          <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.intelCheck.questions.form.standardFilesHint') }}</p>
        </div>
        <div>
          <label class="label">{{ t('admin.intelCheck.questions.form.referenceHtml') }}</label>
          <textarea
            v-model="form.reference_html"
            rows="12"
            class="input font-mono text-xs"
            :placeholder="t('admin.intelCheck.questions.form.referenceHtmlHint')"
          ></textarea>
          <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
            {{ t('admin.intelCheck.questions.form.referenceHtmlHint') }}
          </p>
        </div>

        <div v-for="(_, index) in form.drawing_rules.standard_sources" :key="index">
          <div class="mb-1 flex items-center justify-between">
            <label class="label">{{ t('admin.intelCheck.questions.form.additionalStandard', { n: index + 2 }) }}</label>
            <button type="button" class="btn btn-secondary" @click="form.drawing_rules.standard_sources.splice(index, 1)">{{ t('common.delete') }}</button>
          </div>
          <textarea v-model="form.drawing_rules.standard_sources[index]" rows="4" class="input font-mono text-xs"></textarea>
        </div>

        <div v-if="referenceMetrics" class="rounded-lg bg-gray-50 p-3 dark:bg-dark-800">
          <h4 class="text-xs font-medium text-gray-600 dark:text-gray-300">
            {{ t('admin.intelCheck.questions.form.referenceMetrics') }}
          </h4>
          <div class="mt-2 grid gap-2 sm:grid-cols-3">
            <div v-for="metric in metricItems" :key="metric.key" class="rounded bg-white px-2.5 py-2 dark:bg-dark-900">
              <div class="text-[11px] text-gray-500 dark:text-gray-400">
                {{ t(`admin.intelCheck.questions.metrics.${metric.key}`) }}
              </div>
              <div class="mt-0.5 text-sm font-medium text-gray-900 dark:text-gray-100">
                {{ metric.value }}
              </div>
            </div>
          </div>
        </div>

        <div class="grid gap-4 sm:grid-cols-2">
          <NumberField
            v-model="form.drawing_rules.min_ratio"
            :label="t('admin.intelCheck.questions.form.minRatio')"
            :hint="t('admin.intelCheck.questions.form.minRatioHint')"
            :min="0.01"
            :max="1"
          />
          <NumberField
            v-model="form.drawing_rules.max_bytes"
            :label="t('admin.intelCheck.questions.form.maxBytes')"
            :min="1"
          />
        </div>

        <div>
          <label class="label">{{ t('admin.intelCheck.questions.form.requiredKeywords') }}</label>
          <input
            v-model="form.required_keywords"
            type="text"
            class="input"
            :placeholder="t('admin.intelCheck.questions.form.requiredKeywords')"
          />
          <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
            {{ t('admin.intelCheck.questions.form.requiredKeywordsHint') }}
          </p>
        </div>

      </section>

      <div class="flex items-center gap-3 border-t border-gray-100 pt-4 dark:border-dark-700">
        <Toggle v-model="form.enabled" />
        <span class="text-sm text-gray-700 dark:text-gray-300">
          {{ t('admin.intelCheck.questions.form.enabled') }}
        </span>
      </div>

      <p v-if="savedMetrics" class="rounded-lg bg-emerald-50 px-3 py-2 text-xs text-emerald-700 dark:bg-emerald-950/30 dark:text-emerald-300">
        {{ t('admin.intelCheck.questions.form.referenceMetrics') }}
        {{ t('admin.intelCheck.common.bytes', { n: savedMetrics.html_bytes ?? 0 }) }}
      </p>
    </div>

    <template #footer>
      <div class="flex justify-end gap-2">
        <button type="button" class="btn btn-secondary" @click="emit('close')">
          {{ t('common.close') }}
        </button>
        <button type="button" class="btn btn-primary" :disabled="saving || detailLoading || importing" @click="save">
          {{ saving ? t('common.saving') : t('common.save') }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type {
  IntelCheckDrawingRules,
  IntelCheckKind,
  IntelCheckMatchMode,
  IntelCheckQuestion,
  IntelCheckQuestionListItem,
  IntelCheckQuestionParams,
  IntelCheckReferenceMetrics,
} from '@/api/admin/intelCheck'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Toggle from '@/components/common/Toggle.vue'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import NumberField from './NumberField.vue'

const props = defineProps<{
  show: boolean
  question: IntelCheckQuestionListItem | null
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'saved', question: IntelCheckQuestion): void
}>()

const { t } = useI18n()
const appStore = useAppStore()
const matchModes: IntelCheckMatchMode[] = ['exact', 'numeric', 'contains', 'regex']
const saving = ref(false)
const importing = ref(false)
const detailLoading = ref(false)
const detailQuestion = ref<IntelCheckQuestion | null>(null)
const savedMetrics = ref<IntelCheckReferenceMetrics | null>(null)
let abortController: AbortController | null = null
let importVersion = 0

interface QuestionForm {
  kind: IntelCheckKind
  title: string
  prompt: string
  expected_answer: string
  match_mode: IntelCheckMatchMode
  reference_html: string
  required_keywords: string
  review_rubric: string
  drawing_rules: {
    min_ratio: number
    max_bytes: number
    standard_sources: string[]
  }
  enabled: boolean
}

function emptyForm(): QuestionForm {
  return {
    kind: 'logic',
    title: '',
    prompt: '',
    expected_answer: '',
    match_mode: 'exact',
    reference_html: '',
    required_keywords: '',
    review_rubric: '',
    drawing_rules: {
      min_ratio: 0.7,
      max_bytes: 262144,
      standard_sources: [],
    },
    enabled: true,
  }
}

const form = reactive<QuestionForm>(emptyForm())

const referenceMetrics = computed<IntelCheckReferenceMetrics | null>(() =>
  savedMetrics.value ?? detailQuestion.value?.reference_metrics ?? null,
)

const metricItems = computed(() => {
  const metrics = referenceMetrics.value
  if (!metrics) return []
  return [
    { key: 'structure_shapes', value: metrics.structure_baseline?.shapes ?? '--' },
    { key: 'geometry_values', value: metrics.structure_baseline?.geometry_values ?? '--' },
    { key: 'standard_count', value: metrics.standard_count ?? '--' },
  ]
})

function resetForm() {
  importVersion++
  importing.value = false
  Object.assign(form, emptyForm())
  detailQuestion.value = null
  savedMetrics.value = null
}

function applyQuestion(question: IntelCheckQuestion) {
  const drawingRules = question.drawing_rules ?? {}
  Object.assign(form, {
    kind: question.kind,
    title: question.title,
    prompt: question.prompt,
    expected_answer: question.expected_answer,
    match_mode: (question.match_mode || 'exact') as IntelCheckMatchMode,
    reference_html: question.reference_html,
    required_keywords: Array.isArray(drawingRules.required_keywords)
      ? drawingRules.required_keywords.join(', ')
      : '',
    review_rubric: question.review_rubric,
    drawing_rules: {
      min_ratio: typeof drawingRules.min_ratio === 'number' ? drawingRules.min_ratio : 0.7,
      max_bytes: typeof drawingRules.max_bytes === 'number' ? drawingRules.max_bytes : 262144,
      standard_sources: Array.isArray(drawingRules.standard_sources) ? [...drawingRules.standard_sources] : [],
    },
    enabled: question.enabled,
  })
}

async function loadDetail(id: number) {
  abortController?.abort()
  const controller = new AbortController()
  abortController = controller
  detailLoading.value = true
  try {
    const question = await adminAPI.intelCheck.getQuestion(id)
    if (controller.signal.aborted) return
    detailQuestion.value = question
    applyQuestion(question)
  } catch (err: unknown) {
    const reason = err as { name?: string; code?: string }
    if (reason.name === 'AbortError' || reason.code === 'ERR_CANCELED') return
    appStore.showError(extractApiErrorMessage(err, t('admin.intelCheck.common.loadError')))
  } finally {
    if (abortController === controller) {
      detailLoading.value = false
      abortController = null
    }
  }
}

function handleKindChange() {
  importVersion++
  importing.value = false
  savedMetrics.value = null
  detailQuestion.value = null
  if (form.kind === 'logic') {
    form.reference_html = ''
    form.required_keywords = ''
    form.review_rubric = ''
    form.drawing_rules.standard_sources = []
  } else {
    form.expected_answer = ''
    form.match_mode = 'exact'
  }
}

function buildPayload(): IntelCheckQuestionParams {
  const payload: IntelCheckQuestionParams = {
    kind: form.kind,
    title: form.title.trim(),
    prompt: form.prompt.trim(),
    enabled: form.enabled,
  }

  if (form.kind === 'logic') {
    payload.expected_answer = form.expected_answer.trim()
    payload.match_mode = form.match_mode
  } else {
    const rules: IntelCheckDrawingRules = {
      min_ratio: form.drawing_rules.min_ratio,
      max_bytes: form.drawing_rules.max_bytes,
      standard_sources: [...form.drawing_rules.standard_sources],
      required_keywords: form.required_keywords
        .split(',')
        .map((keyword) => keyword.trim())
        .filter(Boolean),
    }
    payload.reference_html = form.reference_html
    payload.drawing_rules = rules
    payload.review_rubric = form.review_rubric.trim()
  }

  return payload
}

async function save() {
  if (saving.value || detailLoading.value || importing.value) return
  saving.value = true
  try {
    const payload = buildPayload()
    const result = props.question
      ? await adminAPI.intelCheck.updateQuestion(props.question.id, payload)
      : await adminAPI.intelCheck.createQuestion(payload)

    savedMetrics.value = result.reference_metrics
    detailQuestion.value = result
    applyQuestion(result)
    appStore.showSuccess(t('admin.intelCheck.common.saveSuccess'))
    emit('saved', result)
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('admin.intelCheck.common.saveError')))
  } finally {
    saving.value = false
  }
}

async function importStandards(event: Event) {
  const input = event.target as HTMLInputElement
  const files = Array.from(input.files ?? [])
  if (!files.length) return
  if (files.length > 8 || files.some((file) => file.size > 512 * 1024)) {
    appStore.showError(t('admin.intelCheck.questions.form.standardFilesLimit'))
    input.value = ''
    return
  }
  importing.value = true
  const version = ++importVersion
  try {
    const sources = await Promise.all(files.map((file) => file.text()))
    if (version !== importVersion || !props.show) return
    form.reference_html = sources[0] ?? ''
    form.drawing_rules.standard_sources = sources.slice(1)
    savedMetrics.value = null
    detailQuestion.value = null
  } catch {
    if (version === importVersion && props.show) {
      appStore.showError(t('admin.intelCheck.common.loadError'))
    }
  } finally {
    if (version === importVersion) importing.value = false
    input.value = ''
  }
}

watch(
  () => [props.show, props.question] as const,
  ([show, question]) => {
    if (!show) {
      importVersion++
      importing.value = false
      abortController?.abort()
      abortController = null
      return
    }

    resetForm()
    if (question) {
      void loadDetail(question.id)
    }
  },
  { immediate: true },
)
</script>
