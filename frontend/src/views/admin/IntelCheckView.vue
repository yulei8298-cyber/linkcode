<template>
  <AppLayout>
    <div class="w-full min-w-0 space-y-6 pb-8">
      <header
        class="page-header mb-0 rounded-3xl bg-white p-5 shadow-sm ring-1 ring-gray-900/5 dark:bg-dark-800 dark:ring-dark-700 sm:p-6"
      >
        <h1 class="page-title flex items-center gap-2 text-xl font-black text-gray-900 dark:text-white">
          <span
            class="inline-flex h-8 w-8 items-center justify-center rounded-xl bg-teal-50 text-teal-500 dark:bg-teal-900/30 dark:text-teal-400"
          >
            <Icon name="chart" size="sm" />
          </span>
          {{ t('admin.intelCheck.title') }}
        </h1>
        <p class="page-description mt-1.5 text-xs text-gray-500 dark:text-gray-400">
          {{ t('admin.intelCheck.description') }}
        </p>

        <div class="mt-4 border-t border-gray-100 pt-4 dark:border-dark-700">
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
        </div>
      </header>

      <!--
        四个页签用 v-if 而非 v-show：受检分组与题库各自带着列表请求与表单状态，
        全部挂载会在进页面时并发打三四个请求，其中大多数用户当次并不会看。
        代价是切回来要重新加载一次，对管理端这个频率完全可以接受。
      -->
      <IntelCheckOverviewTab v-if="activeTab === 'overview'" />
      <IntelCheckTargetsTab v-else-if="activeTab === 'targets'" />
      <IntelCheckQuestionsTab v-else-if="activeTab === 'questions'" />
      <IntelCheckSettingsTab v-else />
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import IntelCheckOverviewTab from './intel-check/IntelCheckOverviewTab.vue'
import IntelCheckTargetsTab from './intel-check/IntelCheckTargetsTab.vue'
import IntelCheckQuestionsTab from './intel-check/IntelCheckQuestionsTab.vue'
import IntelCheckSettingsTab from './intel-check/IntelCheckSettingsTab.vue'

type TabKey = 'overview' | 'targets' | 'questions' | 'settings'

const { t } = useI18n()

// 默认落在「受检分组」而不是「概览」：功能默认关闭，新装的部署第一次进来时
// 概览必然是空的，而下一步动作一定是先加一个分组。
const activeTab = ref<TabKey>('targets')

const tabs = computed<{ value: TabKey; label: string }[]>(() => [
  { value: 'overview', label: t('admin.intelCheck.tabs.overview') },
  { value: 'targets', label: t('admin.intelCheck.tabs.targets') },
  { value: 'questions', label: t('admin.intelCheck.tabs.questions') },
  { value: 'settings', label: t('admin.intelCheck.tabs.settings') },
])
</script>
