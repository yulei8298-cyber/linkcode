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

      <div class="rounded-xl bg-gray-50 p-4 text-xs leading-6 text-gray-600 dark:bg-dark-800 dark:text-gray-300">
        <p class="font-semibold text-gray-900 dark:text-white">{{ t('admin.packages.enterprise.ruleTitle') }}</p>
        <ul class="mt-1 list-disc pl-5">
          <li v-for="n in 4" :key="n">{{ t(`admin.packages.enterprise.rule${n}`) }}</li>
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
import packagesAdminAPI, { type EnterpriseSettings } from '@/api/admin/packages'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'

const { t } = useI18n()
const appStore = useAppStore()

const settings = ref<EnterpriseSettings | null>(null)
const busy = ref(false)

async function save() {
  const current = settings.value
  if (!current) return
  if (!(current.threshold > 0)) {
    appStore.showError(t('admin.packages.enterprise.invalidThreshold'))
    return
  }
  busy.value = true
  try {
    settings.value = await packagesAdminAPI.updateEnterpriseSettings({ enabled: current.enabled, threshold: current.threshold })
    appStore.showSuccess(t('admin.packages.common.saved'))
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('common.error')))
  } finally {
    busy.value = false
  }
}

onMounted(async () => {
  try {
    settings.value = await packagesAdminAPI.getEnterpriseSettings()
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('admin.packages.common.loadFailed')))
  }
})
</script>
