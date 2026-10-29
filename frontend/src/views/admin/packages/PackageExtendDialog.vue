<template>
  <BaseDialog :show="!!item" :title="t('admin.packages.userPackages.extend.title')" width="narrow" @close="emit('close')">
    <form v-if="item" id="package-extend-form" class="space-y-5" @submit.prevent="submit">
      <div class="space-y-1 rounded-lg bg-gray-50 p-4 text-sm text-gray-600 dark:bg-dark-700 dark:text-gray-400">
        <p class="font-medium text-gray-900 dark:text-white">
          {{ t('admin.packages.userPackages.extend.target', { user: item.user_email || `#${item.user_id}`, name: item.name }) }}
        </p>
        <p>
          {{ t('admin.packages.userPackages.extend.currentExpires') }}:
          <span class="font-medium text-gray-900 dark:text-white">{{ formatDateTimeToMinute(item.expires_at) }}</span>
        </p>
        <p v-if="valid" data-test="extend-preview">
          {{ t('admin.packages.userPackages.extend.newExpires') }}:
          <span class="font-medium text-primary-600 dark:text-primary-400">{{ formatDateTimeToMinute(newExpires) }}</span>
        </p>
        <p v-if="item.status === 'frozen'" class="text-xs text-amber-600 dark:text-amber-400">{{ t('admin.packages.userPackages.extend.frozenNote') }}</p>
      </div>

      <div>
        <label class="input-label" for="package-extend-days">{{ t('admin.packages.userPackages.extend.days') }}</label>
        <div class="mb-2 flex flex-wrap gap-2">
          <button
            v-for="n in PRESET_DAYS"
            :key="n"
            type="button"
            :class="['btn btn-sm', days === n ? 'btn-primary' : 'btn-secondary']"
            data-test="extend-preset"
            @click="days = n"
          >
            {{ t('admin.packages.userPackages.extend.presetDays', { n }) }}
          </button>
        </div>
        <input
          id="package-extend-days"
          v-model.number="days"
          type="number"
          min="1"
          :max="MAX_EXTEND_DAYS"
          step="1"
          class="input text-center"
          data-test="extend-days"
        />
        <p :class="['input-hint', { 'text-red-500': !valid }]">
          {{ valid ? t('admin.packages.userPackages.extend.hint') : t('admin.packages.userPackages.extend.invalid') }}
        </p>
      </div>
    </form>

    <template #footer>
      <div class="flex justify-end gap-3">
        <button type="button" class="btn btn-secondary" @click="emit('close')">{{ t('common.cancel') }}</button>
        <button type="submit" form="package-extend-form" class="btn btn-primary" :disabled="busy || !valid" data-test="extend-confirm">
          {{ t('admin.packages.userPackages.extend.confirm') }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import type { AdminUserPackage } from '@/api/admin/packages'
import { formatDateTimeToMinute } from '@/utils/format'

/** 与后端 MaxPackageExtendDays 一致。 */
const MAX_EXTEND_DAYS = 365
const PRESET_DAYS = [1, 3, 7, 15, 30] as const
const DEFAULT_DAYS = 7
const DAY_MS = 86_400_000

const props = defineProps<{ item: AdminUserPackage | null; busy: boolean }>()
const emit = defineEmits<{ close: []; confirm: [item: AdminUserPackage, days: number] }>()

const { t } = useI18n()
const days = ref<number>(DEFAULT_DAYS)

// 每次打开另一张套餐都从默认天数开始，避免沿用上一次的输入。
watch(() => props.item?.id, () => { days.value = DEFAULT_DAYS })

const valid = computed(() => Number.isInteger(days.value) && days.value >= 1 && days.value <= MAX_EXTEND_DAYS)

/** 预览用：按自然日顺延，与后端 AddDate 在非夏令时切换日一致。 */
const newExpires = computed(() => {
  if (!props.item) return ''
  return new Date(new Date(props.item.expires_at).getTime() + days.value * DAY_MS).toISOString()
})

function submit() {
  if (props.item && valid.value && !props.busy) emit('confirm', props.item, days.value)
}
</script>
