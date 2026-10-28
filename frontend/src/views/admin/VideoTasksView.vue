<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Pagination from '@/components/common/Pagination.vue'
import BrandIcon from '@/components/icons/brands/BrandIcon.vue'
import { adminAPI } from '@/api/admin'
import type { VideoBillingStatus, VideoTask, VideoTaskStatus } from '@/api/admin/videoTasks'
import { listVideoProtocols, videoVendorStyle } from '@/constants/videoBrands'
import { useAppStore } from '@/stores'
import { extractApiErrorMessage } from '@/utils/apiError'
import { formatDateTime } from '@/utils/format'

const { t } = useI18n()
const appStore = useAppStore()

const tasks = ref<VideoTask[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)
const loading = ref(false)
const busyTask = ref('')
const filters = reactive({ keyword: '', model: '', user_id: '', status: '', billing_status: '', protocol: '' })

const protocols = listVideoProtocols()
const statuses: VideoTaskStatus[] = ['queued', 'in_progress', 'completed', 'failed', 'cancelled']
const billingStatuses: VideoBillingStatus[] = ['pending', 'billed', 'skipped', 'failed']

const statusClass: Record<VideoTaskStatus, string> = {
  queued: 'bg-gray-100 text-gray-700 dark:bg-dark-600 dark:text-gray-300',
  in_progress: 'bg-sky-100 text-sky-700 dark:bg-sky-900/30 dark:text-sky-300',
  completed: 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300',
  failed: 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-300',
  cancelled: 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300',
}
const billingClass: Record<VideoBillingStatus, string> = {
  pending: 'text-amber-600 dark:text-amber-300',
  billed: 'text-emerald-600 dark:text-emerald-300',
  skipped: 'text-gray-500 dark:text-gray-400',
  failed: 'text-red-600 dark:text-red-400',
}

async function load() {
  loading.value = true
  try {
    const userID = Number(filters.user_id)
    const res = await adminAPI.videoTasks.list({
      page: page.value,
      page_size: pageSize.value,
      keyword: filters.keyword.trim() || undefined,
      model: filters.model.trim() || undefined,
      user_id: Number.isInteger(userID) && userID > 0 ? userID : undefined,
      status: filters.status || undefined,
      billing_status: filters.billing_status || undefined,
      protocol: filters.protocol || undefined,
    })
    tasks.value = res.items ?? []
    total.value = res.total
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err))
  } finally {
    loading.value = false
  }
}

function search() {
  page.value = 1
  load()
}

async function act(task: VideoTask, action: 'refresh' | 'retryBilling') {
  busyTask.value = task.task_id
  try {
    await adminAPI.videoTasks[action](task.task_id)
    appStore.showSuccess(t(action === 'refresh' ? 'admin.video.tasks.refreshed' : 'admin.video.tasks.retried'))
    await load()
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err))
  } finally {
    busyTask.value = ''
  }
}

const money = (v: number | null | undefined) => (v == null ? '-' : `$${v.toFixed(4)}`)
const seconds = (task: VideoTask) => task.actual_duration_seconds ?? task.duration_seconds

onMounted(load)
</script>

