<template>
  <div class="space-y-5">
    <div class="flex flex-wrap items-center justify-end gap-2">
      <button
        type="button"
        class="btn btn-secondary"
        :disabled="loadingRounds || loadingResults || runningNow"
        :title="t('common.refresh')"
        @click="refresh"
      >
        <Icon name="refresh" size="md" :class="{ 'animate-spin': loadingRounds || loadingResults }" />
        <span class="ml-1.5">{{ t('common.refresh') }}</span>
      </button>
      <button type="button" class="btn btn-primary" :disabled="runningNow" @click="runNow">
        <Icon name="play" size="sm" class="mr-1.5" />
        {{ runningNow ? t('common.processing') : t('admin.intelCheck.overview.runNow') }}
      </button>
    </div>

    <section class="card overflow-hidden">
      <div class="flex flex-wrap items-center justify-between gap-2 border-b border-gray-100 px-5 py-4 dark:border-dark-700">
        <div>
          <h2 class="text-sm font-semibold text-gray-900 dark:text-white">
            {{ t('admin.intelCheck.overview.rounds') }}
          </h2>
          <p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
            {{ t('admin.intelCheck.overview.noRoundsHint') }}
          </p>
        </div>
        <span class="text-xs text-gray-500 dark:text-gray-400">{{ roundsPagination.total }}</span>
      </div>

      <DataTable :columns="roundColumns" :data="rounds" :loading="loadingRounds">
        <template #cell-seq="{ row }">
          <span class="font-mono text-sm font-medium text-gray-900 dark:text-white">#{{ row.seq }}</span>
        </template>
        <template #cell-started_at="{ row }">
          <span class="text-xs text-gray-600 dark:text-gray-300">{{ formatDateTime(row.started_at) }}</span>
        </template>
        <template #cell-finished_at="{ row }">
          <span v-if="row.finished_at" class="text-xs text-gray-600 dark:text-gray-300">{{ formatDateTime(row.finished_at) }}</span>
          <span v-else class="text-xs text-sky-600 dark:text-sky-400">{{ t('admin.intelCheck.overview.stillRunning') }}</span>
        </template>
        <template #cell-trigger_source="{ row }">
          <span class="text-xs text-gray-600 dark:text-gray-300">{{ t(`admin.intelCheck.overview.trigger.${row.trigger_source}`) }}</span>
        </template>
        <template #cell-questions="{ row }">
          <span class="text-xs text-gray-600 dark:text-gray-300">
            {{ formatQuestionIds(row) }}
          </span>
        </template>
        <template #empty>
          <EmptyState
            :title="t('admin.intelCheck.overview.noRounds')"
            :description="t('admin.intelCheck.overview.noRoundsHint')"
          />
        </template>
      </DataTable>

      <Pagination
        v-if="roundsPagination.total > 0"
        :page="roundsPagination.page"
        :total="roundsPagination.total"
        :page-size="roundsPagination.page_size"
        :page-size-options="[20, 50, 100]"
        @update:page="handleRoundsPageChange"
        @update:pageSize="handleRoundsPageSizeChange"
      />
    </section>

    <section class="card overflow-hidden">
      <div class="border-b border-gray-100 px-5 py-4 dark:border-dark-700">
        <div class="flex flex-wrap items-center justify-between gap-2">
          <h2 class="text-sm font-semibold text-gray-900 dark:text-white">
            {{ t('admin.intelCheck.overview.results') }}
          </h2>
          <span class="text-xs text-gray-500 dark:text-gray-400">{{ resultsPagination.total }}</span>
        </div>

        <div class="mt-4 grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
          <div>
            <label class="label" for="intel-check-round-filter">{{ t('admin.intelCheck.overview.roundColumns.seq') }}</label>
            <select id="intel-check-round-filter" v-model="filters.round_id" class="input" @change="reloadResultsFromFirstPage">
              <option value="">{{ t('admin.intelCheck.overview.filterAll') }}</option>
              <option v-for="round in rounds" :key="round.id" :value="round.id">
                #{{ round.seq }}
              </option>
            </select>
          </div>
          <div>
            <label class="label" for="intel-check-target-filter">{{ t('admin.intelCheck.overview.resultColumns.target') }}</label>
            <select id="intel-check-target-filter" v-model="filters.target_id" class="input" @change="reloadResultsFromFirstPage">
              <option value="">{{ t('admin.intelCheck.overview.filterAll') }}</option>
              <option v-for="target in targets" :key="target.id" :value="target.id">
                {{ target.name }}
              </option>
            </select>
          </div>
          <div>
            <label class="label" for="intel-check-kind-filter">{{ t('admin.intelCheck.overview.resultColumns.kind') }}</label>
            <select id="intel-check-kind-filter" v-model="filters.kind" class="input" @change="reloadResultsFromFirstPage">
              <option value="">{{ t('admin.intelCheck.overview.filterAll') }}</option>
              <option value="logic">{{ t('admin.intelCheck.common.logic') }}</option>
              <option value="drawing">{{ t('admin.intelCheck.common.drawing') }}</option>
            </select>
          </div>
          <div>
            <label class="label" for="intel-check-status-filter">{{ t('admin.intelCheck.overview.resultColumns.status') }}</label>
            <select id="intel-check-status-filter" v-model="filters.status" class="input" @change="reloadResultsFromFirstPage">
              <option value="">{{ t('admin.intelCheck.overview.filterAll') }}</option>
              <option v-for="status in statuses" :key="status" :value="status">
                {{ t(`admin.intelCheck.status.${status}`) }}
              </option>
            </select>
          </div>
        </div>
      </div>

      <DataTable :columns="resultColumns" :data="results" :loading="loadingResults">
        <template #cell-target="{ row }">
          <span class="text-sm text-gray-900 dark:text-white">{{ targetName(row.target_id) }}</span>
        </template>
        <template #cell-kind="{ row }">
          <span class="text-xs text-gray-600 dark:text-gray-300">{{ t(`admin.intelCheck.common.${row.kind}`) }}</span>
        </template>
        <template #cell-status="{ row }">
          <span class="inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium" :class="statusClass(row.status)">
            {{ t(`admin.intelCheck.status.${row.status}`) }}
          </span>
        </template>
        <template #cell-latency="{ row }">
          <span class="text-xs text-gray-600 dark:text-gray-300">{{ formatLatency(row.latency_ms) }}</span>
        </template>
        <template #cell-tokens="{ row }">
          <span class="font-mono text-xs text-gray-600 dark:text-gray-300">{{ tokenText(row) }}</span>
        </template>
        <template #cell-error="{ row }">
          <span
            v-if="row.error_message"
            class="block max-w-[360px] truncate text-xs text-gray-600 dark:text-gray-300"
            :title="row.error_message"
          >
            {{ row.error_message }}
          </span>
          <span v-else class="text-xs text-gray-400 dark:text-gray-500">--</span>
        </template>
        <template #cell-checked_at="{ row }">
          <span class="text-xs text-gray-600 dark:text-gray-300">{{ formatDateTime(row.checked_at) }}</span>
        </template>
        <template #empty>
          <EmptyState :title="t('admin.intelCheck.overview.noResults')" />
        </template>
      </DataTable>

      <Pagination
        v-if="resultsPagination.total > 0"
        :page="resultsPagination.page"
        :total="resultsPagination.total"
        :page-size="resultsPagination.page_size"
        :page-size-options="[20]"
        @update:page="handleResultsPageChange"
        @update:pageSize="handleResultsPageSizeChange"
      />
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type {
  IntelCheckResultListItem,
  IntelCheckRound,
  IntelCheckStatus,
  IntelCheckTarget,
} from '@/api/admin/intelCheck'
import type { Column } from '@/components/common/types'
import DataTable from '@/components/common/DataTable.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import Pagination from '@/components/common/Pagination.vue'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'

