<template>
  <div class="space-y-5">
    <section class="card space-y-4 p-5">
      <div class="flex flex-wrap items-start justify-between gap-3">
        <div>
          <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.packages.plans.title') }}</h3>
          <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.packages.plans.desc') }}</p>
        </div>
        <select v-model.number="groupId" class="input w-64" :aria-label="t('admin.packages.plans.group')">
          <option :value="0" disabled>{{ t('admin.packages.plans.groupPlaceholder') }}</option>
          <option v-for="g in groups" :key="g.id" :value="g.id">{{ g.name }} ×{{ g.rate_multiplier }}</option>
        </select>
      </div>

      <div class="grid gap-2 sm:grid-cols-2 xl:grid-cols-4" :aria-label="t('admin.packages.plans.styleMap')">
        <div v-for="combo in COMBOS" :key="combo.key" :class="['style-chip pkg-hue', `pkg-h-${packagePlanStyle(combo.cycle, combo.tier).hue}`]">
          <i></i>{{ t(`admin.packages.plans.cycle${combo.cycle === 'week' ? 'Week' : 'Month'}`) }} · {{ combo.tier }}x
          <small>{{ t(`admin.packages.plans.style.${combo.key}`) }}</small>
        </div>
      </div>

      <div class="flex flex-wrap items-center gap-2 text-xs text-gray-500 dark:text-gray-400">
        {{ t('admin.packages.plans.presets') }}
        <button v-for="pair in NAME_PRESETS" :key="pair[0]" type="button" class="btn btn-secondary btn-sm" :disabled="!plans.length || busy" @click="applyPreset(pair)">
          {{ pair[0] }} / {{ pair[1] }}
        </button>
      </div>

      <p v-if="!groupId" class="text-sm text-gray-500">{{ t('admin.packages.plans.noGroup') }}</p>
      <p v-else-if="!plans.length" class="text-sm text-gray-500">{{ t('admin.packages.plans.empty') }}</p>
      <div v-else class="grid gap-3 md:grid-cols-2">
        <div v-for="plan in plans" :key="plan.id" :class="['plan-row pkg-hue', `pkg-h-${packagePlanStyle(plan.cycle, plan.tier).hue}`, { off: !plan.for_sale }]">
          <div class="flex items-center gap-2">
            <b class="text-gray-900 dark:text-white">{{ plan.name }}</b>
            <span class="pkg-badge">{{ t(`packages.badge.${packagePlanStyle(plan.cycle, plan.tier).badge}`) }}</span>
          </div>
          <div class="mt-1 text-xs text-gray-500 dark:text-gray-400">
            {{ t(`packages.cycle.${plan.cycle}`) }} · {{ plan.tier }}x · <span class="pkg-mono">{{ formatCNY(plan.price) }} · {{ formatUSD(plan.quota_usd) }}</span>
          </div>
          <div class="mt-3 flex flex-wrap gap-2">
            <button type="button" class="btn btn-secondary btn-sm" :disabled="busy" @click="toggleSale(plan)">
              {{ plan.for_sale ? t('admin.packages.plans.onSale') : t('admin.packages.plans.offSale') }}
            </button>
            <button type="button" class="btn btn-secondary btn-sm" @click="editPlan(plan)">{{ t('common.edit') }}</button>
            <button type="button" class="btn btn-danger btn-sm" :disabled="busy" @click="pendingDelete = plan">{{ t('admin.packages.common.delete') }}</button>
          </div>
        </div>
      </div>
    </section>

    <section v-if="groupId" class="card space-y-4 p-5">
      <div class="flex flex-wrap items-center justify-between gap-2">
        <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.packages.plans.addTitle') }}</h3>
        <div :class="['flex items-center gap-2 text-xs text-gray-500 pkg-hue', `pkg-h-${formStyle.hue}`]">
          {{ t('admin.packages.plans.autoStyle') }}
          <span class="pkg-badge">{{ t(`packages.badge.${formStyle.badge}`) }}</span>
          <span v-if="formStyle.ribbon" class="pkg-badge">{{ t('packages.badge.new') }}</span>
        </div>
      </div>
      <div class="grid gap-4 md:grid-cols-2 xl:grid-cols-5">
        <label class="block">
          <span class="input-label">{{ t('admin.packages.plans.cycle') }}</span>
          <select v-model="form.cycle" class="input">
            <option value="week">{{ t('admin.packages.plans.cycleWeek') }}</option>
            <option value="month">{{ t('admin.packages.plans.cycleMonth') }}</option>
          </select>
        </label>
        <label class="block">
          <span class="input-label">{{ t('admin.packages.plans.tier') }}</span>
          <select v-model.number="form.tier" class="input">
            <option :value="1">{{ t('admin.packages.plans.tier1') }}</option>
            <option :value="2">{{ t('admin.packages.plans.tier2') }}</option>
          </select>
        </label>
        <label class="block">
          <span class="input-label">{{ t('admin.packages.plans.name') }}</span>
          <input v-model.trim="form.name" class="input" maxlength="50" />
        </label>
        <label class="block">
          <span class="input-label">{{ t('admin.packages.plans.price') }}</span>
          <input v-model.number="form.price" class="input" type="number" min="0" step="0.01" />
        </label>
        <label class="block">
          <span class="input-label">{{ t('admin.packages.plans.quota') }}</span>
          <input v-model.number="form.quota" class="input" type="number" min="0" step="1" :readonly="form.tier === 2" />
        </label>
      </div>
      <p class="text-xs text-gray-500 dark:text-gray-400">
        {{ existing ? t('admin.packages.plans.overwriteHint', { name: existing.name }) : t('admin.packages.plans.newHint') }}
        {{ form.tier === 2 ? (basePlan ? t('admin.packages.plans.quotaLocked') : t('admin.packages.plans.needBase')) : t('admin.packages.plans.quotaSync') }}
      </p>
      <button type="button" class="btn btn-primary" :disabled="!canSave || busy" @click="save">{{ t('admin.packages.common.save') }}</button>
    </section>

    <ConfirmDialog
      :show="!!pendingDelete"
      :title="t('admin.packages.common.delete')"
      :message="pendingDelete ? t('admin.packages.plans.deleteConfirm', { name: pendingDelete.name }) : ''"
      danger
      @confirm="confirmDelete"
      @cancel="pendingDelete = null"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import { adminAPI } from '@/api/admin'
