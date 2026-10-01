import type { GroupPlatform } from '@/types'

export const OPENAI_CC_SWITCH_CODEX_MODEL = 'gpt-5.5'
export const GROK_CC_SWITCH_MODEL = 'grok-4.5'

export type CcSwitchClientType = 'claude' | 'gemini'

export interface CcSwitchImportConfig {
  app: string
  endpoint: string
  model?: string
}

export interface CcSwitchImportDeeplinkInput {
  baseUrl: string
  platform?: GroupPlatform | null
  clientType: CcSwitchClientType
  providerName: string
  apiKey: string
  usageScript: string
}

export function normalizeGatewayRootEndpoint(baseUrl: string): string {
  return baseUrl.replace(/\/+$/, '').replace(/\/v1$/, '')
}

/**
 * Balance query CC Switch runs against the imported provider. CC Switch fills
 * `{{baseUrl}}` with the provider's base URL as stored — Grok imports carry a
 * trailing `/v1` (see `ensureOpenAIV1Endpoint`), others do not, and users may
 * edit it either way afterwards — then evaluates the script, so the URL strips
 * an existing `/v1` instead of blindly appending one (`/v1/v1/usage` is a 404
 * and CC Switch shows "query failed").
 */
export const CC_SWITCH_USAGE_SCRIPT = `({
    request: {
      url: "{{baseUrl}}".replace(/\\/+$/, "").replace(/\\/v1$/, "") + "/v1/usage",
      method: "GET",
      headers: { "Authorization": "Bearer {{apiKey}}" }
    },
    extractor: function(response) {
      const remaining = response?.remaining ?? response?.quota?.remaining ?? response?.balance;
      const unit = response?.unit ?? response?.quota?.unit ?? "USD";
      return {
        isValid: response?.is_active ?? response?.isValid ?? true,
        remaining,
        unit
      };
    }
  })`

export function ensureOpenAIV1Endpoint(baseUrl: string): string {
  const trimmed = baseUrl.replace(/\/+$/, '')
  return trimmed.endsWith('/v1') ? trimmed : `${trimmed}/v1`
}

export function resolveCcSwitchImportConfig(
  platform: GroupPlatform | undefined | null,
  clientType: CcSwitchClientType,
  baseUrl: string
): CcSwitchImportConfig {
  const rootEndpoint = normalizeGatewayRootEndpoint(baseUrl)

  switch (platform || 'anthropic') {
    case 'antigravity':
      return {
        app: clientType === 'gemini' ? 'gemini' : 'claude',
        endpoint: `${rootEndpoint}/antigravity`
      }
    case 'openai':
      return {
        app: 'codex',
        endpoint: rootEndpoint,
        model: OPENAI_CC_SWITCH_CODEX_MODEL
      }
    case 'gemini':
      return {
        app: 'gemini',
        endpoint: rootEndpoint
      }
    case 'grok':
      return {
        app: 'grokbuild',
        endpoint: ensureOpenAIV1Endpoint(baseUrl),
        model: GROK_CC_SWITCH_MODEL
      }
    default:
      return {
        app: 'claude',
        endpoint: rootEndpoint
      }
  }
}

export function buildCcSwitchImportDeeplink(input: CcSwitchImportDeeplinkInput): string {
  const config = resolveCcSwitchImportConfig(input.platform, input.clientType, input.baseUrl)
  const homepage = normalizeGatewayRootEndpoint(input.baseUrl)
  const entries: [string, string][] = [
    ['resource', 'provider'],
    ['app', config.app],
    ['name', input.providerName],
    ['homepage', homepage],
    ['endpoint', config.endpoint],
    ['apiKey', input.apiKey],
    ['configFormat', 'json'],
    ['usageEnabled', 'true'],
    ['usageScript', btoa(input.usageScript)],
    ['usageAutoInterval', '30']
  ]

  if (config.model) {
    entries.splice(2, 0, ['model', config.model])
  }

  return `ccswitch://v1/import?${new URLSearchParams(entries).toString()}`
}
