import { describe, expect, it } from 'vitest'
import { buildCcSwitchImportDeeplink, normalizeGatewayRootEndpoint } from '../ccswitchImport'

describe('Antigravity CC Switch endpoint', () => {
  it.each([
    ['https://api.example.com', 'https://api.example.com/antigravity'],
    ['https://api.example.com/', 'https://api.example.com/antigravity'],
    ['https://api.example.com///', 'https://api.example.com/antigravity'],
    ['https://api.example.com/sub2api/', 'https://api.example.com/sub2api/antigravity']
  ])('joins the platform path onto %s for both clients', (baseUrl, endpoint) => {
    for (const clientType of ['claude', 'gemini'] as const) {
      const url = new URL(buildCcSwitchImportDeeplink({
        baseUrl, clientType, platform: 'antigravity', providerName: 'Sub2API',
        apiKey: 'sk-test', usageScript: 'return true'
      }))
      expect(url.searchParams.get('endpoint')).toBe(endpoint)
      expect(url.searchParams.get('app')).toBe(clientType)
      // LinkCode 的 homepage 统一使用归一化后的网关根地址。
      expect(url.searchParams.get('homepage')).toBe(normalizeGatewayRootEndpoint(baseUrl))
      expect(url.searchParams.has('model')).toBe(false)
    }
  })
})
