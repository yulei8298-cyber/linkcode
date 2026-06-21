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

    <!-- ===================== Pricing ===================== -->
    <section class="border-t border-gray-200/70 py-16 dark:border-dark-800/70">
      <div class="mb-10 text-center">
        <p class="text-xs font-semibold uppercase tracking-[0.24em] text-gray-400 dark:text-dark-500">
          {{ pricingConfig.eyebrow }}
        </p>
        <h2 class="mt-4 text-3xl font-extrabold tracking-tight sm:text-4xl">
          {{ pricingConfig.title }}
        </h2>
        <p class="mx-auto mt-3 max-w-3xl text-sm leading-relaxed text-gray-500 dark:text-dark-400 sm:text-base">
          {{ pricingConfig.subtitle }}
        </p>
      </div>

      <div class="rounded-2xl border border-gray-200 bg-white/80 p-5 shadow-sm dark:border-dark-700 dark:bg-dark-900/70 sm:p-7">
        <div class="grid gap-6 lg:grid-cols-[1.2fr_0.8fr] lg:items-center">
          <div>
            <p class="text-sm font-medium text-gray-500 dark:text-dark-400">
              {{ pricingConfig.rechargeLabel }}
            </p>
            <div class="mt-3 flex flex-wrap items-end gap-3">
              <span class="text-4xl font-extrabold text-gray-950 dark:text-white sm:text-5xl">
                {{ formatAmount(pricingConfig.yuanAmount) }} ¥
              </span>
              <span class="pb-2 text-2xl font-semibold text-gray-400">=</span>
              <span class="text-4xl font-extrabold text-emerald-500 sm:text-5xl">
                {{ formatAmount(pricingConfig.usdAmount) }} $
              </span>
              <span class="pb-2 text-sm text-gray-500 dark:text-dark-400">
                {{ pricingConfig.creditUnitLabel }}
              </span>
            </div>
            <p class="mt-4 text-sm text-gray-500 dark:text-dark-400">
              {{ pricingConfig.exampleText }}
            </p>
            <p class="mt-3 text-sm font-semibold text-amber-500">
              {{ pricingConfig.highlightText }}
            </p>
          </div>

          <div class="space-y-4 lg:text-right">
            <p class="text-sm font-medium text-gray-500 dark:text-dark-400">
              {{ pricingConfig.recommendedAmountLabel }}
            </p>
            <div class="flex flex-wrap gap-2 lg:justify-end">
              <span
                v-for="amount in pricingConfig.recommendedAmounts"
                :key="amount"
                class="inline-flex min-w-20 items-center justify-center rounded-xl border border-gray-300 px-4 py-2 text-sm font-semibold text-gray-700 dark:border-dark-600 dark:text-dark-200"
              >
                ¥ {{ formatAmount(amount) }}
              </span>
            </div>
            <a
              :href="pricingConfig.rechargeButtonUrl || '/payment'"
              class="inline-flex items-center justify-center rounded-xl bg-gray-950 px-6 py-3 text-sm font-semibold text-white transition hover:bg-gray-800 dark:bg-white dark:text-gray-950 dark:hover:bg-gray-200"
            >
              {{ pricingConfig.rechargeButtonText }}
            </a>
          </div>
        </div>
      </div>

      <div class="mt-6 rounded-2xl border border-gray-200 bg-white/75 p-5 dark:border-dark-700 dark:bg-dark-900/60 sm:p-6">
        <div class="mb-4">
          <h3 class="text-lg font-bold text-gray-950 dark:text-white">
            {{ pricingConfig.benefitsTitle }}
          </h3>
          <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">
            {{ pricingConfig.benefitsDescription }}
          </p>
        </div>
        <div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
          <div
            v-for="benefit in pricingConfig.benefits"
            :key="benefit"
            class="flex items-start gap-2 rounded-xl border border-emerald-100 bg-emerald-50/70 p-3 text-sm text-emerald-700 dark:border-emerald-500/20 dark:bg-emerald-500/10 dark:text-emerald-300"
          >
            <Icon name="check" size="sm" class="mt-0.5 flex-shrink-0" />
            <span>{{ benefit }}</span>
          </div>
        </div>
      </div>

      <div class="mt-12">
        <div class="mb-6 flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
          <div>
            <h3 class="text-2xl font-bold text-gray-950 dark:text-white">
              {{ pricingConfig.channelTitle }}
            </h3>
            <p class="mt-2 max-w-3xl text-sm leading-relaxed text-gray-500 dark:text-dark-400">
              {{ pricingConfig.channelDescription }}
            </p>
          </div>
          <router-link
            to="/portal/pricing"
            class="inline-flex items-center gap-1 text-sm font-semibold text-gray-600 transition hover:text-gray-950 dark:text-dark-300 dark:hover:text-white"
          >
            {{ pricingConfig.channelDetailText }}
            <Icon name="arrowRight" size="sm" />
          </router-link>
        </div>

        <div v-if="pricingLoading" class="flex items-center justify-center py-10">
          <Icon name="refresh" size="lg" class="animate-spin text-gray-400" />
        </div>
        <div
          v-else-if="publicChannelGroups.length === 0"
          class="rounded-2xl border border-dashed border-gray-300 py-10 text-center text-sm text-gray-500 dark:border-dark-700 dark:text-dark-400"
        >
          {{ pricingConfig.channelEmptyText }}
        </div>
        <div v-else class="space-y-8">
          <div v-for="section in publicChannelGroups" :key="section.platform">
            <div class="mb-3 flex items-center gap-2">
              <span class="text-sm font-bold uppercase tracking-wide text-gray-500 dark:text-dark-400">
                {{ section.platform }}
              </span>
              <span class="text-xs text-gray-400">({{ section.groups.length }})</span>
            </div>
            <div class="grid gap-4 md:grid-cols-2">
              <div
                v-for="group in section.groups"
                :key="`${section.platform}-${group.id}`"
                class="rounded-2xl border border-gray-200 bg-white/80 p-5 dark:border-dark-700 dark:bg-dark-900/70"
              >
                <div class="flex items-start justify-between gap-3">
                  <div class="min-w-0">
                    <h4 class="truncate text-base font-bold text-gray-950 dark:text-white">
                      {{ group.name }}
                    </h4>
                    <p class="mt-2 text-sm text-gray-500 dark:text-dark-400">
                      {{ describeGroupRate(group.rate_multiplier) }}
                    </p>
                  </div>
                  <span class="rounded-full bg-gray-100 px-3 py-1 text-xs font-bold uppercase text-gray-600 dark:bg-dark-800 dark:text-dark-300">
                    {{ section.platform }}
                  </span>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </section>
  </PortalLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore, useAppStore } from '@/stores'
