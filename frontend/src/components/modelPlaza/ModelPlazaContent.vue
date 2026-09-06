<template>
  <div class="model-plaza" :class="{ 'plaza-portal': portal }">
    <div class="plaza-page-heading">
      <div>
        <p v-if="portal" class="plaza-eyebrow">MODEL PLAZA</p>
        <h1>{{ t('modelPlaza.title') }}</h1>
        <p class="plaza-page-description">{{ t('modelPlaza.cards.description') }}</p>
      </div>
      <span class="plaza-unit">{{ t('modelPlaza.cards.currencyUnit') }}</span>
    </div>
    <div v-if="descriptionHtml" class="plaza-description" v-html="descriptionHtml"></div>
    <p v-if="!isAuthenticated" class="plaza-anonymous">{{ t('modelPlaza.anonymousHint') }}</p>
    <div v-if="loading" class="plaza-state" role="status" :aria-label="t('modelPlaza.loading')">
      <div class="plaza-skeleton-grid" aria-hidden="true"><div v-for="n in 4" :key="n" class="plaza-skeleton"></div></div>
      <span>{{ t('modelPlaza.loading') }}</span>
    </div>
    <div v-else-if="error" class="plaza-state" role="alert">
      <p>{{ t('modelPlaza.loadFailed') }}</p>
      <button type="button" class="plaza-retry" @click="$emit('retry')">{{ t('modelPlaza.cards.retry') }}</button>
    </div>
    <div v-else-if="!groups.length" class="plaza-state">{{ t('modelPlaza.empty') }}</div>
    <template v-else>
      <nav class="plaza-group-tabs" :aria-label="t('modelPlaza.filters.groupLabel')">
        <button type="button" :aria-pressed="selectedGroupId === 'all'" @click="selectedGroupId = 'all'">{{ t('modelPlaza.cards.allGroups') }}</button>
        <button v-for="group in groups" :key="group.id" type="button" :aria-pressed="selectedGroupId === group.id" @click="selectedGroupId = group.id">
          {{ group.name }} <span class="plaza-rate">× {{ plazaGroupRate(group) }}</span>
          <span v-if="group.is_exclusive" class="plaza-exclusive">{{ t('modelPlaza.badges.exclusive') }}</span>
        </button>
      </nav>
      <div class="plaza-toolbar">
        <label class="plaza-search">
          <Icon name="search" size="sm" />
          <input v-model="searchQuery" type="search" :aria-label="t('modelPlaza.cards.search')" :placeholder="t('modelPlaza.cards.search')" />
        </label>
        <details class="plaza-filter-menu">
          <summary>{{ t('modelPlaza.cards.filters') }}</summary>
          <div class="plaza-filter-fields">
            <label>{{ t('modelPlaza.filters.platformLabel') }}
              <select v-model="selectedPlatform"><option value="all">{{ t('modelPlaza.filters.all') }}</option><option v-for="p in platforms" :key="p" :value="p">{{ plazaProvider(p) }}</option></select>
            </label>
            <label>{{ t('modelPlaza.filters.rateLabel') }}
              <select v-model="selectedRate"><option value="all">{{ t('modelPlaza.filters.all') }}</option><option v-for="rate in rates" :key="rate" :value="rate">× {{ rate }}</option></select>
            </label>
          </div>
        </details>
        <span class="plaza-result-count" aria-live="polite">{{ t('modelPlaza.cards.modelCount', { count: modelCount }) }}</span>
      </div>
      <div v-if="!modelCount" class="plaza-state">{{ t('modelPlaza.noSearchResult') }}</div>
      <section v-for="group in filteredGroups" :key="group.id" class="plaza-group-section" :aria-label="group.name">
        <div v-if="selectedGroupId === 'all' || group.description || group.user_rate_multiplier != null || group.subscription_type === 'subscription'" class="plaza-group-summary">
          <h2 v-if="selectedGroupId === 'all'">{{ group.name }} <span class="plaza-rate">× {{ plazaGroupRate(group) }}</span></h2>
          <p v-if="group.description">{{ group.description }}</p>
          <p v-if="group.user_rate_multiplier != null">{{ t('modelPlaza.cards.personalRate', { rate: plazaGroupRate(group), defaultRate: group.rate_multiplier }) }}</p>
          <p v-if="group.subscription_type === 'subscription'">{{ t('modelPlaza.badges.subscription') }}</p>
        </div>
        <div class="plaza-model-grid">
          <PlazaModelCard v-for="model in group.models" :key="`${model.platform}:${model.name}`" :model="model" :group="group" @details="openDetails(group, model)" />
        </div>
      </section>
      <p class="plaza-pricing-note">{{ t('modelPlaza.cards.pricingNote') }}</p>
    </template>
    <dialog ref="dialog" class="plaza-detail-dialog" :class="{ dark: portal }" aria-labelledby="plaza-detail-title" @close="selectedDetail = null" @click="closeOnBackdrop">
      <header class="plaza-dialog-heading">
        <div><h2 id="plaza-detail-title">{{ selectedDetail?.model.name }} · {{ t('modelPlaza.cards.details') }}</h2><p>{{ t('modelPlaza.cards.standardPriceNote') }}</p></div>
        <button type="button" :aria-label="t('common.close')" @click="closeDetails"><Icon name="x" size="md" /></button>
      </header>
      <div v-if="selectedDetail" class="plaza-dialog-body"><PlazaGroupSection :group="{ ...selectedDetail.group, models: [selectedDetail.model] }" /></div>
    </dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { marked } from 'marked'
