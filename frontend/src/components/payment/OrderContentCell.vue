<template>
  <div class="min-w-[220px] space-y-1 text-sm" data-test="order-content">
    <template v-if="order.order_type === 'package'">
      <template v-if="order.package">
        <div class="font-medium text-gray-900 dark:text-white">
          {{ order.package.plan_name }}
          <span class="ml-1 text-xs font-normal text-gray-500">{{ t(`packages.cycle.${order.package.cycle}`) }} · {{ order.package.tier }}x</span>
        </div>
        <div class="text-xs text-gray-500 dark:text-gray-400">
          {{ t('payment.orders.content.quota', { quota: formatUSD(order.package.quota_usd) }) }}
          <template v-if="order.package.group_name"> · {{ order.package.group_name }}</template>
        </div>
        <template v-if="pkg">
          <div class="flex items-center gap-2">
            <span :class="['badge', PACKAGE_STATUS_BADGE[pkg.status]]" data-test="package-status">{{ t(`packages.mine.status.${pkg.status}`) }}</span>
            <span class="font-mono text-xs text-gray-600 dark:text-gray-300">{{ formatUSD(pkg.used_usd) }} / {{ formatUSD(pkg.quota_usd) }}</span>
          </div>
          <div class="h-1.5 w-40 overflow-hidden rounded-full bg-gray-200 dark:bg-dark-700">
            <div class="h-full rounded-full bg-primary-500" :style="{ width: `${usedPercent(pkg.used_usd, pkg.quota_usd)}%` }"></div>
          </div>
          <div class="text-xs text-gray-500 dark:text-gray-400">
            {{ t('payment.orders.content.period', { start: formatDateTimeToMinute(pkg.starts_at), end: formatDateTimeToMinute(pkg.expires_at) }) }}
          </div>
          <div v-if="pkg.frozen_seconds > 0" class="text-xs text-gray-500 dark:text-gray-400">
            {{ t('payment.orders.content.frozen', { used: (pkg.frozen_seconds / 86400).toFixed(1), max: pkg.max_freeze_days }) }}
          </div>
        </template>
        <div v-else-if="order.status === 'COMPLETED'" class="text-xs text-amber-600 dark:text-amber-400">{{ t('payment.orders.content.shipping') }}</div>
      </template>
      <div v-else class="text-gray-700 dark:text-gray-300">{{ t('payment.orders.content.package') }}</div>
    </template>
    <template v-else-if="order.order_type === 'subscription'">
      <div class="text-gray-700 dark:text-gray-300">{{ t('payment.orders.content.subscription') }}</div>
    </template>
    <template v-else>
      <div class="text-gray-700 dark:text-gray-300">{{ t('payment.orders.content.balance') }}</div>
      <div class="text-xs text-gray-500 dark:text-gray-400">{{ t('payment.orders.creditedAmount') }}: {{ formatUSD(order.amount) }}</div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { PaymentOrder } from '@/types/payment'
import { formatDateTimeToMinute } from '@/utils/format'
import { formatUSD, PACKAGE_STATUS_BADGE, usedPercent } from '@/components/package/packageUtils'

const props = defineProps<{ order: PaymentOrder }>()
const { t } = useI18n()

const pkg = computed(() => props.order.package?.user_package)
</script>
