<template>
  <AppLayout>
    <div class="w-full min-w-0 space-y-6 pb-8">
      <header class="page-header card mb-0 p-3">
        <!-- 标题与说明已在顶栏显示，这里仅保留给读屏器的 h1 与标签页 -->
        <h1 class="page-title sr-only">{{ t('admin.packages.title') }}</h1>
        <div class="tabs inline-flex w-full max-w-2xl flex-wrap sm:w-auto" role="tablist">
          <button
            v-for="tab in tabs"
            :key="tab.value"
            type="button"
            role="tab"
            class="tab flex-1 sm:flex-none"
            :class="activeTab === tab.value ? 'tab-active' : ''"
            :aria-selected="activeTab === tab.value"
            @click="activeTab = tab.value"
          >
            {{ tab.label }}
          </button>
        </div>
      </header>

      <PackagePlansTab v-if="activeTab === 'plans'" />
      <PackageNoticeTab v-else-if="activeTab === 'notice'" />
      <PackageFreezeTab v-else-if="activeTab === 'freeze'" />
      <EnterpriseTab v-else-if="activeTab === 'enterprise'" />
      <UserPackagesTab v-else />
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import PackagePlansTab from './packages/PackagePlansTab.vue'
import PackageNoticeTab from './packages/PackageNoticeTab.vue'
import PackageFreezeTab from './packages/PackageFreezeTab.vue'
import UserPackagesTab from './packages/UserPackagesTab.vue'
import EnterpriseTab from './packages/EnterpriseTab.vue'
import '@/components/package/package.css'

type TabKey = 'plans' | 'notice' | 'freeze' | 'userPackages' | 'enterprise'

const { t } = useI18n()
const activeTab = ref<TabKey>('plans')

const tabs = computed<{ value: TabKey; label: string }[]>(() => [
  { value: 'plans', label: t('admin.packages.tabs.plans') },
  { value: 'notice', label: t('admin.packages.tabs.notice') },
  { value: 'freeze', label: t('admin.packages.tabs.freeze') },
  { value: 'userPackages', label: t('admin.packages.tabs.userPackages') },
  { value: 'enterprise', label: t('admin.packages.tabs.enterprise') },
])
</script>
