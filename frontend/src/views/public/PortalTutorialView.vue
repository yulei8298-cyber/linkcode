<template>
  <PortalLayout>
    <div class="mb-6">
      <h1 class="text-2xl font-bold sm:text-3xl">{{ t('portal.tutorial.title') }}</h1>
      <p class="mt-2 text-sm text-gray-500 dark:text-dark-400">{{ t('portal.tutorial.subtitle') }}</p>
    </div>

    <article
      v-if="hasContent"
      class="portal-tutorial-content rounded-2xl border border-gray-200/80 bg-white/80 p-6 dark:border-dark-700/70 dark:bg-dark-900/60 sm:p-8"
      v-html="renderedHtml"
    ></article>
    <div
      v-else
      class="rounded-2xl border border-dashed border-gray-300 bg-white/60 px-6 py-16 text-center text-sm text-gray-500 dark:border-dark-700 dark:bg-dark-900/40 dark:text-dark-400"
    >
      {{ t('portal.tutorial.empty') }}
    </div>
  </PortalLayout>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { marked } from 'marked'
import DOMPurify from 'dompurify'
import PortalLayout from './components/PortalLayout.vue'
import { useAppStore } from '@/stores/app'

const { t } = useI18n()
const appStore = useAppStore()

marked.setOptions({ breaks: true, gfm: true })

const contentMD = computed(() => appStore.cachedPublicSettings?.tutorial_content_md || '')
const hasContent = computed(() => Boolean(contentMD.value.trim()))

const renderedHtml = computed(() => {
  const content = contentMD.value.trim()
  if (!content) return ''
  const html = marked.parse(content) as string
  return DOMPurify.sanitize(html)
})
</script>

<style scoped>
.portal-tutorial-content {
  line-height: 1.75;
  overflow-wrap: anywhere;
}
.portal-tutorial-content :deep(h1) {
  @apply mb-4 mt-8 border-b border-gray-200 pb-3 text-2xl font-bold first:mt-0 dark:border-dark-700;
}
.portal-tutorial-content :deep(h2) {
  @apply mb-3 mt-7 text-xl font-bold first:mt-0;
}
.portal-tutorial-content :deep(h3) {
  @apply mb-2 mt-6 text-lg font-semibold;
}
.portal-tutorial-content :deep(p) {
  @apply mb-4 text-gray-700 dark:text-dark-200;
}
.portal-tutorial-content :deep(a) {
  @apply text-primary-600 underline underline-offset-4 hover:text-primary-700 dark:text-primary-300 dark:hover:text-primary-200;
}
.portal-tutorial-content :deep(ul) {
  @apply mb-4 list-disc pl-6;
}
.portal-tutorial-content :deep(ol) {
  @apply mb-4 list-decimal pl-6;
}
.portal-tutorial-content :deep(li) {
  @apply mb-1 text-gray-700 dark:text-dark-200;
}
.portal-tutorial-content :deep(blockquote) {
  @apply my-5 border-l-4 border-gray-300 pl-4 text-gray-600 dark:border-dark-600 dark:text-dark-300;
}
.portal-tutorial-content :deep(code) {
  @apply rounded bg-gray-100 px-1.5 py-0.5 font-mono text-sm dark:bg-dark-800;
}
.portal-tutorial-content :deep(pre) {
  @apply my-5 overflow-x-auto rounded-lg bg-gray-950 p-4 text-gray-100;
}
.portal-tutorial-content :deep(pre code) {
  @apply bg-transparent p-0 text-inherit;
}
.portal-tutorial-content :deep(table) {
  @apply my-5 block w-full overflow-x-auto border-collapse;
}
.portal-tutorial-content :deep(th) {
  @apply border border-gray-300 bg-gray-50 px-3 py-2 text-left font-semibold dark:border-dark-600 dark:bg-dark-800;
}
.portal-tutorial-content :deep(td) {
  @apply border border-gray-300 px-3 py-2 dark:border-dark-600;
}
.portal-tutorial-content :deep(img) {
  @apply my-5 h-auto max-w-full rounded-lg;
}
.portal-tutorial-content :deep(hr) {
  @apply my-7 border-gray-200 dark:border-dark-700;
}
</style>