const { t } = useI18n()
const appStore = useAppStore()
const rounds = ref<IntelCheckRound[]>([])
const results = ref<IntelCheckResultListItem[]>([])
const targets = ref<IntelCheckTarget[]>([])
const loadingRounds = ref(false)
const loadingResults = ref(false)
const runningNow = ref(false)
const roundsPagination = reactive({ page: 1, page_size: 20, total: 0 })
const resultsPagination = reactive({ page: 1, page_size: 20, total: 0 })
const filters = reactive<{
  round_id: number | ''
  target_id: number | ''
  kind: '' | 'logic' | 'drawing'
  status: '' | IntelCheckStatus
}>({
  round_id: '',
  target_id: '',
  kind: '',
  status: '',
})
const statuses: IntelCheckStatus[] = ['pass', 'fail', 'unverified', 'request_error', 'running']
let roundsAbortController: AbortController | null = null
let resultsAbortController: AbortController | null = null

const roundColumns = computed<Column[]>(() => [
  { key: 'seq', label: t('admin.intelCheck.overview.roundColumns.seq') },
  { key: 'started_at', label: t('admin.intelCheck.overview.roundColumns.startedAt') },
  { key: 'finished_at', label: t('admin.intelCheck.overview.roundColumns.finishedAt') },
  { key: 'trigger_source', label: t('admin.intelCheck.overview.roundColumns.trigger') },
  { key: 'questions', label: t('admin.intelCheck.overview.roundColumns.questions') },
])

