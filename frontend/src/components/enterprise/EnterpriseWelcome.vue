<template>
  <Transition name="enterprise-welcome">
    <div
      v-if="visible"
      class="enterprise-welcome"
      role="status"
      aria-live="polite"
      data-test="enterprise-welcome"
      @mouseenter="pause"
      @mouseleave="resume"
    >
      <div class="enterprise-welcome-body">
        <p class="enterprise-welcome-label">{{ t('enterprise.badge') }}</p>
        <p class="enterprise-welcome-title">{{ t('enterprise.welcome', { name: displayName }) }}</p>
      </div>
      <button type="button" class="enterprise-welcome-close" :aria-label="t('common.close')" data-test="enterprise-welcome-close" @click="close">×</button>
    </div>
  </Transition>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { useEnterpriseStore } from '@/stores/enterprise'

/** 提示停留时间；鼠标停在上面时暂停计时。 */
const ENTERPRISE_WELCOME_DURATION_MS = 5000

const { t } = useI18n()
const authStore = useAuthStore()
const enterpriseStore = useEnterpriseStore()
const route = useRoute()

// 只在控制台页面提示：路由默认需要登录，首页、门户页等公开页显式标了 requiresAuth: false。
// 应用刚启动、路由还没解析完成时 matched 为空，不算控制台，避免在首页一闪而过；404 页也不算。
const inConsole = computed(() => route.matched.length > 0 && route.name !== 'NotFound' && route.meta.requiresAuth !== false)

const visible = ref(false)
let timer: ReturnType<typeof setTimeout> | undefined

const displayName = computed(() => {
  const user = authStore.user
  return user?.username || user?.email?.split('@')[0] || ''
})

function clearTimer() {
  if (timer !== undefined) clearTimeout(timer)
  timer = undefined
}

function start() {
  clearTimer()
  timer = setTimeout(close, ENTERPRISE_WELCOME_DURATION_MS)
}

function close() {
  clearTimer()
  visible.value = false
}

const pause = clearTimer
const resume = () => {
  if (visible.value) start()
}

// 企业用户进入控制台时弹出一次：弹出即消耗掉「待提示」，之后在控制台里切换页面不再重复。
// 先停在首页的话保持待提示，等进入控制台再弹；登出后状态被清空，提示随之收起。
watch(
  () => [enterpriseStore.welcomePending, inConsole.value, authStore.isAuthenticated, enterpriseStore.isEnterprise] as const,
  ([pending, isConsole, authenticated, enterprise]) => {
    if (pending && isConsole && authenticated) {
      enterpriseStore.dismissWelcome()
      visible.value = true
      start()
    } else if (!authenticated || !enterprise) {
      clearTimer()
      visible.value = false
    }
  },
  { immediate: true },
)

onBeforeUnmount(clearTimer)
</script>

<style scoped>
.enterprise-welcome {
  position: fixed;
  top: 72px;
  left: 50%;
  z-index: 70;
  display: flex;
  align-items: flex-start;
  gap: 12px;
  width: min(92vw, 340px);
  padding: 10px 12px 12px 14px;
  border: 1px solid rgba(176, 124, 30, 0.4);
  border-left: 3px solid #d4a03c;
  border-radius: 14px;
  background: var(--lc-surface, #ffffff);
  color: var(--lc-ink, #111827);
  box-shadow: 0 10px 30px rgba(0, 0, 0, 0.18), 0 0 24px rgba(212, 160, 60, 0.22);
  transform: translateX(-50%);
}

.enterprise-welcome-body {
  min-width: 0;
  flex: 1 1 auto;
}

.enterprise-welcome-label {
  margin: 0 0 2px;
  color: #8a5a12;
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.08em;
}

.enterprise-welcome-title {
  margin: 0;
  overflow: hidden;
  font-size: 17px;
  font-weight: 700;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.enterprise-welcome-close {
  flex: none;
  width: 22px;
  height: 22px;
  border: 0;
  border-radius: 50%;
  background: transparent;
  color: inherit;
  font-size: 18px;
  line-height: 1;
  opacity: 0.55;
  cursor: pointer;
}

.enterprise-welcome-close:hover,
.enterprise-welcome-close:focus-visible {
  opacity: 1;
}

.enterprise-welcome-enter-active,
.enterprise-welcome-leave-active {
  transition: opacity 0.25s ease, transform 0.25s ease;
}

.enterprise-welcome-enter-from,
.enterprise-welcome-leave-to {
  opacity: 0;
  transform: translate(-50%, -10px);
}

@media (prefers-reduced-motion: reduce) {
  .enterprise-welcome-enter-active,
  .enterprise-welcome-leave-active {
    transition: opacity 0.01s;
  }

  .enterprise-welcome-enter-from,
  .enterprise-welcome-leave-to {
    transform: translateX(-50%);
  }
}
</style>

<!-- 深色主题：不带作用域，才能用 html.dark 后代选择器覆盖 -->
<style>
html.dark .enterprise-welcome {
  border-color: rgba(242, 196, 109, 0.45);
  border-left-color: #f2c46d;
  box-shadow: 0 10px 30px rgba(0, 0, 0, 0.4), 0 0 24px rgba(242, 196, 109, 0.2);
}

html.dark .enterprise-welcome-label {
  color: #f2c46d;
}
</style>
