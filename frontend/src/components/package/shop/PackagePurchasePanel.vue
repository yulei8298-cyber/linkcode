<template>
  <div :class="['pkg-card pkg-hue panel', `pkg-h-${style.hue}`]" data-test="package-purchase-panel">
    <p class="panel-label">{{ t('packages.payment.confirmTitle') }}</p>
    <div class="panel-head">
      <span class="pkg-icon-box"><Icon :name="style.icon" size="md" /></span>
      <div class="panel-title">
        <b>{{ state.plan.name }}</b>
        <span class="pkg-badge">{{ t(`packages.badge.${style.badge}`) }}</span>
      </div>
      <span class="panel-price pkg-mono">{{ formatCNY(state.plan.price) }}</span>
    </div>
    <div class="panel-facts">
      <div><dt>{{ t('packages.shop.quota') }}</dt><dd class="pkg-mono">{{ formatUSD(state.plan.quota_usd) }}</dd></div>
      <div><dt>{{ t('packages.shop.groupLabel') }}</dt><dd>{{ state.groupName }}</dd></div>
      <div><dt>{{ t('packages.shop.rate') }}</dt><dd class="pkg-mono">×{{ state.rateMultiplier }}</dd></div>
      <div><dt>{{ t('packages.cycle.' + state.plan.cycle) }}</dt><dd>{{ t('packages.shop.validity', { days: state.plan.validity_days }) }}</dd></div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import { formatCNY, formatUSD, packagePlanStyle } from '../packageUtils'
import type { PackagePurchaseState } from './usePackagePurchase'
import '../package.css'

const props = defineProps<{ state: PackagePurchaseState }>()

const { t } = useI18n()
const style = computed(() => packagePlanStyle(props.state.plan.cycle, props.state.plan.tier))
</script>

<style scoped>
.panel {
  display: grid;
  gap: 14px;
  padding: 20px;
  border-color: var(--pkg-h-line);
  background: radial-gradient(110% 80% at 100% 0%, var(--pkg-h-soft), transparent 62%), var(--lc-surface);
}
.panel-label { margin: 0; font-size: 12px; font-weight: 500; color: var(--lc-ink-3); }
.panel-head { display: flex; align-items: center; gap: 12px; }
.panel-title { display: flex; flex-wrap: wrap; align-items: center; gap: 8px; min-width: 0; }
.panel-title b { font-size: 18px; font-weight: 800; color: var(--lc-ink); }
.panel-price { margin-left: auto; font-size: 28px; font-weight: 800; color: var(--pkg-h-text); }
.panel-facts { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); margin: 0; border: 1px solid var(--pkg-h-line); border-radius: 12px; overflow: hidden; }
.panel-facts > div { min-width: 0; padding: 10px 14px; border-left: 1px solid var(--lc-line); }
.panel-facts > div:first-child { border-left: 0; }
.panel-facts dt { font-size: 12px; color: var(--lc-ink-3); }
.panel-facts dd { margin: 2px 0 0; font-weight: 700; color: var(--lc-ink); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
@media (max-width: 640px) {
  .panel-facts { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .panel-facts > div:nth-child(3) { border-left: 0; }
  .panel-facts > div:nth-child(n + 3) { border-top: 1px solid var(--lc-line); }
}
</style>
