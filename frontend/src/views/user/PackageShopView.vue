<template>
  <AppLayout>
    <div class="shop">
      <div class="pkg-amber-banner">
        <Icon name="infoCircle" size="sm" class="mt-0.5 shrink-0" />
        <div>
          {{ t('packages.shop.banner') }}
          <button type="button" class="shop-link" @click="openNotice(null)">{{ t('packages.shop.noticeLink') }}</button>
        </div>
      </div>

      <div v-if="loading" class="flex justify-center py-16">
        <div class="h-8 w-8 animate-spin rounded-full border-2 border-primary-500 border-t-transparent"></div>
      </div>

      <div v-else-if="!shop || shop.groups.length === 0" class="card p-12 text-center text-gray-500 dark:text-dark-400">
        {{ loadError || t('packages.shop.empty') }}
      </div>

      <template v-else>
        <div class="shop-groups" role="group" :aria-label="t('packages.shop.group')">
          <button
            v-for="group in shop.groups"
            :key="group.group_id"
            type="button"
            :class="['shop-group', { active: group.group_id === activeGroupId }]"
            :aria-pressed="group.group_id === activeGroupId"
            @click="activeGroupId = group.group_id"
          >
            {{ group.group_name }}<small class="pkg-mono">×{{ group.rate_multiplier }}</small>
          </button>
        </div>

        <div v-if="activeGroup" class="shop-plans">
          <PackagePlanCard
            v-for="plan in sortedPlans"
            :key="plan.id"
            :plan="plan"
            :group-name="activeGroup.group_name"
            :rate-multiplier="activeGroup.rate_multiplier"
            :freeze-enabled="shop.freeze_enabled"
            :concurrency="shop.package_concurrency"
            @buy="openNotice"
            @notice="openNotice(null)"
          />
        </div>

        <div class="shop-tiles">
          <div class="shop-tile pkg-hue pkg-h-violet">
            <h3><span class="pkg-icon-box shop-tile-icon"><Icon name="cube" size="sm" /></span>{{ t('packages.shop.tiles.stackTitle') }}</h3>
            <p>{{ t('packages.shop.tiles.stackDesc') }}</p>
          </div>
          <div class="shop-tile pkg-hue pkg-h-orange">
            <h3><span class="pkg-icon-box shop-tile-icon"><Icon name="arrowsUpDown" size="sm" /></span>{{ t('packages.shop.tiles.orderTitle') }}</h3>
            <div class="shop-flow">
              <i><em>1</em>{{ t('packages.shop.tiles.order1') }}</i>
              <i><em>2</em>{{ t('packages.shop.tiles.order2') }}</i>
              <i><em>3</em>{{ t('packages.shop.tiles.order3') }}</i>
            </div>
            <p>{{ t('packages.shop.tiles.orderDesc') }}</p>
          </div>
          <div class="shop-tile pkg-hue pkg-h-ice">
            <h3><span class="pkg-icon-box shop-tile-icon"><Icon name="calendar" size="sm" /></span>{{ t('packages.shop.tiles.freezeTitle') }}</h3>
            <p>{{ shop.freeze_enabled ? t('packages.shop.tiles.freezeDesc', { week: shop.max_freeze_days_week, month: shop.max_freeze_days_month }) : t('packages.shop.tiles.freezeOff') }}</p>
          </div>
        </div>
      </template>
    </div>

    <PackageNoticeDialog
      :show="noticeOpen"
      :text="shop?.notice.text || ''"
      :concurrency="shop?.package_concurrency || 0"
      :max-freeze-days-week="shop?.max_freeze_days_week || 0"
      :max-freeze-days-month="shop?.max_freeze_days_month || 0"
      :purchase="purchase"
      @close="noticeOpen = false"
      @confirm="goPay"
    />
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import PackagePlanCard from '@/components/package/shop/PackagePlanCard.vue'
import PackageNoticeDialog from '@/components/package/shop/PackageNoticeDialog.vue'
import { packagePlanRank, type PackagePurchaseContext } from '@/components/package/packageUtils'
import packagesAPI, { type PackagePlan, type PackageShop } from '@/api/packages'
import { extractApiErrorMessage } from '@/utils/apiError'
import '@/components/package/package.css'