import DOMPurify from 'dompurify'
import Icon from '@/components/icons/Icon.vue'
import PlazaModelCard from './PlazaModelCard.vue'
import PlazaGroupSection from './PlazaGroupSection.vue'
import type { ModelPlazaGroup, ModelPlazaResponse, PlazaModel } from '@/api/modelPlaza'
import { useAuthStore } from '@/stores/auth'
import { plazaGroupRate, plazaProvider, sortPlazaModels } from '@/utils/modelPlazaPricing'
import '@/styles/model-plaza.css'

const props = defineProps<{ response: ModelPlazaResponse | null; loading: boolean; error?: boolean; embedded?: boolean; portal?: boolean }>()
defineEmits<{ retry: [] }>()
const { t } = useI18n()
const authStore = useAuthStore()
const isAuthenticated = computed(() => authStore.isAuthenticated)
const selectedGroupId = ref<number | 'all' | null>(null)
const selectedPlatform = ref('all')
const selectedRate = ref<number | 'all'>('all')
const searchQuery = ref('')
const dialog = ref<HTMLDialogElement | null>(null)
const selectedDetail = ref<{ group: ModelPlazaGroup; model: PlazaModel } | null>(null)
const descriptionHtml = computed(() => DOMPurify.sanitize(marked.parse(props.response?.description?.trim() ?? '') as string))
const groups = computed(() => [...(props.response?.groups ?? [])].sort((a, b) => plazaGroupRate(a) - plazaGroupRate(b) || a.name.localeCompare(b.name)))
const platforms = computed(() => [...new Set(groups.value.flatMap(g => g.models.map(m => m.platform)))].sort())
const rates = computed(() => [...new Set(groups.value.map(plazaGroupRate))].sort((a, b) => a - b))
watch(groups, list => {
  if (selectedGroupId.value !== 'all' && !list.some(g => g.id === selectedGroupId.value)) selectedGroupId.value = list[0]?.id ?? null
}, { immediate: true })
watch(platforms, list => { if (!list.includes(selectedPlatform.value)) selectedPlatform.value = 'all' })
watch(rates, list => { if (selectedRate.value !== 'all' && !list.includes(selectedRate.value)) selectedRate.value = 'all' })
const filteredGroups = computed(() => {
  const query = searchQuery.value.trim().toLowerCase()
  return groups.value.filter(g => (selectedGroupId.value === 'all' || g.id === selectedGroupId.value) && (selectedRate.value === 'all' || plazaGroupRate(g) === selectedRate.value))
    .map(g => ({ ...g, models: sortPlazaModels(g.models.filter(m => (selectedPlatform.value === 'all' || m.platform === selectedPlatform.value) && `${m.name} ${m.platform} ${plazaProvider(m.platform)}`.toLowerCase().includes(query))) }))
    .filter(g => g.models.length > 0)
})
const modelCount = computed(() => filteredGroups.value.reduce((sum, g) => sum + g.models.length, 0))
async function openDetails(group: ModelPlazaGroup, model: PlazaModel) {
  selectedDetail.value = { group, model }
  await nextTick()
  if (selectedDetail.value && !dialog.value?.open) dialog.value?.showModal()
}
function closeDetails() { dialog.value?.close(); selectedDetail.value = null }
function closeOnBackdrop(event: MouseEvent) {
  if (event.target !== dialog.value || !dialog.value) return
  const rect = dialog.value.getBoundingClientRect()
  if (event.clientX < rect.left || event.clientX > rect.right || event.clientY < rect.top || event.clientY > rect.bottom) closeDetails()
}
watch([() => props.response, selectedGroupId], closeDetails)
onBeforeUnmount(closeDetails)
</script>
