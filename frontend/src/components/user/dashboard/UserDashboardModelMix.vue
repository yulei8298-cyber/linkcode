<template>
  <section class="card">
    <div class="dash-card-head">
      <h2>{{ t('dashboard.modelDistribution') }}</h2>
      <RouterLink to="/usage" class="dash-link">{{ t('dashboard.viewAllUsage') }}</RouterLink>
    </div>
    <div v-if="loading" class="flex h-40 items-center justify-center"><LoadingSpinner size="md" /></div>
    <p v-else-if="rows.length === 0" class="px-5 py-10 text-center text-sm text-gray-500 dark:text-dark-400">
      {{ t('dashboard.noDataAvailable') }}
    </p>
    <ul v-else class="mix">
      <li v-for="(row, i) in rows" :key="row.model">
        <div class="mix-top">
          <span class="mix-name" :title="row.model">{{ row.model }}</span>
          <span class="mix-cost" :title="t('dashboard.actual')">
            ${{ formatCost(row.actualCost) }} · {{ row.share }}%
          </span>
        </div>
        <div class="mix-bar"><i :class="{ first: i === 0 }" :style="{ width: `${row.share}%` }"></i></div>
        <p class="mix-meta">
          {{ formatNumber(row.requests) }} {{ t('dashboard.requests') }} · {{ formatTokens(row.tokens) }} tokens
          · {{ t('dashboard.standard') }} ${{ formatCost(row.cost) }}
        </p>
      </li>
    </ul>
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { RouterLink } from 'vue-router'
import { useI18n } from 'vue-i18n'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import type { ModelStat } from '@/types'
import { formatCostFixed as formatCost, formatNumberLocaleString as formatNumber, formatTokensK as formatTokens } from '@/utils/format'

// 按实际费用排序展示前 6 个模型；条形长度是费用占比，比环形图更容易比较
const props = defineProps<{ models: ModelStat[]; loading: boolean }>()
const { t } = useI18n()

const rows = computed(() => {
  const list = [...(props.models ?? [])].sort((a, b) => b.actual_cost - a.actual_cost).slice(0, 6)
  const total = (props.models ?? []).reduce((sum, m) => sum + (m.actual_cost || 0), 0)
  return list.map(m => ({
    model: m.model,
    actualCost: m.actual_cost,
    cost: m.cost,
    requests: m.requests,
    tokens: m.total_tokens,
    share: total > 0 ? Math.round((m.actual_cost / total) * 100) : 0,
  }))
})
</script>

<style scoped>
.mix {
  display: grid;
  gap: 14px;
  padding: 18px 20px 20px;
}

.mix-top {
  display: flex;
  justify-content: space-between;
  gap: 8px;
  font-size: 13px;
}

.mix-name {
  min-width: 0;
  overflow: hidden;
  font-family: var(--lc-font-mono);
  font-size: 13px;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--lc-ink);
}

.mix-cost {
  flex: none;
  font-family: var(--lc-font-mono);
  color: var(--lc-ink-2);
}

.mix-bar {
  height: 6px;
  margin-top: 6px;
  overflow: hidden;
  border-radius: 3px;
  background: var(--lc-surface-3);
}

.mix-bar i {
  display: block;
  height: 100%;
  border-radius: 3px;
  background: var(--lc-ink-2);
}

.mix-bar i.first {
  background: var(--lc-accent);
}

.mix-meta {
  margin-top: 4px;
  font-size: 12px;
  color: var(--lc-ink-3);
}
</style>
