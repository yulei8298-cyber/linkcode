<template>
  <div v-if="hasHomeContent" class="min-h-screen">
    <iframe v-if="isHomeContentUrl" :src="homeContent.trim()" class="h-screen w-full border-0" allowfullscreen></iframe>
    <div v-else v-html="homeContent"></div>
  </div>

  <div
    v-else-if="compactHomeEnabled"
    data-testid="compact-home"
    class="flex min-h-screen flex-col bg-gray-50 text-gray-900 dark:bg-dark-950 dark:text-white"
  >
    <header class="border-b border-gray-200 px-4 py-4 sm:px-6 dark:border-dark-800">
      <nav class="mx-auto flex max-w-5xl flex-wrap items-center justify-between gap-3 sm:gap-4">
        <div class="flex min-w-0 flex-1 items-center gap-3">
          <img :src="siteLogo || '/logo.svg'" alt="Logo" class="h-9 w-9 shrink-0 rounded-lg object-contain" />
          <span class="min-w-0 truncate text-base font-semibold">{{ siteName }}</span>
        </div>
        <div class="flex max-w-full shrink-0 flex-wrap items-center justify-end gap-2">
          <LocaleSwitcher />
          <a
            v-if="docUrl"
            :href="docUrl"
            target="_blank"
            rel="noopener noreferrer"
            class="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg text-gray-500 hover:bg-gray-100 dark:text-dark-400 dark:hover:bg-dark-800"
            :title="t('home.viewDocs')"
          >
            <Icon name="book" size="md" />
          </a>
          <RouterLink
            v-if="showModelPlazaEntry"
            to="/model-plaza"
            class="flex h-10 shrink-0 items-center gap-1.5 rounded-lg px-2.5 text-sm font-medium text-gray-500 hover:bg-gray-100 hover:text-gray-700 dark:text-dark-400 dark:hover:bg-dark-800 dark:hover:text-white"
            :title="t('nav.modelPlaza')"
          >
            <Icon name="grid" size="md" />
            <span class="hidden sm:inline">{{ t('nav.modelPlaza') }}</span>
          </RouterLink>
          <button
            type="button"
            class="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg text-gray-500 hover:bg-gray-100 dark:text-dark-400 dark:hover:bg-dark-800"
            :title="isDark ? t('home.switchToLight') : t('home.switchToDark')"
            @click="toggleTheme"
          >
            <Icon :name="isDark ? 'sun' : 'moon'" size="md" />
          </button>
          <RouterLink
            :to="isAuthenticated ? dashboardPath : '/login'"
            class="inline-flex min-h-10 shrink-0 items-center justify-center rounded-lg bg-gray-900 px-4 py-2 text-sm font-medium text-white hover:bg-gray-800 dark:bg-white dark:text-gray-900 dark:hover:bg-gray-200"
          >
            {{ isAuthenticated ? t('home.dashboard') : t('home.login') }}
          </RouterLink>
        </div>
      </nav>
    </header>

    <main class="flex min-w-0 flex-1 items-center justify-center px-4 py-16 sm:px-6">
      <div class="min-w-0 max-w-2xl text-center">
        <img :src="siteLogo || '/logo.svg'" alt="Logo" class="mx-auto mb-6 h-20 w-20 rounded-2xl object-contain" />
        <h1 class="[overflow-wrap:anywhere] text-3xl font-bold md:text-4xl">{{ siteName }}</h1>
        <p class="mt-4 whitespace-pre-wrap [overflow-wrap:anywhere] text-base text-gray-600 dark:text-dark-300">{{ siteSubtitle }}</p>
        <RouterLink
          :to="isAuthenticated ? dashboardPath : '/login'"
          class="mt-8 inline-flex min-h-10 items-center justify-center rounded-lg bg-primary-600 px-5 py-2.5 text-sm font-medium text-white hover:bg-primary-700"
        >
          {{ isAuthenticated ? t('home.goToDashboard') : t('home.login') }}
        </RouterLink>
      </div>
    </main>

    <footer class="min-w-0 border-t border-gray-200 px-4 py-5 text-center text-sm text-gray-500 [overflow-wrap:anywhere] sm:px-6 dark:border-dark-800 dark:text-dark-400">
      &copy; {{ currentYear }} {{ siteName }}
    </footer>
  </div>

  <PortalLayout v-else data-testid="default-home">
    <HomeHero
      :site-name="siteName"
      :site-logo="siteLogo"
      :api-host="apiHost"
      :is-authenticated="isAuthenticated"
      :dashboard-path="dashboardPath"
      @show-connect="scrollToConnect"
    />
    <HomeTools />
    <HomeConnect :site-name="siteName" :api-base="normalizedApiBaseUrl" />
    <HomeFeatures :has-community="Boolean(qqGroup || telegramGroupUrl)" :show-intel-check="intelCheckEnabled" />
    <HomeBilling
      :yuan="formatNumber(pricingConfig.yuanAmount)"
      :usd="formatNumber(pricingConfig.usdAmount)"
      :unit-label="pricingConfig.creditUnitLabel"
      :affiliate-rate="affiliateRate"
    />
    <HomeFaq />
    <section class="lc-cta">
      <div class="lc-wrap lc-cta-inner">
        <h2 class="lc-section-title">今天就把 Claude Code<br />接上 {{ siteName }}</h2>
        <p class="lc-section-copy">注册、创建密钥、改地址，几分钟的事。</p>
        <div class="lc-hero-actions" style="justify-content:center">
          <RouterLink :to="isAuthenticated ? dashboardPath : '/register'" class="lc-button lc-button-primary lc-button-xl">
            {{ isAuthenticated ? '进入控制台' : '注册账号' }}
          </RouterLink>
        </div>
      </div>
    </section>
  </PortalLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { RouterLink } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useAuthStore, useAppStore } from '@/stores'
