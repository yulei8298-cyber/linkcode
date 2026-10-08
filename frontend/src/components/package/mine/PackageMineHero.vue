<template>
  <div class="hero">
    <div class="hero-main">
      <div class="hero-lab">
        {{ t('packages.mine.available') }}
        <b>{{ t('packages.mine.concurrencyPill', { n: concurrency }) }}</b>
      </div>
      <div class="hero-amt pkg-mono" data-test="package-available">{{ formatUSD(activeRemaining) }}</div>
      <div class="hero-sub">
        {{ frozen.length
          ? t('packages.mine.subFrozen', { count: active.length + frozen.length, frozen: formatUSD(frozenRemaining) })
          : t('packages.mine.sub', { count: active.length }) }}
      </div>
      <div v-if="segments.length" class="hero-stack" aria-hidden="true">
        <i v-for="seg in segments" :key="seg.id" :class="{ fz: seg.frozen }" :style="{ flex: seg.weight, background: seg.color }"></i>
      </div>
      <div class="hero-legend">
        <span v-for="seg in segments" :key="seg.id" :style="{ '--c': seg.frozen ? '#7dd3fc' : seg.color }">
          {{ seg.label }} {{ formatUSD(seg.remaining) }}
        </span>
      </div>
    </div>

    <dl class="hero-side">
      <div class="hero-stat pkg-hue pkg-h-green">
        <span class="pkg-icon-box"><Icon name="bolt" size="sm" /></span>
        <div>
          <dt>{{ t('packages.mine.using') }}</dt>
          <dd v-if="using">{{ using.name }}<small>{{ t('packages.mine.expiresAt', { time: formatDateTimeToMinute(using.expires_at) }) }}</small></dd>
          <dd v-else>{{ t('packages.mine.usingNone') }}<small>{{ t('packages.mine.usingNoneHint') }}</small></dd>
        </div>
      </div>
      <div class="hero-stat pkg-hue pkg-h-ice">
        <span class="pkg-icon-box"><Icon name="clock" size="sm" /></span>
        <div>
          <dt>{{ t('packages.mine.frozenCount') }}</dt>
          <dd>{{ t('packages.mine.frozenUnit', { n: frozen.length }) }}<small>{{ frozen.length ? t('packages.mine.frozenHint') : t('packages.mine.frozenNone') }}</small></dd>
        </div>
      </div>
      <div class="hero-stat pkg-hue pkg-h-orange">
        <span class="pkg-icon-box"><Icon name="creditCard" size="sm" /></span>
        <div>
          <dt>{{ t('packages.mine.balance') }}</dt>
          <dd class="pkg-mono">{{ formatUSD(balance) }}<small>{{ t('packages.mine.balanceHint') }}</small></dd>
        </div>
      </div>
    </dl>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import type { UserPackageView } from '@/api/packages'
import { formatDateTimeToMinute } from '@/utils/format'
import { formatUSD, packagePlanStyle } from '../packageUtils'

const props = defineProps<{
  packages: UserPackageView[]
  concurrency: number
  balance: number
}>()

const { t } = useI18n()

const HUE_COLORS: Record<string, string> = { violet: '#8b5cf6', orange: '#ff7a1a', green: '#10b981', rose: '#f43f5e' }

const active = computed(() => props.packages.filter((p) => p.status === 'active'))
const frozen = computed(() => props.packages.filter((p) => p.status === 'frozen'))
const using = computed(() => active.value.find((p) => p.deduct_order === 1) ?? null)
const activeRemaining = computed(() => active.value.reduce((sum, p) => sum + p.remaining_usd, 0))
const frozenRemaining = computed(() => frozen.value.reduce((sum, p) => sum + p.remaining_usd, 0))

/** 额度构成条：按剩余额度占比分段，冻结的套餐用冰蓝斜纹。 */
const segments = computed(() =>
  [...active.value, ...frozen.value]
    .filter((p) => p.remaining_usd > 0)
    .map((p) => ({
      id: p.id,
      label: p.name,
      remaining: p.remaining_usd,
      weight: p.remaining_usd,
      frozen: p.status === 'frozen',
      color: HUE_COLORS[packagePlanStyle(p.cycle, p.tier).hue],
    })),
)
</script>

<style scoped>
.hero { display: grid; grid-template-columns: minmax(0, 1.6fr) minmax(0, 1fr); gap: 16px; }
.hero-main {
  position: relative;
  overflow: hidden;
  padding: 24px 26px;
  border-radius: 20px;
  color: #fff;
  background:
    radial-gradient(90% 120% at 100% 0%, rgba(255, 79, 109, 0.55), transparent 60%),
    radial-gradient(80% 120% at 0% 100%, rgba(139, 92, 246, 0.55), transparent 60%),
    linear-gradient(135deg, #ff7a1a, #e8452c 45%, #7c3aed);
  box-shadow: 0 24px 60px -30px rgba(232, 69, 44, 0.8);
}
.hero-lab { display: flex; flex-wrap: wrap; align-items: center; gap: 8px; font-size: 13px; opacity: 0.92; }
.hero-lab b { padding: 1px 10px; border-radius: 999px; background: rgba(255, 255, 255, 0.18); font-weight: 600; }
.hero-amt { margin: 10px 0 2px; font-size: 46px; font-weight: 800; line-height: 1.05; letter-spacing: -0.03em; }
.hero-sub { font-size: 14px; opacity: 0.92; }
.hero-stack { display: flex; gap: 2px; height: 12px; margin-top: 18px; border-radius: 6px; overflow: hidden; background: rgba(255, 255, 255, 0.2); }
.hero-stack i { display: block; height: 100%; }
.hero-stack i.fz { background: repeating-linear-gradient(135deg, rgba(186, 230, 253, 0.95) 0 4px, rgba(125, 211, 252, 0.7) 4px 8px) !important; }
.hero-legend { display: flex; flex-wrap: wrap; gap: 4px 14px; margin-top: 10px; font-size: 13px; }
.hero-legend span { display: inline-flex; align-items: center; gap: 6px; }
.hero-legend span::before { content: ''; width: 9px; height: 9px; border-radius: 3px; background: var(--c); }
.hero-side { display: grid; gap: 12px; margin: 0; }
.hero-stat {
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 14px 18px;
  border-radius: 16px;
  border: 1px solid var(--pkg-h-line);
  background: linear-gradient(120deg, var(--pkg-h-soft), transparent 75%), var(--lc-surface);
}
.hero-stat dt { font-size: 13px; color: var(--lc-ink-3); }
.hero-stat dd { margin: 0; font-size: 18px; font-weight: 800; color: var(--lc-ink); }
.hero-stat dd small { margin-left: 6px; font-size: 13px; font-weight: 500; color: var(--lc-ink-3); }
@media (max-width: 1100px) {
  .hero { grid-template-columns: minmax(0, 1fr); }
  .hero-side { grid-template-columns: repeat(3, minmax(0, 1fr)); }
}
@media (max-width: 640px) {
  .hero-side { grid-template-columns: minmax(0, 1fr); }
  .hero-amt { font-size: 38px; }
}
</style>
