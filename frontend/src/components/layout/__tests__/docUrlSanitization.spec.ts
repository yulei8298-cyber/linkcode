import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const dir = dirname(fileURLToPath(import.meta.url))
const headerSource = readFileSync(resolve(dir, '../AppHeader.vue'), 'utf8')
const homeViewSource = readFileSync(resolve(dir, '../../../views/HomeView.vue'), 'utf8')
const portalLayoutSource = readFileSync(resolve(dir, '../../../views/public/components/PortalLayout.vue'), 'utf8')
const keyUsageViewSource = readFileSync(resolve(dir, '../../../views/KeyUsageView.vue'), 'utf8')

describe('doc_url sanitization', () => {
  it('AppHeader imports sanitizeUrl', () => {
    expect(headerSource).toContain("import { sanitizeUrl } from '@/utils/url'")
  })

  it('AppHeader applies sanitizeUrl to docUrl', () => {
    expect(headerSource).toContain('sanitizeUrl(appStore.docUrl)')
  })

  it('HomeView imports sanitizeUrl', () => {
    expect(homeViewSource).toContain("import { sanitizeUrl } from '@/utils/url'")
  })

  it('PortalLayout applies sanitizeUrl to the home page docUrl', () => {
    expect(portalLayoutSource).toContain('sanitizeUrl(appStore.cachedPublicSettings?.doc_url || appStore.docUrl')
  })

  // KeyUsageView 改用门户外壳，文档链接由 PortalLayout 净化后渲染，自身不得再直接读取 doc_url
  it('KeyUsageView renders doc links only through PortalLayout', () => {
    expect(keyUsageViewSource).toContain("import PortalLayout from '@/views/public/components/PortalLayout.vue'")
    expect(keyUsageViewSource).not.toContain('doc_url')
  })
})