const resultColumns = computed<Column[]>(() => [
  { key: 'target', label: t('admin.intelCheck.overview.resultColumns.target') },
  { key: 'kind', label: t('admin.intelCheck.overview.resultColumns.kind') },
  { key: 'status', label: t('admin.intelCheck.overview.resultColumns.status') },
  { key: 'latency', label: t('admin.intelCheck.overview.resultColumns.latency') },
  { key: 'tokens', label: t('admin.intelCheck.overview.resultColumns.tokens') },
  { key: 'error', label: t('admin.intelCheck.overview.resultColumns.error') },
  { key: 'checked_at', label: t('admin.intelCheck.overview.resultColumns.checkedAt') },
])

function formatDateTime(value: string | null | undefined): string {
  if (!value) return '--'
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString(undefined, { hour12: false })
}

function formatLatency(value: number | null): string {
  if (value == null) return '--'
  if (value < 1000) return `${Math.round(value)} ms`
  if (value < 60_000) return `${(value / 1000).toFixed(1)} s`
  const minutes = Math.floor(value / 60_000)
  const seconds = Math.round((value % 60_000) / 1000)
  return `${minutes}m ${seconds}s`
}

function formatQuestionIds(round: IntelCheckRound): string {
  const ids = [round.logic_question_id, round.drawing_question_id].filter((id): id is number => id != null)
  return ids.length ? ids.map((id) => `#${id}`).join(' / ') : '--'
}

function targetName(id: number): string {
  const target = targets.value.find((item) => item.id === id)
  return target ? target.name : `#${id}`
}

function tokenText(row: IntelCheckResultListItem): string {
  if (row.input_tokens == null && row.output_tokens == null) return '--'
  return `${row.input_tokens ?? 0} / ${row.output_tokens ?? 0}`
}

function statusClass(status: IntelCheckStatus): string {
  switch (status) {
    case 'pass':
      return 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-300'
    case 'fail':
      return 'bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-300'
    case 'request_error':
      return 'bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-300'
    case 'unverified':
      return 'bg-slate-100 text-slate-700 dark:bg-slate-800 dark:text-slate-300'
    default:
      return 'bg-sky-100 text-sky-700 dark:bg-sky-900/40 dark:text-sky-300'
  }
}

async function loadTargets() {
  try {
    const response = await adminAPI.intelCheck.listTargets({ page: 1, page_size: 100 })
    targets.value = response.items ?? []
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('admin.intelCheck.common.loadError')))
  }
}