<template>
  <AppLayout>
    <div class="space-y-4">
      <div class="flex flex-wrap items-end gap-2 rounded-xl border border-gray-200 bg-white p-4 dark:border-dark-500 dark:bg-dark-800">
        <input v-model="filters.keyword" class="input w-56" :placeholder="t('admin.video.tasks.keyword')" @keyup.enter="search" />
        <input v-model="filters.model" class="input w-44" :placeholder="t('admin.video.tasks.model')" @keyup.enter="search" />
        <input v-model="filters.user_id" class="input w-28" :placeholder="t('admin.video.tasks.userId')" @keyup.enter="search" />
        <select v-model="filters.status" class="input w-36">
          <option value="">{{ t('admin.video.tasks.allStatus') }}</option>
          <option v-for="s in statuses" :key="s" :value="s">{{ t(`admin.video.tasks.status.${s}`) }}</option>
        </select>
        <select v-model="filters.billing_status" class="input w-36">
          <option value="">{{ t('admin.video.tasks.allBilling') }}</option>
          <option v-for="s in billingStatuses" :key="s" :value="s">{{ t(`admin.video.tasks.billingStatus.${s}`) }}</option>
        </select>
        <select v-model="filters.protocol" class="input w-48">
          <option value="">{{ t('admin.video.tasks.allProtocols') }}</option>
          <option v-for="p in protocols" :key="p.protocol" :value="p.protocol">{{ p.display_name }}</option>
        </select>
        <button class="btn btn-primary" :disabled="loading" @click="search">{{ t('admin.video.tasks.search') }}</button>
      </div>

      <div class="overflow-x-auto rounded-xl border border-gray-200 bg-white dark:border-dark-500 dark:bg-dark-800">
        <table class="w-full min-w-[1100px] text-sm">
          <thead class="bg-gray-50 text-left text-xs text-gray-500 dark:bg-dark-700 dark:text-gray-400">
            <tr>
              <th class="px-3 py-2">{{ t('admin.video.tasks.columns.task') }}</th>
              <th class="px-3 py-2">{{ t('admin.video.tasks.columns.user') }}</th>
              <th class="px-3 py-2">{{ t('admin.video.tasks.columns.model') }}</th>
              <th class="px-3 py-2">{{ t('admin.video.tasks.columns.protocol') }}</th>
              <th class="px-3 py-2">{{ t('admin.video.tasks.columns.status') }}</th>
              <th class="px-3 py-2">{{ t('admin.video.tasks.columns.spec') }}</th>
              <th class="px-3 py-2">{{ t('admin.video.tasks.columns.billing') }}</th>
              <th class="px-3 py-2">{{ t('admin.video.tasks.columns.cost') }}</th>
              <th class="px-3 py-2">{{ t('admin.video.tasks.columns.createdAt') }}</th>
              <th class="px-3 py-2">{{ t('admin.video.tasks.columns.actions') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-if="!loading && tasks.length === 0">
              <td colspan="10" class="px-3 py-10 text-center text-gray-400">{{ t('admin.video.tasks.empty') }}</td>
            </tr>
            <tr v-for="task in tasks" :key="task.task_id" class="border-t border-gray-100 align-top dark:border-dark-600">
              <td class="px-3 py-2">
                <div class="font-mono text-xs">{{ task.task_id }}</div>
                <div class="mt-0.5 max-w-[220px] truncate text-[11px] text-gray-400" :title="task.upstream_task_id">
                  {{ t('admin.video.tasks.upstreamTaskId', { id: task.upstream_task_id }) }}
                </div>
              </td>
              <td class="px-3 py-2 text-xs">#{{ task.user_id }}</td>
              <td class="px-3 py-2">
                <div class="font-mono text-xs">{{ task.model }}</div>
                <div v-if="task.upstream_model && task.upstream_model !== task.model" class="text-[11px] text-gray-400">→ {{ task.upstream_model }}</div>
              </td>
              <td class="px-3 py-2">
                <span :class="['inline-flex items-center gap-1 rounded-md px-2 py-0.5 text-xs', videoVendorStyle(task.protocol).badge]">
                  <BrandIcon v-if="videoVendorStyle(task.protocol).icon" :brand="videoVendorStyle(task.protocol).icon" size="xs" />
                  {{ videoVendorStyle(task.protocol).label }}
                </span>
              </td>
              <td class="px-3 py-2">
                <span :class="['rounded-full px-2 py-0.5 text-xs', statusClass[task.status]]">{{ t(`admin.video.tasks.status.${task.status}`) }}</span>
                <div v-if="task.status === 'in_progress'" class="mt-1 text-[11px] text-gray-400">{{ task.progress }}%</div>
                <div v-if="task.error_message" class="mt-1 max-w-[220px] truncate text-[11px] text-red-500" :title="task.error_message">{{ task.error_message }}</div>
              </td>
              <td class="px-3 py-2 text-xs">
                {{ task.resolution || '-' }} · {{ t('admin.video.tasks.seconds', { n: seconds(task) }) }}
                <span v-if="task.generate_audio"> · 🔊</span>
              </td>
              <td class="px-3 py-2 text-xs">
                <span :class="billingClass[task.billing_status]">{{ t(`admin.video.tasks.billingStatus.${task.billing_status}`) }}</span>
                <div v-if="task.billing_error" class="mt-1 max-w-[200px] truncate text-[11px] text-red-500" :title="task.billing_error">{{ task.billing_error }}</div>
              </td>
              <td class="px-3 py-2 font-mono text-xs">{{ money(task.estimated_cost) }} / {{ money(task.actual_cost) }}</td>
              <td class="px-3 py-2 text-xs text-gray-500">{{ formatDateTime(task.created_at) }}</td>
              <td class="space-x-2 whitespace-nowrap px-3 py-2 text-xs">
                <button class="text-primary-600 hover:underline disabled:opacity-50" :disabled="busyTask === task.task_id" @click="act(task, 'refresh')">{{ t('admin.video.tasks.refresh') }}</button>
                <button
                  v-if="task.status === 'completed' && task.billing_status !== 'billed'"
                  class="text-amber-600 hover:underline disabled:opacity-50"
                  :disabled="busyTask === task.task_id"
                  @click="act(task, 'retryBilling')"
                >{{ t('admin.video.tasks.retryBilling') }}</button>
                <a v-if="task.content_url" :href="task.content_url" target="_blank" rel="noopener noreferrer" class="text-fuchsia-600 hover:underline">{{ t('admin.video.tasks.openVideo') }}</a>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
      <Pagination
        v-if="total > 0"
        :total="total"
        :page="page"
        :page-size="pageSize"
        @update:page="(p: number) => { page = p; load() }"
        @update:pageSize="(s: number) => { pageSize = s; page = 1; load() }"
      />
    </div>
  </AppLayout>
</template>
