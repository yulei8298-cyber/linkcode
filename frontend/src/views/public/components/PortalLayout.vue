<template>
  <div class="lc-portal">
    <div class="lc-topbar">
      <div class="lc-wrap lc-topbar-inner">
        <RouterLink to="/portal/status" class="lc-live">
          <span class="lc-live-dot"></span>
          调价、线路变更和故障，群里第一时间通知
        </RouterLink>
        <div class="lc-toplinks">
          <button v-if="qqGroup" type="button" class="lc-toplink" :title="`复制 ${qqGroupLabel}`" @click="copyQQGroup">
            QQ 群 <b>{{ qqGroupNumber }}</b>
            <Icon name="copy" size="xs" />
          </button>
          <a
            v-if="telegramGroupUrl"
            class="lc-toplink"
            :href="telegramGroupUrl"
            target="_blank"
            rel="noopener noreferrer"
          >
            Telegram 群
            <Icon name="externalLink" size="xs" />
          </a>
        </div>
      </div>
    </div>

    <header class="lc-header">
      <nav class="lc-wrap lc-nav">
        <RouterLink to="/home" class="lc-brand" :aria-label="siteName">
          <BrandLogo :name="siteName" :logo-url="siteLogo" :size="28" />
        </RouterLink>

        <div class="lc-navlinks" :class="{ open: mobileOpen }">
          <RouterLink to="/home" class="lc-navlink" @click="closeMenu">首页</RouterLink>
          <RouterLink to="/portal/status" class="lc-navlink" @click="closeMenu">可用性检测</RouterLink>
          <RouterLink to="/portal/pricing" class="lc-navlink" @click="closeMenu">定价方案</RouterLink>
          <RouterLink v-if="showIntelCheck" to="/portal/intel-check" class="lc-navlink" @click="closeMenu">智力检测</RouterLink>
          <RouterLink v-if="showModelPlaza" to="/model-plaza" class="lc-navlink" @click="closeMenu">模型广场</RouterLink>
          <a
            v-if="chatStationUrl"
            :href="chatStationUrl"
            target="_blank"
            rel="noopener noreferrer"
            class="lc-navlink"
            @click="handleChatStationClick"
          >对话站 ↗</a>
        </div>

        <div class="lc-nav-actions">
          <RouterLink
            v-if="isAuthenticated"
            :to="dashboardPath"
            class="lc-button lc-button-primary lc-button-small"
          >进入控制台</RouterLink>
          <template v-else>
            <RouterLink to="/login" class="lc-button lc-button-small lc-login-link">登录</RouterLink>
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
      <div class="lc-wrap lc-footer-grid">
        <div>
          <BrandLogo :name="siteName" :logo-url="siteLogo" :size="26" />
          <p class="lc-footer-copy">&copy; {{ currentYear }} {{ siteName }}</p>
        </div>
        <div>
          <h4>产品</h4>
          <ul>
            <li><RouterLink to="/portal/pricing">定价方案</RouterLink></li>
            <li v-if="showModelPlaza"><RouterLink to="/model-plaza">模型广场</RouterLink></li>
            <li v-if="chatStationUrl"><a :href="chatStationUrl" target="_blank" rel="noopener noreferrer" @click="handleChatStationClick">对话站 ↗</a></li>
          </ul>
        </div>
        <div>
          <h4>状态</h4>
          <ul>
            <li><RouterLink to="/portal/status">可用性检测</RouterLink></li>
            <li v-if="showIntelCheck"><RouterLink to="/portal/intel-check">智力检测</RouterLink></li>
          </ul>
        </div>
        <div>
          <h4>帮助</h4>
          <ul>
            <li><RouterLink to="/key-usage">Key 用量查询</RouterLink></li>
            <li v-if="docUrl"><a :href="docUrl" target="_blank" rel="noopener noreferrer">使用文档</a></li>
          </ul>
        </div>
        <div v-if="qqGroup || telegramGroupUrl">
          <h4>社群</h4>
          <ul>
            <li v-if="qqGroup">QQ 群 <span class="lc-mono">{{ qqGroupNumber }}</span></li>
            <li v-if="telegramGroupUrl"><a :href="telegramGroupUrl" target="_blank" rel="noopener noreferrer">Telegram 群 ↗</a></li>
          </ul>
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
import BrandLogo from '@/components/brand/BrandLogo.vue'
import { useClipboard } from '@/composables/useClipboard'
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
const qqGroupNumber = computed(() => qqGroup.value.replace(/^QQ\s*(?:群)?\s*[:：]?\s*/i, ''))
const qqGroupLabel = computed(() => `QQ群 ${qqGroupNumber.value}`)
// 点击时再取剪贴板工具，避免外壳组件在挂载阶段依赖 store
function copyQQGroup() {
  void useClipboard().copyToClipboard(qqGroupNumber.value, '已复制 QQ 群号')
}
const isAuthenticated = computed(() => authStore.isAuthenticated)
const showModelPlaza = computed(() => settings.value?.model_plaza_enabled === true &&
  (isAuthenticated.value || settings.value?.model_plaza_require_auth !== true))
// 严格 === true：开关默认关闭，设置未加载时不应先把入口显出来再收回去。
// 与模型广场不同，这里没有「强制登录」那一档——公开可见正是本页的用途。
const showIntelCheck = computed(() => settings.value?.intel_check_enabled === true)
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