const { t } = useI18n()
const router = useRouter()

const loading = ref(true)
const loadError = ref('')
const shop = ref<PackageShop | null>(null)
const activeGroupId = ref<number | null>(null)
const noticeOpen = ref(false)
const purchase = ref<PackagePurchaseContext | null>(null)

const activeGroup = computed(() => shop.value?.groups.find((g) => g.group_id === activeGroupId.value) ?? null)
const sortedPlans = computed(() =>
  [...(activeGroup.value?.plans ?? [])].sort((a, b) => packagePlanRank(a) - packagePlanRank(b)),
)

function openNotice(plan: PackagePlan | null) {
  purchase.value = plan && activeGroup.value ? { plan, groupName: activeGroup.value.group_name } : null
  noticeOpen.value = true
}

/** 读完须知并勾选后进入支付页，由支付页沿用现有支付方式与回跳流程完成下单。 */
function goPay(plan: PackagePlan) {
  noticeOpen.value = false
  router.push({
    path: '/purchase',
    query: { package_plan: String(plan.id), notice_version: String(shop.value?.notice.version ?? 0) },
  })
}

onMounted(async () => {
  try {
    shop.value = await packagesAPI.getShop()
    activeGroupId.value = shop.value.groups[0]?.group_id ?? null
  } catch (err: unknown) {
    loadError.value = extractApiErrorMessage(err, t('packages.shop.loadFailed'))
  } finally {
    loading.value = false
  }
})
</script>

<style scoped>
.shop { display: grid; gap: 20px; }
.shop-link { font-weight: 600; text-decoration: underline; text-underline-offset: 3px; }
.shop-groups { display: flex; flex-wrap: wrap; gap: 10px; }
.shop-group {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  height: 44px;
  padding: 0 16px;
  border-radius: 13px;
  border: 1px solid var(--lc-line);
  background: var(--lc-surface);
  font-weight: 600;
  color: var(--lc-ink);
}
.shop-group small { font-size: 12px; color: var(--lc-ink-3); }
.shop-group.active { border-color: transparent; background: linear-gradient(135deg, #ff8a4c, #ff4f6d); color: #fff; box-shadow: 0 12px 28px -14px #ff5f4c; }
.shop-group.active small { color: rgba(255, 255, 255, 0.85); }
.shop-plans { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 18px; }
.shop-tiles { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 14px; }
.shop-tile {
  display: grid;
  align-content: start;
  gap: 8px;
  padding: 18px;
  border-radius: 16px;
  border: 1px solid var(--pkg-h-line);
  background: linear-gradient(160deg, var(--pkg-h-soft), transparent 70%), var(--lc-surface);
}
.shop-tile h3 { display: flex; align-items: center; gap: 8px; margin: 0; font-size: 15px; font-weight: 700; color: var(--lc-ink); }
.shop-tile p { margin: 0; font-size: 14px; color: var(--lc-ink-2); }
.shop-tile-icon { width: 32px; height: 32px; border-radius: 10px; }
.shop-flow { display: flex; flex-wrap: wrap; gap: 6px; font-size: 13px; font-weight: 600; color: var(--lc-ink); }
.shop-flow i { display: inline-flex; align-items: center; gap: 5px; padding: 3px 9px; border-radius: 999px; font-style: normal; border: 1px solid var(--lc-line); background: var(--lc-surface-2); }
.shop-flow em { display: grid; place-items: center; width: 17px; height: 17px; border-radius: 50%; font-size: 11px; font-style: normal; color: #fff; background: var(--pkg-h-btn); }
@media (min-width: 1560px) { .shop-plans { grid-template-columns: repeat(4, minmax(0, 1fr)); } }
@media (max-width: 900px) { .shop-tiles { grid-template-columns: minmax(0, 1fr); } }
@media (max-width: 720px) { .shop-plans { grid-template-columns: minmax(0, 1fr); } }
</style>
