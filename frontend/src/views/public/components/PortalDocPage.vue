<template>
  <PortalLayout>
    <div class="lc-wrap lc-doc">
      <header class="lc-doc-head">
        <p class="lc-crumb"><RouterLink to="/home">首页</RouterLink> / {{ crumb || title }}</p>
        <h1 class="lc-doc-title">{{ title }}</h1>
        <p v-if="meta" class="lc-doc-meta">{{ meta }}</p>
      </header>

      <div class="lc-doc-body" :class="{ 'has-toc': toc.length > 1 }">
        <nav v-if="toc.length > 1" class="lc-doc-toc" aria-label="目录">
          <p class="lc-doc-toc-title">目录</p>
          <a
            v-for="item in toc"
            :key="item.id"
            :href="`#${item.id}`"
            :class="{ active: item.id === activeId }"
            @click.prevent="scrollToHeading(item.id)"
          >{{ item.text }}</a>
        </nav>

        <article v-if="html" ref="articleRef" class="lc-prose" v-html="html"></article>
        <div v-else class="lc-doc-state"><slot /></div>
      </div>
    </div>
  </PortalLayout>
</template>

<script setup lang="ts">
// 公开文档页骨架：面包屑 + 标题 + 左侧目录 + 720 宽正文。html 须由调用方净化
// （见 utils/markdownDoc.ts）；没有正文时渲染默认插槽，用于加载中、空、出错等状态。
import { nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { RouterLink } from 'vue-router'
import PortalLayout from './PortalLayout.vue'
import type { DocHeading } from '@/utils/markdownDoc'

const props = defineProps<{
  title: string
  crumb?: string
  meta?: string
  html: string
  toc: DocHeading[]
}>()

const articleRef = ref<HTMLElement | null>(null)
const activeId = ref('')
let observer: IntersectionObserver | null = null

function scrollToHeading(id: string) {
  const target = document.getElementById(id)
  if (!target) return
  target.scrollIntoView({ behavior: 'smooth', block: 'start' })
  activeId.value = id
}

// 目录高亮当前阅读的小节：取最靠上的、已进入视口上半部分的标题
function observeHeadings() {
  observer?.disconnect()
  observer = null
  activeId.value = props.toc[0]?.id || ''
  if (!articleRef.value || props.toc.length < 2 || typeof IntersectionObserver === 'undefined') return
  observer = new IntersectionObserver((entries) => {
    const visible = entries.filter(entry => entry.isIntersecting)
    if (visible.length) activeId.value = visible[0].target.id
  }, { rootMargin: '-80px 0px -60% 0px' })
  props.toc.forEach(item => {
    const el = articleRef.value?.querySelector(`#${item.id}`)
    if (el) observer?.observe(el)
  })
}

watch(() => props.html, async () => {
  await nextTick()
  observeHeadings()
}, { immediate: true })

onBeforeUnmount(() => observer?.disconnect())
</script>
