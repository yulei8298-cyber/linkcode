<template>
  <div class="space-y-4">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <div class="flex items-center gap-2">
        <label class="text-sm text-gray-600 dark:text-gray-300" for="intel-check-question-kind">
          {{ t('admin.intelCheck.questions.form.kind') }}
        </label>
        <select id="intel-check-question-kind" v-model="kindFilter" class="input w-36" @change="reloadFromFirstPage">
          <option value="">{{ t('admin.intelCheck.overview.filterAll') }}</option>
          <option value="logic">{{ t('admin.intelCheck.common.logic') }}</option>
          <option value="drawing">{{ t('admin.intelCheck.common.drawing') }}</option>
        </select>
      </div>
      <button type="button" class="btn btn-primary" @click="openCreate">
        {{ t('admin.intelCheck.questions.create') }}
      </button>
    </div>

    <DataTable :columns="columns" :data="questions" :loading="loading">
      <template #cell-title="{ row }">
        <div class="max-w-[360px]">
          <div class="font-medium text-gray-900 dark:text-white">{{ row.title }}</div>
          <div class="mt-0.5 truncate text-xs text-gray-500 dark:text-gray-400" :title="row.prompt">
            {{ row.prompt }}
          </div>
        </div>
      </template>

      <template #cell-kind="{ row }">
        <span class="badge" :class="row.kind === 'drawing' ? 'badge-info' : 'badge-neutral'">
          {{ t(`admin.intelCheck.common.${row.kind}`) }}
        </span>
      </template>

      <template #cell-answer="{ row }">
        <div v-if="row.kind === 'logic'" class="max-w-[220px]">
          <div class="truncate text-sm text-gray-700 dark:text-gray-200" :title="row.expected_answer">
            {{ row.expected_answer || '--' }}
          </div>
          <div class="text-xs text-gray-500 dark:text-gray-400">
            {{ t(`admin.intelCheck.matchMode.${row.match_mode}`) }}
          </div>
        </div>
        <span v-else class="text-sm text-gray-400 dark:text-gray-500">--</span>
      </template>

      <template #cell-reference="{ row }">
        <div v-if="row.kind === 'drawing'" class="space-y-0.5 text-xs">
          <div :class="row.reference_html_bytes > 0 ? 'text-gray-700 dark:text-gray-200' : 'text-amber-600 dark:text-amber-400'">
            {{ row.reference_html_bytes > 0 ? t('admin.intelCheck.common.bytes', { n: row.reference_html_bytes }) : t('admin.intelCheck.questions.noReference') }}
          </div>
          <div v-if="row.reference_metrics?.structure_baseline" class="text-gray-500 dark:text-gray-400">
            {{ row.reference_metrics.structure_baseline.shapes }} {{ t('admin.intelCheck.questions.metrics.structure_shapes') }}
          </div>
        </div>
        <span v-else class="text-sm text-gray-400 dark:text-gray-500">--</span>
      </template>

      <template #cell-standards="{ row }">
        <span
          v-if="row.kind === 'drawing'"
          class="text-xs text-gray-700 dark:text-gray-200"
        >
          {{ row.reference_metrics?.standard_count ?? (row.reference_html_bytes > 0 ? 1 : 0) }}
        </span>
        <span v-else class="text-sm text-gray-400 dark:text-gray-500">--</span>
      </template>

      <template #cell-enabled="{ row }">
        <span class="text-xs" :class="row.enabled ? 'text-emerald-600 dark:text-emerald-400' : 'text-gray-500 dark:text-gray-400'">
          {{ row.enabled ? t('common.enabled') : t('common.disabled') }}
        </span>
      </template>

      <template #cell-actions="{ row }">
        <div class="flex flex-wrap items-center gap-x-3 gap-y-1">
          <button type="button" class="btn-link" @click="openEdit(row)">{{ t('common.edit') }}</button>
          <button type="button" class="btn-link" @click="openTrial(row, 'dry-run')">
            {{ t('admin.intelCheck.trial.dryRun') }}
          </button>
          <button
            v-if="row.kind === 'drawing'"
            type="button"
            class="btn-link"
            @click="openTrial(row, 'evaluate')"
          >
            {{ t('admin.intelCheck.trial.evaluate') }}
          </button>
          <button type="button" class="btn-link text-red-600" @click="askDelete(row)">
            {{ t('common.delete') }}
          </button>
        </div>
      </template>

      <template #empty>
        <EmptyState
          :title="t('admin.intelCheck.questions.empty')"
          :description="t('admin.intelCheck.questions.emptyHint')"
          :action-text="t('admin.intelCheck.questions.create')"
          @action="openCreate"
        />
      </template>
    </DataTable>

    <Pagination
      v-if="pagination.total > 0"
      :page="pagination.page"
      :total="pagination.total"
      :page-size="pagination.page_size"
      :page-size-options="[20, 50, 100]"
      @update:page="handlePageChange"
      @update:pageSize="handlePageSizeChange"
    />

    <IntelCheckQuestionDialog
      :show="showQuestionDialog"
      :question="editing"
      @close="closeQuestionDialog"
      @saved="onQuestionSaved"
    />

    <IntelCheckTrialDialog
      :show="showTrialDialog"
      :question="trialQuestion"
      :mode="trialMode"
      @close="closeTrialDialog"
    />

    <ConfirmDialog
      :show="showDelete"
      :title="t('common.delete')"
      :message="deleteMessage"
      :confirm-text="t('common.delete')"
      :cancel-text="t('common.cancel')"
      :danger="true"
      @confirm="confirmDelete"
      @cancel="showDelete = false"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type {
  IntelCheckKind,
  IntelCheckQuestion,
  IntelCheckQuestionListItem,
} from '@/api/admin/intelCheck'
import type { Column } from '@/components/common/types'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import DataTable from '@/components/common/DataTable.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import Pagination from '@/components/common/Pagination.vue'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import IntelCheckQuestionDialog from './IntelCheckQuestionDialog.vue'
import IntelCheckTrialDialog from './IntelCheckTrialDialog.vue'

