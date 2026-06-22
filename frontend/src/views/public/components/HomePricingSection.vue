<template>
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
              {{ pricingRange.yuan }}
            </span>
            <span class="pb-2 text-2xl font-semibold text-gray-400">=</span>
            <span class="text-4xl font-extrabold text-emerald-500 sm:text-5xl">
              {{ pricingRange.usd }}
            </span>
            <span class="pb-2 text-sm text-gray-500 dark:text-dark-400">
              {{ pricingConfig.creditUnitLabel }}
            </span>
          </div>
          <div
            v-if="pricingConfig.activityText"
            class="mt-4 inline-flex max-w-full items-center rounded-full border border-amber-200 bg-amber-50 px-3 py-1 text-xs font-semibold text-amber-700 dark:border-amber-500/25 dark:bg-amber-500/10 dark:text-amber-300"
          >
            <span class="mr-2 rounded-full bg-amber-500 px-1.5 py-0.5 text-[10px] uppercase leading-none text-white">
              {{ pricingConfig.activityLabel }}
            </span>
            <span class="truncate">{{ pricingConfig.activityText }}</span>
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
          v-if="showChannelDetailLink"
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
              class="overflow-hidden rounded-2xl border border-gray-200 bg-white/85 p-5 shadow-sm transition hover:-translate-y-0.5 hover:border-gray-300 hover:shadow-md dark:border-dark-700 dark:bg-dark-900/75 dark:hover:border-dark-600"
            >
              <div class="flex items-start gap-4">
                <div :class="['flex h-12 w-12 flex-shrink-0 items-center justify-center rounded-2xl', platformIconShellClass(section.platform)]">
                  <PlatformIcon
                    :platform="section.platform as GroupPlatform"
                    size="lg"
                    :class="platformIconTextClass(section.platform)"
                  />
                </div>
                <div class="min-w-0 flex-1">
                  <div class="flex flex-wrap items-center gap-2">
                    <h4 class="min-w-0 truncate text-base font-bold text-gray-950 dark:text-white">
                      {{ group.name }}
                    </h4>
                    <span :class="['rounded-full px-2.5 py-1 text-[11px] font-bold uppercase tracking-wide', platformBadgeClass(section.platform)]">
                      {{ platformLabel(section.platform) }}
                    </span>
                  </div>
                  <div class="mt-3 flex flex-wrap items-center gap-2">
                    <span class="rounded-full bg-gray-100 px-2.5 py-1 text-xs font-semibold text-gray-700 dark:bg-dark-800 dark:text-dark-200">
                      {{ formatAmount(group.rate_multiplier) }}x
                    </span>
                    <span class="text-sm text-gray-500 dark:text-dark-400">
                      {{ describeGroupRate(group.rate_multiplier) }}
                    </span>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores'
import Icon from '@/components/icons/Icon.vue'
import PlatformIcon from '@/components/common/PlatformIcon.vue'
import { getPublicPricing } from '@/api/public'
import type { UserAvailableChannel, UserAvailableGroup } from '@/api/channels'
import type { GroupPlatform } from '@/types'
import { parsePricingDisplayConfig } from '@/utils/pricingDisplayConfig'
import { extractApiErrorMessage } from '@/utils/apiError'

withDefaults(defineProps<{
  showChannelDetailLink?: boolean
}>(), {
  showChannelDetailLink: true,
})

const { t } = useI18n()
const appStore = useAppStore()

const pricingConfig = computed(() =>
  parsePricingDisplayConfig(appStore.cachedPublicSettings?.pricing_display_config || ''),
)
const publicPricingChannels = ref<UserAvailableChannel[]>([])
const pricingLoading = ref(false)

const pricingRange = computed(() => ({
  yuan: formatCurrencyRange(pricingConfig.value.yuanAmount, pricingConfig.value.yuanAmountMax, '¥'),
  usd: formatCurrencyRange(pricingConfig.value.usdAmount, pricingConfig.value.usdAmountMax, '$'),
}))

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

function formatAmount(value: number): string {
  if (!Number.isFinite(value)) return '0'
  return Number.isInteger(value) ? String(value) : value.toFixed(2).replace(/\.?0+$/, '')
}

function formatCurrencyRange(min: number, max: number | null, unit: string): string {
  const normalizedMax = max != null && Number.isFinite(max) && max !== min ? max : null
  const value = normalizedMax == null
    ? formatAmount(min)
    : `${formatAmount(Math.min(min, normalizedMax))} - ${formatAmount(Math.max(min, normalizedMax))}`
  return `${value} ${unit}`
}

function formatPriceRangeForMultiplier(multiplier: number): string {
  const min = multiplier * pricingConfig.value.yuanAmount
  const maxBase = pricingConfig.value.yuanAmountMax
  if (maxBase == null || maxBase === pricingConfig.value.yuanAmount) {
    return formatAmount(min)
  }
  return `${formatAmount(Math.min(min, multiplier * maxBase))} - ${formatAmount(Math.max(min, multiplier * maxBase))}`
}

function describeGroupRate(multiplier: number): string {
  return pricingConfig.value.channelGroupDescriptionTemplate
    .replace('{multiplier}', formatAmount(multiplier))
    .replace('{price}', formatPriceRangeForMultiplier(multiplier))
}

function platformLabel(platform: string): string {
  const labels: Record<string, string> = {
    anthropic: 'Claude',
    openai: 'OpenAI',
    gemini: 'Gemini',
    antigravity: 'Antigravity',
  }
  return labels[platform] || platform
}

function platformIconShellClass(platform: string): string {
  const classes: Record<string, string> = {
    anthropic: 'bg-orange-50 text-orange-500 ring-1 ring-orange-100 dark:bg-orange-500/10 dark:text-orange-300 dark:ring-orange-500/20',
    openai: 'bg-emerald-50 text-emerald-600 ring-1 ring-emerald-100 dark:bg-emerald-500/10 dark:text-emerald-300 dark:ring-emerald-500/20',
    gemini: 'bg-blue-50 text-blue-600 ring-1 ring-blue-100 dark:bg-blue-500/10 dark:text-blue-300 dark:ring-blue-500/20',
    antigravity: 'bg-violet-50 text-violet-600 ring-1 ring-violet-100 dark:bg-violet-500/10 dark:text-violet-300 dark:ring-violet-500/20',
  }
  return classes[platform] || 'bg-gray-100 text-gray-600 ring-1 ring-gray-200 dark:bg-dark-800 dark:text-dark-300 dark:ring-dark-700'
}

function platformIconTextClass(platform: string): string {
  const classes: Record<string, string> = {
    anthropic: 'text-orange-500 dark:text-orange-300',
    openai: 'text-emerald-600 dark:text-emerald-300',
    gemini: 'text-blue-600 dark:text-blue-300',
    antigravity: 'text-violet-600 dark:text-violet-300',
  }
  return classes[platform] || 'text-gray-600 dark:text-dark-300'
}

function platformBadgeClass(platform: string): string {
  const classes: Record<string, string> = {
    anthropic: 'bg-orange-100 text-orange-700 dark:bg-orange-500/15 dark:text-orange-300',
    openai: 'bg-emerald-100 text-emerald-700 dark:bg-emerald-500/15 dark:text-emerald-300',
    gemini: 'bg-blue-100 text-blue-700 dark:bg-blue-500/15 dark:text-blue-300',
    antigravity: 'bg-violet-100 text-violet-700 dark:bg-violet-500/15 dark:text-violet-300',
  }
  return classes[platform] || 'bg-gray-100 text-gray-600 dark:bg-dark-800 dark:text-dark-300'
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
  loadPublicPricing()
})
</script>