import packagesAdminAPI from '@/api/admin/packages'
import type { PackageCycle, PackagePlan, PackageTier } from '@/api/packages'
import type { AdminGroup } from '@/types'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import { formatCNY, formatUSD, packagePlanStyle } from '@/components/package/packageUtils'

const COMBOS: { key: string; cycle: PackageCycle; tier: PackageTier }[] = [
  { key: 'week1', cycle: 'week', tier: 1 },
  { key: 'week2', cycle: 'week', tier: 2 },
  { key: 'month1', cycle: 'month', tier: 1 },
  { key: 'month2', cycle: 'month', tier: 2 },
]
/** 名称预设：1x / 2x 两档的前缀，后面拼「周卡 / 月卡」。 */
const NAME_PRESETS: [string, string][] = [['摸鱼', '爆肝'], ['单核', '双核'], ['巡航', '超频']]
const PRESET_SUFFIX: Record<PackageCycle, string> = { week: '周卡', month: '月卡' }

const { t } = useI18n()
const appStore = useAppStore()

const groups = ref<AdminGroup[]>([])
const groupId = ref(0)
const plans = ref<PackagePlan[]>([])
const busy = ref(false)
const pendingDelete = ref<PackagePlan | null>(null)
const form = reactive({ cycle: 'week' as PackageCycle, tier: 1 as PackageTier, name: '', price: 0, quota: 0 })

const formStyle = computed(() => packagePlanStyle(form.cycle, form.tier))
const existing = computed(() => plans.value.find((p) => p.cycle === form.cycle && p.tier === form.tier) ?? null)
const basePlan = computed(() => plans.value.find((p) => p.cycle === form.cycle && p.tier === 1) ?? null)
const canSave = computed(
  () => !!groupId.value && !!form.name && form.price > 0 && (form.tier === 2 ? !!basePlan.value : form.quota > 0),
)

