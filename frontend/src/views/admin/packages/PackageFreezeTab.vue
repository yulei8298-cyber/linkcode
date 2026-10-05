<template>
  <div class="space-y-5">
    <div v-if="!settings" class="card p-6 text-center text-sm text-gray-500">{{ t('admin.packages.common.loading') }}</div>
    <template v-else>
      <section class="card space-y-4 p-5">
        <div class="flex items-start justify-between gap-4">
          <div>
            <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.packages.freeze.switchTitle') }}</h3>
            <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.packages.freeze.switchDesc') }}</p>
          </div>
          <Toggle v-model="settings.freeze_enabled" />
        </div>
        <div class="flex items-start justify-between gap-4 border-t border-gray-100 pt-4 dark:border-dark-700">
          <div>
            <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.packages.freeze.weekendTitle') }}</h3>
            <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.packages.freeze.weekendDesc') }}</p>
          </div>
          <span class="badge badge-primary whitespace-nowrap">{{ t('admin.packages.freeze.fixed') }}</span>
        </div>
        <div class="flex flex-wrap items-start justify-between gap-4 border-t border-gray-100 pt-4 dark:border-dark-700">
          <div>
            <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.packages.freeze.capTitle') }}</h3>
            <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.packages.freeze.capDesc') }}</p>
          </div>
          <label class="flex items-center gap-2 text-sm text-gray-600 dark:text-gray-300">
            <input v-model.number="settings.max_freeze_days" type="number" min="1" max="30" class="input w-24" />
            {{ t('admin.packages.freeze.daysUnit') }}
          </label>
        </div>
        <div class="grid gap-4 border-t border-gray-100 pt-4 dark:border-dark-700 md:grid-cols-[auto_1fr]">
          <label class="flex items-center gap-3 text-sm font-semibold text-gray-900 dark:text-white">
            <Toggle v-model="settings.holiday_sync_enabled" />{{ t('admin.packages.freeze.syncEnabled') }}
          </label>
          <label class="block">
            <span class="input-label">{{ t('admin.packages.freeze.sourceUrl') }}</span>
            <input v-model.trim="settings.holiday_source_url" class="input font-mono text-xs" />
            <span class="input-hint">{{ t('admin.packages.freeze.sourceUrlHint', { year: '{year}' }) }}</span>
          </label>
        </div>
        <button type="button" class="btn btn-primary" :disabled="busy" @click="saveSettings">{{ t('admin.packages.common.save') }}</button>
      </section>

      <section class="card space-y-4 p-5">
        <div class="flex flex-wrap items-start justify-between gap-3">
          <div class="max-w-3xl">
            <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.packages.freeze.holidayTitle') }}</h3>
            <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.packages.freeze.holidayDesc') }}</p>
            <p class="mt-2 text-xs text-gray-600 dark:text-gray-300">{{ syncSummary }}</p>
            <p v-if="settings.holiday_sync.last_error" class="mt-1 text-xs text-red-600">{{ t('admin.packages.freeze.syncError', { msg: settings.holiday_sync.last_error }) }}</p>
          </div>
          <div class="flex items-center gap-2">
            <select v-model.number="year" class="input w-28" :aria-label="t('admin.packages.freeze.year')">
              <option v-for="y in yearOptions" :key="y" :value="y">{{ y }}</option>
            </select>
            <button type="button" class="btn btn-secondary" :disabled="busy" @click="syncNow">{{ t('admin.packages.freeze.syncNow') }}</button>
          </div>
        </div>

        <div class="overflow-x-auto rounded-xl border border-gray-200 dark:border-dark-700">
          <table class="w-full text-sm">
            <thead class="bg-gray-50 text-left text-xs text-gray-500 dark:bg-dark-800 dark:text-gray-400">
              <tr>
                <th class="px-4 py-2.5">{{ t('admin.packages.freeze.table.name') }}</th>
                <th class="px-4 py-2.5">{{ t('admin.packages.freeze.table.start') }}</th>
                <th class="px-4 py-2.5">{{ t('admin.packages.freeze.table.end') }}</th>
                <th class="px-4 py-2.5">{{ t('admin.packages.freeze.table.days') }}</th>
                <th class="px-4 py-2.5">{{ t('admin.packages.freeze.table.kind') }}</th>
                <th class="px-4 py-2.5">{{ t('admin.packages.freeze.table.source') }}</th>
                <th class="px-4 py-2.5"></th>
              </tr>
            </thead>
            <tbody>
              <tr v-if="!ranges.length"><td colspan="7" class="px-4 py-6 text-center text-gray-500">{{ t('admin.packages.freeze.empty') }}</td></tr>
              <tr v-for="r in ranges" :key="`${r.source}-${r.kind}-${r.name}-${r.start}`" class="border-t border-gray-100 dark:border-dark-700">
                <td class="px-4 py-2.5 font-medium text-gray-900 dark:text-white">{{ r.name }}</td>
                <td class="px-4 py-2.5 font-mono">{{ r.start }}</td>
                <td class="px-4 py-2.5 font-mono">{{ r.end }}</td>
                <td class="px-4 py-2.5">{{ t('admin.packages.common.days', { n: r.days }) }}</td>
                <td class="px-4 py-2.5">{{ r.kind === 'off' ? t('admin.packages.freeze.kindOff') : t('admin.packages.freeze.kindWork') }}</td>
                <td class="px-4 py-2.5">
                  <span :class="['badge', r.source === 'auto' ? 'badge-primary' : 'badge-gray']">
                    {{ r.source === 'auto' ? t('admin.packages.freeze.sourceAuto') : t('admin.packages.freeze.sourceManual') }}
                  </span>
                </td>
                <td class="px-4 py-2.5 text-right">
                  <button v-if="r.source === 'manual'" type="button" class="btn btn-danger btn-sm" :disabled="busy" @click="removeHoliday(r)">
                    {{ t('admin.packages.common.delete') }}
                  </button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>

        <div class="grid gap-3 md:grid-cols-[1.4fr_1fr_1fr_auto] md:items-end">
          <label class="block">
            <span class="input-label">{{ t('admin.packages.freeze.addName') }}</span>
            <input v-model.trim="draft.name" class="input" maxlength="50" :placeholder="t('admin.packages.freeze.addNamePlaceholder')" />
          </label>
          <label class="block">
            <span class="input-label">{{ t('admin.packages.freeze.start') }}</span>
            <input v-model="draft.start" type="date" class="input" />
          </label>
          <label class="block">
            <span class="input-label">{{ t('admin.packages.freeze.end') }}</span>
            <input v-model="draft.end" type="date" class="input" />
          </label>
          <button type="button" class="btn btn-primary" :disabled="busy || !draft.name || !draft.start || !draft.end" @click="addHoliday">
            {{ t('admin.packages.freeze.add') }}
          </button>
        </div>
      </section>
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import Toggle from '@/components/common/Toggle.vue'
import packagesAdminAPI, { type PackageHolidayRange, type PackageSettingsResponse } from '@/api/admin/packages'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import { formatDateTimeToMinute } from '@/utils/format'

