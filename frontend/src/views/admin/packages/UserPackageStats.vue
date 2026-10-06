<template>
  <dl class="dash-kpi is-even" data-test="package-stats">
    <div class="card dash-kpi-card">
      <dt>{{ t('admin.packages.userPackages.stats.active') }}</dt>
      <dd>{{ stats.active }}</dd>
      <p class="dash-kpi-meta">
        {{ t('admin.packages.userPackages.stats.activeMeta', { frozen: stats.frozen, ended: endedCount }) }}
      </p>
    </div>
    <div class="card dash-kpi-card">
      <dt>{{ t('admin.packages.userPackages.stats.quota') }}</dt>
      <dd>{{ formatUSD(stats.live_used_usd) }}<small> / {{ formatUSD(stats.live_quota_usd) }}</small></dd>
      <p class="dash-kpi-meta">{{ t('admin.packages.userPackages.stats.quotaMeta', { pct: usedPercent(stats.live_used_usd, stats.live_quota_usd) }) }}</p>
    </div>
    <div class="card dash-kpi-card">
      <dt>{{ t('admin.packages.userPackages.stats.sold', { days: stats.window_days }) }}</dt>
      <dd>{{ t('admin.packages.userPackages.stats.soldUnit', { n: stats.recent_sold }) }}</dd>
      <p class="dash-kpi-meta">{{ t('admin.packages.userPackages.stats.soldMeta', { total: stats.total }) }}</p>
    </div>
    <div class="card dash-kpi-card">
      <dt>{{ t('admin.packages.userPackages.stats.revenue', { days: stats.window_days }) }}</dt>
      <dd>{{ formatCNY(stats.recent_revenue) }}</dd>
      <p class="dash-kpi-meta">{{ t('admin.packages.userPackages.stats.revenueMeta') }}</p>
    </div>
  </dl>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { AdminUserPackageStats } from '@/api/admin/packages'
import { formatCNY, formatUSD, usedPercent } from '@/components/package/packageUtils'

const props = defineProps<{ stats: AdminUserPackageStats }>()
const { t } = useI18n()

// 已结束 = 用完 + 到期 + 作废，与「生效中 / 冻结中」互补，合计等于累计张数。
const endedCount = computed(() => props.stats.exhausted + props.stats.expired + props.stats.voided)
</script>
