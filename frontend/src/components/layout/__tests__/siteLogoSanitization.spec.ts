import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const dir = dirname(fileURLToPath(import.meta.url))
const sidebarSource = readFileSync(resolve(dir, '../AppSidebar.vue'), 'utf8')
const portalLayoutSource = readFileSync(resolve(dir, '../../../views/public/components/PortalLayout.vue'), 'utf8')
const keyUsageViewSource = readFileSync(resolve(dir, '../../../views/KeyUsageView.vue'), 'utf8')

describe('site_logo sanitization', () => {
  it('AppSidebar imports sanitizeUrl and applies it to siteLogo', () => {
    expect(sidebarSource).toContain("import { sanitizeUrl } from '@/utils/url'")
    expect(sidebarSource).toContain('sanitizeUrl(appStore.siteLogo')
  })

  it('PortalLayout applies sanitizeUrl to the home page siteLogo', () => {
    expect(portalLayoutSource).toContain('sanitizeUrl(appStore.cachedPublicSettings?.site_logo || appStore.siteLogo')
  })

  // KeyUsageView 改用门户外壳渲染 Logo，自身不得再直接读取未净化的 site_logo
  it('KeyUsageView renders the logo only through PortalLayout', () => {
    expect(keyUsageViewSource).toContain("import PortalLayout from '@/views/public/components/PortalLayout.vue'")
    expect(keyUsageViewSource).not.toContain('site_logo')
  })

  it('logo renderers pass allowRelative and allowDataUrl options', () => {
    for (const src of [sidebarSource, portalLayoutSource]) {
      expect(src).toContain('allowRelative: true')
      expect(src).toContain('allowDataUrl: true')
    }
  })
})
