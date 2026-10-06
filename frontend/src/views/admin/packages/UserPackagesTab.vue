<template>
  <section class="space-y-4">
    <UserPackageStats v-if="stats" :stats="stats" />

    <div class="card space-y-4 p-5">
      <div>
        <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.packages.userPackages.title') }}</h3>
        <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.packages.userPackages.desc') }}</p>
      </div>

      <div class="flex flex-wrap items-end gap-3">
        <input v-model="filters.keyword" type="search" class="input w-64" data-test="keyword" :placeholder="t('admin.packages.userPackages.search')" />
        <select v-model="filters.status" class="input w-36" data-test="status">
          <option value="">{{ t('admin.packages.userPackages.allStatus') }}</option>
          <option v-for="s in STATUSES" :key="s" :value="s">{{ t(`packages.mine.status.${s}`) }}</option>
        </select>
        <select v-model="filters.cycle" class="input w-32" data-test="cycle">
          <option value="">{{ t('admin.packages.userPackages.allCycle') }}</option>
          <option value="week">{{ t('packages.cycle.week') }}</option>
          <option value="month">{{ t('packages.cycle.month') }}</option>
        </select>
        <select v-model.number="filters.group_id" class="input w-44" data-test="group">
          <option :value="0">{{ t('admin.packages.userPackages.allGroup') }}</option>
          <option v-for="g in groups" :key="g.id" :value="g.id">{{ g.name }}</option>
        </select>
        <button type="button" class="btn btn-secondary" @click="reset">{{ t('admin.packages.userPackages.reset') }}</button>
        <button type="button" class="btn btn-secondary" :disabled="loading" @click="refresh">{{ t('common.refresh') }}</button>
      </div>

      <div class="overflow-x-auto rounded-xl border border-gray-200 dark:border-dark-700">
        <table class="w-full min-w-[1300px] text-sm">
          <thead class="bg-gray-50 text-left text-xs text-gray-500 dark:bg-dark-800 dark:text-gray-400">
            <tr>
              <th v-for="col in COLUMNS" :key="col" class="whitespace-nowrap px-4 py-2.5">{{ t(`admin.packages.userPackages.table.${col}`) }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-if="!items.length">
              <td :colspan="COLUMNS.length" class="px-4 py-8 text-center text-gray-500">{{ loading ? t('common.loading') : t('admin.packages.userPackages.empty') }}</td>
            </tr>
            <tr v-for="p in items" :key="p.id" class="border-t border-gray-100 align-top dark:border-dark-700" data-test="package-row">
              <td class="px-4 py-2.5">
                <div class="font-medium text-gray-900 dark:text-white">{{ p.user_email || '—' }}</div>
                <div class="text-xs text-gray-500">#{{ p.user_id }}<template v-if="p.username"> · {{ p.username }}</template></div>
              </td>
              <td class="whitespace-nowrap px-4 py-2.5">
                <div class="font-medium text-gray-900 dark:text-white">{{ p.name }}</div>
                <div class="text-xs text-gray-500">{{ t(`packages.cycle.${p.cycle}`) }} · {{ p.tier }}x</div>
              </td>
              <td class="whitespace-nowrap px-4 py-2.5">{{ p.group_name || `#${p.group_id}` }}</td>
              <td class="min-w-[170px] px-4 py-2.5">
                <div class="font-mono text-xs">{{ formatUSD(p.used_usd) }} / {{ formatUSD(p.quota_usd) }}</div>
                <div class="mt-1 h-1.5 overflow-hidden rounded-full bg-gray-200 dark:bg-dark-700">
                  <div class="h-full rounded-full bg-primary-500" :style="{ width: `${usedPercent(p.used_usd, p.quota_usd)}%` }"></div>
                </div>
                <div class="mt-1 text-xs text-gray-500">{{ t('admin.packages.userPackages.remaining', { n: formatUSD(p.remaining_usd) }) }}</div>
              </td>
              <td class="whitespace-nowrap px-4 py-2.5 font-mono">{{ p.order_id ? formatCNY(p.paid_amount) : t('admin.packages.userPackages.manual') }}</td>
              <td class="whitespace-nowrap px-4 py-2.5"><span :class="['badge', PACKAGE_STATUS_BADGE[p.status]]">{{ t(`packages.mine.status.${p.status}`) }}</span></td>
              <td class="whitespace-nowrap px-4 py-2.5">{{ formatDateTimeToMinute(p.expires_at) }}</td>
              <td class="whitespace-nowrap px-4 py-2.5 font-mono text-xs">{{ t('admin.packages.userPackages.frozenValue', { used: toDays(p.frozen_seconds), max: p.max_freeze_days }) }}</td>
              <td class="whitespace-nowrap px-4 py-2.5">{{ formatDateTimeToMinute(p.created_at) }}</td>
              <td class="px-4 py-2.5">
                <div class="flex gap-2 whitespace-nowrap">
                  <button v-if="p.status === 'frozen'" type="button" class="btn btn-secondary btn-sm" :disabled="busy" @click="unfreeze(p)">{{ t('admin.packages.userPackages.unfreeze') }}</button>
                  <button v-if="p.status === 'active' || p.status === 'frozen'" type="button" class="btn btn-danger btn-sm" :disabled="busy" @click="pendingVoid = p">{{ t('admin.packages.userPackages.void') }}</button>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <Pagination v-if="total > 0" :page="filters.page" :total="total" :page-size="filters.page_size" @update:page="onPage" @update:pageSize="onPageSize" />
    </div>

    <ConfirmDialog
      :show="!!pendingVoid"
      :title="t('admin.packages.userPackages.void')"
      :message="pendingVoid ? t('admin.packages.userPackages.voidConfirm', { name: pendingVoid.name }) : ''"
      danger
      @confirm="confirmVoid"
      @cancel="pendingVoid = null"
    />
  </section>
</template>

<script setup lang="ts">
import { onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import Pagination from '@/components/common/Pagination.vue'
import { adminAPI } from '@/api/admin'
import packagesAdminAPI, { type AdminUserPackage, type AdminUserPackageStats } from '@/api/admin/packages'
import type { AdminGroup } from '@/types'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import { formatDateTimeToMinute } from '@/utils/format'
import { formatCNY, formatUSD, PACKAGE_STATUS_BADGE, usedPercent } from '@/components/package/packageUtils'
import UserPackageStats from './UserPackageStats.vue'

const STATUSES = ['active', 'frozen', 'exhausted', 'expired', 'voided'] as const
const COLUMNS = ['user', 'plan', 'group', 'usage', 'paid', 'status', 'expires', 'frozen', 'bought', 'actions'] as const
const KEYWORD_DEBOUNCE_MS = 300

const { t } = useI18n()
const appStore = useAppStore()

const filters = reactive({ keyword: '', status: '', cycle: '', group_id: 0, page: 1, page_size: 20 })
const items = ref<AdminUserPackage[]>([])
const total = ref(0)
const stats = ref<AdminUserPackageStats | null>(null)
const groups = ref<AdminGroup[]>([])
const loading = ref(false)
const busy = ref(false)
const pendingVoid = ref<AdminUserPackage | null>(null)
let keywordTimer: ReturnType<typeof setTimeout> | undefined
let loadSeq = 0

const toDays = (seconds: number) => (seconds / 86400).toFixed(1)

async function loadList() {
  const seq = ++loadSeq
  loading.value = true
  try {
    const page = await packagesAdminAPI.listUserPackages({
      keyword: filters.keyword.trim() || undefined,
      status: filters.status || undefined,
      cycle: filters.cycle || undefined,
      group_id: filters.group_id || undefined,
      page: filters.page,
      page_size: filters.page_size,
    })
    if (seq !== loadSeq) return // 已有更新的请求，丢弃过期结果
    items.value = page.items
    total.value = page.total
  } catch (err: unknown) {
    if (seq === loadSeq) appStore.showError(extractApiErrorMessage(err, t('admin.packages.common.loadFailed')))
  } finally {
    if (seq === loadSeq) loading.value = false
  }
}

async function loadStats() {
  try {
    stats.value = await packagesAdminAPI.getUserPackageStats()
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('admin.packages.common.loadFailed')))
  }
}

