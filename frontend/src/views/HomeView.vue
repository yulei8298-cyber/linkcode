<template>
  <!-- Custom Home Content: Full Page Mode (admin-configured, highest priority) -->
  <div v-if="homeContent" class="min-h-screen">
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
    <!-- ===================== Hero ===================== -->
    <section class="relative py-16 text-center sm:py-24">
      <!-- Logo mark -->
      <div class="mb-8 flex justify-center">
        <span class="flex h-20 w-20 items-center justify-center rounded-3xl bg-white shadow-xl shadow-primary-500/10 ring-1 ring-gray-100 dark:bg-dark-800 dark:ring-dark-700">
          <img :src="siteLogo || '/logo.svg'" alt="logo" class="h-14 w-14 object-contain" />
        </span>
      </div>

      <div
        class="mx-auto mb-6 inline-flex items-center gap-2 rounded-full border border-emerald-200 bg-emerald-50 px-4 py-1.5 text-xs font-medium text-emerald-700 dark:border-emerald-500/30 dark:bg-emerald-500/10 dark:text-emerald-300"
      >
        <span class="h-1.5 w-1.5 animate-pulse rounded-full bg-emerald-500"></span>
        {{ t('portal.hero.badge') }}
      </div>

      <h1 class="mx-auto max-w-4xl text-4xl font-extrabold tracking-tight sm:text-5xl lg:text-6xl">
        {{ t('portal.hero.title') }}
        <span class="bg-gradient-to-r from-primary-500 to-emerald-500 bg-clip-text text-transparent">{{ t('portal.hero.titleHighlight') }}</span>
      </h1>
      <p class="mx-auto mt-6 max-w-2xl text-base leading-relaxed text-gray-600 dark:text-dark-300 sm:text-lg">
        {{ siteSubtitle || t('portal.hero.subtitle') }}
      </p>

      <div class="mt-9 flex flex-wrap items-center justify-center gap-3">
        <router-link
          :to="isAuthenticated ? dashboardPath : '/login'"
          class="inline-flex items-center gap-2 rounded-full bg-primary-600 px-7 py-3.5 text-sm font-semibold text-white shadow-lg shadow-primary-600/25 transition hover:bg-primary-700"
        >
          {{ isAuthenticated ? t('portal.hero.goToConsole') : t('portal.hero.getStarted') }}
          <Icon name="arrowRight" size="md" :stroke-width="2" />
        </router-link>
        <router-link
          to="/portal/pricing"
          class="inline-flex items-center gap-2 rounded-full border border-gray-300 bg-white px-7 py-3.5 text-sm font-semibold text-gray-700 transition hover:bg-gray-50 dark:border-dark-700 dark:bg-dark-800 dark:text-dark-100 dark:hover:bg-dark-700"
        >
          {{ t('portal.hero.viewPricing') }}
        </router-link>
      </div>

      <!-- Stats -->
      <div class="mx-auto mt-14 grid max-w-2xl grid-cols-3 gap-4">
        <div class="stat-card">
          <div class="stat-num">4+</div>
          <div class="stat-label">{{ t('portal.stats.providers') }}</div>
        </div>
        <div class="stat-card">
          <div class="stat-num">0.2x</div>
          <div class="stat-label">{{ t('portal.stats.lowestRate') }}</div>
        </div>
        <div class="stat-card">
          <div class="stat-num">{{ t('portal.stats.permanentValue') }}</div>
          <div class="stat-label">{{ t('portal.stats.balance') }}</div>
        </div>
      </div>
    </section>

    <!-- ===================== Feature entries (4 functions) ===================== -->
    <section class="grid gap-5 pb-4 sm:grid-cols-2 lg:grid-cols-4">
      <router-link to="/portal/status" class="feature-card group">
        <span class="feature-icon bg-gradient-to-br from-emerald-500 to-emerald-600">
          <Icon name="chart" size="lg" class="text-white" />
        </span>
        <h3 class="feature-title">{{ t('portal.features.status') }}</h3>
        <p class="feature-desc">{{ t('portal.features.statusDesc') }}</p>
      </router-link>

      <router-link to="/portal/pricing" class="feature-card group">
        <span class="feature-icon bg-gradient-to-br from-primary-500 to-primary-600">
          <Icon name="chart" size="lg" class="text-white" />
        </span>
        <h3 class="feature-title">{{ t('portal.features.pricing') }}</h3>
        <p class="feature-desc">{{ t('portal.features.pricingDesc') }}</p>
      </router-link>

      <router-link to="/portal/tutorial" class="feature-card group">
        <span class="feature-icon bg-gradient-to-br from-blue-500 to-blue-600">
          <Icon name="book" size="lg" class="text-white" />
        </span>
        <h3 class="feature-title">{{ t('portal.features.tutorial') }}</h3>
        <p class="feature-desc">{{ t('portal.features.tutorialDesc') }}</p>
      </router-link>

      <component
        :is="showChat ? 'a' : 'div'"
        :href="showChat ? chatStationUrl : undefined"
        :target="showChat ? '_blank' : undefined"
        :rel="showChat ? 'noopener noreferrer' : undefined"
        class="feature-card group"
        :class="{ 'cursor-default opacity-80': !showChat }"
      >
        <span class="feature-icon bg-gradient-to-br from-purple-500 to-purple-600">
          <Icon name="chat" size="lg" class="text-white" />
        </span>
        <h3 class="feature-title">
          {{ t('portal.features.chat') }}
          <span v-if="!showChat" class="ml-1 rounded bg-gray-100 px-1.5 py-0.5 text-[10px] font-medium text-gray-500 dark:bg-dark-700 dark:text-dark-400">{{ t('portal.features.soon') }}</span>
        </h3>
        <p class="feature-desc">{{ t('portal.features.chatDesc') }}</p>
      </component>
    </section>

    <!-- ===================== Why us ===================== -->
    <section class="py-16">
      <div class="mb-10 text-center">
        <h2 class="text-2xl font-bold sm:text-3xl">{{ t('portal.why.title') }}</h2>
        <p class="mx-auto mt-3 max-w-xl text-sm text-gray-500 dark:text-dark-400">{{ t('portal.why.subtitle') }}</p>
      </div>
      <div class="grid gap-6 md:grid-cols-3">
        <div class="why-card">
          <span class="why-icon bg-primary-50 text-primary-600 dark:bg-primary-500/10 dark:text-primary-300">
            <Icon name="swap" size="md" />
          </span>
          <h3 class="why-title">{{ t('portal.why.unified') }}</h3>
          <p class="why-desc">{{ t('portal.why.unifiedDesc') }}</p>
        </div>
        <div class="why-card">
          <span class="why-icon bg-emerald-50 text-emerald-600 dark:bg-emerald-500/10 dark:text-emerald-300">
            <Icon name="chart" size="md" />
          </span>
          <h3 class="why-title">{{ t('portal.why.billing') }}</h3>
          <p class="why-desc">{{ t('portal.why.billingDesc') }}</p>
        </div>
        <div class="why-card">
          <span class="why-icon bg-blue-50 text-blue-600 dark:bg-blue-500/10 dark:text-blue-300">
            <Icon name="shield" size="md" />
          </span>
          <h3 class="why-title">{{ t('portal.why.stable') }}</h3>
          <p class="why-desc">{{ t('portal.why.stableDesc') }}</p>
        </div>
      </div>
    </section>

    <!-- ===================== Supported providers ===================== -->
    <section class="border-t border-gray-200/70 py-16 text-center dark:border-dark-800/70">
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
      </div>

      <div class="mt-12">
        <router-link
          :to="isAuthenticated ? dashboardPath : '/login'"
          class="inline-flex items-center gap-2 rounded-full bg-primary-600 px-7 py-3.5 text-sm font-semibold text-white shadow-lg shadow-primary-600/25 transition hover:bg-primary-700"
        >
          {{ isAuthenticated ? t('portal.hero.goToConsole') : t('portal.hero.getStarted') }}
          <Icon name="arrowRight" size="md" :stroke-width="2" />
        </router-link>
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

