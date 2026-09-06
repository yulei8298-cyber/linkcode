<template>
  <article class="plaza-model-card">
    <div class="plaza-card-heading">
      <span class="plaza-provider-icon"><PlatformIcon :platform="model.platform as GroupPlatform" size="lg" /></span>
      <div class="plaza-model-identity"><h3>{{ model.name }}</h3><p>{{ plazaProvider(model.platform) }}</p></div>
      <span class="plaza-billing-badge">{{ billingLabel }}</span>
    </div>
    <div class="plaza-card-prices" :class="{ 'plaza-single-price': !quote.token }">
      <div v-for="metric in quote.metrics" :key="metric.field" class="plaza-price-metric">
        <span class="plaza-price-label">{{ metric.field === 'input_price' ? t('modelPlaza.cards.inputPrice') : metric.field === 'output_price' ? t('modelPlaza.cards.outputPrice') : billingLabel }}</span>
        <div class="plaza-price-value">{{ metric.price }}<small v-if="quote.tiered && metric.available">{{ t('modelPlaza.cards.baseTier') }}</small></div>
        <div v-if="metric.available" class="plaza-base-price">{{ t(metric.isOfficial ? 'modelPlaza.cards.officialBase' : 'modelPlaza.cards.configuredBase') }} {{ metric.base }} × {{ quote.rate }}</div>
        <div v-else class="plaza-base-price">{{ t('modelPlaza.detail.noPricing') }}</div>
        <div v-if="!metric.isOfficial && metric.official !== '-'" class="plaza-base-price">{{ t('modelPlaza.cards.officialReference') }} {{ metric.official }}</div>
      </div>
    </div>
    <div v-if="specialRules.length" class="plaza-rule-tags"><span v-for="rule in specialRules" :key="rule">{{ rule }}</span></div>
    <footer class="plaza-card-footer">
      <span>{{ group.name }} · {{ unit }}</span>
      <button type="button" :aria-label="`${t('modelPlaza.cards.details')} · ${model.name}`" @click="$emit('details', model)">{{ t('modelPlaza.cards.details') }} <Icon name="arrowRight" size="xs" /></button>
    </footer>
  </article>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import PlatformIcon from '@/components/common/PlatformIcon.vue'
import type { GroupPlatform } from '@/types'
import type { ModelPlazaGroup, PlazaModel } from '@/api/modelPlaza'
import { plazaCardPricing, plazaProvider } from '@/utils/modelPlazaPricing'

const props = defineProps<{ model: PlazaModel; group: ModelPlazaGroup }>()
defineEmits<{ details: [model: PlazaModel] }>()
const { t } = useI18n()
const quote = computed(() => plazaCardPricing(props.model, props.group))
const billingLabel = computed(() => t(quote.value.token ? 'modelPlaza.cards.tokenBilling' : quote.value.mode === 'image' ? 'modelPlaza.table.perImage' : 'modelPlaza.table.perRequest'))
const unit = computed(() => t(quote.value.token ? 'modelPlaza.cards.perMillion' : quote.value.mode === 'image' ? 'modelPlaza.table.perUnitImage' : 'modelPlaza.table.perUnitRequest'))
const specialRules = computed(() => {
  const labels: string[] = []
  if (quote.value.tiered) labels.push(t('modelPlaza.cards.tiered'))
  if (props.model.time_pricing?.periods.length) labels.push(t('modelPlaza.cards.timePricing'))
  if (props.group.peak_rate_enabled) labels.push(t('modelPlaza.cards.peakPricing'))
  if (props.model.pricing?.max_reasoning_effort_multiplier != null) labels.push(t('modelPlaza.table.maxReasoningMultiplierBadge', { multiplier: props.model.pricing.max_reasoning_effort_multiplier }))
  if (quote.value.mode === 'image' && props.group.image_rate_independent) labels.push(t('modelPlaza.cards.imageRate'))
  return labels
})
</script>
