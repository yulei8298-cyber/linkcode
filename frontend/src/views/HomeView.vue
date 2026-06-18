<template>
  <!-- Custom Home Content: Full Page Mode (admin-configured, highest priority) -->
  <div v-if="homeContent" class="min-h-screen">
    <!-- iframe mode -->
    <iframe
      v-if="isHomeContentUrl"
      :src="homeContent.trim()"
      class="h-screen w-full border-0"
      allowfullscreen
    ></iframe>
    <!-- HTML mode - SECURITY: homeContent is admin-only setting, XSS risk is acceptable -->
    <div v-else v-html="homeContent"></div>
  </div>

  <!-- Default Home Page (tabcode-style public portal) -->
  <PortalLayout v-else>
    <!-- Hero -->
    <section class="py-12 text-center sm:py-20">
      <div
        class="mx-auto mb-6 inline-flex items-center gap-2 rounded-full border border-emerald-200 bg-emerald-50 px-3.5 py-1.5 text-xs font-medium text-emerald-700 dark:border-emerald-500/30 dark:bg-emerald-500/10 dark:text-emerald-300"
      >
        <span class="h-1.5 w-1.5 animate-pulse rounded-full bg-emerald-500"></span>
        {{ t('portal.hero.badge') }}
      </div>

      <h1 class="mx-auto max-w-3xl text-4xl font-bold tracking-tight sm:text-5xl lg:text-6xl">
        {{ siteName }}
      </h1>
      <p class="mx-auto mt-5 max-w-2xl text-base text-gray-600 dark:text-dark-300 sm:text-lg">
        {{ siteSubtitle || t('portal.hero.subtitle') }}
      </p>

      <div class="mt-8 flex flex-wrap items-center justify-center gap-3">
        <router-link
          :to="isAuthenticated ? dashboardPath : '/login'"
          class="inline-flex items-center gap-2 rounded-full bg-primary-600 px-6 py-3 text-sm font-semibold text-white shadow-lg shadow-primary-600/25 transition hover:bg-primary-700"
        >
          {{ isAuthenticated ? t('portal.hero.goToConsole') : t('portal.hero.getStarted') }}
          <Icon name="arrowRight" size="md" :stroke-width="2" />
        </router-link>
        <router-link
          v-if="showPricing"
          to="/portal/pricing"
          class="inline-flex items-center gap-2 rounded-full border border-gray-300 bg-white px-6 py-3 text-sm font-semibold text-gray-700 transition hover:bg-gray-50 dark:border-dark-700 dark:bg-dark-800 dark:text-dark-100 dark:hover:bg-dark-700"
        >
          {{ t('portal.hero.viewPricing') }}
        </router-link>
      </div>
    </section>

    <!-- Feature entries (the 4 public functions) -->
    <section class="grid gap-5 pb-12 sm:grid-cols-2 lg:grid-cols-4">
      <router-link
        v-if="showStatus"
        to="/portal/status"
        class="feature-card group"
      >
        <span class="feature-icon bg-gradient-to-br from-emerald-500 to-emerald-600">
          <Icon name="chart" size="lg" class="text-white" />
        </span>
        <h3 class="feature-title">{{ t('portal.features.status') }}</h3>
        <p class="feature-desc">{{ t('portal.features.statusDesc') }}</p>
      </router-link>

      <router-link
        v-if="showPricing"
        to="/portal/pricing"
        class="feature-card group"
      >
        <span class="feature-icon bg-gradient-to-br from-primary-500 to-primary-600">
          <Icon name="chart" size="lg" class="text-white" />
        </span>
        <h3 class="feature-title">{{ t('portal.features.pricing') }}</h3>
        <p class="feature-desc">{{ t('portal.features.pricingDesc') }}</p>
      </router-link>

      <router-link
        v-if="showTutorial"
        to="/portal/tutorial"
        class="feature-card group"
      >
        <span class="feature-icon bg-gradient-to-br from-blue-500 to-blue-600">
          <Icon name="book" size="lg" class="text-white" />
        </span>
        <h3 class="feature-title">{{ t('portal.features.tutorial') }}</h3>
        <p class="feature-desc">{{ t('portal.features.tutorialDesc') }}</p>
      </router-link>

      <a
        v-if="showChat"
        :href="chatStationUrl"
        target="_blank"
        rel="noopener noreferrer"
        class="feature-card group"
      >
        <span class="feature-icon bg-gradient-to-br from-purple-500 to-purple-600">
          <Icon name="server" size="lg" class="text-white" />
        </span>
        <h3 class="feature-title">{{ t('portal.features.chat') }}</h3>
        <p class="feature-desc">{{ t('portal.features.chatDesc') }}</p>
      </a>
    </section>

    <!-- Supported providers -->
    <section class="border-t border-gray-200/70 py-12 text-center dark:border-dark-800/70">
      <h2 class="text-xl font-bold sm:text-2xl">{{ t('home.providers.title') }}</h2>
      <p class="mt-2 text-sm text-gray-500 dark:text-dark-400">{{ t('home.providers.description') }}</p>

      <div class="mt-8 flex flex-wrap items-center justify-center gap-3">
        <div class="provider-chip">
          <span class="flex h-7 w-7 items-center justify-center rounded-lg bg-gradient-to-br from-orange-400 to-orange-500 text-xs font-bold text-white">C</span>
          {{ t('home.providers.claude') }}
        </div>
        <div class="provider-chip">
          <span class="flex h-7 w-7 items-center justify-center rounded-lg bg-gradient-to-br from-green-500 to-green-600 text-xs font-bold text-white">G</span>
          GPT
        </div>
        <div class="provider-chip">
          <span class="flex h-7 w-7 items-center justify-center rounded-lg bg-gradient-to-br from-blue-500 to-blue-600 text-xs font-bold text-white">G</span>
          {{ t('home.providers.gemini') }}
        </div>
        <div class="provider-chip">
          <span class="flex h-7 w-7 items-center justify-center rounded-lg bg-gradient-to-br from-rose-500 to-pink-600 text-xs font-bold text-white">A</span>
          {{ t('home.providers.antigravity') }}
        </div>
      </div>
    </section>
  </PortalLayout>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore, useAppStore } from '@/stores'
