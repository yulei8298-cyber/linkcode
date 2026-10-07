<template>
  <div data-test="user-enterprise">
    <label class="input-label">{{ t('admin.packages.enterprise.userLabel') }}</label>
    <select v-model="mode" class="input" :disabled="!status || saving" data-test="enterprise-mode" @change="change">
      <option value="auto">{{ t('admin.packages.enterprise.modeAuto') }}</option>
      <option value="on">{{ t('admin.packages.enterprise.modeOn') }}</option>
      <option value="off">{{ t('admin.packages.enterprise.modeOff') }}</option>
    </select>
    <p v-if="status" class="input-hint" data-test="enterprise-hint">
      {{ t('admin.packages.enterprise.userHint', { total: status.total.toFixed(2), threshold: status.threshold }) }}
      <strong :class="status.enterprise ? 'text-emerald-600 dark:text-emerald-400' : ''">
        {{ status.enterprise ? t('admin.packages.enterprise.nowOn') : t('admin.packages.enterprise.nowOff') }}
      </strong>
      <template v-if="!status.enabled">{{ t('admin.packages.enterprise.globalOff') }}</template>
    </p>
  </div>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import packagesAdminAPI from '@/api/admin/packages'
import type { EnterpriseMode, EnterpriseStatus } from '@/api/enterprise'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'

// 编辑用户时手动开通 / 关闭企业尊享。选择后立即保存，不跟随「保存」按钮。
const props = defineProps<{ userId: number | null }>()
const { t } = useI18n()
const appStore = useAppStore()

const status = ref<EnterpriseStatus | null>(null)
const mode = ref<EnterpriseMode>('auto')
const saving = ref(false)

async function load(userId: number | null) {
  status.value = null
  if (!userId) return
  try {
    const loaded = await packagesAdminAPI.getUserEnterprise(userId)
    if (userId !== props.userId) return // 已切换到别的用户，丢弃过期结果
    status.value = loaded
    mode.value = loaded.mode
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('admin.packages.common.loadFailed')))
  }
}

async function change() {
  if (!props.userId) return
  saving.value = true
  try {
    status.value = await packagesAdminAPI.setUserEnterprise(props.userId, mode.value)
    appStore.showSuccess(t('admin.packages.common.saved'))
  } catch (err: unknown) {
    mode.value = status.value?.mode ?? 'auto'
    appStore.showError(extractApiErrorMessage(err, t('common.error')))
  } finally {
    saving.value = false
  }
}

watch(() => props.userId, load, { immediate: true })
</script>
