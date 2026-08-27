<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'

defineOptions({ name: 'InfiniteCanvasView' })
import {
  ensureInfiniteCanvasConfig,
  INFINITE_CANVAS_BASE_URL,
  INFINITE_CANVAS_MODEL,
  type InfiniteCanvasConfig,
} from '@/api/infiniteCanvas'

const config = ref<InfiniteCanvasConfig | null>(null)
const loading = ref(true)
const errorMessage = ref('')
const configuredCanvasUrl = (import.meta.env.VITE_INFINITE_CANVAS_ORIGIN || '').trim()
const configuredCanvas = configuredCanvasUrl ? new URL(configuredCanvasUrl) : null
const canvasOrigin = configuredCanvas ? `${configuredCanvas.protocol}//${configuredCanvas.host}` : ''
const canvasBasePath = configuredCanvas ? configuredCanvas.pathname.replace(/\/+$/, '') : ''
const UPSTREAM_SIGNATURE_KEY = 'linkcode-infinite-canvas:upstream-signature'
const CANVAS_ROUTE_KEY = 'linkcode-infinite-canvas:route'

function hostUserId(): string {
  let userId = 'anonymous'
  try {
    const raw = localStorage.getItem('auth_user')
    const user = raw ? JSON.parse(raw) as { id?: number } : null
    if (user?.id) userId = String(user.id)
  } catch { /* ignore malformed auth cache */ }
  return userId
}
function hostConfigKey(): string {
  return `linkcode-infinite-canvas:${hostUserId()}:config`
}

function canvasRouteKey(): string {
  return `linkcode-infinite-canvas:${hostUserId()}:${CANVAS_ROUTE_KEY}`
}

function validCanvasRoute(value: unknown): value is string {
  if (typeof value !== 'string' || !value.startsWith('/') || value.startsWith('//')) return false
  const pathname = value.split(/[?#]/, 1)[0]
  return pathname === '/' || ['/image', '/video', '/assets', '/prompts', '/config'].includes(pathname) || /^\/canvas(?:\/[^/]+)?$/.test(pathname)
}

function readCanvasRoute(): string {
  try {
    const value = localStorage.getItem(canvasRouteKey())
    return validCanvasRoute(value) ? value : '/'
  } catch { return '/' }
}

function saveCanvasRoute(value: string): void {
  if (!validCanvasRoute(value)) return
  try { localStorage.setItem(canvasRouteKey(), value) } catch { /* ignore storage failures */ }
}

const initialCanvasRoute = readCanvasRoute()

function readHostConfig(): InfiniteCanvasConfig | null {
  try {
    const raw = localStorage.getItem(hostConfigKey())
    if (!raw) return null
    const value = JSON.parse(raw) as Partial<InfiniteCanvasConfig>
    if (value.provider !== 'linkcode' || value.model !== INFINITE_CANVAS_MODEL || !value.apiKey || !value.baseUrl || !value.groupId) return null
    return { ...value, baseUrl: INFINITE_CANVAS_BASE_URL } as InfiniteCanvasConfig
  } catch { return null }
}

function upstreamConfig(value: InfiniteCanvasConfig): Record<string, unknown> {
  const model = value.model
  return {
    channelMode: 'local',
    baseUrl: value.baseUrl,
    apiKey: value.apiKey,
    apiFormat: 'openai',
    channels: [{ id: 'linkcode', name: 'LinkCode', baseUrl: value.baseUrl, apiKey: value.apiKey, apiFormat: 'openai', models: [{ name: model, capability: 'image' }] }],
    model: `linkcode::${model}`,
    imageModel: `linkcode::${model}`,
    videoModel: '',
    textModel: '',
    audioModel: '',
    audioVoice: 'alloy',
    audioFormat: 'mp3',
    audioSpeed: '1',
    audioInstructions: '',
    videoSeconds: '6',
    vquality: '720',
    videoGenerateAudio: 'true',
    videoWatermark: 'false',
    systemPrompt: '',
    reasoningEffort: 'auto',
    models: [`linkcode::${model}`],
    quality: 'auto',
    size: '1:1',
    background: '',
    count: '1',
    canvasImageCount: '3',
  }
}

function upstreamSignature(value: InfiniteCanvasConfig): string {
  return `${hostUserId()}:${value.apiKeyId}:${value.groupId}:${value.baseUrl}`
}

const iframeSrc = computed(() => {
  if (!config.value || !canvasOrigin) return ''
  const routePath = `${canvasBasePath}${initialCanvasRoute === '/' ? '/' : initialCanvasRoute}` || '/'
  const url = new URL(routePath, canvasOrigin)
  // Bust a previously opened iframe document after embedded UI bootstrap changes.
  url.searchParams.set('linkcodeEmbed', '8')
  const signature = upstreamSignature(config.value)
  if (localStorage.getItem(UPSTREAM_SIGNATURE_KEY) !== signature) {
    // hash 不会发到服务器；静态站点在原版 React 启动前写入其既有配置存储。
    url.hash = new URLSearchParams({
      linkcodeConfig: JSON.stringify({ signature, config: upstreamConfig(config.value) }),
    }).toString()
  }
  return url.toString()
})

function markUpstreamConfigured(): void {
  if (config.value) localStorage.setItem(UPSTREAM_SIGNATURE_KEY, upstreamSignature(config.value))
}

function onUpstreamMessage(event: MessageEvent): void {
  if (!canvasOrigin || event.origin !== canvasOrigin || event.data?.type !== 'linkcode-infinite-canvas-route') return
  if (typeof event.data.path !== 'string' || !validCanvasRoute(event.data.path)) return
  saveCanvasRoute(event.data.path)
}

async function initialize(): Promise<void> {
  try {
    const persisted = readHostConfig()
    config.value = persisted || await ensureInfiniteCanvasConfig()
    if (!persisted) localStorage.setItem(hostConfigKey(), JSON.stringify(config.value))
    if (!canvasOrigin) throw new Error('无限画布静态站点尚未配置。')
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '无限画布初始化失败。'
  } finally { loading.value = false }
}

onMounted(() => {
  window.addEventListener('message', onUpstreamMessage)
  void initialize()
})

onBeforeUnmount(() => window.removeEventListener('message', onUpstreamMessage))
</script>

<template>
  <AppLayout>
    <div class="upstream-canvas-host">
      <div v-if="loading" class="upstream-canvas-loading">正在准备无限画布配置…</div>
      <div v-else-if="errorMessage" class="upstream-canvas-error">{{ errorMessage }}</div>
      <iframe
        v-else
        title="无限画布"
        :src="iframeSrc"
        class="upstream-canvas-frame"
        sandbox="allow-scripts allow-same-origin allow-forms allow-modals allow-downloads"
        allow="clipboard-read; clipboard-write"
        referrerpolicy="no-referrer"
        @load="markUpstreamConfigured"
      />
    </div>
  </AppLayout>
</template>

<style scoped>
.upstream-canvas-host { width: 100%; height: calc(100vh - 10rem); min-height: 720px; overflow: hidden; background: #fff; }
.upstream-canvas-frame { display: block; width: 100%; height: 100%; min-height: 720px; border: 0; }
.upstream-canvas-loading, .upstream-canvas-error { display: grid; min-height: 100%; place-items: center; padding: 2rem; color: #4b5563; font-size: .95rem; text-align: center; }
.upstream-canvas-error { color: #b42318; }
</style>
