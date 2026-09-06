<template>
  <!-- 后台内嵌形态:?embedded=1 且已登录,套完整后台布局 -->
  <AppLayout v-if="isEmbedded">
    <ModelPlazaContent :response="data" :loading="loading" :error="loadFailed" embedded @retry="loadData" />
  </AppLayout>

  <PortalLayout v-else>
    <div class="lc-wrap plaza-portal-wrap">
      <ModelPlazaContent :response="data" :loading="loading" :error="loadFailed" portal @retry="loadData" />
    </div>
  </PortalLayout>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import AppLayout from '@/components/layout/AppLayout.vue'
import PortalLayout from '@/views/public/components/PortalLayout.vue'
import ModelPlazaContent from '@/components/modelPlaza/ModelPlazaContent.vue'
import { getModelPlaza, type ModelPlazaResponse } from '@/api/modelPlaza'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'

const route = useRoute()
const appStore = useAppStore()
const authStore = useAuthStore()

// embedded=1 但未登录(如转发的链接)自动降级为独立形态。
const isEmbedded = computed(() => route.query.embedded === '1' && authStore.isAuthenticated)

const data = ref<ModelPlazaResponse | null>(null)
const loading = ref(true)
const loadFailed = ref(false)

let pending: AbortController | null = null
async function loadData() {
  pending?.abort()
  const request = new AbortController()
  pending = request
  loading.value = true
  loadFailed.value = false
  data.value = null
  try {
    const result = await getModelPlaza({ signal: request.signal })
    if (!request.signal.aborted) data.value = result
  } catch {
    if (!request.signal.aborted) loadFailed.value = true
  } finally {
    if (!request.signal.aborted) loading.value = false
  }
}

// Clear privileged prices immediately when the account changes; stale requests cannot restore them.
watch(() => authStore.user?.id, () => { void loadData() }, { immediate: true })
onMounted(() => { void appStore.fetchPublicSettings() })
onBeforeUnmount(() => pending?.abort())
</script>
