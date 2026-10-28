<template>
  <div class="space-y-2" data-test="plaza-video-pricing">
    <div class="flex flex-wrap items-center gap-2 text-xs">
      <span class="rounded-full bg-fuchsia-100 px-2 py-0.5 font-medium text-fuchsia-700 dark:bg-fuchsia-900/30 dark:text-fuchsia-300">{{ unitLabel }}</span>
      <span v-if="durationRange" class="text-gray-500 dark:text-gray-400">{{ durationRange }}</span>
    </div>
    <ul v-if="spec.unit !== 'token'" class="space-y-1 text-sm">
      <li v-for="row in rows" :key="row.key" class="flex items-baseline justify-between gap-3">
        <span class="text-gray-600 dark:text-gray-300">{{ row.label }}</span>
        <span class="font-mono font-semibold text-gray-900 dark:text-white">{{ row.price }}</span>
      </li>
    </ul>
    <div v-else class="font-mono text-sm font-semibold text-gray-900 dark:text-white">{{ tokenPrice }}</div>
    <p v-if="rate !== 1" class="text-[11px] text-gray-400">{{ t('modelPlaza.video.rateNote', { rate }) }}</p>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { VideoPricingSpec } from '@/api/admin/modelBasePricing'
import { VIDEO_TIERS } from '@/utils/videoPricing'

const props = defineProps<{ spec: VideoPricingSpec; rate: number }>()
const { t } = useI18n()

// 至少保留两位小数（$1.60），四位以内去掉多余的 0（$0.416、$0.0425）。
const fmt = (v: number) => {
  const value = v * props.rate
  const [int, frac = ''] = value.toFixed(4).split('.')
  return `$${int}.${frac.replace(/0+$/, '').padEnd(2, '0')}`
}
const tierLabel = (tier: string) => (tier === '*' ? t('modelPlaza.video.anyResolution') : VIDEO_TIERS.find(item => item.value === tier)?.label ?? tier)

const unitLabel = computed(() => t(`modelPlaza.video.${props.spec.unit === 'per_second' ? 'perSecond' : props.spec.unit === 'per_video' ? 'perVideo' : 'token'}`))

const durationRange = computed(() => {
  const { min_duration_seconds: min, max_duration_seconds: max } = props.spec
  return min && max ? t('modelPlaza.video.durationRange', { min, max }) : ''
})

const rows = computed(() => {
  const order = VIDEO_TIERS.map(item => item.value)
  const entries = Object.entries(props.spec.prices ?? {}).sort(([a], [b]) => order.indexOf(a) - order.indexOf(b))
  const out: Array<{ key: string; label: string; price: string }> = []
  for (const [tier, price] of entries) {
    if (props.spec.unit === 'per_second' && price.per_second != null) {
      let text = `${fmt(price.per_second)}${t('modelPlaza.video.perSecondUnit')}`
      if (price.per_second_with_audio != null && price.per_second_with_audio !== price.per_second) {
        text += ` · ${t('modelPlaza.video.withAudio')} ${fmt(price.per_second_with_audio)}`
      }
      out.push({ key: tier, label: tierLabel(tier), price: text })
    }
    if (props.spec.unit === 'per_video') {
      for (const [seconds, value] of Object.entries(price.per_video ?? {})) {
        const duration = seconds === '*' ? t('modelPlaza.video.anyDuration') : t('modelPlaza.video.seconds', { n: seconds })
        out.push({ key: `${tier}-${seconds}`, label: `${tierLabel(tier)} · ${duration}`, price: `${fmt(value)}${t('modelPlaza.video.perVideoUnit')}` })
      }
    }
  }
  return out
})

const tokenPrice = computed(() =>
  props.spec.token_price_per_million != null ? `${fmt(props.spec.token_price_per_million)}${t('modelPlaza.video.tokenUnit')}` : '-'
)
</script>
