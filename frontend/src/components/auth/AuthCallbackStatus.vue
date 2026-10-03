<template>
  <div v-if="failed" class="cb-status cb-failed" role="alert" data-testid="auth-callback-failed">
    <p class="cb-line"><i class="cb-dot" aria-hidden="true"></i>{{ t('auth.callbackStatus.failedTitle') }}</p>
    <p class="cb-reason">{{ message }}</p>
    <p class="cb-hint">{{ t('auth.callbackStatus.failedHint') }}</p>
    <div class="cb-actions">
      <RouterLink to="/login" class="btn btn-primary">{{ t('auth.callbackStatus.backToLogin') }}</RouterLink>
      <RouterLink to="/home" class="btn btn-secondary">{{ t('auth.callbackStatus.backHome') }}</RouterLink>
    </div>
  </div>
  <p v-else-if="processing" class="cb-status cb-processing" aria-hidden="true">
    <span class="cb-spinner"></span>
  </p>
</template>

<script setup lang="ts">
// 第三方登录回调页的状态区：处理中只给转圈（文字说明由页面副标题承担）；失败时把原因留在页面上并给出下一步，
// 而不是只弹一次 toast 后停在「正在完成登录」。是否展示由调用方按自身流程状态决定。
import { computed } from 'vue'
import { RouterLink } from 'vue-router'
import { useI18n } from 'vue-i18n'

const props = defineProps<{
  processing: boolean
  message: string
}>()

const { t } = useI18n()
const failed = computed(() => !props.processing && Boolean(props.message))
</script>

<style scoped>
.cb-status {
  font-size: 14px;
}

.cb-processing {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 10px;
  color: var(--lc-ink-2);
}

.cb-spinner {
  width: 14px;
  height: 14px;
  border: 2px solid var(--lc-line-2);
  border-top-color: var(--lc-ink);
  border-radius: 50%;
  animation: cb-spin 0.8s linear infinite;
}

.cb-failed {
  display: grid;
  gap: 8px;
  padding: 16px;
  border: 1px solid var(--lc-line);
  border-radius: 10px;
  background: var(--lc-surface-2);
}

.cb-line {
  display: flex;
  align-items: center;
  gap: 8px;
  font-weight: 600;
  color: var(--lc-ink);
}

.cb-dot {
  width: 8px;
  height: 8px;
  border-radius: 2px;
  background: var(--lc-bad);
}

.cb-reason {
  font-family: var(--lc-font-mono);
  font-size: 13px;
  color: var(--lc-ink);
  overflow-wrap: anywhere;
}

.cb-hint {
  font-size: 13px;
  color: var(--lc-ink-2);
}

.cb-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-top: 6px;
}

@keyframes cb-spin {
  to {
    transform: rotate(360deg);
  }
}

@media (prefers-reduced-motion: reduce) {
  .cb-spinner {
    animation-duration: 2.4s;
  }
}
</style>
