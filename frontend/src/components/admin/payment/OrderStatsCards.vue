<template>
  <dl class="dash-kpi is-even">
    <div class="card dash-kpi-card">
      <dt>{{ t('payment.admin.todayRevenue') }}</dt>
      <dd v-for="[currency, amount] in sortedAmounts(stats.today_amount)" :key="currency">{{ formatMoney(currency, amount) }}</dd>
      <p class="dash-kpi-meta"><b>{{ stats.today_count }}</b> {{ t('payment.admin.orders') }}</p>
    </div>
    <div class="card dash-kpi-card">
      <dt>{{ t('payment.admin.totalRevenue') }}</dt>
      <dd v-for="[currency, amount] in sortedAmounts(stats.total_amount)" :key="currency">{{ formatMoney(currency, amount) }}</dd>
      <p class="dash-kpi-meta"><b>{{ stats.total_count }}</b> {{ t('payment.admin.orders') }}</p>
    </div>
    <div class="card dash-kpi-card">
      <dt>{{ t('payment.admin.todayOrders') }}</dt>
      <dd>{{ stats.today_count }}</dd>
    </div>
    <div class="card dash-kpi-card">
      <dt>{{ t('payment.admin.avgAmount') }}</dt>
      <dd v-for="[currency, amount] in sortedAmounts(stats.avg_amount)" :key="currency">{{ formatMoney(currency, amount) }}</dd>
    </div>
  </dl>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { CurrencyAmounts, DashboardStats } from '@/types/payment'

const { t } = useI18n()

defineProps<{
  stats: DashboardStats
}>()

function sortedAmounts(amounts: CurrencyAmounts): [string, number][] {
  return Object.entries(amounts).sort(([left], [right]) => left.localeCompare(right))
}

function formatMoney(currency: string, amount: number): string {
  return new Intl.NumberFormat(undefined, { style: 'currency', currency }).format(amount)
}
</script>
