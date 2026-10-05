<template>
  <article :class="['pkg-card pkg-hue item', `pkg-h-${style.hue}`, { using: isUsing, frozen: isFrozen }]" :data-test="`user-package-${item.id}`">
    <span class="item-order pkg-mono" :aria-label="t('packages.mine.activeHint')">
      <Icon v-if="isFrozen" name="clock" size="sm" />
      <template v-else>{{ item.deduct_order }}</template>
    </span>

    <div class="item-main">
      <div class="item-title">
        <b>{{ item.name }}</b>
        <span class="item-group">{{ item.group_name }}</span>
        <span :class="['pkg-chip', chipClass]">{{ statusLabel }}</span>
      </div>
      <div class="item-amount">
        <b class="pkg-mono">{{ formatUSD(item.remaining_usd) }}</b>
        <span>{{ t('packages.mine.remaining') }} / {{ formatUSD(item.quota_usd) }}</span>
        <span class="item-pct pkg-mono">{{ t('packages.mine.usedPct', { n: percent }) }}</span>
      </div>
      <div :class="['pkg-bar', { 'item-bar-frozen': isFrozen }]" role="progressbar" :aria-valuenow="percent" aria-valuemin="0" aria-valuemax="100">
        <i :style="{ width: `${percent}%` }"></i>
      </div>
      <div class="item-meta">
        <span>{{ t('packages.mine.boughtAt') }} <b class="pkg-mono">{{ formatDateTimeToMinute(item.starts_at) }}</b></span>
        <span>{{ t('packages.mine.expireAt') }} <b class="pkg-mono">{{ formatDateTimeToMinute(item.expires_at) }}</b></span>
        <span>{{ t('packages.mine.leftTime') }} <b>{{ isFrozen ? t('packages.mine.paused') : leftTimeLabel }}</b></span>
      </div>
      <div v-if="isFrozen" class="item-ice">
        <Icon name="clock" size="sm" />
        {{ t('packages.mine.frozenNote', { time: formatDateTimeToMinute(item.frozen_at), used: frozenDaysLabel, max: item.max_freeze_days }) }}
      </div>
    </div>

    <div class="item-actions">
      <template v-if="isFrozen">
        <button type="button" class="pkg-btn pkg-btn-sm pkg-btn-warm" :disabled="busy" data-test="package-unfreeze" @click="emit('unfreeze', item)">
          {{ t('packages.mine.unfreezeBtn') }}
        </button>
        <small>{{ t('packages.mine.frozenUsed', { used: frozenDaysLabel, max: item.max_freeze_days }) }}</small>
      </template>
      <template v-else-if="freezeEnabled">
        <button
          type="button"
          class="pkg-btn pkg-btn-sm pkg-btn-ice"
          :disabled="busy || !canFreeze"
          data-test="package-freeze"
          @click="emit('freeze', item)"
        >
          {{ t('packages.mine.freezeBtn') }}
        </button>
        <small v-if="item.freeze_left_seconds <= 0">{{ t('packages.mine.capUsed') }}</small>
        <small v-else-if="!todayFreezable">
          {{ t('packages.mine.onlyHoliday') }}<template v-if="nextFreezableDate"><br />{{ t('packages.mine.nextDay', { date: nextFreezableDate }) }}</template>
        </small>
        <small v-else>{{ t('packages.mine.freezeLeft', { left: formatDuration(item.freeze_left_seconds) }) }}</small>
      </template>
      <button type="button" class="pkg-btn pkg-btn-sm pkg-btn-ghost" @click="emit('rebuy')">
        <Icon name="creditCard" size="sm" />{{ t('packages.mine.rebuy') }}
      </button>
    </div>
  </article>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import type { UserPackageView } from '@/api/packages'
import { formatDateTimeToMinute } from '@/utils/format'
import { formatUSD, packagePlanStyle, splitDuration, usedPercent } from '../packageUtils'

const props = defineProps<{
  item: UserPackageView
  freezeEnabled: boolean
  todayFreezable: boolean
  nextFreezableDate: string
  busy: boolean
}>()

const emit = defineEmits<{
  freeze: [item: UserPackageView]
  unfreeze: [item: UserPackageView]
  rebuy: []
}>()

const { t } = useI18n()

const style = computed(() => packagePlanStyle(props.item.cycle, props.item.tier))
const isFrozen = computed(() => props.item.status === 'frozen')
const isUsing = computed(() => props.item.status === 'active' && props.item.deduct_order === 1)
const percent = computed(() => usedPercent(props.item.used_usd, props.item.quota_usd))
const canFreeze = computed(() => props.todayFreezable && props.item.freeze_left_seconds > 0)