const { t } = useI18n()
const appStore = useAppStore()
const questions = ref<IntelCheckQuestionListItem[]>([])
const loading = ref(false)
const kindFilter = ref<IntelCheckKind | ''>('')
const showQuestionDialog = ref(false)
const editing = ref<IntelCheckQuestionListItem | null>(null)
const showTrialDialog = ref(false)
const trialQuestion = ref<IntelCheckQuestionListItem | null>(null)
const trialMode = ref<'evaluate' | 'dry-run'>('dry-run')
const showDelete = ref(false)
const deleting = ref<IntelCheckQuestionListItem | null>(null)
const pagination = reactive({ page: 1, page_size: 20, total: 0 })
let abortController: AbortController | null = null

const columns = computed<Column[]>(() => [
  { key: 'title', label: t('admin.intelCheck.questions.columns.title') },
  { key: 'kind', label: t('admin.intelCheck.questions.columns.kind') },
  { key: 'answer', label: t('admin.intelCheck.questions.columns.answer') },
  { key: 'reference', label: t('admin.intelCheck.questions.columns.reference') },
  { key: 'standards', label: t('admin.intelCheck.questions.metrics.standard_count') },
  { key: 'enabled', label: t('admin.intelCheck.common.enabled') },
  { key: 'actions', label: t('admin.intelCheck.common.actions') },
])

const deleteMessage = computed(() =>
  t('admin.intelCheck.questions.deleteConfirm', { title: deleting.value?.title ?? '' }),
)

async function reload() {
  abortController?.abort()
  const controller = new AbortController()
  abortController = controller
  loading.value = true
  try {
    const result = await adminAPI.intelCheck.listQuestions(
      {
        page: pagination.page,
        page_size: pagination.page_size,
        kind: kindFilter.value || undefined,
      },
      { signal: controller.signal },
    )
    if (controller.signal.aborted) return
    questions.value = result.items ?? []
    pagination.total = result.total
    pagination.page = result.page
    pagination.page_size = result.page_size
  } catch (err: unknown) {
    const reason = err as { name?: string; code?: string }
    if (reason.name === 'AbortError' || reason.code === 'ERR_CANCELED') return
    appStore.showError(extractApiErrorMessage(err, t('admin.intelCheck.common.loadError')))
  } finally {
    if (abortController === controller) {
      loading.value = false
      abortController = null
    }
  }
}

function reloadFromFirstPage() {
  pagination.page = 1
  void reload()
}

function handlePageChange(page: number) {
  pagination.page = page
  void reload()
}

function handlePageSizeChange(pageSize: number) {
  pagination.page_size = Math.min(pageSize, 100)
  pagination.page = 1
  void reload()
}

function openCreate() {
  editing.value = null
  showQuestionDialog.value = true
}

function openEdit(row: IntelCheckQuestionListItem) {
  editing.value = row
  showQuestionDialog.value = true
}

function closeQuestionDialog() {
  showQuestionDialog.value = false
  editing.value = null
}

function onQuestionSaved(_question: IntelCheckQuestion) {
  closeQuestionDialog()
  void reload()
}

function openTrial(question: IntelCheckQuestionListItem, mode: 'evaluate' | 'dry-run') {
  trialQuestion.value = question
  trialMode.value = mode
  showTrialDialog.value = true
}

function closeTrialDialog() {
  showTrialDialog.value = false
  trialQuestion.value = null
}

function askDelete(question: IntelCheckQuestionListItem) {
  deleting.value = question
  showDelete.value = true
}

async function confirmDelete() {
  if (!deleting.value) return
  try {
    await adminAPI.intelCheck.deleteQuestion(deleting.value.id)
    appStore.showSuccess(t('admin.intelCheck.common.deleteSuccess'))
    showDelete.value = false
    deleting.value = null
    if (questions.value.length === 1 && pagination.page > 1) pagination.page -= 1
    await reload()
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('admin.intelCheck.common.saveError')))
  }
}

onMounted(() => void reload())
onUnmounted(() => abortController?.abort())
</script>
