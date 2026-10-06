<template>
  <section class="hh">
    <div class="lc-wrap hh-inner">
      <p class="hh-eyebrow"><span class="lc-live-dot"></span>Claude Code · Codex · OpenCode</p>
      <h1>Claude 与 GPT 的<br /><em>低价稳定</em>中转站</h1>
      <p class="hh-lead">换一个接口地址，Claude Code、Codex 和你手上的客户端照常使用。按分组倍率计费，每一笔都查得到。</p>
      <div class="hh-cta">
        <RouterLink :to="primaryTo" class="lc-button lc-button-primary lc-button-xl">{{ primaryText }}</RouterLink>
        <button type="button" class="lc-button lc-button-xl" @click="$emit('show-connect')">看看怎么接入</button>
      </div>

      <div class="hh-window" data-testid="home-route" aria-label="请求经由网关转发到各家模型">
        <div class="hh-bar">
          <span>{{ siteName.toLowerCase() }} / route</span>
          <span><i class="lc-live-dot"></i>{{ apiHost }}</span>
        </div>
        <div ref="routeRef" class="hh-route">
          <svg class="hh-wires" :viewBox="`0 0 ${box.w} ${box.h}`" aria-hidden="true">
            <path v-for="(d, i) in wires" :key="`b${i}`" :d="d" />
            <path v-for="(d, i) in wires" :key="`f${i}`" :d="d" class="flow" :style="{ animationDelay: `${i * 0.6}s` }" />
          </svg>
          <div ref="leftRef" class="hh-col">
            <div v-for="node in clients" :key="node.name" class="hh-node">
              <span class="hm-pm hh-icon" :class="`is-${node.platform}`">
                <PlatformIcon v-if="node.platform" :platform="node.platform" size="sm" />
                <template v-else>{ }</template>
              </span>
              <div><small>CLIENT</small><b>{{ node.name }}</b><span>{{ node.note }}</span></div>
            </div>
          </div>
          <div class="hh-hubcol">
            <div ref="hubRef" class="hh-hub">
              <span class="hh-hub-tag">GATEWAY</span>
              <BrandLogo :logo-url="siteLogo" :size="44" :show-name="false" />
              <b>{{ siteName }}</b>
              <span>鉴权 · 选号池 · 计费</span>
            </div>
          </div>
          <div ref="rightRef" class="hh-col">
            <div v-for="node in providers" :key="node.name" class="hh-node">
              <span class="hm-pm hh-icon" :class="`is-${node.platform}`"><PlatformIcon :platform="node.platform" size="sm" /></span>
              <div><small>MODEL</small><b>{{ node.name }}</b><span>{{ node.note }}</span></div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { RouterLink } from 'vue-router'
import PlatformIcon from '@/components/common/PlatformIcon.vue'
import BrandLogo from '@/components/brand/BrandLogo.vue'
import type { GroupPlatform } from '@/types'

const props = defineProps<{
  siteName: string
  siteLogo: string
  apiHost: string
  isAuthenticated: boolean
  dashboardPath: string
}>()
defineEmits<{ (e: 'show-connect'): void }>()

const primaryTo = computed(() => (props.isAuthenticated ? props.dashboardPath : '/register'))
const primaryText = computed(() => (props.isAuthenticated ? '进入控制台' : '开始使用'))

interface RouteNode { name: string; note: string; platform: GroupPlatform | '' }
const clients: RouteNode[] = [
  { name: 'Claude Code', note: 'Anthropic 协议', platform: 'anthropic' },
  { name: 'Codex CLI', note: 'Responses 协议', platform: 'openai' },
  { name: 'OpenCode', note: 'OpenAI 协议', platform: 'opencode_go' as GroupPlatform },
  { name: '你的程序', note: '官方 SDK', platform: '' },
]
const providers: (RouteNode & { platform: GroupPlatform })[] = [
  { name: 'Claude', note: 'Anthropic', platform: 'anthropic' },
  { name: 'GPT', note: 'OpenAI', platform: 'openai' },
  { name: 'Grok', note: 'xAI', platform: 'grok' },
  { name: 'DeepSeek · Kimi', note: '国产模型', platform: 'deepseek' },
]

// 连线：按节点实际位置画贝塞尔曲线，尺寸变化时重画
const routeRef = ref<HTMLElement | null>(null)
const leftRef = ref<HTMLElement | null>(null)
const rightRef = ref<HTMLElement | null>(null)
const hubRef = ref<HTMLElement | null>(null)
const box = ref({ w: 1, h: 1 })
const wires = ref<string[]>([])
let observer: ResizeObserver | null = null

function drawWires() {
  const route = routeRef.value
  const hub = hubRef.value
  if (!route || !hub || !leftRef.value || !rightRef.value) return
  const R = route.getBoundingClientRect()
  const H = hub.getBoundingClientRect()
  box.value = { w: Math.max(1, R.width), h: Math.max(1, R.height) }
  const hy = H.top + H.height / 2 - R.top
  const curve = (x1: number, y1: number, x2: number, y2: number) => {
    const mx = (x1 + x2) / 2
    return `M${x1},${y1} C${mx},${y1} ${mx},${y2} ${x2},${y2}`
  }
  const paths: string[] = []
  for (const node of Array.from(leftRef.value.children)) {
    const b = node.getBoundingClientRect()
    paths.push(curve(b.right - R.left, b.top + b.height / 2 - R.top, H.left - R.left, hy))
  }
  for (const node of Array.from(rightRef.value.children)) {
    const b = node.getBoundingClientRect()
    paths.push(curve(H.right - R.left, hy, b.left - R.left, b.top + b.height / 2 - R.top))
  }
  wires.value = paths
}

