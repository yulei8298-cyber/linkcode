<template>
  <article class="plaza-model-card" :class="{ 'plaza-model-card-exclusive': exclusive }">
    <span v-if="exclusive" class="plaza-exclusive-price" :class="{ 'is-enterprise': group.enterprise_rate }" data-test="exclusive-price">
      {{ group.enterprise_rate ? t('modelPlaza.cards.enterprisePrice') : t('modelPlaza.cards.exclusivePrice') }}
    </span>
    <div class="plaza-card-heading">
      <span class="plaza-provider-icon">
        <BrandIcon v-if="videoVendorIcon" :brand="videoVendorIcon" size="lg" />
        <PlatformIcon v-else :platform="model.platform as GroupPlatform" size="lg" />
      </span>
      <div class="plaza-model-identity"><h3>{{ model.name }}</h3><p>{{ providerLabel }}</p></div>
      <span class="plaza-billing-badge">{{ billingLabel }}</span>
    </div>
    <PlazaVideoPricing v-if="model.video_pricing" :spec="model.video_pricing" :rate="videoRate" class="plaza-card-prices plaza-single-price" />
    <div v-else class="plaza-card-prices" :class="{ 'plaza-single-price': !quote.token }">
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
import BrandIcon from '@/components/icons/brands/BrandIcon.vue'
import PlazaVideoPricing from './PlazaVideoPricing.vue'
import { inferVideoVendor, videoVendorStyleByVendor } from '@/constants/videoBrands'
import type { GroupPlatform } from '@/types'
import type { ModelPlazaGroup, PlazaModel } from '@/api/modelPlaza'
import { plazaCardPricing, plazaHasExclusiveRate, plazaProvider } from '@/utils/modelPlazaPricing'
import { REASONING_EFFORT_LEVELS } from '@/constants/channel'

const props = defineProps<{ model: PlazaModel; group: ModelPlazaGroup }>()
defineEmits<{ details: [model: PlazaModel] }>()
const { t } = useI18n()
const quote = computed(() => plazaCardPricing(props.model, props.group))
const exclusive = computed(() => plazaHasExclusiveRate(props.group))
// 视频模型：按模型名推断厂商标志，实付倍率 = 视频独立倍率或分组 / 专属倍率。
const videoVendorIcon = computed(() => (props.model.platform === 'video' ? videoVendorStyleByVendor(inferVideoVendor(props.model.name)).icon : ''))
// 视频模型显示厂商名（Seedance、可灵 Kling…），平台代号 video 对用户没有意义。
const providerLabel = computed(() => (props.model.platform === 'video' ? videoVendorStyleByVendor(inferVideoVendor(props.model.name)).label : plazaProvider(props.model.platform)))
const videoRate = computed(() => props.group.video_rate_independent ? props.group.video_rate_multiplier : (props.group.user_rate_multiplier ?? props.group.rate_multiplier))
const billingLabel = computed(() => props.model.video_pricing ? t(`modelPlaza.video.${props.model.video_pricing.unit === 'per_second' ? 'perSecond' : props.model.video_pricing.unit === 'per_video' ? 'perVideo' : 'token'}`) : t(quote.value.token ? 'modelPlaza.cards.tokenBilling' : quote.value.mode === 'image' ? 'modelPlaza.table.perImage' : 'modelPlaza.table.perRequest'))
const unit = computed(() => props.model.video_pricing ? billingLabel.value : t(quote.value.token ? 'modelPlaza.cards.perMillion' : quote.value.mode === 'image' ? 'modelPlaza.table.perUnitImage' : 'modelPlaza.table.perUnitRequest'))
const specialRules = computed(() => {
  const labels: string[] = []
  if (quote.value.tiered) labels.push(t('modelPlaza.cards.tiered'))
  if (props.model.time_pricing?.periods.length) labels.push(t('modelPlaza.cards.timePricing'))
  if (props.group.peak_rate_enabled) labels.push(t('modelPlaza.cards.peakPricing'))
  const multipliers = props.model.pricing?.reasoning_effort_multipliers
  for (const effort of REASONING_EFFORT_LEVELS) {
    const multiplier = multipliers?.[effort]
    if (typeof multiplier === 'number' && Number.isFinite(multiplier) && multiplier > 0) {
      labels.push(t('modelPlaza.table.reasoningMultiplierBadge', { effort, multiplier }))
    }
  }
  if (quote.value.mode === 'image' && props.group.image_rate_independent) labels.push(t('modelPlaza.cards.imageRate'))
  return labels
})
</script>
