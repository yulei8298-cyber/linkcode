<template>
  <AppLayout>
    <div class="mine">
      <div v-if="loading" class="flex justify-center py-16">
        <div class="h-8 w-8 animate-spin rounded-full border-2 border-primary-500 border-t-transparent"></div>
      </div>

      <div v-else-if="!mine" class="card p-12 text-center text-gray-500 dark:text-dark-400">{{ loadError }}</div>

      <template v-else>
        <PackageMineHero :packages="mine.active" :concurrency="mine.package_concurrency" :balance="balance" />

        <PackageFreezeCalendar
          :today="mine.today"
          :next-freezable="mine.next_freezable"
          :freeze-enabled="mine.freeze_enabled"
          :max-freeze-days-week="mine.max_freeze_days_week"
          :max-freeze-days-month="mine.max_freeze_days_month"
        />

        <section class="mine-sec" aria-labelledby="pkg-active-title">
          <div class="mine-sec-head">
            <h2 id="pkg-active-title">{{ t('packages.mine.activeTitle') }}</h2>
            <p>{{ t('packages.mine.activeHint') }}</p>
          </div>
          <div v-if="orderedActive.length === 0" class="card p-10 text-center">
            <p class="mb-4 text-gray-500 dark:text-dark-400">{{ t('packages.mine.empty') }}</p>
            <router-link to="/packages" class="btn btn-primary">{{ t('packages.mine.goShop') }}</router-link>
          </div>
          <PackageItemCard
            v-for="item in orderedActive"
            :key="item.id"
            :item="item"
            :freeze-enabled="mine.freeze_enabled"
            :today-freezable="mine.today.freezable"
            :next-freezable-date="mine.next_freezable?.date || ''"
            :busy="busyId === item.id"
            @freeze="freezeTarget = $event"
            @unfreeze="unfreeze"
            @rebuy="router.push('/packages')"
          />
        </section>

        <section v-if="mine.ended.length" class="mine-sec" aria-labelledby="pkg-ended-title">
          <div class="mine-sec-head">
            <h2 id="pkg-ended-title">{{ t('packages.mine.endedTitle') }}</h2>
            <p>{{ t('packages.mine.endedHint') }}</p>
          </div>
          <div class="pkg-card overflow-x-auto">
            <table class="mine-table">
              <thead>
                <tr>
                  <th>{{ t('packages.mine.table.name') }}</th>
                  <th>{{ t('packages.mine.table.group') }}</th>
                  <th>{{ t('packages.mine.table.quota') }}</th>
                  <th>{{ t('packages.mine.table.used') }}</th>
                  <th>{{ t('packages.mine.table.bought') }}</th>
                  <th>{{ t('packages.mine.table.ended') }}</th>
                  <th>{{ t('packages.mine.table.status') }}</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="item in mine.ended" :key="item.id">
                  <td><span :class="['mine-dot pkg-hue', `pkg-h-${packagePlanStyle(item.cycle, item.tier).hue}`]">{{ item.name }}</span></td>
                  <td>{{ item.group_name }}</td>
                  <td class="pkg-mono">{{ formatUSD(item.quota_usd) }}</td>
                  <td class="pkg-mono">{{ formatUSD(item.used_usd) }}</td>
                  <td>{{ formatDateTimeToMinute(item.starts_at) }}</td>
                  <td>{{ formatDateTimeToMinute(item.updated_at) }}</td>
                  <td><span class="pkg-chip pkg-chip-ended">{{ t(`packages.mine.status.${item.status}`) }}</span></td>
                </tr>
              </tbody>
            </table>
          </div>
        </section>
      </template>
    </div>

    <PackageFreezeDialog
      :item="freezeTarget"
      :today-label="todayLabel"
      :busy="busyId !== null"
      @close="freezeTarget = null"
      @confirm="freeze"
    />
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import AppLayout from '@/components/layout/AppLayout.vue'
import PackageMineHero from '@/components/package/mine/PackageMineHero.vue'
import PackageFreezeCalendar from '@/components/package/mine/PackageFreezeCalendar.vue'
import PackageItemCard from '@/components/package/mine/PackageItemCard.vue'
import PackageFreezeDialog from '@/components/package/mine/PackageFreezeDialog.vue'
import { formatUSD, packagePlanStyle } from '@/components/package/packageUtils'
import packagesAPI, { type PackageMine, type UserPackageView } from '@/api/packages'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import { extractApiErrorMessage } from '@/utils/apiError'
import { formatDateTimeToMinute } from '@/utils/format'
import '@/components/package/package.css'

const { t } = useI18n()
const router = useRouter()
const appStore = useAppStore()
const authStore = useAuthStore()

const loading = ref(true)
const loadError = ref('')
const mine = ref<PackageMine | null>(null)
const busyId = ref<number | null>(null)
const freezeTarget = ref<UserPackageView | null>(null)

const balance = computed(() => authStore.user?.balance ?? 0)

/** 按扣费顺序排列：可扣费的在前（按序号），冻结的排在最后。 */
const orderedActive = computed(() =>
  [...(mine.value?.active ?? [])].sort((a, b) => (a.deduct_order || 999) - (b.deduct_order || 999)),
)

const todayLabel = computed(() => {
  const today = mine.value?.today
  if (!today) return ''
  const kind = today.kind === 'weekend' ? t('packages.calendar.weekend') : today.kind === 'makeup' ? t('packages.calendar.makeup') : today.label
  return kind ? `${today.date} · ${kind}` : today.date
})

async function load() {
  try {
    mine.value = await packagesAPI.getMine()
  } catch (err: unknown) {
    loadError.value = extractApiErrorMessage(err, t('packages.mine.loadFailed'))
  } finally {
    loading.value = false
  }
}

async function freeze(item: UserPackageView) {
  busyId.value = item.id
  try {
    await packagesAPI.freezePackage(item.id)
    appStore.showSuccess(t('packages.mine.freezeDone', { name: item.name }))
    freezeTarget.value = null
    await load()
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('common.error')))
  } finally {
    busyId.value = null
  }
}

async function unfreeze(item: UserPackageView) {
  busyId.value = item.id
  try {
    await packagesAPI.unfreezePackage(item.id)
    appStore.showSuccess(t('packages.mine.unfreezeDone', { name: item.name }))
    await load()
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('common.error')))
  } finally {
    busyId.value = null
  }
}

onMounted(load)
</script>

<style scoped>
.mine { display: grid; gap: 20px; }
.mine-sec { display: grid; gap: 12px; }
.mine-sec-head { display: flex; flex-wrap: wrap; align-items: flex-end; justify-content: space-between; gap: 8px 16px; }
.mine-sec-head h2 { margin: 0; font-size: 17px; font-weight: 700; color: var(--lc-ink); }
.mine-sec-head p { margin: 0; font-size: 12.5px; color: var(--lc-ink-3); }
.mine-table { width: 100%; border-collapse: collapse; font-size: 13.5px; color: var(--lc-ink); }
.mine-table th { padding: 11px 16px; text-align: left; font-size: 12.5px; font-weight: 500; white-space: nowrap; color: var(--lc-ink-3); background: var(--lc-surface-2); border-bottom: 1px solid var(--lc-line); }
.mine-table td { padding: 12px 16px; white-space: nowrap; border-bottom: 1px solid var(--lc-line); }
.mine-table tr:last-child td { border-bottom: 0; }
.mine-dot { display: inline-flex; align-items: center; gap: 6px; font-weight: 600; }
.mine-dot::before { content: ''; width: 8px; height: 8px; border-radius: 3px; background: var(--pkg-h); }
</style>
