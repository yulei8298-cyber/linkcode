import { beforeAll, beforeEach, describe, expect, it, vi } from 'vitest'

type NavigationGuard = (
  to: Record<string, any>,
  from: Record<string, any>,
  next: ReturnType<typeof vi.fn>,
) => Promise<void>

const harness = vi.hoisted(() => ({ guard: null as NavigationGuard | null }))

const authStore = vi.hoisted(() => ({
  checkAuth: vi.fn(),
  isAuthenticated: false,
  isAdmin: false,
  isSimpleMode: false,
  hasPendingAuthSession: false,
}))

const appStore = vi.hoisted(() => ({
  siteName: 'Sub2API',
  backendModeEnabled: false,
  publicSettingsLoaded: false,
  cachedPublicSettings: null as null | { intel_check_enabled?: boolean },
  fetchPublicSettings: vi.fn(),
}))

vi.mock('vue-router', () => ({
  createWebHistory: vi.fn(() => ({})),
  createRouter: vi.fn(() => ({
    beforeEach: vi.fn((guard: NavigationGuard) => {
      harness.guard = guard
    }),
    afterEach: vi.fn(),
    onError: vi.fn(),
  })),
}))

vi.mock('@/stores/auth', () => ({ useAuthStore: () => authStore }))
vi.mock('@/stores/app', () => ({ useAppStore: () => appStore }))
vi.mock('@/stores/adminSettings', () => ({ useAdminSettingsStore: () => ({ customMenuItems: [] }) }))
vi.mock('@/stores/adminCompliance', () => ({
  useAdminComplianceStore: () => ({ initialized: true, fetchStatus: vi.fn(), requireAcknowledgement: vi.fn() }),
}))
vi.mock('@/composables/useNavigationLoading', () => ({
  useNavigationLoadingState: () => ({ startNavigation: vi.fn(), endNavigation: vi.fn(), isLoading: { value: false } }),
}))
vi.mock('@/composables/useRoutePrefetch', () => ({
  useRoutePrefetch: () => ({ triggerPrefetch: vi.fn(), cancelPendingPrefetch: vi.fn(), resetPrefetchState: vi.fn() }),
}))

function runGuard() {
  if (!harness.guard) throw new Error('router guard was not registered')
  const next = vi.fn()
  const navigation = harness.guard(
    {
      path: '/portal/intel-check',
      fullPath: '/portal/intel-check',
      name: 'PortalIntelCheck',
      params: {},
      meta: { requiresAuth: false },
    },
    {},
    next,
  )
  return { navigation, next }
}

describe('智力检测公开页路由守卫', () => {
  beforeAll(async () => {
    await import('@/router')
  })

  beforeEach(() => {
    authStore.isAuthenticated = false
    authStore.isAdmin = false
    authStore.isSimpleMode = false
    appStore.publicSettingsLoaded = false
    appStore.cachedPublicSettings = null
    appStore.fetchPublicSettings.mockReset()
  })

  it('在设置请求失败时保留路由，让后端 404 兜底', async () => {
    appStore.fetchPublicSettings.mockRejectedValueOnce(new Error('temporary failure'))

    const { navigation, next } = runGuard()
    await navigation

    expect(next).toHaveBeenCalledOnce()
    expect(next).toHaveBeenCalledWith()
  })

  it('只在已加载且明确关闭时把匿名访客送回门户首页', async () => {
    appStore.publicSettingsLoaded = true
    appStore.cachedPublicSettings = { intel_check_enabled: false }

    const { navigation, next } = runGuard()
    await navigation

    expect(next).toHaveBeenCalledOnce()
    expect(next).toHaveBeenCalledWith('/home')
  })

  it('明确开启时允许匿名访问', async () => {
    appStore.publicSettingsLoaded = true
    appStore.cachedPublicSettings = { intel_check_enabled: true }

    const { navigation, next } = runGuard()
    await navigation

    expect(next).toHaveBeenCalledOnce()
    expect(next).toHaveBeenCalledWith()
  })
})
