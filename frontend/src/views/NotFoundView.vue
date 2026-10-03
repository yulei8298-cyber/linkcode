<template>
  <main class="nf">
    <div class="nf-box">
      <p class="nf-code" aria-hidden="true">404</p>
      <h1 class="nf-title">{{ t('errors.pageNotFound') }}</h1>
      <p class="nf-hint">{{ t('errors.pageNotFoundHint') }}</p>
      <p class="nf-path lc-mono">{{ route.fullPath }}</p>

      <div class="nf-actions">
        <button type="button" class="btn btn-secondary" @click="goBack">
          <Icon name="arrowLeft" size="sm" class="mr-1.5" />
          {{ t('errors.goBack') }}
        </button>
        <RouterLink to="/dashboard" class="btn btn-primary">{{ t('errors.backDashboard') }}</RouterLink>
      </div>

      <p v-if="qqGroupNumber" class="nf-help">QQ 群 <span class="lc-mono">{{ qqGroupNumber }}</span></p>
    </div>
  </main>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore } from '@/stores/app'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const appStore = useAppStore()
const qqGroupNumber = computed(() =>
  (appStore.cachedPublicSettings?.qq_group?.trim() || '').replace(/^QQ\s*(?:群)?\s*[:：]?\s*/i, '')
)

function goBack(): void {
  router.back()
}
</script>

<style scoped>
.nf {
  display: grid;
  min-height: 100vh;
  place-items: center;
  padding: 24px 16px;
  background: var(--lc-bg);
  color: var(--lc-ink);
}

.nf-box {
  width: min(440px, 100%);
}

.nf-code {
  font-family: var(--lc-font-mono);
  font-size: 64px;
  font-weight: 600;
  line-height: 1;
  letter-spacing: -0.04em;
  color: var(--lc-ink-3);
}

.nf-title {
  margin-top: 20px;
  font-size: 22px;
  font-weight: 700;
}

.nf-hint {
  margin-top: 8px;
  font-size: 14px;
  color: var(--lc-ink-2);
}

.nf-path {
  margin-top: 16px;
  padding: 8px 12px;
  border: 1px solid var(--lc-line);
  border-radius: 8px;
  background: var(--lc-surface);
  font-size: 12.5px;
  color: var(--lc-ink-2);
  overflow-wrap: anywhere;
}

.nf-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-top: 24px;
}

.nf-help {
  margin-top: 28px;
  font-size: 12px;
  color: var(--lc-ink-3);
}
</style>
