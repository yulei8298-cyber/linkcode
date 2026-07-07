<template>
  <div
    class="relative flex min-h-screen flex-col bg-white text-gray-900 dark:bg-dark-950 dark:text-white"
  >
    <!-- Dotted grid background + soft color blooms -->
    <div class="pointer-events-none fixed inset-0 z-0 overflow-hidden">
      <div
        class="absolute inset-0 bg-[radial-gradient(circle,rgba(0,0,0,0.045)_1px,transparent_1px)] bg-[size:22px_22px] dark:bg-[radial-gradient(circle,rgba(255,255,255,0.05)_1px,transparent_1px)]"
      ></div>
      <!-- 顶部柔和渐变光斑：青绿 / 蓝 / 紫，整体很淡 -->
      <div
        class="absolute left-1/2 top-0 h-[520px] w-[820px] -translate-x-1/2 rounded-full bg-gradient-to-br from-primary-300/25 via-sky-300/20 to-purple-300/20 blur-3xl dark:from-primary-500/15 dark:via-sky-500/10 dark:to-purple-500/10"
      ></div>
      <div
        class="absolute -right-40 -top-40 h-96 w-96 rounded-full bg-primary-400/10 blur-3xl"
      ></div>
      <div
        class="absolute -bottom-40 -left-40 h-96 w-96 rounded-full bg-sky-400/10 blur-3xl"
      ></div>
    </div>

    <!-- Header -->
    <header
      class="sticky top-0 z-30 border-b border-gray-200/70 bg-white/80 backdrop-blur-md dark:border-dark-800/70 dark:bg-dark-950/80"
    >
      <nav class="mx-auto flex h-16 max-w-6xl items-center justify-between px-4 sm:px-6">
        <!-- Logo -->
        <RouterLink to="/home" class="flex min-w-0 items-center gap-2.5">
          <span
            class="flex h-9 w-9 flex-shrink-0 items-center justify-center overflow-hidden rounded-xl bg-white shadow-sm ring-1 ring-gray-200 dark:bg-dark-800 dark:ring-dark-700"
          >
            <img :src="siteLogo || '/logo.svg'" alt="Logo" class="h-full w-full object-contain" />
          </span>
          <span class="hidden truncate text-base font-semibold sm:inline">{{ siteName }}</span>
        </RouterLink>

        <!-- Center nav (desktop) -->
        <div class="hidden items-center gap-1 md:flex">
          <RouterLink
            v-if="showStatus"
            to="/portal/status"
            class="nav-link"
            active-class="nav-link-active"
          >
            {{ t('portal.nav.status') }}
          </RouterLink>
          <RouterLink
            v-if="showPricing"
            to="/portal/pricing"
            class="nav-link"
            active-class="nav-link-active"
          >
            {{ t('portal.nav.pricing') }}
          </RouterLink>
          <RouterLink
            v-if="showTutorial"
            to="/portal/tutorial"
            class="nav-link"
            active-class="nav-link-active"
          >
            {{ t('portal.nav.tutorial') }}
          </RouterLink>
          <a
            v-if="showChat"
            :href="chatStationUrl"
            target="_blank"
            rel="noopener noreferrer"
            class="nav-link"
            @click="handleChatStationClick"
          >
            {{ t('portal.nav.chat') }}
          </a>
          <span
            v-if="showChat"
            class="inline-flex items-center gap-1.5 rounded-full border border-sky-200 bg-sky-50 px-3 py-1 text-xs font-semibold text-sky-700 dark:border-sky-500/30 dark:bg-sky-500/10 dark:text-sky-200"
          >
            <Icon name="users" size="sm" :stroke-width="2" />
            QQ 群 1025176993
          </span>
        </div>

        <!-- Right actions -->
        <div class="flex items-center gap-2">
          <LocaleSwitcher />
          <button
            @click="toggleTheme"
            class="rounded-lg p-2 text-gray-500 transition-colors hover:bg-gray-100 hover:text-gray-700 dark:text-dark-400 dark:hover:bg-dark-800 dark:hover:text-white"
            :title="isDark ? t('home.switchToLight') : t('home.switchToDark')"
          >
            <Icon v-if="isDark" name="sun" size="md" />
            <Icon v-else name="moon" size="md" />
          </button>
          <RouterLink
            v-if="isAuthenticated"
            :to="dashboardPath"
            class="inline-flex items-center rounded-full bg-gray-900 px-3.5 py-1.5 text-xs font-medium text-white transition-colors hover:bg-gray-800 dark:bg-gray-800 dark:hover:bg-gray-700"
          >
            {{ t('home.dashboard') }}
          </RouterLink>
          <RouterLink
            v-else
            to="/login"
            class="inline-flex items-center rounded-full bg-primary-600 px-3.5 py-1.5 text-xs font-medium text-white shadow-sm shadow-primary-600/20 transition-colors hover:bg-primary-700"
          >
            {{ t('home.login') }}
          </RouterLink>
        </div>
      </nav>

      <!-- Mobile nav row -->
      <div
        v-if="hasAnyNav"
        class="flex items-center gap-1 overflow-x-auto border-t border-gray-100 px-4 py-2 md:hidden dark:border-dark-800"
      >
        <RouterLink v-if="showStatus" to="/portal/status" class="nav-link-mobile" active-class="nav-link-active">{{ t('portal.nav.status') }}</RouterLink>
        <RouterLink v-if="showPricing" to="/portal/pricing" class="nav-link-mobile" active-class="nav-link-active">{{ t('portal.nav.pricing') }}</RouterLink>
        <RouterLink v-if="showTutorial" to="/portal/tutorial" class="nav-link-mobile" active-class="nav-link-active">{{ t('portal.nav.tutorial') }}</RouterLink>
        <a v-if="showChat" :href="chatStationUrl" target="_blank" rel="noopener noreferrer" class="nav-link-mobile" @click="handleChatStationClick">{{ t('portal.nav.chat') }}</a>
        <span
          v-if="showChat"
          class="inline-flex shrink-0 items-center gap-1.5 rounded-full border border-sky-200 bg-sky-50 px-3 py-1.5 text-xs font-semibold text-sky-700 dark:border-sky-500/30 dark:bg-sky-500/10 dark:text-sky-200"
        >
          <Icon name="users" size="sm" :stroke-width="2" />
          QQ 群 1025176993
        </span>
      </div>
    </header>

    <!-- Main content -->
    <main class="relative z-10 mx-auto w-full max-w-6xl flex-1 px-4 py-8 sm:px-6">
      <slot />
    </main>

    <!-- Footer -->
    <footer class="relative z-10 border-t border-gray-200/70 px-4 py-8 dark:border-dark-800/70">
      <div
        class="mx-auto flex max-w-6xl flex-col items-center justify-center gap-3 text-center sm:flex-row sm:justify-between sm:text-left"
      >
        <p class="text-sm text-gray-500 dark:text-dark-400">
          &copy; {{ currentYear }} {{ siteName }}
        </p>
        <div class="flex items-center gap-4">
          <a
            v-if="docUrl"
            :href="docUrl"
            target="_blank"
            rel="noopener noreferrer"
            class="text-sm text-gray-500 transition-colors hover:text-gray-700 dark:text-dark-400 dark:hover:text-white"
          >
            {{ t('home.docs') }}
          </a>
        </div>
      </div>
    </footer>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink } from 'vue-router'