const statusLabel = computed(() => {
  if (isFrozen.value) return t('packages.mine.status.frozen')
  return isUsing.value ? t('packages.mine.status.active') : t('packages.mine.status.queued')
})
const chipClass = computed(() => {
  if (isFrozen.value) return 'pkg-chip-frozen'
  return isUsing.value ? 'pkg-chip-using' : 'pkg-chip-queued'
})

function formatDuration(seconds: number): string {
  const { days, hours } = splitDuration(seconds)
  if (days && hours) return t('packages.mine.duration.dayHours', { d: days, h: hours })
  if (days) return t('packages.mine.duration.days', { d: days })
  return t('packages.mine.duration.hours', { h: hours })
}

const leftTimeLabel = computed(() => formatDuration((Date.parse(props.item.expires_at) - Date.now()) / 1000))
/** 已冻结天数保留一位小数，便于看出还差多久到上限。 */
const frozenDaysLabel = computed(() => (props.item.frozen_seconds / 86400).toFixed(1))
</script>

<style scoped>
.item {
  display: grid;
  grid-template-columns: 46px minmax(0, 1fr) auto;
  gap: 18px;
  align-items: start;
  padding: 20px 22px 20px 20px;
  background: linear-gradient(100deg, var(--pkg-h-soft), transparent 45%), var(--lc-surface);
}
.item::before { content: ''; position: absolute; inset: 0 auto 0 0; width: 4px; background: var(--pkg-h); }
.item.using { border-color: var(--pkg-h-line); }
.item.frozen {
  border-color: color-mix(in oklab, #0ea5e9 50%, transparent);
  background:
    repeating-linear-gradient(135deg, color-mix(in oklab, #0ea5e9 6%, transparent) 0 10px, transparent 10px 22px),
    linear-gradient(100deg, color-mix(in oklab, #0ea5e9 14%, transparent), transparent 55%),
    var(--lc-surface);
}
.item.frozen::before { background: linear-gradient(180deg, #38bdf8, #6366f1); }
.item-order {
  display: grid;
  place-items: center;
  width: 46px;
  height: 46px;
  border-radius: 13px;
  border: 1px solid var(--pkg-h-line);
  background: var(--pkg-h-soft);
  color: var(--pkg-h-text);
  font-size: 18px;
  font-weight: 800;
}
.item.using .item-order { border-color: transparent; background: var(--pkg-h-btn); color: #fff; box-shadow: 0 10px 22px -10px var(--pkg-h); }
.item-main { min-width: 0; display: grid; gap: 8px; }
.item-title { display: flex; flex-wrap: wrap; align-items: center; gap: 8px; }
.item-title b { font-size: 17px; font-weight: 800; color: var(--lc-ink); }
.item-group { font-size: 13px; color: var(--lc-ink-3); }
.item-amount { display: flex; flex-wrap: wrap; align-items: baseline; gap: 8px; }
.item-amount b { font-size: 28px; font-weight: 800; line-height: 1.1; letter-spacing: -0.02em; color: var(--pkg-h-text); }
.item-amount span { font-size: 13px; color: var(--lc-ink-3); }
.item-pct { margin-left: auto; color: var(--lc-ink-2) !important; }
.item-bar-frozen > i { background: repeating-linear-gradient(135deg, #7dd3fc 0 6px, #38bdf8 6px 12px); }
.item-meta { display: flex; flex-wrap: wrap; gap: 6px 22px; font-size: 13px; color: var(--lc-ink-3); }
.item-meta b { font-weight: 600; color: var(--lc-ink); }
.item-ice {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 9px 13px;
  border-radius: 11px;
  border: 1px solid color-mix(in oklab, #0ea5e9 35%, transparent);
  background: color-mix(in oklab, #0ea5e9 12%, transparent);
  font-size: 13px;
  font-weight: 600;
  color: #0369a1;
}
.dark .item-ice { color: #7dd3fc; }
.item-actions { display: grid; gap: 8px; min-width: 128px; }
.item-actions small { font-size: 12px; line-height: 1.4; text-align: center; color: var(--lc-ink-3); }
@media (max-width: 640px) {
  .item { grid-template-columns: minmax(0, 1fr); padding: 18px 16px 18px 18px; }
  .item-order { display: none; }
  .item-actions { grid-template-columns: 1fr 1fr; min-width: 0; }
  /* 手机上两个按钮并排，说明文字放到按钮下面 */
  .item-actions small { grid-column: 1 / -1; order: 3; }
}
</style>
