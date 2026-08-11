<template>
  <div class="lc-shell">
    <div class="lc-topbar">
      <div class="lc-wrap lc-topbar-inner">
        <RouterLink to="/portal/status" class="lc-live">
          <span class="lc-live-dot"></span>
          系统状态实时监控
        </RouterLink>
        <div class="lc-toplinks">
          <a v-if="qqGroup" class="lc-toplink" href="#">
            <Icon name="users" size="sm" />
            {{ qqGroupLabel }}
          </a>
          <a
            v-if="telegramGroupUrl"
            class="lc-toplink"
            :href="telegramGroupUrl"
            target="_blank"
            rel="noopener noreferrer"
          >
            <Icon name="externalLink" size="sm" />
            Telegram 群
          </a>
        </div>
      </div>
    </div>

    <header class="lc-header">
      <nav class="lc-wrap lc-nav">
        <RouterLink to="/home" class="lc-brand">
          <span class="lc-brand-mark">
            <img v-if="siteLogo" :src="siteLogo" alt="" />
            <span v-else>&lt;&gt;</span>
          </span>
          <span class="truncate">{{ siteName }}</span>
        </RouterLink>

        <div class="lc-navlinks" :class="{ open: mobileOpen }">
          <RouterLink to="/home" class="lc-navlink" @click="closeMenu">首页</RouterLink>
          <RouterLink to="/portal/status" class="lc-navlink" @click="closeMenu">可用性检测</RouterLink>
          <RouterLink to="/portal/pricing" class="lc-navlink" @click="closeMenu">定价方案</RouterLink>
          <a
            v-if="chatStationUrl"
            :href="chatStationUrl"
            target="_blank"
            rel="noopener noreferrer"
            class="lc-navlink"
            @click="handleChatStationClick"
          >对话站</a>
        </div>

        <div class="lc-nav-actions">
          <RouterLink
            v-if="isAuthenticated"
            :to="dashboardPath"
            class="lc-button lc-button-primary lc-button-small"
          >进入控制台</RouterLink>
          <template v-else>
            <RouterLink to="/login" class="lc-navlink lc-login-link">登录</RouterLink>
            <RouterLink to="/register" class="lc-button lc-button-primary lc-button-small">注册</RouterLink>
          </template>
          <button
            type="button"
            class="lc-menu-button"
            :aria-expanded="mobileOpen"
            aria-label="打开导航"
            @click="mobileOpen = !mobileOpen"
          >
            <Icon :name="mobileOpen ? 'x' : 'menu'" size="md" />
          </button>
        </div>
      </nav>
    </header>

    <main class="lc-main">
      <slot />
    </main>

    <footer class="lc-footer">
      <div class="lc-wrap lc-footer-inner">
        <div>&copy; {{ currentYear }} {{ siteName }} · 稳定、透明、好用的 AI API 网关</div>
        <div class="lc-footer-links">
          <RouterLink to="/portal/pricing">定价方案</RouterLink>
          <RouterLink to="/portal/status">可用性检测</RouterLink>
          <a v-if="docUrl" :href="docUrl" target="_blank" rel="noopener noreferrer">使用文档</a>
          <a v-if="telegramGroupUrl" :href="telegramGroupUrl" target="_blank" rel="noopener noreferrer">Telegram</a>
        </div>
      </div>
    </footer>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { RouterLink } from 'vue-router'
import { useAuthStore, useAppStore } from '@/stores'
import { lobeHubSSOAPI } from '@/api'
import Icon from '@/components/icons/Icon.vue'
import { sanitizeUrl } from '@/utils/url'
import { normalizeSiteName } from '@/utils/branding'

const authStore = useAuthStore()
const appStore = useAppStore()
const mobileOpen = ref(false)

const settings = computed(() => appStore.cachedPublicSettings)
const siteName = computed(() => normalizeSiteName(settings.value?.site_name || appStore.siteName))
const siteLogo = computed(() => sanitizeUrl(appStore.cachedPublicSettings?.site_logo || appStore.siteLogo || '', {
  allowRelative: true,
  allowDataUrl: true,
}))
const docUrl = computed(() => sanitizeUrl(appStore.cachedPublicSettings?.doc_url || appStore.docUrl || ''))
const chatStationUrl = computed(() => sanitizeUrl(settings.value?.chat_station_url || ''))
const telegramGroupUrl = computed(() => sanitizeUrl(settings.value?.telegram_group_url || ''))
const qqGroup = computed(() => settings.value?.qq_group?.trim() || '')
const qqGroupLabel = computed(() => {
  const value = qqGroup.value.replace(/^QQ\s*(?:群)?\s*[:：]?\s*/i, '')
  return `QQ群 ${value}`
})
const isAuthenticated = computed(() => authStore.isAuthenticated)
const dashboardPath = computed(() => (authStore.isAdmin ? '/admin/dashboard' : '/dashboard'))
const currentYear = new Date().getFullYear()

function closeMenu() {
  mobileOpen.value = false
}

async function handleChatStationClick(event: MouseEvent) {
  closeMenu()
  const url = chatStationUrl.value
  if (!url || !isAuthenticated.value) return

  event.preventDefault()
  const chatWindow = window.open(url, '_blank')
  if (chatWindow) chatWindow.opener = null
  try {
    const result = await lobeHubSSOAPI.authorize('/')
    const redirect = result.redirect_url || url
    if (chatWindow) chatWindow.location.replace(redirect)
    else window.open(redirect, '_blank', 'noopener,noreferrer')
  } catch (error) {
    console.error('Failed to start LobeHub SSO:', error)
    if (!chatWindow) window.open(url, '_blank', 'noopener,noreferrer')
  }
}

onMounted(() => {
  void appStore.fetchPublicSettings()
  void authStore.checkAuth()
})
</script>
