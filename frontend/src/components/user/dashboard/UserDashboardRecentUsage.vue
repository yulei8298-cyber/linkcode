<template>
  <section class="card">
    <div class="dash-card-head">
      <h2>{{ t('dashboard.recentUsage') }}</h2>
      <div class="flex items-center gap-3">
        <span class="badge badge-gray">{{ t('dashboard.last7Days') }}</span>
        <RouterLink to="/usage" class="dash-link">{{ t('dashboard.viewAllUsage') }}</RouterLink>
      </div>
    </div>
    <div v-if="loading" class="flex items-center justify-center py-12">
      <LoadingSpinner size="lg" />
    </div>
    <div v-else-if="data.length === 0" class="py-8">
      <EmptyState :title="t('dashboard.noUsageRecords')" :description="t('dashboard.startUsingApi')" />
    </div>
    <div v-else class="overflow-x-auto">
      <table class="recent">
        <thead>
          <tr>
            <th>{{ t('usage.time') }}</th>
            <th>{{ t('dashboard.model') }}</th>
            <th class="num">Tokens</th>
            <th class="num">{{ t('dashboard.actual') }}</th>
            <th class="num">{{ t('dashboard.standard') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="log in data" :key="log.id">
            <td class="mono">{{ formatDateTime(log.created_at) }}</td>
            <td class="mono model">{{ log.model }}</td>
            <td class="num">{{ (log.input_tokens + log.output_tokens).toLocaleString() }}</td>
            <td class="num strong">${{ formatCost(log.actual_cost) }}</td>
            <td class="num dim">${{ formatCost(log.total_cost) }}</td>
          </tr>
        </tbody>
      </table>
    </div>
  </section>
</template>

<script setup lang="ts">
import { RouterLink } from 'vue-router'
import { useI18n } from 'vue-i18n'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import { formatDateTime } from '@/utils/format'
import type { UsageLog } from '@/types'

defineProps<{
  data: UsageLog[]
  loading: boolean
}>()
const { t } = useI18n()
const formatCost = (c: number) => c.toFixed(4)
</script>

<style scoped>
.recent {
  width: 100%;
  border-collapse: collapse;
  font-size: 13px;
}

.recent th {
  padding: 10px 20px;
  border-bottom: 1px solid var(--lc-line);
  color: var(--lc-ink-3);
  font-size: 12px;
  font-weight: 500;
  text-align: left;
  white-space: nowrap;
}

.recent td {
  height: 46px;
  padding: 0 20px;
  border-bottom: 1px solid var(--lc-line);
  color: var(--lc-ink-2);
  white-space: nowrap;
}

.recent tbody tr:last-child td {
  border-bottom: 0;
}

.recent tbody tr:hover td {
  background: var(--lc-surface-2);
}

.recent .num {
  text-align: right;
  font-family: var(--lc-font-mono);
  font-variant-numeric: tabular-nums;
}

.recent .mono {
  font-family: var(--lc-font-mono);
  font-size: 12.5px;
}

.recent .model,
.recent .strong {
  color: var(--lc-ink);
}

.recent .dim {
  color: var(--lc-ink-3);
}
</style>
