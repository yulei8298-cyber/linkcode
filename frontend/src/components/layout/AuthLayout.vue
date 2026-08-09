<template>
  <div class="lc-auth">
    <RouterLink to="/home" class="lc-button lc-button-small" style="position:fixed;left:20px;top:20px;z-index:2">
      <Icon name="arrowLeft" size="sm" />
      返回首页
    </RouterLink>
    <div class="lc-auth-container">
      <div class="lc-auth-brand">
        <template v-if="settingsLoaded">
          <div class="lc-auth-logo"><img :src="siteLogo || '/logo.svg'" alt="Logo" /></div>
          <h1>{{ siteName }}</h1>
          <p>{{ siteSubtitle }}</p>
        </template>
      </div>
      <div class="lc-auth-card"><slot /></div>
      <div class="lc-auth-footer"><slot name="footer" /></div>
      <div class="lc-copyright">&copy; {{ currentYear }} {{ siteName }}. All rights reserved.</div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { RouterLink } from 'vue-router'
import { useAppStore } from '@/stores'
import Icon from '@/components/icons/Icon.vue'
import { sanitizeUrl } from '@/utils/url'

const appStore = useAppStore()
const siteName = computed(() => appStore.siteName || 'LinkCode')
const siteLogo = computed(() => sanitizeUrl(appStore.siteLogo || '', { allowRelative: true, allowDataUrl: true }))
const siteSubtitle = computed(() => appStore.cachedPublicSettings?.site_subtitle || '稳定、透明的 AI API 网关')
const settingsLoaded = computed(() => appStore.publicSettingsLoaded)
const currentYear = new Date().getFullYear()

onMounted(() => { void appStore.fetchPublicSettings() })
</script>
