<template>
  <PortalDocPage :title="pageTitle" :meta="pageMeta" :html="doc.html" :toc="doc.toc">
    <template v-if="loading">{{ t('common.loading') }}</template>
    <template v-else-if="loadError">{{ t('legal.retryLater') }}</template>
    <template v-else-if="!currentDocument">{{ t('legal.notFoundDescription') }}</template>
    <template v-else>{{ t('legal.empty') }}</template>
  </PortalDocPage>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import PortalDocPage from './components/PortalDocPage.vue'
import { getLocale } from '@/i18n'
import { useAppStore } from '@/stores/app'
import { renderMarkdownDoc } from '@/utils/markdownDoc'
import type { LoginAgreementDocument } from '@/types'
import zhAdminCompliance from '../../../../docs/legal/admin-compliance.zh.md?raw'
import enAdminCompliance from '../../../../docs/legal/admin-compliance.en.md?raw'

const route = useRoute()
const { t } = useI18n()
const appStore = useAppStore()
const settings = computed(() => appStore.cachedPublicSettings)
const loading = ref(!settings.value)
const loadError = ref(false)

const documentId = computed(() => String(route.params.documentId || ''))
const isAdminComplianceDocument = computed(() => documentId.value === 'admin-compliance')
const documents = computed(() => settings.value?.login_agreement_documents ?? [])
const updatedAt = computed(() =>
  isAdminComplianceDocument.value ? '' : settings.value?.login_agreement_updated_at || ''
)
const documentTypeLabel = computed(() =>
  isAdminComplianceDocument.value ? t('legal.adminCompliance') : t('legal.loginAgreement')
)

const currentDocument = computed<LoginAgreementDocument | null>(() => {
  if (isAdminComplianceDocument.value) {
    return {
      id: 'admin-compliance',
      title: t('adminCompliance.title'),
      content_md: getLocale() === 'zh' ? zhAdminCompliance : enAdminCompliance
    }
  }
  const id = documentId.value
  if (!id) {
    return null
  }
  return documents.value.find((doc) => doc.id === id) ?? null
})

const doc = computed(() => renderMarkdownDoc(currentDocument.value?.content_md || ''))

const pageTitle = computed(() => {
  if (currentDocument.value) return currentDocument.value.title
  if (loading.value) return documentTypeLabel.value
  return loadError.value ? t('legal.loadFailed') : t('legal.notFound')
})

const pageMeta = computed(() => {
  if (!currentDocument.value) return ''
  return updatedAt.value
    ? `${documentTypeLabel.value} · ${t('legal.updatedAt', { date: updatedAt.value })}`
    : documentTypeLabel.value
})

onMounted(async () => {
  loadError.value = false
  const loadedSettings = await appStore.fetchPublicSettings()
  if (!loadedSettings) {
    loadError.value = true
  }
  loading.value = false
})
</script>
