<template>
  <BaseDialog
    :show="show"
    :title="target ? t('admin.intelCheck.targets.edit') : t('admin.intelCheck.targets.create')"
    width="wide"
    @close="emit('close')"
  >
    <div class="grid gap-4 sm:grid-cols-2">
      <div>
        <label class="label">{{ t('admin.intelCheck.targets.form.name') }}</label>
        <input
          v-model="form.name"
          type="text"
          class="input"
          maxlength="100"
          :placeholder="t('admin.intelCheck.targets.form.namePlaceholder')"
        />
      </div>
      <div>
        <label class="label">{{ t('admin.intelCheck.targets.form.model') }}</label>
        <input
          v-model="form.model"
          type="text"
          class="input"
          maxlength="200"
          :placeholder="t('admin.intelCheck.targets.form.modelPlaceholder')"
        />
      </div>

      <div class="sm:col-span-2">
        <label class="label">{{ t('admin.intelCheck.targets.form.description') }}</label>
        <input
          v-model="form.description"
          type="text"
          class="input"
          maxlength="500"
          :placeholder="t('admin.intelCheck.targets.form.descriptionPlaceholder')"
        />
      </div>

      <div class="sm:col-span-2">
        <label class="label">{{ t('admin.intelCheck.targets.form.baseUrl') }}</label>
        <input
          v-model="form.base_url"
          type="text"
          class="input"
          maxlength="500"
          :placeholder="t('admin.intelCheck.targets.form.baseUrlPlaceholder')"
        />
        <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
          {{ t('admin.intelCheck.targets.form.baseUrlHint') }}
        </p>
      </div>

      <div class="sm:col-span-2">
        <label class="label">{{ t('admin.intelCheck.targets.form.apiKey') }}</label>
        <input
          v-model="form.api_key"
          type="password"
          class="input"
          autocomplete="new-password"
          maxlength="2000"
          :placeholder="
            target
              ? t('admin.intelCheck.targets.form.apiKeyPlaceholderEdit')
              : t('admin.intelCheck.targets.form.apiKeyPlaceholderCreate')
          "
        />
      </div>

      <div>
        <label class="label">{{ t('admin.intelCheck.targets.form.apiMode') }}</label>
        <select v-model="form.api_mode" class="input">
          <option value="responses">{{ t('admin.intelCheck.apiMode.responses') }}</option>
          <option value="chat_completions">
            {{ t('admin.intelCheck.apiMode.chat_completions') }}
          </option>
        </select>
        <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
          {{ t('admin.intelCheck.targets.form.apiModeHint') }}
        </p>
      </div>

      <div>
        <label class="label">{{ t('admin.intelCheck.targets.form.reasoningEffort') }}</label>
        <select v-model="form.reasoning_effort" class="input">
          <option v-for="effort in efforts" :key="effort" :value="effort">
            {{ t(`admin.intelCheck.effort.${effort}`) }}
          </option>
        </select>
        <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
          {{ t('admin.intelCheck.targets.form.reasoningEffortHint') }}
        </p>
      </div>

      <div>
        <label class="label">{{ t('admin.intelCheck.targets.form.rateLabel') }}</label>
        <input
          v-model="form.rate_label"
          type="text"
          class="input"
          maxlength="20"
          :placeholder="t('admin.intelCheck.targets.form.rateLabelPlaceholder')"
        />
        <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
          {{ t('admin.intelCheck.targets.form.rateLabelHint') }}
        </p>
      </div>

      <NumberField
        v-model="form.sort_order"
        :label="t('admin.intelCheck.targets.form.sortOrder')"
        :hint="t('admin.intelCheck.targets.form.sortOrderHint')"
        :min="0"
      />

      <div class="flex items-center gap-3 sm:col-span-2">
        <Toggle v-model="form.enabled" />
        <span class="text-sm text-gray-700 dark:text-gray-300">
          {{ t('admin.intelCheck.targets.form.enabled') }}
        </span>
      </div>
    </div>

    <template #footer>
      <div class="flex justify-end gap-2">
        <button type="button" class="btn btn-secondary" @click="emit('close')">
          {{ t('common.cancel') }}
        </button>
        <button type="button" class="btn btn-primary" :disabled="saving" @click="save">
          {{ saving ? t('common.saving') : t('common.save') }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import { adminAPI } from '@/api/admin'
import type {
  IntelCheckAPIMode,
  IntelCheckEffort,
  IntelCheckTarget,
  IntelCheckTargetParams,
} from '@/api/admin/intelCheck'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Toggle from '@/components/common/Toggle.vue'
import NumberField from './NumberField.vue'

const props = defineProps<{
  show: boolean
  target: IntelCheckTarget | null
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'saved'): void
}>()

const { t } = useI18n()
const appStore = useAppStore()
const efforts: IntelCheckEffort[] = ['low', 'medium', 'high', 'xhigh']
const saving = ref(false)

interface TargetForm {
  name: string
  description: string
  base_url: string
  api_key: string
  api_mode: IntelCheckAPIMode
  model: string
  reasoning_effort: IntelCheckEffort
  rate_label: string
  sort_order: number
  enabled: boolean
}

function emptyForm(): TargetForm {
  return {
    name: '',
    description: '',
    base_url: '',
    api_key: '',
    api_mode: 'responses',
    model: '',
    // 默认 high：本功能的前提是让模型尽可能发挥，默认值不该低于此。
    reasoning_effort: 'high',
    rate_label: '',
    sort_order: 0,
    enabled: true,
  }
}

const form = reactive<TargetForm>(emptyForm())

watch(
  () => [props.show, props.target] as const,
  ([show, target]) => {
    if (!show) return
    Object.assign(form, emptyForm())
    if (!target) return
    Object.assign(form, {
      name: target.name,
      description: target.description,
      base_url: target.base_url,
      // 凭据永远留空：管理端拿到的是掩码，回填它会把 "sk-1***" 当成新 key 写进库。
      api_key: '',
      api_mode: target.api_mode,
      model: target.model,
      reasoning_effort: (target.reasoning_effort || 'high') as IntelCheckEffort,
      rate_label: target.rate_label,
      sort_order: target.sort_order,
      enabled: target.enabled,
    })
  },
  { immediate: true },
)

async function save() {
  if (saving.value) return
  saving.value = true
  try {
    const payload: IntelCheckTargetParams = {
      name: form.name.trim(),
      description: form.description.trim(),
      base_url: form.base_url.trim(),
      api_mode: form.api_mode,
      model: form.model.trim(),
      reasoning_effort: form.reasoning_effort,
      rate_label: form.rate_label.trim(),
      sort_order: form.sort_order,
      enabled: form.enabled,
    }
    // 只在真填了内容时带上 api_key：编辑场景下空串的含义是「保留原凭据」，
    // 而 binding 标签是 omitempty，发空串与不发等价，这里显式省略更清楚。
    const key = form.api_key.trim()
    if (key) payload.api_key = key

    if (props.target) await adminAPI.intelCheck.updateTarget(props.target.id, payload)
    else await adminAPI.intelCheck.createTarget(payload)

    appStore.showSuccess(t('admin.intelCheck.common.saveSuccess'))
    emit('saved')
  } catch (err: unknown) {
    // 校验失败的具体原因（地址格式、名称过长等）由后端给出，原样透出给管理员。
    appStore.showError(extractApiErrorMessage(err, t('admin.intelCheck.common.saveError')))
  } finally {
    saving.value = false
  }
}
</script>
