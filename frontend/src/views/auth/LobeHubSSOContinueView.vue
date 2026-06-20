<template>
  <div class="flex min-h-screen items-center justify-center bg-gray-50 px-4 dark:bg-dark-950">
    <div class="w-full max-w-md rounded-xl bg-white p-6 text-center shadow-sm dark:bg-dark-900">
      <div
        class="mx-auto mb-4 h-10 w-10 animate-spin rounded-full border-2 border-primary-500 border-t-transparent"
      ></div>
      <h1 class="text-lg font-semibold text-gray-900 dark:text-white">
        {{ t('common.processing') }}
      </h1>
      <p v-if="errorMessage" class="mt-3 text-sm text-red-600 dark:text-red-400">
        {{ errorMessage }}
      </p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute } from 'vue-router'
import { lobeHubSSOAPI } from '@/api'
import { useAppStore } from '@/stores'

const { t } = useI18n()
const route = useRoute()
const appStore = useAppStore()
const errorMessage = ref('')

const returnTo = computed(() => {
  const value = route.query.returnTo
  return typeof value === 'string' && value.trim() ? value.trim() : '/'
})

onMounted(async () => {
  try {
    const result = await lobeHubSSOAPI.authorize(returnTo.value)
    window.location.href = result.redirect_url
  } catch (error) {
    console.error('Failed to continue LobeHub SSO:', error)
    errorMessage.value = t('auth.errors.loginFailed')
    const fallback = appStore.cachedPublicSettings?.chat_station_url || ''
    if (fallback) {
      window.setTimeout(() => {
        window.location.href = fallback
      }, 1200)
    }
  }
})
</script>