async function loadRounds() {
  roundsAbortController?.abort()
  const controller = new AbortController()
  roundsAbortController = controller
  loadingRounds.value = true
  try {
    const response = await adminAPI.intelCheck.listRounds(
      { page: roundsPagination.page, page_size: roundsPagination.page_size },
      { signal: controller.signal },
    )
    if (controller.signal.aborted) return
    rounds.value = response.items ?? []
    roundsPagination.total = response.total
    roundsPagination.page = response.page
    roundsPagination.page_size = response.page_size
  } catch (err: unknown) {
    const reason = err as { name?: string; code?: string }
    if (reason.name === 'AbortError' || reason.code === 'ERR_CANCELED') return
    appStore.showError(extractApiErrorMessage(err, t('admin.intelCheck.common.loadError')))
  } finally {
    if (roundsAbortController === controller) {
      loadingRounds.value = false
      roundsAbortController = null
    }
  }
}

async function loadResults() {
  resultsAbortController?.abort()
  const controller = new AbortController()
  resultsAbortController = controller
  loadingResults.value = true
  try {
    const response = await adminAPI.intelCheck.listResults(
      {
        page: resultsPagination.page,
        page_size: Math.min(resultsPagination.page_size, 20),
        round_id: filters.round_id || undefined,
        target_id: filters.target_id || undefined,
        kind: filters.kind || undefined,
        status: filters.status || undefined,
      },
      { signal: controller.signal },
    )
    if (controller.signal.aborted) return
    results.value = response.items ?? []
    resultsPagination.total = response.total
    resultsPagination.page = response.page
    resultsPagination.page_size = Math.min(response.page_size, 20)
  } catch (err: unknown) {
    const reason = err as { name?: string; code?: string }
    if (reason.name === 'AbortError' || reason.code === 'ERR_CANCELED') return
    appStore.showError(extractApiErrorMessage(err, t('admin.intelCheck.common.loadError')))
  } finally {
    if (resultsAbortController === controller) {
      loadingResults.value = false
      resultsAbortController = null
    }
  }
}

function refresh() {
  void Promise.all([loadTargets(), loadRounds(), loadResults()])
}

function reloadResultsFromFirstPage() {
  resultsPagination.page = 1
  void loadResults()
}

function handleRoundsPageChange(page: number) {
  roundsPagination.page = page
  void loadRounds()
}

function handleRoundsPageSizeChange(pageSize: number) {
  roundsPagination.page_size = Math.min(pageSize, 100)
  roundsPagination.page = 1
  void loadRounds()
}

function handleResultsPageChange(page: number) {
  resultsPagination.page = page
  void loadResults()
}

function handleResultsPageSizeChange(pageSize: number) {
  resultsPagination.page_size = Math.min(pageSize, 20)
  resultsPagination.page = 1
  void loadResults()
}

function isConflictError(err: unknown): boolean {
  if (!err || typeof err !== 'object') return false
  const value = err as { status?: number; response?: { status?: number } }
  return value.status === 409 || value.response?.status === 409
}

async function runNow() {
  if (runningNow.value) return
  runningNow.value = true
  try {
    await adminAPI.intelCheck.runNow()
    appStore.showSuccess(t('admin.intelCheck.overview.runStarted'))
    await Promise.all([loadRounds(), loadResults()])
  } catch (err: unknown) {
    if (isConflictError(err)) {
      appStore.showWarning(t('admin.intelCheck.overview.runInFlight'))
    } else {
      appStore.showError(extractApiErrorMessage(err, t('admin.intelCheck.overview.runFailed')))
    }
  } finally {
    runningNow.value = false
  }
}

onMounted(() => {
  void Promise.all([loadTargets(), loadRounds(), loadResults()])
})

onUnmounted(() => {
  roundsAbortController?.abort()
  resultsAbortController?.abort()
})
</script>
