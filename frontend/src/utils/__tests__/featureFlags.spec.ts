import { describe, it, expect, beforeEach, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useAppStore } from '@/stores/app'
import {
  FeatureFlags,
  isFeatureFlagEnabled,
  makeSidebarFlag,
  resetFeatureFlagMemory,
  resolveFeatureFlag,
} from '@/utils/featureFlags'
import type { PublicSettings } from '@/types'

vi.mock('@/api/admin/system', () => ({
  checkUpdates: vi.fn(),
}))

vi.mock('@/api/auth', () => ({
  getPublicSettings: vi.fn(),
}))

describe('FeatureFlags.subscription', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    resetFeatureFlagMemory()
    delete (window as any).__APP_CONFIG__
  })

  it('reads subscription_enabled as an opt-out flag: visible before settings load', () => {
    expect(FeatureFlags.subscription.key).toBe('subscription_enabled')
    expect(FeatureFlags.subscription.mode).toBe('opt-out')
    expect(useAppStore().cachedPublicSettings).toBeNull()
    expect(isFeatureFlagEnabled(FeatureFlags.subscription)).toBe(true)
  })

  it('hides only when the backend explicitly sends false', () => {
    const store = useAppStore()
    const sidebarFlag = makeSidebarFlag(FeatureFlags.subscription)

    store.cachedPublicSettings = { subscription_enabled: false } as PublicSettings
    expect(sidebarFlag()).toBe(false)

    store.cachedPublicSettings = { subscription_enabled: true } as PublicSettings
    expect(sidebarFlag()).toBe(true)

    store.cachedPublicSettings = {} as PublicSettings
    expect(sidebarFlag()).toBe(true)
  })
})

describe('resolveFeatureFlag', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    resetFeatureFlagMemory()
  })

  it('reads an explicit boolean from the given settings object', () => {
    expect(resolveFeatureFlag({ subscription_enabled: false } as PublicSettings, FeatureFlags.subscription)).toBe(false)
    expect(resolveFeatureFlag({ subscription_enabled: true } as PublicSettings, FeatureFlags.subscription)).toBe(true)
    expect(resolveFeatureFlag({ available_channels_enabled: true } as PublicSettings, FeatureFlags.availableChannels)).toBe(true)
  })

  it('falls back to the declared mode when settings are missing or the key is absent', () => {
    expect(resolveFeatureFlag(undefined, FeatureFlags.subscription)).toBe(true)
    expect(resolveFeatureFlag(null, FeatureFlags.subscription)).toBe(true)
    expect(resolveFeatureFlag({} as PublicSettings, FeatureFlags.subscription)).toBe(true)
    expect(resolveFeatureFlag({} as PublicSettings, FeatureFlags.availableChannels)).toBe(false)
  })

  it('backs isFeatureFlagEnabled with the same resolution', () => {
    useAppStore().cachedPublicSettings = { subscription_enabled: false } as PublicSettings
    expect(isFeatureFlagEnabled(FeatureFlags.subscription)).toBe(false)
  })
})

describe('feature flag memory', () => {
  beforeEach(() => {
    resetFeatureFlagMemory()
  })

  it('reuses the last resolved value while settings are not loaded', () => {
    resolveFeatureFlag({ available_channels_enabled: true } as PublicSettings, FeatureFlags.availableChannels)
    resolveFeatureFlag({ subscription_enabled: false } as PublicSettings, FeatureFlags.subscription)

    // 记忆优先于 mode 默认值：opt-in 记住了 true，opt-out 记住了 false。
    expect(resolveFeatureFlag({} as PublicSettings, FeatureFlags.availableChannels)).toBe(true)
    expect(resolveFeatureFlag(null, FeatureFlags.subscription)).toBe(false)
  })

  it('lets a loaded value override and replace the remembered one', () => {
    resolveFeatureFlag({ available_channels_enabled: true } as PublicSettings, FeatureFlags.availableChannels)
    expect(resolveFeatureFlag({ available_channels_enabled: false } as PublicSettings, FeatureFlags.availableChannels)).toBe(false)
    expect(resolveFeatureFlag(undefined, FeatureFlags.availableChannels)).toBe(false)
  })

  it('persists across a cold start through localStorage', () => {
    resolveFeatureFlag({ available_channels_enabled: true } as PublicSettings, FeatureFlags.availableChannels)
    const stored = JSON.parse(window.localStorage.getItem('sub2api:feature-flag-memory') ?? '{}')
    expect(stored).toEqual({ available_channels_enabled: true })
  })

  it('falls back to the declared mode after the memory is reset', () => {
    resolveFeatureFlag({ available_channels_enabled: true } as PublicSettings, FeatureFlags.availableChannels)
    resetFeatureFlagMemory()
    expect(resolveFeatureFlag({} as PublicSettings, FeatureFlags.availableChannels)).toBe(false)
    expect(window.localStorage.getItem('sub2api:feature-flag-memory')).toBeNull()
  })
})
