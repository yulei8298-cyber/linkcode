<template>
  <section class="card space-y-4 p-5">
    <div>
      <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.packages.userPackages.title') }}</h3>
      <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.packages.userPackages.desc') }}</p>
    </div>
    <form class="flex flex-wrap items-end gap-3" @submit.prevent="query">
      <label class="block">
        <span class="input-label">{{ t('admin.packages.userPackages.userId') }}</span>
        <input v-model.number="userId" type="number" min="1" class="input w-48" />
      </label>
      <button type="submit" class="btn btn-primary" :disabled="busy || !(userId > 0)">{{ t('admin.packages.userPackages.query') }}</button>
    </form>

    <div v-if="queried" class="overflow-x-auto rounded-xl border border-gray-200 dark:border-dark-700">
      <table class="w-full text-sm">
        <thead class="bg-gray-50 text-left text-xs text-gray-500 dark:bg-dark-800 dark:text-gray-400">
          <tr>
            <th class="px-4 py-2.5">{{ t('admin.packages.userPackages.table.id') }}</th>
            <th class="px-4 py-2.5">{{ t('admin.packages.userPackages.table.name') }}</th>
            <th class="px-4 py-2.5">{{ t('admin.packages.userPackages.table.group') }}</th>
            <th class="px-4 py-2.5">{{ t('admin.packages.userPackages.table.quota') }}</th>
            <th class="px-4 py-2.5">{{ t('admin.packages.userPackages.table.used') }}</th>
            <th class="px-4 py-2.5">{{ t('admin.packages.userPackages.table.expires') }}</th>
            <th class="px-4 py-2.5">{{ t('admin.packages.userPackages.table.status') }}</th>
            <th class="px-4 py-2.5">{{ t('admin.packages.userPackages.table.actions') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-if="!packages.length"><td colspan="8" class="px-4 py-6 text-center text-gray-500">{{ t('admin.packages.userPackages.empty') }}</td></tr>
          <tr v-for="p in packages" :key="p.id" class="border-t border-gray-100 dark:border-dark-700">
            <td class="px-4 py-2.5 font-mono">{{ p.id }}</td>
            <td class="px-4 py-2.5 font-medium text-gray-900 dark:text-white">{{ p.name }}</td>
            <td class="px-4 py-2.5 font-mono">{{ p.group_id }}</td>
            <td class="px-4 py-2.5 font-mono">{{ formatUSD(p.quota_usd) }}</td>
            <td class="px-4 py-2.5 font-mono">{{ formatUSD(p.used_usd) }}</td>
            <td class="px-4 py-2.5">{{ formatDateTimeToMinute(p.expires_at) }}</td>
            <td class="px-4 py-2.5">{{ t(`packages.mine.status.${p.status}`) }}</td>
            <td class="px-4 py-2.5">
              <div class="flex gap-2">
                <button v-if="p.status === 'frozen'" type="button" class="btn btn-secondary btn-sm" :disabled="busy" @click="unfreeze(p)">
                  {{ t('admin.packages.userPackages.unfreeze') }}
                </button>
                <button v-if="p.status === 'active' || p.status === 'frozen'" type="button" class="btn btn-danger btn-sm" :disabled="busy" @click="pendingVoid = p">
                  {{ t('admin.packages.userPackages.void') }}
                </button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <ConfirmDialog
      :show="!!pendingVoid"
      :title="t('admin.packages.userPackages.void')"
      :message="pendingVoid ? t('admin.packages.userPackages.voidConfirm', { name: pendingVoid.name }) : ''"
      danger
      @confirm="confirmVoid"
      @cancel="pendingVoid = null"
    />
  </section>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import packagesAdminAPI from '@/api/admin/packages'
import type { UserPackage } from '@/api/packages'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import { formatDateTimeToMinute } from '@/utils/format'
import { formatUSD } from '@/components/package/packageUtils'

const { t } = useI18n()
const appStore = useAppStore()

const userId = ref(0)
const queried = ref(false)
const packages = ref<UserPackage[]>([])
const busy = ref(false)
const pendingVoid = ref<UserPackage | null>(null)

async function query() {
  busy.value = true
  try {
    packages.value = await packagesAdminAPI.listUserPackages(userId.value)
    queried.value = true
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('admin.packages.common.loadFailed')))
  } finally {
    busy.value = false
  }
}

async function act(action: () => Promise<unknown>) {
  busy.value = true
  try {
    await action()
    appStore.showSuccess(t('admin.packages.userPackages.done'))
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('common.error')))
  } finally {
    busy.value = false
  }
  await query()
}

const unfreeze = (p: UserPackage) => act(() => packagesAdminAPI.unfreezeUserPackage(p.id))

function confirmVoid() {
  const p = pendingVoid.value
  pendingVoid.value = null
  if (p) act(() => packagesAdminAPI.voidUserPackage(p.id))
}
</script>
