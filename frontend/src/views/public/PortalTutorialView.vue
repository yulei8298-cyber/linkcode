<template>
  <PortalDocPage
    :title="t('portal.tutorial.title')"
    :meta="t('portal.tutorial.subtitle')"
    :html="doc.html"
    :toc="doc.toc"
  >
    {{ t('portal.tutorial.empty') }}
  </PortalDocPage>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import PortalDocPage from './components/PortalDocPage.vue'
import { useAppStore } from '@/stores/app'
import { renderMarkdownDoc } from '@/utils/markdownDoc'
import { DEFAULT_TUTORIAL_MD } from './defaultTutorial'

const { t } = useI18n()
const appStore = useAppStore()

// 后台配置了教程内容则用后台的，否则回退到内置默认教程。
const contentMD = computed(() => {
  const configured = appStore.cachedPublicSettings?.tutorial_content_md || ''
  return configured.trim() ? configured : DEFAULT_TUTORIAL_MD
})
const doc = computed(() => renderMarkdownDoc(contentMD.value))
</script>