/** 切换周期 / 档位时：已有套餐则带出其内容，2x 额度始终按 1x 两倍展示。 */
function fillForm() {
  form.name = existing.value?.name ?? ''
  form.price = existing.value?.price ?? 0
  form.quota = form.tier === 2 ? (basePlan.value?.quota_usd ?? 0) * 2 : (existing.value?.quota_usd ?? 0)
}

function editPlan(plan: PackagePlan) {
  form.cycle = plan.cycle
  form.tier = plan.tier
  fillForm()
}

async function loadPlans() {
  plans.value = groupId.value ? await packagesAdminAPI.listPlans(groupId.value) : []
  fillForm()
}

async function run(action: () => Promise<unknown>, success: string) {
  busy.value = true
  try {
    await action()
    appStore.showSuccess(success)
    await loadPlans()
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('common.error')))
  } finally {
    busy.value = false
  }
}

function planInput(plan: PackagePlan, patch: Partial<{ name: string; for_sale: boolean }>) {
  return { id: plan.id, group_id: plan.group_id, cycle: plan.cycle, tier: plan.tier, name: plan.name, price: plan.price, quota_usd: plan.quota_usd, for_sale: plan.for_sale, ...patch }
}

const save = () =>
  run(
    () => packagesAdminAPI.savePlan({
      id: existing.value?.id, group_id: groupId.value, cycle: form.cycle, tier: form.tier,
      name: form.name, price: form.price, quota_usd: form.quota, for_sale: existing.value?.for_sale ?? true,
    }),
    t('admin.packages.common.saved'),
  )

const toggleSale = (plan: PackagePlan) =>
  run(() => packagesAdminAPI.savePlan(planInput(plan, { for_sale: !plan.for_sale })), t('admin.packages.plans.toggled'))

/** 1x 先保存，2x 的额度依赖它。 */
const applyPreset = (pair: [string, string]) =>
  run(async () => {
    for (const plan of [...plans.value].sort((a, b) => a.tier - b.tier)) {
      await packagesAdminAPI.savePlan(planInput(plan, { name: `${pair[plan.tier - 1]}${PRESET_SUFFIX[plan.cycle]}` }))
    }
  }, t('admin.packages.plans.presetApplied', { pair: pair.join(' / ') }))

function confirmDelete() {
  const plan = pendingDelete.value
  pendingDelete.value = null
  if (plan) run(() => packagesAdminAPI.deletePlan(plan.id), t('admin.packages.plans.deleted'))
}

watch(groupId, () => {
  loadPlans().catch((err: unknown) => appStore.showError(extractApiErrorMessage(err, t('admin.packages.common.loadFailed'))))
})
watch(() => [form.cycle, form.tier], fillForm)

onMounted(async () => {
  try {
    const all = await adminAPI.groups.getAll()
    // 套餐只能绑定启用中的普通（余额）分组
    groups.value = all.filter((g) => g.status === 'active' && g.subscription_type === 'standard' && !g.is_free)
    groupId.value = groups.value[0]?.id ?? 0
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('admin.packages.common.loadFailed')))
  }
})
</script>

<style scoped>
.style-chip { display: flex; flex-wrap: wrap; align-items: center; gap: 8px; padding: 10px 12px; border-radius: 12px; border: 1px solid var(--pkg-h-line); background: var(--pkg-h-soft); font-size: 13px; font-weight: 600; color: var(--lc-ink); }
.style-chip i { width: 12px; height: 12px; border-radius: 4px; background: var(--pkg-h); }
.style-chip small { font-weight: 500; color: var(--lc-ink-3); }
.plan-row { padding: 14px; border-radius: 14px; border: 1px solid var(--pkg-h-line); background: linear-gradient(120deg, var(--pkg-h-soft), transparent 70%), var(--lc-surface); }
.plan-row.off { opacity: 0.55; }
</style>