onMounted(() => {
  drawWires()
  if (typeof ResizeObserver !== 'undefined' && routeRef.value) {
    observer = new ResizeObserver(drawWires)
    observer.observe(routeRef.value)
  }
})
onBeforeUnmount(() => observer?.disconnect())
</script>

<style scoped>
.hh { position: relative; overflow: hidden; background: radial-gradient(55% 42% at 50% 0%, var(--lc-glow), transparent 72%), var(--lc-bg); }
.hh-inner { padding: 92px 0 76px; text-align: center; }
.hh-eyebrow { display: inline-flex; align-items: center; gap: 8px; height: 30px; padding: 0 12px; border: 1px solid var(--lc-line); border-radius: 999px; background: var(--lc-surface); color: var(--lc-ink-2); font: 500 12.5px var(--lc-font-mono); }
h1 { margin: 22px auto 0; font-size: clamp(38px, 5.8vw, 70px); line-height: 1.06; font-weight: 700; letter-spacing: -0.03em; text-wrap: balance; }
h1 em { font-style: normal; color: var(--lc-accent); }
.hh-lead { max-width: 34em; margin: 22px auto 0; color: var(--lc-ink-2); font-size: 17px; line-height: 1.75; }
.hh-cta { display: flex; justify-content: center; gap: 12px; flex-wrap: wrap; margin-top: 34px; }

.hh-window { max-width: max(1080px, 62vw); margin: 64px auto 0; overflow: hidden; border: 1px solid var(--lc-line-2); border-radius: 16px; background: var(--lc-surface); box-shadow: inset 0 1px 0 var(--lc-hi), var(--lc-shadow-win); text-align: left; }
.hh-bar { display: flex; justify-content: space-between; align-items: center; gap: 12px; height: 42px; padding: 0 18px; border-bottom: 1px solid var(--lc-line); color: var(--lc-ink-3); font: 12px var(--lc-font-mono); }
.hh-bar span:last-child { display: inline-flex; align-items: center; gap: 8px; }
.hh-route { position: relative; display: grid; grid-template-columns: minmax(0, 1fr) 220px minmax(0, 1fr); gap: 72px; align-items: center; padding: 36px 40px; background-image: radial-gradient(var(--lc-dot) 1px, transparent 1px); background-size: 18px 18px; }
.hh-wires { position: absolute; inset: 0; width: 100%; height: 100%; overflow: visible; pointer-events: none; }
.hh-wires path { fill: none; stroke: var(--lc-line-2); stroke-width: 1.2; }
.hh-wires path.flow { stroke: var(--lc-accent); stroke-width: 1.6; stroke-dasharray: 14 260; stroke-linecap: round; animation: hh-flow 3.2s linear infinite; }
@keyframes hh-flow { from { stroke-dashoffset: 274; } to { stroke-dashoffset: 0; } }
.hh-col { position: relative; z-index: 1; display: grid; gap: 14px; }
.hh-node { display: flex; align-items: center; gap: 12px; padding: 12px 14px; border: 1px solid var(--lc-line); border-radius: 10px; background: var(--lc-surface); box-shadow: inset 0 1px 0 var(--lc-hi); }
.hh-icon { width: 34px; height: 34px; border-radius: 8px; font-size: 11px; }
.hh-node div { display: grid; min-width: 0; }
.hh-node small { color: var(--lc-ink-3); font: 500 10px var(--lc-font-mono); letter-spacing: .08em; }
.hh-node b { font-size: 14px; font-weight: 600; }
.hh-node span:not(.hm-pm) { overflow: hidden; color: var(--lc-ink-3); font-size: 12px; text-overflow: ellipsis; white-space: nowrap; }
.hh-hubcol { position: relative; z-index: 1; display: grid; place-items: center; }
.hh-hub { display: grid; justify-items: center; gap: 8px; width: 100%; padding: 24px 16px 22px; border: 1px solid var(--lc-accent-line); border-radius: 16px; background: var(--lc-surface); box-shadow: 0 0 0 6px var(--lc-accent-soft), inset 0 1px 0 var(--lc-hi); text-align: center; }
.hh-hub-tag { padding: 2px 8px; border: 1px solid var(--lc-accent-line); border-radius: 999px; color: var(--lc-accent); font: 600 10px var(--lc-font-mono); letter-spacing: .12em; }
.hh-hub b { font-size: 18px; }
.hh-hub > span:last-child { color: var(--lc-ink-3); font-size: 12px; }

@media (prefers-reduced-motion: reduce) {
  .hh-wires path.flow { display: none; }
}

@media (max-width: 1024px) {
  .hh-route { grid-template-columns: minmax(0, 1fr) 180px minmax(0, 1fr); gap: 40px; padding: 28px; }
}

@media (max-width: 760px) {
  .hh-inner { padding: 52px 0 40px; }
  .hh-lead { font-size: 15px; }
  .hh-window { margin-top: 40px; border-radius: 12px; }
  .hh-route { grid-template-columns: minmax(0, 1fr); gap: 12px; padding: 16px; }
  .hh-wires { display: none; }
  .hh-col { grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 8px; }
  .hh-node { gap: 8px; padding: 10px; }
  .hh-icon { width: 28px; height: 28px; }
  .hh-node small { display: none; }
  .hh-hub { grid-auto-flow: column; justify-content: center; align-items: center; padding: 12px; box-shadow: 0 0 0 4px var(--lc-accent-soft); }
  .hh-hub-tag, .hh-hub > span:last-child { display: none; }
}
</style>