const refresh = () => Promise.all([loadList(), loadStats()])

// 筛选条件变化回到第一页；关键词输入做防抖，避免每敲一个字就请求。
watch(() => [filters.status, filters.cycle, filters.group_id], () => {
  filters.page = 1
  void loadList()
})
watch(() => filters.keyword, () => {
  clearTimeout(keywordTimer)
  keywordTimer = setTimeout(() => {
    filters.page = 1
    void loadList()
  }, KEYWORD_DEBOUNCE_MS)
})

function reset() {
  clearTimeout(keywordTimer)
  Object.assign(filters, { keyword: '', status: '', cycle: '', group_id: 0, page: 1 })
  void loadList()
}

function onPage(page: number) {
  filters.page = page
  void loadList()
}

function onPageSize(size: number) {
  filters.page_size = size
  filters.page = 1
  void loadList()
}

async function act(action: () => Promise<unknown>) {
  busy.value = true
  try {
    await action()
    appStore.showSuccess(t('admin.packages.userPackages.done'))
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('common.error')))
  } finally {
    busy.value = false
  }
  await refresh()
}

const unfreeze = (p: AdminUserPackage) => act(() => packagesAdminAPI.unfreezeUserPackage(p.id))

function confirmVoid() {
  const p = pendingVoid.value
  pendingVoid.value = null
  if (p) void act(() => packagesAdminAPI.voidUserPackage(p.id))
}

onMounted(async () => {
  void refresh()
  try {
    groups.value = (await adminAPI.groups.getAll()).filter((g) => g.subscription_type === 'standard' && !g.is_free)
  } catch {
    groups.value = [] // 分组下拉只是筛选辅助，加载失败不影响列表
  }
})
onBeforeUnmount(() => clearTimeout(keywordTimer))
</script>
