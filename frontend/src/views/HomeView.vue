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
        <a
          v-if="showChat"
          :href="chatStationUrl"
          target="_blank"
          rel="noopener noreferrer"
          class="inline-flex items-center gap-2 rounded-full bg-gradient-to-r from-fuchsia-600 to-orange-500 px-7 py-3.5 text-sm font-semibold text-white shadow-lg shadow-fuchsia-600/25 ring-1 ring-white/30 transition hover:from-fuchsia-700 hover:to-orange-600 hover:shadow-orange-500/30 dark:ring-white/10"
          @click="handleChatStationClick"
        >
          {{ t('portal.hero.goToChat') }}
          <Icon name="chat" size="md" :stroke-width="2" />
        </a>
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
          <div class="stat-num">2+</div>
          <div class="stat-label">{{ t('portal.stats.providers') }}</div>
        </div>
        <div class="stat-card">
          <div class="stat-num">0.1x</div>
          <div class="stat-label">{{ t('portal.stats.lowestRate') }}</div>
        </div>
        <div class="stat-card">
          <div class="stat-num">{{ t('portal.stats.permanentValue') }}</div>
          <div class="stat-label">{{ t('portal.stats.balance') }}</div>
        </div>
      </div>
    </section>

    <!-- ===================== Global access ===================== -->
    <section class="py-16">
      <div class="mb-10 text-center">
        <h2 class="text-2xl font-bold sm:text-3xl">{{ t('portal.global.title') }}</h2>
        <p class="mx-auto mt-3 max-w-xl text-sm leading-relaxed text-gray-500 dark:text-dark-400">{{ t('portal.global.subtitle') }}</p>
      </div>

      <div class="relative mx-auto max-w-4xl">
        <!-- 点阵世界地图（原创：径向点阵 + 连线节点动画） -->
        <div class="map-dots relative aspect-[2/1] w-full rounded-3xl">
          <svg viewBox="0 0 800 400" class="absolute inset-0 h-full w-full" fill="none" xmlns="http://www.w3.org/2000/svg">
            <defs>
              <linearGradient id="link-line" x1="0" y1="0" x2="1" y2="0">
                <stop offset="0" stop-color="#2dd4bf" stop-opacity="0.1"/>
                <stop offset="0.5" stop-color="#0ea5e9" stop-opacity="0.9"/>
                <stop offset="1" stop-color="#2dd4bf" stop-opacity="0.1"/>
              </linearGradient>
            </defs>
            <!-- 连线 -->
            <g stroke="url(#link-line)" stroke-width="1.6" fill="none" stroke-linecap="round">
              <path class="link-path" d="M150,150 Q330,40 420,210" />
              <path class="link-path" style="animation-delay:.6s" d="M420,210 Q560,120 660,180" />
              <path class="link-path" style="animation-delay:1.2s" d="M150,150 Q260,300 420,210" />
              <path class="link-path" style="animation-delay:1.8s" d="M660,180 Q700,260 600,300" />
            </g>
            <!-- 节点 -->
            <g>
              <circle class="node" cx="150" cy="150" r="5" />
              <circle class="node" style="animation-delay:.4s" cx="420" cy="210" r="6" />
              <circle class="node" style="animation-delay:.8s" cx="660" cy="180" r="5" />
              <circle class="node" style="animation-delay:1.2s" cx="600" cy="300" r="4" />
              <circle class="node" style="animation-delay:1.6s" cx="260" cy="250" r="4" />
            </g>
          </svg>
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
import { lobeHubSSOAPI } from '@/api'

const { t } = useI18n()

const authStore = useAuthStore()
const appStore = useAppStore()

const siteSubtitle = computed(() => appStore.cachedPublicSettings?.site_subtitle || '')
const homeContent = computed(() => appStore.cachedPublicSettings?.home_content || '')
const chatStationUrl = computed(() => appStore.cachedPublicSettings?.chat_station_url || '')

const isHomeContentUrl = computed(() => {
  const content = homeContent.value.trim()
  return content.startsWith('http://') || content.startsWith('https://')
})

const isAuthenticated = computed(() => authStore.isAuthenticated)
const isAdmin = computed(() => authStore.isAdmin)
const dashboardPath = computed(() => (isAdmin.value ? '/admin/dashboard' : '/dashboard'))
const showChat = computed(() => Boolean(chatStationUrl.value.trim()))

function initTheme() {
  const savedTheme = localStorage.getItem('theme')
  if (
    savedTheme === 'dark' ||
    (!savedTheme && window.matchMedia('(prefers-color-scheme: dark)').matches)
  ) {
    document.documentElement.classList.add('dark')
  }
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

onMounted(() => {
  initTheme()
  authStore.checkAuth()
  appStore.fetchPublicSettings(true)
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
/* 点阵世界地图背景：用径向点阵 mask 出地图质感 */
.map-dots {
  background-image: radial-gradient(currentColor 1px, transparent 1.4px);
  background-size: 14px 14px;
  color: rgba(100, 116, 139, 0.25);
}
:global(.dark) .map-dots {
  color: rgba(148, 163, 184, 0.18);
}

/* 连线流光 */
.link-path {
  stroke-dasharray: 6 10;
  animation: link-flow 3s linear infinite;
}
@keyframes link-flow {
  to {
    stroke-dashoffset: -160;
  }
}

/* 节点脉冲 */
.node {
  fill: #0ea5e9;
  transform-box: fill-box;
  transform-origin: center;
  animation: node-pulse 2.4s ease-in-out infinite;
}
@keyframes node-pulse {
  0%,
  100% {
    opacity: 0.55;
    r: 4;
  }
  50% {
    opacity: 1;
    r: 6;
  }
}
</style>
