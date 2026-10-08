<template>
  <section class="card">
    <div class="dash-card-head">
      <h2>{{ t('dashboard.quickActions') }}</h2>
    </div>
    <div class="qa">
      <!-- 接口地址：用户最常复制的内容放在最上面 -->
      <div class="qa-url">
        <span>{{ apiBase }}</span>
        <button type="button" class="qa-copy" :aria-label="`${t('common.copy')} ${apiBase}`" @click="copyBase">
          <Icon name="copy" size="xs" />{{ t('common.copy') }}
        </button>
      </div>
      <dl class="qa-eps">
        <div><dt>Claude</dt><dd>/v1/messages</dd></div>
        <div><dt>OpenAI</dt><dd>/v1/responses</dd></div>
      </dl>

      <div class="qa-actions">
        <button type="button" class="qa-action" @click="router.push('/keys')">
          <Icon name="key" size="md" class="qa-icon" />
          <span><b>{{ t('dashboard.createApiKey') }}</b><small>{{ t('dashboard.generateNewKey') }}</small></span>
        </button>
        <button type="button" class="qa-action" @click="router.push('/usage')">
          <Icon name="chart" size="md" class="qa-icon" />
          <span><b>{{ t('dashboard.viewUsage') }}</b><small>{{ t('dashboard.checkDetailedLogs') }}</small></span>
        </button>
        <button v-if="canUseBatchImage" type="button" class="qa-action" @click="router.push('/batch-image')">
          <Icon name="sparkles" size="md" class="qa-icon" />
          <span><b>{{ t('dashboard.batchImageAgent') }}</b><small>{{ t('dashboard.batchImageAgentDesc') }}</small></span>
        </button>
        <button type="button" class="qa-action" @click="router.push('/redeem')">
          <Icon name="gift" size="md" class="qa-icon" />
          <span><b>{{ t('dashboard.redeemCode') }}</b><small>{{ t('dashboard.addBalanceWithCode') }}</small></span>
        </button>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore } from '@/stores/app'
import { useClipboard } from '@/composables/useClipboard'
import { useBatchImageAccess } from '@/composables/useBatchImageAccess'

const router = useRouter()
const { t } = useI18n()
const appStore = useAppStore()
const { canUseBatchImage, refreshBatchImageAccess } = useBatchImageAccess()

const apiBase = computed(() => {
  const url = appStore.cachedPublicSettings?.api_base_url?.trim() || window.location.origin
  return url.replace(/\/+$/, '').replace(/\/v1$/i, '')
})

function copyBase() {
  void useClipboard().copyToClipboard(apiBase.value, t('dashboard.baseUrlCopied'))
}

onMounted(() => {
  void refreshBatchImageAccess()
})
</script>

<style scoped>
.qa {
  display: grid;
  gap: 12px;
  padding: 18px 20px 20px;
}

.qa-url {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  height: 40px;
  min-width: 0;
  padding: 0 6px 0 12px;
  border: 1px solid var(--lc-line);
  border-radius: 8px;
  background: var(--lc-bg-2);
  font: 13px var(--lc-font-mono);
  color: var(--lc-ink);
}

.qa-url span {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.qa-copy {
  display: inline-flex;
  flex: none;
  align-items: center;
  gap: 5px;
  height: 28px;
  padding: 0 8px;
  border: 1px solid transparent;
  border-radius: 6px;
  color: var(--lc-ink-3);
  font: 12px var(--lc-font-mono);
}

.qa-copy:hover {
  border-color: var(--lc-line-2);
  color: var(--lc-ink);
}

.qa-eps {
  border: 1px solid var(--lc-line);
  border-radius: 8px;
}

.qa-eps div {
  display: flex;
  justify-content: space-between;
  gap: 8px;
  padding: 8px 12px;
  border-top: 1px solid var(--lc-line);
  font-size: 13px;
}

.qa-eps div:first-child {
  border-top: 0;
}

.qa-eps dt {
  color: var(--lc-ink-2);
}

.qa-eps dd {
  font-family: var(--lc-font-mono);
  color: var(--lc-ink);
}

.qa-actions {
  display: grid;
  gap: 4px;
  margin-top: 4px;
}

.qa-action {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px;
  border-radius: 8px;
  text-align: left;
  transition: background-color 0.15s;
}

.qa-action:hover {
  background: var(--lc-surface-2);
}

.qa-icon {
  flex: none;
  color: var(--lc-accent);
}

.qa-action span {
  display: grid;
  min-width: 0;
}

.qa-action b {
  font-size: 14px;
  font-weight: 500;
  color: var(--lc-ink);
}

.qa-action small {
  overflow: hidden;
  font-size: 12px;
  color: var(--lc-ink-3);
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