import { useAuthStore, useAppStore } from '@/stores'
import { lobeHubSSOAPI } from '@/api'
import LocaleSwitcher from '@/components/common/LocaleSwitcher.vue'
import Icon from '@/components/icons/Icon.vue'

const { t } = useI18n()
const authStore = useAuthStore()
const appStore = useAppStore()

const siteName = computed(() => appStore.cachedPublicSettings?.site_name || appStore.siteName || 'linkcode')
const siteLogo = computed(() => appStore.cachedPublicSettings?.site_logo || appStore.siteLogo || '')
const docUrl = computed(() => appStore.cachedPublicSettings?.doc_url || appStore.docUrl || '')
const chatStationUrl = computed(() => appStore.cachedPublicSettings?.chat_station_url || '')

// 状态/定价/教程入口始终显示（页面本身在无数据时有空状态提示）；
// 对话站仅在后台配置了 URL 后才显示。
const showStatus = computed(() => true)
const showPricing = computed(() => true)
const showTutorial = computed(() => true)
const showChat = computed(() => Boolean(chatStationUrl.value.trim()))
const hasAnyNav = computed(() => showStatus.value || showPricing.value || showTutorial.value || showChat.value)

const isAuthenticated = computed(() => authStore.isAuthenticated)
const dashboardPath = computed(() => (authStore.isAdmin ? '/admin/dashboard' : '/dashboard'))

const currentYear = computed(() => new Date().getFullYear())

const isDark = ref(document.documentElement.classList.contains('dark'))
function toggleTheme() {
  isDark.value = !isDark.value
  document.documentElement.classList.toggle('dark', isDark.value)
  localStorage.setItem('theme', isDark.value ? 'dark' : 'light')
}
async function handleChatStationClick(event: MouseEvent) {
  const url = chatStationUrl.value.trim()
  if (!url || !isAuthenticated.value) {
    return
  }

  event.preventDefault()
  const chatWindow = window.open(url, '_blank')
  if (chatWindow) {
    chatWindow.opener = null
  }

  try {
    const result = await lobeHubSSOAPI.authorize('/')
    if (chatWindow) {
      chatWindow.location.replace(result.redirect_url || url)
    } else {
      window.open(result.redirect_url || url, '_blank', 'noopener,noreferrer')
    }
  } catch (error) {
    console.error('Failed to start LobeHub SSO:', error)
    if (!chatWindow) {
      window.open(url, '_blank', 'noopener,noreferrer')
    }
  }
}
</script>

<style scoped>
.nav-link {
  @apply rounded-lg px-3 py-2 text-sm font-medium text-gray-600 transition-colors hover:bg-gray-100 hover:text-gray-900 dark:text-dark-300 dark:hover:bg-dark-800 dark:hover:text-white;
}
.nav-link-mobile {
  @apply flex-shrink-0 rounded-lg px-3 py-1.5 text-sm font-medium text-gray-600 transition-colors hover:bg-gray-100 hover:text-gray-900 dark:text-dark-300 dark:hover:bg-dark-800 dark:hover:text-white;
}
.nav-link-active {
  @apply bg-primary-50 text-primary-700 dark:bg-primary-500/10 dark:text-primary-300;
}
</style>