import PortalLayout from '@/views/public/components/PortalLayout.vue'
import Icon from '@/components/icons/Icon.vue'

const { t } = useI18n()

const authStore = useAuthStore()
const appStore = useAppStore()

const siteName = computed(() => appStore.cachedPublicSettings?.site_name || appStore.siteName || 'Sub2API')
const siteSubtitle = computed(() => appStore.cachedPublicSettings?.site_subtitle || '')
const homeContent = computed(() => appStore.cachedPublicSettings?.home_content || '')
const chatStationUrl = computed(() => appStore.cachedPublicSettings?.chat_station_url || '')
const tutorialContentMD = computed(() => appStore.cachedPublicSettings?.tutorial_content_md || '')

const isHomeContentUrl = computed(() => {
  const content = homeContent.value.trim()
  return content.startsWith('http://') || content.startsWith('https://')
})

// Feature entry visibility — mirror PortalLayout nav logic.
const showStatus = computed(() => appStore.cachedPublicSettings?.channel_monitor_enabled !== false)
const showPricing = computed(() => appStore.cachedPublicSettings?.available_channels_enabled === true)
const showTutorial = computed(() => Boolean(tutorialContentMD.value.trim()))
const showChat = computed(() => Boolean(chatStationUrl.value.trim()))

const isAuthenticated = computed(() => authStore.isAuthenticated)
const isAdmin = computed(() => authStore.isAdmin)
const dashboardPath = computed(() => (isAdmin.value ? '/admin/dashboard' : '/dashboard'))

function initTheme() {
  const savedTheme = localStorage.getItem('theme')
  if (
    savedTheme === 'dark' ||
    (!savedTheme && window.matchMedia('(prefers-color-scheme: dark)').matches)
  ) {
    document.documentElement.classList.add('dark')
  }
}

onMounted(() => {
  initTheme()
  authStore.checkAuth()
  if (!appStore.publicSettingsLoaded) {
    appStore.fetchPublicSettings()
  }
})
</script>

<style scoped>
.feature-card {
  @apply flex flex-col items-start rounded-2xl border border-gray-200/80 bg-white/80 p-6 text-left shadow-sm backdrop-blur-sm transition-all duration-300 hover:-translate-y-1 hover:shadow-xl hover:shadow-primary-500/10 dark:border-dark-700/70 dark:bg-dark-800/60;
}
.feature-icon {
  @apply mb-4 flex h-12 w-12 items-center justify-center rounded-xl shadow-lg transition-transform;
}
.feature-card:hover .feature-icon {
  @apply scale-110;
}
.feature-title {
  @apply mb-1.5 text-lg font-semibold text-gray-900 dark:text-white;
}
.feature-desc {
  @apply text-sm leading-relaxed text-gray-600 dark:text-dark-400;
}
.provider-chip {
  @apply inline-flex items-center gap-2 rounded-xl border border-gray-200 bg-white/70 px-4 py-2.5 text-sm font-medium text-gray-700 dark:border-dark-700 dark:bg-dark-800/60 dark:text-dark-200;
}
</style>
