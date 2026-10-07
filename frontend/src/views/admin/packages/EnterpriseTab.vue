<template>
  <section class="card space-y-5 p-5" data-test="enterprise-tab">
    <div v-if="!settings" class="text-center text-sm text-gray-500">{{ t('admin.packages.common.loading') }}</div>
    <template v-else>
      <div class="flex items-start justify-between gap-4">
        <div>
          <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.packages.enterprise.switchTitle') }}</h3>
          <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.packages.enterprise.switchDesc') }}</p>
        </div>
        <Toggle v-model="settings.enabled" />
      </div>

      <label class="block border-t border-gray-100 pt-4 dark:border-dark-700">
        <span class="input-label">{{ t('admin.packages.enterprise.thresholdLabel') }}</span>
        <input v-model.number="settings.threshold" type="number" min="1" step="1" class="input w-48" data-test="threshold" />
        <span class="input-hint block">{{ t('admin.packages.enterprise.thresholdHint') }}</span>
      </label>

      <div class="space-y-3 border-t border-gray-100 pt-4 dark:border-dark-700" data-test="group-rates">
        <div>
          <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.packages.enterprise.ratesTitle') }}</h3>
          <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.packages.enterprise.ratesDesc') }}</p>
        </div>
        <div v-for="(row, index) in settings.group_rates" :key="index" class="flex flex-wrap items-center gap-3" data-test="group-rate-row">
          <select v-model.number="row.group_id" class="input w-56">
            <option :value="0" disabled>{{ t('admin.packages.enterprise.pickGroup') }}</option>
            <option v-for="g in groups" :key="g.id" :value="g.id" :disabled="isUsedElsewhere(g.id, index)">{{ g.name }}</option>
          </select>
          <span class="text-sm text-gray-500">{{ t('admin.packages.enterprise.baseRate', { rate: baseRate(row.group_id) }) }}</span>
          <label class="flex items-center gap-2 text-sm text-gray-600 dark:text-gray-300">
            {{ t('admin.packages.enterprise.enterpriseRate') }}
            <input v-model.number="row.multiplier" type="number" min="0.0001" step="0.01" class="input w-28" data-test="group-rate-input" />
          </label>
          <button type="button" class="btn btn-secondary btn-sm" @click="settings.group_rates.splice(index, 1)">{{ t('admin.packages.common.delete') }}</button>
        </div>
        <button type="button" class="btn btn-secondary btn-sm" data-test="add-group-rate" @click="settings.group_rates.push({ group_id: 0, multiplier: 0 })">
          {{ t('admin.packages.enterprise.addRate') }}
        </button>
      </div>

      <div class="rounded-xl bg-gray-50 p-4 text-xs leading-6 text-gray-600 dark:bg-dark-800 dark:text-gray-300">
        <p class="font-semibold text-gray-900 dark:text-white">{{ t('admin.packages.enterprise.ruleTitle') }}</p>
        <ul class="mt-1 list-disc pl-5">
          <li v-for="n in 5" :key="n">{{ t(`admin.packages.enterprise.rule${n}`) }}</li>
        </ul>
      </div>

      <button type="button" class="btn btn-primary" :disabled="busy" data-test="save" @click="save">{{ t('admin.packages.common.save') }}</button>
    </template>
  </section>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Toggle from '@/components/common/Toggle.vue'
import { adminAPI } from '@/api/admin'
import packagesAdminAPI, { type EnterpriseSettings } from '@/api/admin/packages'
import type { AdminGroup } from '@/types'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'

const { t } = useI18n()
const appStore = useAppStore()

const settings = ref<EnterpriseSettings | null>(null)
const groups = ref<AdminGroup[]>([])
const busy = ref(false)

const RATE_MAX = 100

const baseRate = (groupId: number) => groups.value.find((g) => g.id === groupId)?.rate_multiplier ?? '-'
const isUsedElsewhere = (groupId: number, index: number) =>
  settings.value?.group_rates.some((r, i) => i !== index && r.group_id === groupId) ?? false

// 返回第一条不合法的提示；全部合法时返回空串。
function groupRateError(rates: EnterpriseSettings['group_rates']): string {
  const seen = new Set<number>()
  for (const r of rates) {
    if (!(r.group_id > 0)) return t('admin.packages.enterprise.invalidGroup')
    if (seen.has(r.group_id)) return t('admin.packages.enterprise.duplicateGroup')
    seen.add(r.group_id)
    if (!(r.multiplier > 0) || r.multiplier > RATE_MAX) return t('admin.packages.enterprise.invalidRate')
  }
  return ''
}

async function save() {
  const current = settings.value
  if (!current) return
  if (!(current.threshold > 0)) {
    appStore.showError(t('admin.packages.enterprise.invalidThreshold'))
    return
  }
  const rateError = groupRateError(current.group_rates)
  if (rateError) {
    appStore.showError(rateError)
    return
  }
  busy.value = true
  try {
    settings.value = await packagesAdminAPI.updateEnterpriseSettings({
      enabled: current.enabled,
      threshold: current.threshold,
      group_rates: current.group_rates.map((r) => ({ group_id: r.group_id, multiplier: r.multiplier })),
    })
    appStore.showSuccess(t('admin.packages.common.saved'))
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('common.error')))
  } finally {
    busy.value = false
  }
}

onMounted(async () => {
  try {
    const loaded = await packagesAdminAPI.getEnterpriseSettings()
    settings.value = { ...loaded, group_rates: loaded.group_rates ?? [] }
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('admin.packages.common.loadFailed')))
  }
  try {
    // 企业倍率只对按量（余额）分组有意义，订阅分组不在可选范围内。
    groups.value = (await adminAPI.groups.getAll()).filter((g) => g.subscription_type === 'standard')
  } catch {
    groups.value = []
  }
})
</script>
