<template>
  <div class="space-y-4">
    <div class="flex justify-end">
      <button type="button" class="btn btn-primary" @click="openCreate">
        {{ t('admin.intelCheck.targets.create') }}
      </button>
    </div>

    <DataTable :columns="columns" :data="targets" :loading="loading">
      <template #cell-name="{ row }">
        <div class="flex items-center gap-1.5">
          <span class="font-medium text-gray-900 dark:text-white">{{ row.name }}</span>
          <HelpTooltip
            v-if="row.api_key_decrypt_failed"
            :content="t('admin.intelCheck.targets.decryptFailed')"
          >
            <Icon name="exclamationTriangle" size="sm" class="text-red-500" />
          </HelpTooltip>
        </div>
        <p v-if="row.description" class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
          {{ row.description }}
        </p>
      </template>

      <template #cell-model="{ row }">
        <span class="text-sm text-gray-900 dark:text-gray-100">{{ row.model }}</span>
        <span class="ml-1.5 text-xs text-gray-500 dark:text-gray-400">
          {{ t(`admin.intelCheck.effort.${row.reasoning_effort}`) }}
        </span>
      </template>

      <template #cell-base_url="{ row }">
        <span class="break-all text-xs text-gray-600 dark:text-gray-300">{{ row.base_url }}</span>
        <span class="ml-1 text-xs text-gray-400">
          {{ t(`admin.intelCheck.apiMode.${row.api_mode}`) }}
        </span>
      </template>

      <template #cell-api_key_masked="{ row }">
        <code class="text-xs text-gray-500 dark:text-gray-400">{{ row.api_key_masked }}</code>
      </template>

      <template #cell-enabled="{ row }">
        <Toggle :model-value="row.enabled" @update:model-value="toggleEnabled(row)" />
      </template>

      <template #cell-actions="{ row }">
        <div class="flex gap-2">
          <button type="button" class="btn-link" @click="openEdit(row)">{{ t('common.edit') }}</button>
          <button type="button" class="btn-link text-red-600" @click="askDelete(row)">
            {{ t('common.delete') }}
          </button>
        </div>
      </template>

      <template #empty>
        <EmptyState
          :title="t('admin.intelCheck.targets.empty')"
          :description="t('admin.intelCheck.targets.emptyHint')"
          :action-text="t('admin.intelCheck.targets.create')"
          @action="openCreate"
        />
      </template>
    </DataTable>

    <IntelCheckTargetDialog
      :show="showDialog"
      :target="editing"
      @close="showDialog = false"
      @saved="onSaved"
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
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import { adminAPI } from '@/api/admin'
import type { IntelCheckTarget } from '@/api/admin/intelCheck'
import type { Column } from '@/components/common/types'
import DataTable from '@/components/common/DataTable.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import HelpTooltip from '@/components/common/HelpTooltip.vue'
import Toggle from '@/components/common/Toggle.vue'
import Icon from '@/components/icons/Icon.vue'
import IntelCheckTargetDialog from './IntelCheckTargetDialog.vue'

const { t } = useI18n()
const appStore = useAppStore()

const targets = ref<IntelCheckTarget[]>([])
const loading = ref(false)
const showDialog = ref(false)
const editing = ref<IntelCheckTarget | null>(null)
const showDelete = ref(false)
const deleting = ref<IntelCheckTarget | null>(null)
let abortController: AbortController | null = null

const columns = computed<Column[]>(() => [
  { key: 'name', label: t('admin.intelCheck.targets.columns.name'), sortable: false },
  { key: 'model', label: t('admin.intelCheck.targets.columns.model'), sortable: false },
  { key: 'base_url', label: t('admin.intelCheck.targets.columns.baseUrl'), sortable: false },
  { key: 'api_key_masked', label: t('admin.intelCheck.targets.columns.apiKey'), sortable: false },
  { key: 'enabled', label: t('admin.intelCheck.common.enabled'), sortable: false },
  { key: 'actions', label: t('admin.intelCheck.common.actions'), sortable: false },
])

const deleteMessage = computed(() =>
  t('admin.intelCheck.targets.deleteConfirm', { name: deleting.value?.name ?? '' }),
)

async function reload() {
  abortController?.abort()
  const controller = new AbortController()
  abortController = controller
  loading.value = true
  try {
    // 分组数量天然很少（一个部署通常几个到十几个），一次取满即可，不做分页。
    const res = await adminAPI.intelCheck.listTargets(
      { page: 1, page_size: 100 },
      { signal: controller.signal },
    )
    if (!controller.signal.aborted) targets.value = res.items || []
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

function openCreate() {
  editing.value = null
  showDialog.value = true
}

function openEdit(row: IntelCheckTarget) {
  editing.value = row
  showDialog.value = true
}

function onSaved() {
  showDialog.value = false
  void reload()
}

async function toggleEnabled(row: IntelCheckTarget) {
  const next = !row.enabled
  try {
    // 后端是整块覆盖语义，不能只发 { enabled }——那会把名称、地址等一并清空。
    // api_key 留空表示保留原凭据，这正是此处需要的。
    await adminAPI.intelCheck.updateTarget(row.id, {
      name: row.name,
      description: row.description,
      base_url: row.base_url,
      api_mode: row.api_mode,
      model: row.model,
      reasoning_effort: row.reasoning_effort as never,
      rate_label: row.rate_label,
      sort_order: row.sort_order,
      enabled: next,
    })
    row.enabled = next
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('admin.intelCheck.common.saveError')))
  }
}

function askDelete(row: IntelCheckTarget) {
  deleting.value = row
  showDelete.value = true
}

async function confirmDelete() {
  if (!deleting.value) return
  try {
    await adminAPI.intelCheck.deleteTarget(deleting.value.id)
    appStore.showSuccess(t('admin.intelCheck.common.deleteSuccess'))
    showDelete.value = false
    deleting.value = null
    void reload()
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('common.error')))
  }
}

onMounted(() => void reload())
onUnmounted(() => abortController?.abort())
</script>
