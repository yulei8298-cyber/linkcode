import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const source = readFileSync(
  resolve(dirname(fileURLToPath(import.meta.url)), '../IntelCheckView.vue'),
  'utf8',
)

describe('IntelCheckView 内嵌形态', () => {
  it('仅在登录用户携带 embedded=1 时使用控制台布局', () => {
    expect(source).toContain(
      "const isEmbedded = computed(() => route.query.embedded === '1' && authStore.isAuthenticated)",
    )
    expect(source).toContain('<component :is="isEmbedded ? AppLayout : PortalLayout">')
    expect(source).toContain('</component>')
  })

  it('两种布局共用状态页内容，并分别应用门户留白与深色变量', () => {
    expect(source).toContain('<div class="ic-root" :class="{ \'ic-portal\': !isEmbedded }">')
    expect(source).toContain('<div class="ic-topbar">')
    expect(source).toContain('.lc-shell .ic-root,')
    expect(source).toContain('.dark .ic-root {')
  })
})