import LocaleSwitcher from '@/components/common/LocaleSwitcher.vue'
import PortalLayout from '@/views/public/components/PortalLayout.vue'
import HomeHero from '@/views/public/components/home/HomeHero.vue'
import HomeTools from '@/views/public/components/home/HomeTools.vue'
import HomeConnect from '@/views/public/components/home/HomeConnect.vue'
import HomeFeatures from '@/views/public/components/home/HomeFeatures.vue'
import HomeBilling from '@/views/public/components/home/HomeBilling.vue'
import HomeFaq from '@/views/public/components/home/HomeFaq.vue'
import '@/views/public/components/home/home.css'
import Icon from '@/components/icons/Icon.vue'
import { parsePricingDisplayConfig } from '@/utils/pricingDisplayConfig'
import { sanitizeUrl } from '@/utils/url'
import { normalizeSiteName } from '@/utils/branding'
import { FeatureFlags, isFeatureFlagEnabled } from '@/utils/featureFlags'
import { applyTheme, shouldUseDarkTheme } from '@/utils/theme'

const authStore = useAuthStore()
const appStore = useAppStore()
const { t } = useI18n()
const settings = computed(() => appStore.cachedPublicSettings)
const siteName = computed(() => normalizeSiteName(settings.value?.site_name || appStore.siteName))
const siteLogo = computed(() => sanitizeUrl(settings.value?.site_logo || appStore.siteLogo || '', { allowRelative: true, allowDataUrl: true }))
const siteSubtitle = computed(() => settings.value?.site_subtitle || '')
const homeContent = computed(() => settings.value?.home_content || '')
const hasHomeContent = computed(() => homeContent.value.trim().length > 0)
const compactHomeEnabled = computed(() => settings.value?.compact_home_enabled === true)
const modelPlazaEnabled = computed(() => isFeatureFlagEnabled(FeatureFlags.modelPlaza))
const intelCheckEnabled = computed(() => settings.value?.intel_check_enabled === true)
const isHomeContentUrl = computed(() => /^https?:\/\//.test(homeContent.value.trim()))
const isAuthenticated = computed(() => authStore.isAuthenticated)
const modelPlazaRequiresAuth = computed(() => settings.value?.model_plaza_require_auth === true)
const showModelPlazaEntry = computed(
  () => modelPlazaEnabled.value && (isAuthenticated.value || !modelPlazaRequiresAuth.value),
)
const dashboardPath = computed(() => (authStore.isAdmin ? '/admin/dashboard' : '/dashboard'))
const apiBaseUrl = computed(() => settings.value?.api_base_url?.trim() || '')
const normalizedApiBaseUrl = computed(() => {
  const configuredUrl = apiBaseUrl.value || window.location.origin
  return configuredUrl.replace(/\/+$/, '').replace(/\/v1$/i, '')
})
const apiHost = computed(() => normalizedApiBaseUrl.value.replace(/^https?:\/\//, ''))
const telegramGroupUrl = computed(() => sanitizeUrl(settings.value?.telegram_group_url || ''))
const qqGroup = computed(() => settings.value?.qq_group?.trim() || '')
const docUrl = computed(() => sanitizeUrl(settings.value?.doc_url || appStore.docUrl || ''))
const pricingConfig = computed(() => parsePricingDisplayConfig(settings.value?.pricing_display_config || ''))
const affiliateRate = computed(() =>
  settings.value?.affiliate_enabled === true ? Number(settings.value?.affiliate_rebate_rate || 0) : 0,
)
const isDark = ref(document.documentElement.classList.contains('dark'))
const currentYear = new Date().getFullYear()

function formatNumber(value: number) {
  return Number.isInteger(value) ? String(value) : value.toFixed(2).replace(/\.?0+$/, '')
}

function scrollToConnect() {
  document.getElementById('how')?.scrollIntoView({ behavior: 'smooth', block: 'start' })
}

function toggleTheme() {
  isDark.value = !isDark.value
  applyTheme(isDark.value)
}

function initTheme() {
  if (shouldUseDarkTheme()) {
    isDark.value = true
    document.documentElement.classList.add('dark')
  }
}

onMounted(() => {
  initTheme()
  if (!appStore.publicSettingsLoaded) void appStore.fetchPublicSettings()
  void authStore.checkAuth()
})
</script>
