<template>
  <div class="lc-auth">
    <header class="lc-auth-top">
      <RouterLink to="/home" class="lc-auth-brand" :aria-label="siteName">
        <BrandLogo :name="siteName" :logo-url="siteLogo" :size="28" />
      </RouterLink>
      <RouterLink to="/home" class="lc-auth-back">
        <Icon name="arrowLeft" size="sm" />
        返回首页
      </RouterLink>
    </header>

    <main class="lc-auth-main">
      <div class="lc-auth-container">
        <div class="lc-auth-mark">
          <BrandLogo :logo-url="siteLogo" :size="44" :show-name="false" />
        </div>
        <div class="lc-auth-card"><slot /></div>
        <div class="lc-auth-footer"><slot name="footer" /></div>
        <p class="lc-auth-facts">
          <span><i class="lc-live-dot"></i>{{ apiHost }}</span>
          <span>OpenAI / Anthropic 协议</span>
        </p>
      </div>
    </main>

    <footer class="lc-copyright">&copy; {{ currentYear }} {{ siteName }}</footer>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { RouterLink } from 'vue-router'
import { useAppStore } from '@/stores'
import Icon from '@/components/icons/Icon.vue'
import BrandLogo from '@/components/brand/BrandLogo.vue'
import { sanitizeUrl } from '@/utils/url'
import { normalizeSiteName } from '@/utils/branding'

const appStore = useAppStore()
const siteName = computed(() => normalizeSiteName(appStore.siteName))
const siteLogo = computed(() => sanitizeUrl(appStore.siteLogo || '', { allowRelative: true, allowDataUrl: true }))
// 只展示接口域名，提示用户这里就是要填进客户端的地址
const apiHost = computed(() => {
  const url = appStore.cachedPublicSettings?.api_base_url?.trim() || window.location.origin
  return url.replace(/^https?:\/\//, '').replace(/\/+$/, '').replace(/\/v1$/i, '')
})
const currentYear = new Date().getFullYear()

onMounted(() => { void appStore.fetchPublicSettings() })
</script>

<style scoped>
.lc-auth {
  position: relative;
  display: grid;
  grid-template-rows: auto 1fr auto;
  background: radial-gradient(50% 45% at 50% 0%, var(--lc-glow), transparent 72%), var(--lc-bg);
}

/* 点阵只在中心可见，向四周淡出 */
.lc-auth::before {
  content: '';
  position: absolute;
  inset: 0;
  pointer-events: none;
  background-image: radial-gradient(var(--lc-dot) 1px, transparent 1px);
  background-size: 20px 20px;
  mask-image: radial-gradient(60% 55% at 50% 40%, #000 0%, transparent 75%);
  -webkit-mask-image: radial-gradient(60% 55% at 50% 40%, #000 0%, transparent 75%);
}

.lc-auth-top {
  position: relative;
  z-index: 1;
  display: flex;
  align-items: center;
  justify-content: space-between;
  height: 64px;
  padding: 0 28px;
}

.lc-auth-back {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  color: var(--lc-ink-2);
  font-size: 13px;
}

.lc-auth-back:hover {
  color: var(--lc-ink);
}

.lc-auth-main {
  position: relative;
  z-index: 1;
  display: grid;
  place-items: center;
  padding: 16px 16px 48px;
}

.lc-auth-container {
  width: min(420px, 100%);
}

.lc-auth-mark {
  display: flex;
  justify-content: center;
  margin-bottom: 20px;
}

.lc-auth-card {
  padding: 28px;
  border: 1px solid var(--lc-line-2);
  border-radius: 16px;
  background: var(--lc-surface);
  box-shadow: inset 0 1px 0 var(--lc-hi), var(--lc-shadow-win);
}

.lc-auth-footer {
  margin-top: 18px;
  text-align: center;
  font-size: 13px;
  color: var(--lc-ink-2);
}

.lc-auth-footer :deep(a) {
  color: var(--lc-accent);
  font-weight: 500;
}

.lc-auth-facts {
  display: flex;
  flex-wrap: wrap;
  justify-content: center;
  gap: 6px 16px;
  margin-top: 28px;
  font-size: 12px;
  color: var(--lc-ink-3);
}

.lc-auth-facts span {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.lc-copyright {
  position: relative;
  z-index: 1;
  padding: 18px;
  text-align: center;
  font-size: 12px;
  color: var(--lc-ink-3);
}

@media (max-width: 640px) {
  .lc-auth-top {
    padding: 0 16px;
  }

  .lc-auth-card {
    padding: 20px;
  }
}
</style>
