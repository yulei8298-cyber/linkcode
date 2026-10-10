<template>
  <BaseDialog :show="show" :title="t('admin.groups.userConcurrencyTitle')" width="wide" @close="emit('close')">
    <div v-if="group" class="space-y-4" data-testid="group-user-concurrency">
      <!-- 分组信息 -->
      <div class="flex flex-wrap items-center gap-3 rounded-lg bg-gray-50 px-4 py-2.5 text-sm dark:bg-dark-700">
        <span class="inline-flex items-center gap-1.5 text-gray-700 dark:text-gray-300">
          <PlatformIcon :platform="group.platform" size="sm" />
          {{ t('admin.groups.platforms.' + group.platform) }}
        </span>
        <span class="text-gray-400">|</span>
        <span class="font-medium text-gray-900 dark:text-white">{{ group.name }}</span>
        <span class="ml-auto flex items-center gap-2 text-xs text-gray-400">
          <span>{{ t('admin.groups.userConcurrencyAutoRefresh') }}</span>
          <button
            type="button"
            data-testid="user-concurrency-refresh"
            class="rounded p-1 text-gray-500 hover:bg-gray-200 hover:text-gray-700 dark:hover:bg-dark-600 dark:hover:text-gray-200"
            :title="t('common.refresh')"
            @click="load"
          >
            <Icon name="refresh" size="sm" :class="{ 'animate-spin': loading }" />
          </button>
        </span>
      </div>

      <!-- 汇总 -->
      <dl class="grid grid-cols-3 gap-3">
        <div class="rounded-lg border border-gray-200 px-3 py-2 dark:border-dark-600">
          <dt class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.groups.userConcurrencyTotal') }}</dt>
          <dd class="font-mono text-xl font-semibold tabular-nums text-gray-900 dark:text-white" data-testid="user-concurrency-total">{{ summary?.total ?? 0 }}</dd>
        </div>
        <div class="rounded-lg border border-gray-200 px-3 py-2 dark:border-dark-600">
          <dt class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.groups.userConcurrencyUsers') }}</dt>
          <dd class="font-mono text-xl font-semibold tabular-nums text-gray-900 dark:text-white">{{ summary?.users.length ?? 0 }}</dd>
        </div>
        <div class="rounded-lg border border-gray-200 px-3 py-2 dark:border-dark-600">
          <dt class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.groups.userConcurrencyKeys') }}</dt>
          <dd class="font-mono text-xl font-semibold tabular-nums text-gray-900 dark:text-white">{{ summary?.api_key_count ?? 0 }}</dd>
        </div>
      </dl>

      <div v-if="loading && !summary" class="flex justify-center py-6">
        <div class="h-6 w-6 animate-spin rounded-full border-2 border-primary-500 border-t-transparent"></div>
      </div>

      <div
        v-else-if="!summary || summary.users.length === 0"
        class="py-6 text-center text-sm text-gray-400 dark:text-gray-500"
        data-testid="user-concurrency-empty"
      >
        {{ t('admin.groups.userConcurrencyEmpty') }}
      </div>

      <div v-else class="overflow-hidden rounded-lg border border-gray-200 dark:border-dark-600">
        <div class="max-h-[420px] overflow-auto">
          <table class="w-full min-w-max text-sm">
            <thead class="sticky top-0 z-[1]">
              <tr class="border-b border-gray-200 bg-gray-50 dark:border-dark-600 dark:bg-dark-700">
                <th class="px-3 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.groups.userConcurrencyUser') }}</th>
                <th class="px-3 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-400">ID</th>
                <th class="px-3 py-2 text-right text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.groups.userConcurrencyCount') }}</th>
                <th class="px-3 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.groups.userConcurrencyKeyDetail') }}</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-100 dark:divide-dark-600">
              <tr v-for="user in summary.users" :key="user.user_id" data-testid="user-concurrency-row" class="hover:bg-gray-50 dark:hover:bg-dark-700/50">
                <td class="px-3 py-2">
                  <div class="font-medium text-gray-900 dark:text-white">{{ user.username || user.email }}</div>
                  <div v-if="user.username" class="text-xs text-gray-400">{{ user.email }}</div>
                </td>
                <td class="whitespace-nowrap px-3 py-2 text-gray-400 dark:text-gray-500">{{ user.user_id }}</td>
                <td class="whitespace-nowrap px-3 py-2 text-right font-mono text-base font-semibold tabular-nums text-emerald-600 dark:text-emerald-400">{{ user.concurrency }}</td>
                <td class="px-3 py-2">
                  <div class="flex flex-wrap gap-1.5">
                    <span
                      v-for="key in user.api_keys"
                      :key="key.api_key_id"
                      class="inline-flex items-center gap-1 rounded bg-gray-100 px-1.5 py-0.5 text-xs text-gray-700 dark:bg-dark-600 dark:text-gray-300"
                    >
                      <span class="max-w-[140px] truncate">{{ key.api_key_name }}</span>
                      <span class="font-mono font-semibold tabular-nums">×{{ key.concurrency }}</span>
                    </span>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </div>
  </BaseDialog>
</template>

<script setup lang="ts">
import { ref, watch, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { adminAPI } from '@/api/admin'
import type { GroupUserConcurrencySummary } from '@/api/admin/groups'
import type { AdminGroup } from '@/types'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import PlatformIcon from '@/components/common/PlatformIcon.vue'

/** 弹窗打开期间的刷新间隔；并发变化快，3 秒一刷。 */
const REFRESH_INTERVAL_MS = 3000

const props = defineProps<{
  show: boolean
  group: AdminGroup | null
}>()

const emit = defineEmits<{
  close: []
}>()

const { t } = useI18n()
const appStore = useAppStore()

const loading = ref(false)
const summary = ref<GroupUserConcurrencySummary | null>(null)

let timer: ReturnType<typeof setInterval> | undefined
// 切换分组或关闭弹窗时 +1：丢弃还在路上的旧响应，也不让旧请求挡住新分组的加载
let generation = 0
let inflightGeneration = -1

async function load() {
  const group = props.group
  if (!group || inflightGeneration === generation) return
  const current = generation
  inflightGeneration = current
  loading.value = true
  try {
    const result = await adminAPI.groups.getUserConcurrency(group.id)
    if (current === generation) summary.value = result
  } catch (error) {
    if (current === generation) appStore.showError(t('admin.groups.userConcurrencyLoadFailed'))
    console.error('Error loading group user concurrency:', error)
  } finally {
    if (inflightGeneration === current) {
      inflightGeneration = -1
      loading.value = false
    }
  }
}

function stop() {
  generation++
  if (timer !== undefined) {
    clearInterval(timer)
    timer = undefined
  }
}

function start() {
  stop()
  summary.value = null
  void load()
  timer = setInterval(() => {
    if (!document.hidden) void load()
  }, REFRESH_INTERVAL_MS)
}

watch(
  () => [props.show, props.group?.id] as const,
  ([show, groupId]) => {
    if (show && groupId !== undefined) start()
    else stop()
  },
  { immediate: true }
)

onUnmounted(stop)
</script>
