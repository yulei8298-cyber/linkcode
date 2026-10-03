<template>
  <AppLayout>
    <div class="mx-auto max-w-3xl space-y-5">
      <!-- 当前余额与并发：两张摘要卡 -->
      <dl class="grid grid-cols-2 gap-4">
        <div class="card dash-kpi-card">
          <dt>{{ t('redeem.currentBalance') }}</dt>
          <dd>${{ user?.balance?.toFixed(2) || '0.00' }}</dd>
        </div>
        <div class="card dash-kpi-card">
          <dt>{{ t('redeem.concurrency') }}</dt>
          <dd>{{ user?.concurrency || 0 }}<small class="ml-1 text-sm font-medium text-gray-500 dark:text-dark-400">{{ t('redeem.requests') }}</small></dd>
        </div>
      </dl>

      <!-- 兑换表单 + 规则说明 -->
      <section class="card p-5">
        <form class="space-y-2" @submit.prevent="handleRedeem">
          <label for="code" class="input-label">{{ t('redeem.redeemCodeLabel') }}</label>
          <div class="flex flex-col gap-2 sm:flex-row">
            <div class="relative flex-1">
              <div class="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-3.5">
                <Icon name="gift" size="md" class="text-gray-400 dark:text-dark-500" />
              </div>
              <input
                id="code"
                v-model="redeemCode"
                type="text"
                required
                :placeholder="t('redeem.redeemCodePlaceholder')"
                :disabled="submitting"
                class="input pl-11 font-mono"
              />
            </div>
            <button type="submit" :disabled="!redeemCode || submitting" class="btn btn-primary sm:w-32">
              <span v-if="submitting" class="mr-2 inline-block h-4 w-4 animate-spin rounded-full border-2 border-current border-r-transparent" aria-hidden="true"></span>
              {{ submitting ? t('redeem.redeeming') : t('redeem.redeemButton') }}
            </button>
          </div>
          <p class="input-hint">{{ t('redeem.redeemCodeHint') }}</p>
        </form>

        <div class="mt-5 border-t border-gray-200 pt-4 dark:border-dark-700">
          <h3 class="text-[13px] font-semibold text-gray-900 dark:text-dark-50">{{ t('redeem.aboutCodes') }}</h3>
          <ul class="mt-2 list-inside list-disc space-y-1 text-[13px] text-gray-600 dark:text-dark-300">
            <li>{{ t('redeem.codeRule1') }}</li>
            <li>{{ t('redeem.codeRule2') }}</li>
            <li>
              {{ t('redeem.codeRule3') }}
              <span v-if="contactInfo" class="ml-1 font-mono font-medium text-gray-900 dark:text-dark-50">{{ contactInfo }}</span>
            </li>
            <li>{{ t('redeem.codeRule4') }}</li>
          </ul>
        </div>
      </section>

      <!-- 兑换结果 -->
      <transition name="fade">
        <section v-if="redeemResult" class="card p-5" role="status">
          <p class="flex items-center text-sm font-semibold text-gray-900 dark:text-dark-50">
            <i class="dash-dot ok"></i>{{ t('redeem.redeemSuccess') }}
          </p>
          <p class="mt-2 text-sm text-gray-600 dark:text-dark-300">{{ redeemResult.message }}</p>
          <dl class="mt-3 space-y-1 text-sm text-gray-700 dark:text-dark-200">
            <p v-if="redeemResult.type === 'balance'" class="font-medium">
              {{ t('redeem.added') }}: ${{ redeemResult.value.toFixed(2) }}
            </p>
            <p v-else-if="redeemResult.type === 'concurrency'" class="font-medium">
              {{ t('redeem.added') }}: {{ redeemResult.value }} {{ t('redeem.concurrentRequests') }}
            </p>
            <p v-else-if="redeemResult.type === 'subscription'" class="font-medium">
              {{ t('redeem.subscriptionAssigned') }}
              <span v-if="redeemResult.group_name"> - {{ redeemResult.group_name }}</span>
              <span v-if="redeemResult.validity_days"> ({{ t('redeem.subscriptionDays', { days: redeemResult.validity_days }) }})</span>
            </p>
            <p v-if="redeemResult.new_balance !== undefined">
              {{ t('redeem.newBalance') }}: <span class="font-semibold">${{ redeemResult.new_balance.toFixed(2) }}</span>
            </p>
            <p v-if="redeemResult.new_concurrency !== undefined">
              {{ t('redeem.newConcurrency') }}: <span class="font-semibold">{{ redeemResult.new_concurrency }} {{ t('redeem.requests') }}</span>
            </p>
          </dl>
        </section>
      </transition>

      <transition name="fade">
        <section v-if="errorMessage" class="card p-5" role="alert">
          <p class="flex items-center text-sm font-semibold text-gray-900 dark:text-dark-50">
            <i class="dash-dot bad"></i>{{ t('redeem.redeemFailed') }}
          </p>
          <p class="mt-2 text-sm text-gray-600 dark:text-dark-300">{{ errorMessage }}</p>
        </section>
      </transition>

      <!-- 最近记录 -->
      <section class="card overflow-hidden">
        <header class="dash-card-head">
          <h2>{{ t('redeem.recentActivity') }}</h2>
        </header>

        <div v-if="loadingHistory" class="flex items-center justify-center py-10">
          <span class="inline-block h-5 w-5 animate-spin rounded-full border-2 border-gray-300 border-r-transparent dark:border-dark-500 dark:border-r-transparent" aria-hidden="true"></span>
        </div>

        <ul v-else-if="history.length > 0" class="divide-y divide-gray-100 dark:divide-dark-700">
          <li v-for="item in history" :key="item.id" class="flex items-center justify-between gap-4 px-5 py-3.5">
            <div class="min-w-0">
              <p class="truncate text-sm font-medium text-gray-900 dark:text-dark-50">{{ getHistoryItemTitle(item) }}</p>
              <p class="mt-0.5 text-xs text-gray-500 dark:text-dark-400">{{ formatDateTime(item.used_at) }}</p>
            </div>
            <div class="shrink-0 text-right">
              <p
                class="font-mono text-sm font-semibold"
                :class="isBalanceType(item.type) && item.value < 0 ? 'text-red-600 dark:text-red-400' : isBalanceType(item.type) ? 'text-green-600 dark:text-green-400' : 'text-gray-900 dark:text-dark-50'"
              >
                {{ formatHistoryValue(item) }}
              </p>
              <p v-if="!isAdminAdjustment(item.type)" class="font-mono text-xs text-gray-400 dark:text-dark-500">
                {{ item.code.slice(0, 8) }}...
              </p>
              <p v-else class="text-xs text-gray-400 dark:text-dark-500">{{ t('redeem.adminAdjustment') }}</p>
              <p v-if="item.notes" class="mt-1 max-w-[200px] truncate text-xs text-gray-500 dark:text-dark-400" :title="item.notes">
                {{ item.notes }}
              </p>
            </div>
          </li>
        </ul>

        <p v-else class="px-5 py-10 text-center text-sm text-gray-500 dark:text-dark-400">
          {{ t('redeem.historyWillAppear') }}
        </p>

        <footer class="flex flex-wrap items-center gap-3 border-t border-gray-200 px-5 py-3 text-[13px] text-gray-600 dark:border-dark-700 dark:text-dark-300">
          <span>{{ t('common.total') }}: {{ historyTotal }} {{ t('pagination.results') }}</span>
          <label class="ml-auto flex items-center gap-2">
            {{ t('pagination.perPage') }}
            <select
              v-model="historyPageSize"
              class="input h-8 w-20 py-0"
              :disabled="loadingHistory || submitting"
              @change="fetchHistory(1)"
            >
              <option v-for="size in [20, 50, 100]" :key="size" :value="size">{{ size }}</option>
            </select>
          </label>
          <button
            class="btn btn-secondary btn-sm"
            :disabled="loadingHistory || submitting || historyPage <= 1"
            @click="fetchHistory(historyPage - 1)"
          >{{ t('pagination.previous') }}</button>
          <button
            class="btn btn-secondary btn-sm"
            :disabled="loadingHistory || submitting || historyPage * historyPageSize >= historyTotal"
            @click="fetchHistory(historyPage + 1)"
          >{{ t('pagination.next') }}</button>
        </footer>
      </section>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import { useAppStore } from '@/stores/app'