import PortalLayout from '@/views/public/components/PortalLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import { getPublicPricing } from '@/api/public'
import type { UserAvailableChannel, UserAvailableGroup } from '@/api/channels'
import { parsePricingDisplayConfig } from '@/utils/pricingDisplayConfig'
import { extractApiErrorMessage } from '@/utils/apiError'

const { t } = useI18n()

const authStore = useAuthStore()
const appStore = useAppStore()

const siteLogo = computed(() => appStore.cachedPublicSettings?.site_logo || appStore.siteLogo || '')
const siteSubtitle = computed(() => appStore.cachedPublicSettings?.site_subtitle || '')
const homeContent = computed(() => appStore.cachedPublicSettings?.home_content || '')
const pricingConfig = computed(() =>
  parsePricingDisplayConfig(appStore.cachedPublicSettings?.pricing_display_config || ''),
)
const publicPricingChannels = ref<UserAvailableChannel[]>([])
const pricingLoading = ref(false)

const isHomeContentUrl = computed(() => {
  const content = homeContent.value.trim()
  return content.startsWith('http://') || content.startsWith('https://')
})

const isAuthenticated = computed(() => authStore.isAuthenticated)
const isAdmin = computed(() => authStore.isAdmin)
const dashboardPath = computed(() => (isAdmin.value ? '/admin/dashboard' : '/dashboard'))

const publicChannelGroups = computed(() => {
  if (!pricingConfig.value.showPublicChannelGroups) return []
  const sections = new Map<string, UserAvailableGroup[]>()
  for (const channel of publicPricingChannels.value) {
    for (const platform of channel.platforms) {
      const publicGroups = platform.groups.filter((group) => !group.is_exclusive)
      if (publicGroups.length === 0) continue
      const current = sections.get(platform.platform) || []
      for (const group of publicGroups) {
        if (!current.some((item) => item.id === group.id)) {
          current.push(group)
        }
      }
      sections.set(platform.platform, current)
    }
  }
  return Array.from(sections.entries())
    .map(([platform, groups]) => ({
      platform,
      groups: groups.sort((a, b) => a.rate_multiplier - b.rate_multiplier || a.name.localeCompare(b.name)),
    }))
    .sort((a, b) => a.platform.localeCompare(b.platform))
})

function initTheme() {
  const savedTheme = localStorage.getItem('theme')
  if (
    savedTheme === 'dark' ||
    (!savedTheme && window.matchMedia('(prefers-color-scheme: dark)').matches)
  ) {
    document.documentElement.classList.add('dark')
  }
}

function formatAmount(value: number): string {
  if (!Number.isFinite(value)) return '0'
  return Number.isInteger(value) ? String(value) : value.toFixed(2).replace(/\.?0+$/, '')
}

function describeGroupRate(multiplier: number): string {
  const yuanPerDollar = multiplier * pricingConfig.value.yuanAmount
  return pricingConfig.value.channelGroupDescriptionTemplate
    .replace('{multiplier}', formatAmount(multiplier))
    .replace('{price}', formatAmount(yuanPerDollar))
}

async function loadPublicPricing() {
  pricingLoading.value = true
  try {
    publicPricingChannels.value = await getPublicPricing()
  } catch (error: unknown) {
    appStore.showError(extractApiErrorMessage(error, t('common.error')))
  } finally {
    pricingLoading.value = false
  }
}

onMounted(() => {
  initTheme()
  authStore.checkAuth()
  if (!appStore.publicSettingsLoaded) {
    appStore.fetchPublicSettings()
  }
  loadPublicPricing()
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
.provider-chip {
  @apply inline-flex items-center gap-2 rounded-xl border border-gray-200 bg-white/70 px-4 py-2.5 text-sm font-medium text-gray-700 dark:border-dark-700 dark:bg-dark-800/60 dark:text-dark-200;
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
