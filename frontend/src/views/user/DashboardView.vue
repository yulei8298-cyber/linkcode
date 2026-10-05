<template>
  <AppLayout>
    <div class="space-y-5">
      <div class="flex flex-wrap items-end justify-between gap-4">
        <div>
          <h1 class="text-2xl font-bold tracking-tight text-gray-900 dark:text-dark-50">{{ greeting }}</h1>
          <p v-if="stats" class="mt-1 text-sm text-gray-600 dark:text-dark-300">
            {{ t('dashboard.todaySummary', { count: (stats.today_requests || 0).toLocaleString(), cost: `$${(stats.today_actual_cost || 0).toFixed(2)}` }) }}
          </p>
        </div>
        <div class="flex flex-wrap gap-2">
          <RouterLink to="/usage" class="btn btn-secondary">{{ t('dashboard.viewUsage') }}</RouterLink>
          <RouterLink to="/keys" class="btn btn-primary">
            <Icon name="plus" size="sm" />{{ t('dashboard.createApiKey') }}
          </RouterLink>
        </div>
      </div>

      <div v-if="loading" class="flex items-center justify-center py-12"><LoadingSpinner /></div>
      <template v-else-if="stats">
        <UserDashboardStats :stats="stats" :balance="user?.balance || 0" :total-recharged="user?.total_recharged || 0" :is-simple="authStore.isSimpleMode" :platform-quotas="platformQuotas" />
        <div class="grid grid-cols-1 gap-4 xl:grid-cols-3">
          <div class="min-w-0 xl:col-span-2">
            <UserDashboardCharts v-model:startDate="startDate" v-model:endDate="endDate" v-model:granularity="granularity" :loading="loadingCharts" :trend="trendData" @dateRangeChange="loadCharts" @granularityChange="loadCharts" @refresh="refreshAll" />
          </div>
          <UserDashboardQuickActions />
        </div>
        <div class="grid grid-cols-1 gap-4 xl:grid-cols-3">
          <div class="min-w-0 xl:col-span-2"><UserDashboardRecentUsage :data="recentUsage" :loading="loadingUsage" /></div>
          <UserDashboardModelMix :models="modelStats" :loading="loadingCharts" />
        </div>
      </template>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'; import { RouterLink } from 'vue-router'; import { useI18n } from 'vue-i18n'; import { useAuthStore } from '@/stores/auth'; import { usageAPI, type UserDashboardStats as UserStatsType } from '@/api/usage'
import AppLayout from '@/components/layout/AppLayout.vue'; import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import UserDashboardStats from '@/components/user/dashboard/UserDashboardStats.vue'; import UserDashboardCharts from '@/components/user/dashboard/UserDashboardCharts.vue'
import UserDashboardRecentUsage from '@/components/user/dashboard/UserDashboardRecentUsage.vue'; import UserDashboardQuickActions from '@/components/user/dashboard/UserDashboardQuickActions.vue'
import UserDashboardModelMix from '@/components/user/dashboard/UserDashboardModelMix.vue'
import Icon from '@/components/icons/Icon.vue'
import type { UsageLog, TrendDataPoint, ModelStat, PlatformQuotaItem } from '@/types'
import { getMyPlatformQuotas } from '@/api/user'
import { formatDateLocalInput } from '@/utils/format'

const authStore = useAuthStore(); const user = computed(() => authStore.user)
const { t } = useI18n()
// 按本地时间问候；名字优先用户名，没有时用邮箱前缀
const greeting = computed(() => {
  const name = user.value?.username || user.value?.email?.split('@')[0] || ''
  const hour = new Date().getHours()
  const key = hour < 11 ? 'dashboard.greetingMorning' : hour < 18 ? 'dashboard.greetingAfternoon' : 'dashboard.greetingEvening'
  return t(key, { name })
})
const stats = ref<UserStatsType | null>(null); const loading = ref(false); const loadingUsage = ref(false); const loadingCharts = ref(false)
const trendData = ref<TrendDataPoint[]>([]); const modelStats = ref<ModelStat[]>([]); const recentUsage = ref<UsageLog[]>([])
const platformQuotas = ref<PlatformQuotaItem[] | null>(null)

const startDate = ref(formatDateLocalInput(new Date(Date.now() - 6 * 86400000))); const endDate = ref(formatDateLocalInput(new Date())); const granularity = ref('day')

const loadStats = async () => { loading.value = true; try { await authStore.refreshUser(); stats.value = await usageAPI.getDashboardStats() } catch (error) { console.error('Failed to load dashboard stats:', error) } finally { loading.value = false } }
const loadCharts = async () => { loadingCharts.value = true; try { const res = await Promise.all([usageAPI.getDashboardTrend({ start_date: startDate.value, end_date: endDate.value, granularity: granularity.value as any }), usageAPI.getDashboardModels({ start_date: startDate.value, end_date: endDate.value })]); trendData.value = res[0].trend || []; modelStats.value = res[1].models || [] } catch (error) { console.error('Failed to load charts:', error) } finally { loadingCharts.value = false } }
const loadRecent = async () => { loadingUsage.value = true; try { const res = await usageAPI.getByDateRange(startDate.value, endDate.value); recentUsage.value = res.items.slice(0, 5) } catch (error) { console.error('Failed to load recent usage:', error) } finally { loadingUsage.value = false } }
const loadPlatformQuotas = async () => { try { const data = await getMyPlatformQuotas(); platformQuotas.value = data.platform_quotas ?? [] } catch (error) { console.warn('Failed to load platform quotas:', error); platformQuotas.value = [] } }
const refreshAll = () => { loadStats(); loadCharts(); loadRecent(); loadPlatformQuotas() }

onMounted(() => { refreshAll() })
</script>