const { t } = useI18n()
const appStore = useAppStore()

const settings = ref<PackageSettingsResponse | null>(null)
const ranges = ref<PackageHolidayRange[]>([])
const busy = ref(false)
const currentYear = new Date().getFullYear()
const year = ref(currentYear)
const yearOptions = [currentYear - 1, currentYear, currentYear + 1]
const draft = reactive({ name: '', start: '', end: '' })

const syncSummary = computed(() => {
  const sync = settings.value?.holiday_sync
  if (!sync?.last_synced_at) return t('admin.packages.freeze.neverSynced')
  const years = Object.entries(sync.years || {}).map(([y, n]) =>
    n > 0 ? t('admin.packages.freeze.syncYears', { year: y, n }) : t('admin.packages.freeze.syncNotPublished', { year: y }),
  )
  return [t('admin.packages.freeze.syncAt', { time: formatDateTimeToMinute(sync.last_synced_at) }), ...years].join(' · ')
})

async function run(action: () => Promise<unknown>, success: string) {
  busy.value = true
  try {
    await action()
    appStore.showSuccess(success)
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('common.error')))
  } finally {
    busy.value = false
  }
}

async function loadRanges() {
  ranges.value = await packagesAdminAPI.listHolidays(year.value)
}

const saveSettings = () =>
  run(async () => {
    const s = settings.value!
    const saved = await packagesAdminAPI.updateSettings({
      freeze_enabled: s.freeze_enabled,
      max_freeze_days: s.max_freeze_days,
      holiday_sync_enabled: s.holiday_sync_enabled,
      holiday_source_url: s.holiday_source_url,
      notice_text: s.notice_text,
      notice_version: s.notice_version,
    })
    settings.value = { ...s, ...saved }
  }, t('admin.packages.common.saved'))

const syncNow = () =>
  run(async () => {
    const state = await packagesAdminAPI.syncHolidays()
    if (settings.value) settings.value.holiday_sync = state
    await loadRanges()
  }, t('admin.packages.freeze.synced'))

const addHoliday = () =>
  run(async () => {
    await packagesAdminAPI.addHoliday({ ...draft })
    draft.name = ''
    draft.start = ''
    draft.end = ''
    await loadRanges()
  }, t('admin.packages.freeze.added'))

const removeHoliday = (r: PackageHolidayRange) =>
  run(async () => {
    await packagesAdminAPI.deleteHoliday({ name: r.name, start: r.start, end: r.end })
    await loadRanges()
  }, t('admin.packages.plans.deleted'))

watch(year, () => {
  loadRanges().catch((err: unknown) => appStore.showError(extractApiErrorMessage(err, t('admin.packages.common.loadFailed'))))
})

onMounted(async () => {
  try {
    settings.value = await packagesAdminAPI.getSettings()
    await loadRanges()
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('admin.packages.common.loadFailed')))
  }
})
</script>