import { useSubscriptionStore } from '@/stores/subscriptions'
import { redeemAPI, authAPI, type RedeemHistoryItem } from '@/api'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import { formatDateTime } from '@/utils/format'

const { t } = useI18n()
const authStore = useAuthStore()
const appStore = useAppStore()
const subscriptionStore = useSubscriptionStore()

const user = computed(() => authStore.user)

const redeemCode = ref('')
const submitting = ref(false)
const redeemResult = ref<{
  message: string
  type: string
  value: number
  new_balance?: number
  new_concurrency?: number
  group_name?: string
  validity_days?: number
} | null>(null)
const errorMessage = ref('')

// History data
const history = ref<RedeemHistoryItem[]>([])
const loadingHistory = ref(false)
const historyPage = ref(1)
const historyPageSize = ref(20)
const historyTotal = ref(0)
let historyRequest = 0
let loadedHistoryPageSize = 20
const contactInfo = ref('')

// Helper functions for history display
const isBalanceType = (type: string) => {
  return type === 'balance' || type === 'admin_balance'
}

const isSubscriptionType = (type: string) => {
  return type === 'subscription'
}

const isAdminAdjustment = (type: string) => {
  return type === 'admin_balance' || type === 'admin_concurrency'
}

const getHistoryItemTitle = (item: RedeemHistoryItem) => {
  if (item.type === 'balance') {
    return t('redeem.balanceAddedRedeem')
  } else if (item.type === 'admin_balance') {
    return item.value >= 0 ? t('redeem.balanceAddedAdmin') : t('redeem.balanceDeductedAdmin')
  } else if (item.type === 'concurrency') {
    return t('redeem.concurrencyAddedRedeem')
  } else if (item.type === 'admin_concurrency') {
    return item.value >= 0 ? t('redeem.concurrencyAddedAdmin') : t('redeem.concurrencyReducedAdmin')
  } else if (item.type === 'subscription') {
    return t('redeem.subscriptionAssigned')
  }
  return t('common.unknown')
}

