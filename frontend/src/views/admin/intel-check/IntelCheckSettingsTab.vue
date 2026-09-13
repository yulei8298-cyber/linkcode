<template>
  <div class="space-y-5">
    <div v-if="loading" class="card p-6 text-center text-sm text-gray-500">
      {{ t('common.loading') }}
    </div>

    <template v-else-if="form">
      <!-- 总开关 -->
      <section class="card space-y-3 p-5">
        <div class="flex items-start justify-between gap-4">
          <div>
            <h3 class="text-sm font-semibold text-gray-900 dark:text-white">
              {{ t('admin.intelCheck.settings.enabled') }}
            </h3>
            <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
              {{ t('admin.intelCheck.settings.enabledHint') }}
            </p>
          </div>
          <Toggle v-model="form.enabled" />
        </div>
        <!-- 本地先给出提示，而不是等提交后被后端退回：后端的
             ValidateIntelCheckSettings 会拒绝「开启但未配评审模型」，
             在这里提前说明能省掉一次失败往返。 -->
        <p v-if="form.enabled && !judgeConfigured" class="text-xs text-amber-600 dark:text-amber-400">
          {{ t('admin.intelCheck.settings.requireJudge') }}
        </p>
      </section>

      <!-- 调度 -->
      <section class="card space-y-4 p-5">
        <h3 class="text-sm font-semibold text-gray-900 dark:text-white">
          {{ t('admin.intelCheck.settings.schedule') }}
        </h3>
        <div class="grid gap-4 sm:grid-cols-2">
          <NumberField
            v-model="form.interval_minutes"
            :label="t('admin.intelCheck.settings.intervalMinutes')"
            :hint="t('admin.intelCheck.settings.intervalHint')"
            :min="5"
            :max="1440"
          />
          <NumberField
            v-model="form.request_timeout_seconds"
            :label="t('admin.intelCheck.settings.requestTimeoutSeconds')"
            :hint="t('admin.intelCheck.settings.requestTimeoutHint')"
            :min="30"
            :max="900"
          />
          <NumberField
            v-model="form.concurrency"
            :label="t('admin.intelCheck.settings.concurrency')"
            :hint="t('admin.intelCheck.settings.concurrencyHint')"
            :min="1"
            :max="32"
          />
          <NumberField
            v-model="form.retention_days"
            :label="t('admin.intelCheck.settings.retentionDays')"
            :min="1"
            :max="365"
          />
        </div>
      </section>

      <!-- 源码评审 -->
      <section class="card space-y-4 p-5">
        <h3 class="text-sm font-semibold text-gray-900 dark:text-white">
          {{ t('admin.intelCheck.settings.judge') }}
        </h3>
        <div class="grid gap-4 sm:grid-cols-2">
          <div>
            <label class="label">{{ t('admin.intelCheck.settings.judgeTarget') }}</label>
            <select v-model.number="form.drawing_judge.target_id" class="input">
              <option :value="0">{{ t('admin.intelCheck.common.none') }}</option>
              <option v-for="target in targets" :key="target.id" :value="target.id">
                {{ target.name }}
              </option>
            </select>
            <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
              {{ t('admin.intelCheck.settings.judgeTargetHint') }}
            </p>
          </div>
          <div>
            <label class="label">{{ t('admin.intelCheck.settings.judgeModel') }}</label>
            <input v-model="form.drawing_judge.model" type="text" class="input" />
          </div>
          <div>
            <label class="label">{{ t('admin.intelCheck.settings.judgeEffort') }}</label>
            <select v-model="form.drawing_judge.reasoning_effort" class="input">
              <option v-for="effort in efforts" :key="effort" :value="effort">
                {{ t(`admin.intelCheck.effort.${effort}`) }}
              </option>
            </select>
          </div>
          <NumberField
            v-model="form.drawing_judge.pass_score"
            :label="t('admin.intelCheck.settings.passScore')"
            :hint="t('admin.intelCheck.settings.passScoreHint')"
            :min="1"
            :max="100"
          />
        </div>
      </section>

      <!-- 降智判定 -->
      <section class="card space-y-4 p-5">
        <h3 class="text-sm font-semibold text-gray-900 dark:text-white">
          {{ t('admin.intelCheck.settings.degraded') }}
        </h3>
        <p class="text-xs text-gray-500 dark:text-gray-400">
          {{ t('admin.intelCheck.settings.degradedHint') }}
        </p>
        <div class="grid gap-4 sm:grid-cols-2">
          <NumberField
            v-model="form.degraded_rule.fail_streak"
            :label="t('admin.intelCheck.settings.failStreak')"
            :min="1"
            :max="20"
          />
          <NumberField
            v-model="form.degraded_rule.recover_streak"
            :label="t('admin.intelCheck.settings.recoverStreak')"
            :min="1"
            :max="20"
          />
        </div>
      </section>

      <!-- 公开页展示 -->
      <section class="card space-y-4 p-5">
        <h3 class="text-sm font-semibold text-gray-900 dark:text-white">
          {{ t('admin.intelCheck.settings.display') }}
        </h3>
        <div class="grid gap-4 sm:grid-cols-2">
          <NumberField
            v-model="form.timeline_points"
            :label="t('admin.intelCheck.settings.timelinePoints')"
            :hint="t('admin.intelCheck.settings.timelinePointsHint')"
            :min="12"
            :max="200"
          />
          <div>
            <label class="label">{{ t('admin.intelCheck.settings.introTitle') }}</label>
            <input v-model="form.intro_title" type="text" class="input" maxlength="100" />
          </div>
        </div>
        <div>
          <label class="label">{{ t('admin.intelCheck.settings.introText') }}</label>
          <textarea v-model="form.intro_text" rows="4" class="input" maxlength="2000"></textarea>
        </div>
      </section>

      <div class="flex justify-end">
        <button type="button" class="btn btn-primary" :disabled="saving" @click="save">
          {{ saving ? t('common.saving') : t('common.save') }}
        </button>
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import { adminAPI } from '@/api/admin'
import type { IntelCheckSettings, IntelCheckTarget } from '@/api/admin/intelCheck'
import Toggle from '@/components/common/Toggle.vue'
import NumberField from './NumberField.vue'

const { t } = useI18n()
const appStore = useAppStore()

const efforts = ['low', 'medium', 'high', 'xhigh'] as const

const form = ref<IntelCheckSettings | null>(null)
const targets = ref<IntelCheckTarget[]>([])
const loading = ref(false)
const saving = ref(false)

const judgeConfigured = computed(
  () => (form.value?.drawing_judge.target_id ?? 0) > 0 && !!form.value?.drawing_judge.model?.trim(),
)

async function load() {
  loading.value = true
  try {
    // 并行：评审分组下拉需要分组列表，但它失败不该挡住设置本体的渲染。
    const [settings, targetList] = await Promise.all([
      adminAPI.intelCheck.getSettings(),
      adminAPI.intelCheck.listTargets({ page: 1, page_size: 100 }).catch(() => null),
    ])
    form.value = settings
    targets.value = targetList?.items ?? []
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('admin.intelCheck.common.loadError')))
  } finally {
    loading.value = false
  }
}

async function save() {
  if (!form.value || saving.value) return
  saving.value = true
  try {
    // 整块提交：后端是覆盖语义，缺字段会被 Normalize 补成默认值。
    form.value = await adminAPI.intelCheck.updateSettings(form.value)
    appStore.showSuccess(t('admin.intelCheck.common.saveSuccess'))
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('admin.intelCheck.common.saveError')))
  } finally {
    saving.value = false
  }
}

onMounted(() => void load())
</script>
