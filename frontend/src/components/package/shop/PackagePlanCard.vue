<template>
  <article
    :class="['pkg-card pkg-hue plan', `pkg-h-${style.hue}`, { 'plan-hot': style.hot }]"
    :data-test="`package-plan-${plan.id}`"
  >
    <span v-if="style.ribbon" class="plan-ribbon">{{ t('packages.badge.new') }}</span>
    <div class="plan-head">
      <div class="plan-name">
        {{ plan.name }}
        <span class="pkg-badge">{{ t(`packages.badge.${style.badge}`) }}</span>
      </div>
      <span v-if="!style.ribbon" class="pkg-icon-box"><Icon :name="style.icon" size="md" /></span>
    </div>

    <div class="plan-price">
      <b class="pkg-mono">{{ formatCNY(plan.price) }}</b>
      <span>{{ t('packages.shop.buyQuota', { quota: `$${plan.quota_usd}` }) }}</span>
      <span class="pkg-badge">{{ t('packages.shop.validity', { days: plan.validity_days }) }}</span>
    </div>

    <div class="plan-perk">
      <div class="plan-perk-head">
        {{ t('packages.shop.perks') }}
        <span><Icon name="trendingUp" size="xs" />{{ t('packages.shop.stackable') }}</span>
      </div>
      <div class="plan-perk-row">
        <Icon name="dollar" size="sm" />{{ t('packages.shop.quota') }}
        <b class="pkg-mono">{{ formatUSD(plan.quota_usd) }}</b>
      </div>
      <div class="plan-perk-row">
        <Icon name="link" size="sm" />{{ t('packages.shop.groupLabel') }}
        <b>{{ groupName }}</b>
      </div>
      <div class="plan-perk-row">
        <Icon name="bolt" size="sm" />{{ t('packages.shop.rate') }}
        <b class="pkg-mono">×{{ rateMultiplier }}</b>
      </div>
      <div class="plan-perk-row">
        <Icon name="users" size="sm" />{{ t('packages.shop.concurrency') }}
        <b>{{ t('packages.shop.concurrencyValue', { n: concurrency }) }}</b>
      </div>
      <div v-if="freezeEnabled" class="plan-perk-row">
        <Icon name="calendar" size="sm" />{{ t('packages.shop.freezable') }}
      </div>
      <div class="plan-perk-foot">
        <Icon name="shield" size="xs" />{{ t('packages.shop.expireNote') }}
      </div>
    </div>

    <div class="plan-actions">
      <button type="button" class="pkg-btn pkg-btn-outline" @click="emit('notice')">
        <Icon name="document" size="sm" />{{ t('packages.shop.noticeBtn') }}
      </button>
      <button type="button" class="pkg-btn pkg-btn-fill" data-test="package-buy" @click="emit('buy', plan)">
        <Icon name="creditCard" size="sm" />{{ t('packages.shop.buyBtn') }}
      </button>
    </div>
  </article>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import type { PackagePlan } from '@/api/packages'
import { formatCNY, formatUSD, packagePlanStyle } from '../packageUtils'

const props = defineProps<{
  plan: PackagePlan
  groupName: string
  rateMultiplier: number
  freezeEnabled: boolean
  concurrency: number
}>()

const emit = defineEmits<{
  buy: [plan: PackagePlan]
  notice: []
}>()

const { t } = useI18n()
const style = computed(() => packagePlanStyle(props.plan.cycle, props.plan.tier))
</script>

<style scoped>
.plan {
  display: grid;
  gap: 16px;
  padding: 22px;
  border-color: var(--pkg-h-line);
  background: radial-gradient(110% 70% at 100% 0%, var(--pkg-h-soft), transparent 62%), var(--lc-surface);
}
.plan-hot { border-color: var(--pkg-h); box-shadow: 0 0 0 1px var(--pkg-h), 0 24px 60px -28px var(--pkg-h); }
.plan-ribbon {
  position: absolute;
  top: 20px;
  right: -40px;
  width: 150px;
  transform: rotate(45deg);
  padding: 4px 0;
  text-align: center;
  font-size: 12px;
  font-weight: 800;
  letter-spacing: 0.1em;
  color: #fff;
  background: linear-gradient(90deg, #ffb020, #ff6a00);
}
.plan-head { display: flex; align-items: center; justify-content: space-between; gap: 10px; }
.plan-name { display: flex; align-items: center; gap: 10px; font-size: 20px; font-weight: 800; color: var(--lc-ink); }
.plan-price { display: flex; flex-wrap: wrap; align-items: center; gap: 6px 12px; color: var(--lc-ink); }
.plan-price b { font-size: 38px; line-height: 1; font-weight: 800; letter-spacing: -0.03em; color: var(--pkg-h-text); }
.plan-price > span:first-of-type { font-size: 15px; font-weight: 700; }
.plan-perk { border: 1px solid var(--pkg-h-line); border-radius: 14px; overflow: hidden; }
.plan-perk-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 10px 16px;
  background: var(--pkg-h-soft);
  font-size: 14px;
  font-weight: 700;
  color: var(--lc-ink);
}
.plan-perk-head span { display: inline-flex; align-items: center; gap: 4px; font-size: 12.5px; color: var(--pkg-h-text); }
.plan-perk-row {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 11px 16px;
  border-top: 1px solid var(--lc-line);
  font-size: 14px;
  color: var(--lc-ink-2);
}
.plan-perk-row :deep(svg) { color: var(--pkg-h-text); }
.plan-perk-row b { margin-left: auto; text-align: right; font-weight: 700; color: var(--lc-ink); }
.plan-perk-foot {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  padding: 9px 16px;
  border-top: 1px solid var(--lc-line);
  font-size: 12.5px;
  color: var(--lc-ink-3);
}
.plan-actions { display: grid; grid-template-columns: 1fr 1.4fr; gap: 10px; padding-top: 16px; border-top: 1px solid var(--lc-line); }
@media (max-width: 480px) {
  .plan-price b { font-size: 32px; }
}
</style>
