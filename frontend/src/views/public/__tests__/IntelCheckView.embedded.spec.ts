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

  it('门户标题和控制台标题按形态互斥显示', () => {
    expect(source).toContain('<section v-if="!isEmbedded" class="lc-page-head">')
    expect(source).toContain('<div v-if="isEmbedded" class="lc-card lc-ic-embedded-head">')
  })
})
