<template>
  <section class="flex flex-wrap items-center justify-end gap-2">
    <div class="dash-seg" role="group" :aria-label="t('channelStatus.title')">
      <button
        v-for="opt in windowOptions"
        :key="opt.value"
        type="button"
        :aria-pressed="window === opt.value"
        @click="emit('update:window', opt.value)"
      >
        {{ opt.label }}
      </button>
    </div>

    <button
      type="button"
      class="btn btn-secondary btn-sm"
      :disabled="loading"
      :title="t('common.refresh')"
      @click="emit('refresh')"
    >
      <Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />
    </button>

    <AutoRefreshButton
      v-if="autoRefresh"
      :enabled="autoRefresh.enabled.value"
      :interval-seconds="autoRefresh.intervalSeconds.value"
      :countdown="autoRefresh.countdown.value"
      :intervals="autoRefresh.intervals"
      @update:enabled="autoRefresh.setEnabled"
      @update:interval="autoRefresh.setInterval"
    />
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import AutoRefreshButton from '@/components/common/AutoRefreshButton.vue'
export type MonitorWindow = '7d' | '15d' | '30d'

defineProps<{
  intervalSeconds: number
  window: MonitorWindow
  loading: boolean
  autoRefresh?: {
    enabled: { value: boolean }
    intervalSeconds: { value: number }
    countdown: { value: number }
    intervals: readonly number[]
    setEnabled: (v: boolean) => void
    setInterval: (v: number) => void
  }
}>()

const emit = defineEmits<{
  (e: 'update:window', value: MonitorWindow): void
  (e: 'refresh'): void
}>()

const { t } = useI18n()

const windowOptions = computed<{ value: MonitorWindow; label: string }[]>(() => [
  { value: '7d', label: t('channelStatus.windowTab.7d') },
  { value: '15d', label: t('channelStatus.windowTab.15d') },
  { value: '30d', label: t('channelStatus.windowTab.30d') },
])
</script>
