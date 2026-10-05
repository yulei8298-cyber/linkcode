<template>
  <section class="pkg-card cal" aria-labelledby="pkg-cal-title">
    <div class="cal-head">
      <h2 id="pkg-cal-title">{{ t('packages.calendar.title') }}</h2>
      <div class="cal-nav">
        <button type="button" class="cal-nav-btn" :aria-label="t('packages.calendar.prev')" @click="month = shiftMonth(month, -1)">
          <Icon name="chevronLeft" size="sm" />
        </button>
        <span class="pkg-mono" data-test="package-calendar-month">{{ monthLabel }}</span>
        <button type="button" class="cal-nav-btn" :aria-label="t('packages.calendar.next')" @click="month = shiftMonth(month, 1)">
          <Icon name="chevronRight" size="sm" />
        </button>
      </div>
    </div>

    <div class="cal-status">
      <span class="cal-live" :style="{ '--c': statusColor }"></span>
      <template v-if="!freezeEnabled"><b>{{ t('packages.calendar.disabled') }}</b></template>
      <template v-else-if="today.freezable">
        <b>{{ t('packages.calendar.todayOk', { date: today.date, label: dayLabel(today) }) }}</b>
        <span>{{ t('packages.calendar.todayOkHint', { week: maxFreezeDaysWeek, month: maxFreezeDaysMonth }) }}</span>
      </template>
      <template v-else>
        <b>{{ t('packages.calendar.todayNo') }}</b>
        <span>{{ t('packages.calendar.todayNoHint', { date: nextLabel }) }}</span>
      </template>
    </div>

    <div class="cal-grid">
      <div v-for="(wd, i) in weekdays" :key="`wd-${i}`" class="cal-wd">{{ wd }}</div>
      <div
        v-for="(cell, i) in cells"
        :key="i"
        :class="['cal-day', cell.day ? `k-${cell.day.kind}` : 'empty', { today: cell.isToday, off: cell.day && !cell.day.freezable }]"
        :title="cell.day ? dayLabel(cell.day) || t('packages.calendar.none') : ''"
      >
        <template v-if="cell.day">
          <b class="pkg-mono">{{ Number(cell.day.date.slice(8)) }}</b>
          <em>{{ cell.isToday ? t('packages.calendar.today') : dayLabel(cell.day) || '—' }}</em>
        </template>
      </div>
    </div>
    <p v-if="error" class="cal-error">{{ error }}</p>

    <div class="cal-legend">
      <span style="--c: #0ea5e9">{{ t('packages.calendar.weekend') }}</span>
      <span style="--c: #f43f5e">{{ t('packages.calendar.holiday') }}</span>
      <span style="--c: var(--lc-line-2)">{{ t('packages.calendar.none') }}</span>
      <span class="cal-note">{{ t('packages.calendar.note') }}</span>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import packagesAPI, { type PackageCalendarDay } from '@/api/packages'
import { extractApiErrorMessage } from '@/utils/apiError'
import { buildCalendarGrid, shiftMonth } from '../packageUtils'

const props = defineProps<{
  today: PackageCalendarDay
  nextFreezable?: PackageCalendarDay | null
  freezeEnabled: boolean
  maxFreezeDaysWeek: number
  maxFreezeDaysMonth: number
}>()

const { t } = useI18n()
// 以服务端的「今天」定位月份，避免浏览器时区与服务时区不一致。
const month = ref(props.today.date.slice(0, 7))
const days = ref<PackageCalendarDay[]>([])
const error = ref('')

const weekdays = computed(() => t('packages.calendar.weekdays').split(','))
const cells = computed(() => buildCalendarGrid(days.value, props.today.date))
const monthLabel = computed(() => {
  const [y, m] = month.value.split('-')
  return t('packages.calendar.monthLabel', { y, m: Number(m) })
})
const nextLabel = computed(() => {
  const next = props.nextFreezable
  return next ? `${next.date} · ${dayLabel(next)}` : t('packages.calendar.noNext')
})
/** 周末、调休用界面语言显示；节假日显示数据里的节日名。 */
function dayLabel(day: PackageCalendarDay): string {
  if (day.kind === 'weekend') return t('packages.calendar.weekend')
  if (day.kind === 'makeup') return t('packages.calendar.makeup')
  return day.kind === 'holiday' ? day.label : ''
}

