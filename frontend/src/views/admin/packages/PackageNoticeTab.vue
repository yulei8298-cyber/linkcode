<template>
  <div class="space-y-5">
    <div v-if="!settings" class="card p-6 text-center text-sm text-gray-500">{{ t('admin.packages.common.loading') }}</div>
    <section v-else class="card space-y-4 p-5">
      <div class="flex flex-wrap items-start justify-between gap-3">
        <div>
          <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.packages.notice.title') }}</h3>
          <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
            {{ t('admin.packages.notice.desc', { conc: '{并发}', cap: '{冻结上限}' }) }}
          </p>
          <p class="mt-1 text-xs font-semibold text-gray-700 dark:text-gray-300">{{ t('admin.packages.notice.version', { n: settings.notice_version }) }}</p>
        </div>
        <div class="flex gap-2">
          <button type="button" class="btn btn-secondary btn-sm" @click="previewOpen = true">{{ t('admin.packages.notice.preview') }}</button>
          <button type="button" class="btn btn-secondary btn-sm" :disabled="saving" @click="resetToDefault">{{ t('admin.packages.notice.reset') }}</button>
        </div>
      </div>
      <textarea v-model="settings.notice_text" class="input min-h-[320px] font-mono text-sm leading-relaxed" :aria-label="t('admin.packages.notice.title')"></textarea>
      <button type="button" class="btn btn-primary" :disabled="saving" @click="save">{{ t('admin.packages.common.save') }}</button>
    </section>

    <PackageNoticeDialog
      :show="previewOpen"
      :text="settings?.notice_text || ''"
      :concurrency="5"
      :max-freeze-days="settings?.max_freeze_days || 0"
      :purchase="null"
      @close="previewOpen = false"
    />
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import PackageNoticeDialog from '@/components/package/shop/PackageNoticeDialog.vue'
import packagesAdminAPI, { type PackageSettingsResponse } from '@/api/admin/packages'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'

const { t } = useI18n()
const appStore = useAppStore()

const settings = ref<PackageSettingsResponse | null>(null)
const saving = ref(false)
const previewOpen = ref(false)

async function save() {
  if (!settings.value) return
  saving.value = true
  try {
    const s = settings.value
    const saved = await packagesAdminAPI.updateSettings({
      freeze_enabled: s.freeze_enabled,
      max_freeze_days: s.max_freeze_days,
      holiday_sync_enabled: s.holiday_sync_enabled,
      holiday_source_url: s.holiday_source_url,
      notice_text: s.notice_text,
      notice_version: s.notice_version,
    })
    settings.value = { ...settings.value, ...saved }
    appStore.showSuccess(t('admin.packages.common.saved'))
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('common.error')))
  } finally {
    saving.value = false
  }
}

/** 提交空正文，由后端回填默认须知（版本号随之 +1）。 */
async function resetToDefault() {
  if (!settings.value) return
  settings.value.notice_text = ''
  await save()
}

onMounted(async () => {
  try {
    settings.value = await packagesAdminAPI.getSettings()
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('admin.packages.common.loadFailed')))
  }
})
</script>