const formatHistoryValue = (item: RedeemHistoryItem) => {
  if (isBalanceType(item.type)) {
    const sign = item.value >= 0 ? '+' : ''
    return `${sign}$${item.value.toFixed(2)}`
  } else if (isSubscriptionType(item.type)) {
    // 订阅类型显示有效天数和分组名称
    const days = item.validity_days || Math.round(item.value)
    const groupName = item.group?.name || ''
    return groupName ? `${days}${t('redeem.days')} - ${groupName}` : `${days}${t('redeem.days')}`
  } else {
    const sign = item.value >= 0 ? '+' : ''
    return `${sign}${item.value} ${t('redeem.requests')}`
  }
}

const fetchHistory = async (page = 1) => {
  const request = ++historyRequest
  const pageSize = historyPageSize.value
  loadingHistory.value = true
  try {
    const result = await redeemAPI.getHistory(page, pageSize)
    if (request !== historyRequest) return
    history.value = result.items
    historyTotal.value = result.total
    historyPage.value = page
    historyPageSize.value = pageSize
    loadedHistoryPageSize = pageSize
  } catch (error) {
    if (request !== historyRequest) return
    historyPageSize.value = loadedHistoryPageSize
    appStore.showError(t('redeem.historyLoadFailed'))
    console.error('Failed to fetch history:', error)
  } finally {
    if (request === historyRequest) loadingHistory.value = false
  }
}

const handleRedeem = async () => {
  if (!redeemCode.value.trim()) {
    appStore.showError(t('redeem.pleaseEnterCode'))
    return
  }

  submitting.value = true
  errorMessage.value = ''
  redeemResult.value = null

  try {
    const result = await redeemAPI.redeem(redeemCode.value.trim())

    redeemResult.value = result

    // Refresh user data to get updated balance/concurrency
    try {
      await authStore.refreshUser()
    } catch (error) {
      console.error('Failed to refresh user after redeem:', error)
      appStore.showWarning(t('redeem.userRefreshFailed'))
    }

    // If subscription type, immediately refresh subscription status
    if (result.type === 'subscription') {
      try {
        await subscriptionStore.fetchActiveSubscriptions(true) // force refresh
      } catch (error) {
        console.error('Failed to refresh subscriptions after redeem:', error)
        appStore.showWarning(t('redeem.subscriptionRefreshFailed'))
      }
    }

    // Clear the input
    redeemCode.value = ''

    // Refresh history
    await fetchHistory()

    // Show success toast
    appStore.showSuccess(t('redeem.codeRedeemSuccess'))
  } catch (error: any) {
    errorMessage.value = error.response?.data?.detail || t('redeem.failedToRedeem')

    appStore.showError(t('redeem.redeemFailed'))
  } finally {
    submitting.value = false
  }
}

onMounted(async () => {
  fetchHistory()
  try {
    const settings = await authAPI.getPublicSettings()
    contactInfo.value = settings.contact_info || ''
  } catch (error) {
    console.error('Failed to load contact info:', error)
  }
})
</script>

<style scoped>
.fade-enter-active,
.fade-leave-active {
  transition: all 0.3s ease;
}

.fade-enter-from,
.fade-leave-to {
  opacity: 0;
  transform: translateY(-8px);
}
</style>
