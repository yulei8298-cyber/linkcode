<template>
  <section class="card">
    <div class="dash-card-head">
      <h2>{{ t('admin.dashboard.tokenUsageTrend') }}</h2>
      <div class="dash-chart-controls">
        <DateRangePicker
          :start-date="startDate"
          :end-date="endDate"
          @update:startDate="$emit('update:startDate', $event)"
          @update:endDate="$emit('update:endDate', $event)"
          @change="$emit('dateRangeChange', $event)"
        />
        <div class="w-28">
          <Select
            :model-value="granularity"
            :options="[{ value: 'day', label: t('dashboard.day') }, { value: 'hour', label: t('dashboard.hour') }]"
            :aria-label="t('dashboard.granularity')"
            @update:model-value="$emit('update:granularity', $event)"
            @change="$emit('granularityChange')"
          />
        </div>
        <button type="button" class="btn btn-secondary btn-sm" :disabled="loading" @click="$emit('refresh')">
          {{ t('common.refresh') }}
        </button>
      </div>
    </div>
    <div class="px-5 pb-5 pt-4">
      <TokenUsageTrend :trend-data="trend" :loading="loading" bare />
    </div>
  </section>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import DateRangePicker from '@/components/common/DateRangePicker.vue'
import Select from '@/components/common/Select.vue'
import TokenUsageTrend from '@/components/charts/TokenUsageTrend.vue'
import type { TrendDataPoint } from '@/types'

defineProps<{ loading: boolean; startDate: string; endDate: string; granularity: string; trend: TrendDataPoint[] }>()
defineEmits(['update:startDate', 'update:endDate', 'update:granularity', 'dateRangeChange', 'granularityChange', 'refresh'])
const { t } = useI18n()
</script>