const siteName = computed(() => appStore.cachedPublicSettings?.site_name || appStore.siteName || 'linkcode')
const siteLogo = computed(() => appStore.cachedPublicSettings?.site_logo || appStore.siteLogo || '')
const siteSubtitle = computed(() => appStore.cachedPublicSettings?.site_subtitle || '')
const homeContent = computed(() => appStore.cachedPublicSettings?.home_content || '')
const chatStationUrl = computed(() => appStore.cachedPublicSettings?.chat_station_url || '')

const isHomeContentUrl = computed(() => {
  const content = homeContent.value.trim()
  return content.startsWith('http://') || content.startsWith('https://')
})

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
.stat-card {
  @apply rounded-2xl border border-gray-200/70 bg-white/70 px-4 py-5 dark:border-dark-700/70 dark:bg-dark-800/50;
}
.stat-num {
  @apply text-2xl font-extrabold text-primary-600 dark:text-primary-400 sm:text-3xl;
}
.stat-label {
  @apply mt-1 text-xs text-gray-500 dark:text-dark-400;
}
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
.why-card {
  @apply rounded-2xl border border-gray-200/70 bg-white/60 p-7 dark:border-dark-700/70 dark:bg-dark-800/40;
}
.why-icon {
  @apply mb-4 flex h-11 w-11 items-center justify-center rounded-xl;
}
.why-title {
  @apply mb-2 text-base font-semibold text-gray-900 dark:text-white;
}
.why-desc {
  @apply text-sm leading-relaxed text-gray-600 dark:text-dark-400;
}
.provider-chip {
  @apply inline-flex items-center gap-2 rounded-xl border border-gray-200 bg-white/70 px-4 py-2.5 text-sm font-medium text-gray-700 dark:border-dark-700 dark:bg-dark-800/60 dark:text-dark-200;
}
</style>