const statusColor = computed(() => {
  if (!props.freezeEnabled) return '#8a8a84'
  return props.today.freezable ? '#0ea5e9' : '#f59e0b'
})

watch(
  month,
  async (value) => {
    error.value = ''
    try {
      days.value = await packagesAPI.getCalendar(value)
    } catch (err: unknown) {
      days.value = []
      error.value = extractApiErrorMessage(err, t('packages.calendar.loadFailed'))
    }
  },
  { immediate: true },
)
</script>

<style scoped>
.cal { display: grid; gap: 14px; padding: 18px 20px; }
.cal-head { display: flex; align-items: center; justify-content: space-between; gap: 12px; flex-wrap: wrap; }
.cal-head h2 { margin: 0; font-size: 17px; font-weight: 700; color: var(--lc-ink); }
.cal-nav { display: flex; align-items: center; gap: 10px; font-weight: 700; color: var(--lc-ink); }
.cal-nav-btn { display: grid; place-items: center; width: 32px; height: 32px; border-radius: 9px; border: 1px solid var(--lc-line); background: var(--lc-surface); color: var(--lc-ink-2); }
.cal-nav-btn:hover { color: var(--lc-ink); border-color: var(--lc-line-2); }
.cal-status { display: flex; flex-wrap: wrap; align-items: center; gap: 10px; font-size: 14px; color: var(--lc-ink); }
.cal-status span:not(.cal-live) { font-size: 13px; color: var(--lc-ink-3); }
.cal-live { width: 10px; height: 10px; border-radius: 50%; background: var(--c); box-shadow: 0 0 0 4px color-mix(in oklab, var(--c) 22%, transparent); flex: none; }
.cal-grid { display: grid; grid-template-columns: repeat(7, minmax(0, 1fr)); gap: 6px; }
.cal-wd { text-align: center; font-size: 12px; color: var(--lc-ink-3); }
.cal-day {
  display: grid;
  justify-items: center;
  gap: 1px;
  min-width: 0;
  padding: 8px 2px;
  border-radius: 11px;
  border: 1px solid var(--lc-line);
  background: var(--lc-surface-2);
  color: var(--lc-ink-3);
  font-size: 11.5px;
}
.cal-day.empty { visibility: hidden; }
.cal-day b { font-size: 15px; font-weight: 700; color: var(--lc-ink-2); }
.cal-day em { max-width: 100%; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-size: 11px; font-style: normal; }
.cal-day.k-weekend, .cal-day.k-makeup {
  border-color: color-mix(in oklab, #0ea5e9 45%, transparent);
  background: linear-gradient(180deg, color-mix(in oklab, #0ea5e9 18%, transparent), color-mix(in oklab, #0ea5e9 6%, transparent));
  color: #0369a1;
}
.cal-day.k-holiday {
  border-color: color-mix(in oklab, #f43f5e 45%, transparent);
  background: linear-gradient(180deg, color-mix(in oklab, #f43f5e 18%, transparent), color-mix(in oklab, #f43f5e 6%, transparent));
  color: #be123c;
}
.dark .cal-day.k-weekend, .dark .cal-day.k-makeup { color: #7dd3fc; }
.dark .cal-day.k-holiday { color: #fda4af; }
.cal-day.k-weekend b, .cal-day.k-makeup b, .cal-day.k-holiday b { color: inherit; }
.cal-day.off:not(.k-none) { opacity: 0.55; }
.cal-day.today { outline: 2px solid var(--lc-ink); outline-offset: 1px; }
.cal-error { margin: 0; font-size: 13px; color: #e11d48; }
.cal-legend { display: flex; flex-wrap: wrap; gap: 6px 16px; font-size: 12.5px; color: var(--lc-ink-3); }
.cal-legend span { display: inline-flex; align-items: center; gap: 6px; }
.cal-legend span:not(.cal-note)::before { content: ''; width: 10px; height: 10px; border-radius: 3px; background: var(--c); }
@media (max-width: 640px) {
  .cal-day em { display: none; }
}
</style>
