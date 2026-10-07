import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { nextTick, reactive } from 'vue'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return { ...actual, useI18n: () => ({ t: (key: string, params?: Record<string, unknown>) => (params ? `${key}:${JSON.stringify(params)}` : key) }) }
})

const authState = reactive({ isAuthenticated: true, user: { username: 'yuxixi', email: 'yu@example.com' } as { username?: string; email?: string } | null })
vi.mock('@/stores/auth', () => ({ useAuthStore: () => authState }))

// 当前路由：控制台页面 requiresAuth 缺省（默认需要登录），首页显式为 false
const routeState = reactive({ matched: [{}] as unknown[], meta: {} as Record<string, unknown> })
vi.mock('vue-router', () => ({ useRoute: () => routeState }))
const goConsole = () => Object.assign(routeState, { matched: [{}], meta: {} })
const goHome = () => Object.assign(routeState, { matched: [{}], meta: { requiresAuth: false } })

const getEnterpriseStatus = vi.hoisted(() => vi.fn())
vi.mock('@/api/enterprise', () => ({ getEnterpriseStatus }))

import EnterpriseWelcome from '../EnterpriseWelcome.vue'
import { useEnterpriseStore } from '@/stores/enterprise'

const status = (enterprise: boolean) => ({ enterprise, mode: 'auto', total: enterprise ? 3500 : 10, threshold: 3000, enabled: true })

describe('企业尊享欢迎提示', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    setActivePinia(createPinia())
    authState.isAuthenticated = true
    authState.user = { username: 'yuxixi', email: 'yu@example.com' }
    goConsole()
    getEnterpriseStatus.mockReset()
  })
  afterEach(() => vi.useRealTimers())

  it('企业用户拿到状态后弹出欢迎提示，显示用户名', async () => {
    getEnterpriseStatus.mockResolvedValue(status(true))
    const wrapper = mount(EnterpriseWelcome, { global: { stubs: { Transition: false } } })
    expect(wrapper.find('[data-test="enterprise-welcome"]').exists()).toBe(false)

    await useEnterpriseStore().fetch()
    await nextTick()
    const toast = wrapper.get('[data-test="enterprise-welcome"]')
    expect(toast.text()).toContain('enterprise.badge')
    expect(toast.text()).toContain('"name":"yuxixi"')
  })

  it('非企业用户不弹出', async () => {
    getEnterpriseStatus.mockResolvedValue(status(false))
    const wrapper = mount(EnterpriseWelcome)
    await useEnterpriseStore().fetch()
    await nextTick()
    expect(wrapper.find('[data-test="enterprise-welcome"]').exists()).toBe(false)
  })

  it('约 5 秒后自动消失，且不会再次弹出（同一次访问内只提示一次）', async () => {
    getEnterpriseStatus.mockResolvedValue(status(true))
    const wrapper = mount(EnterpriseWelcome)
    const store = useEnterpriseStore()
    await store.fetch()
    await nextTick()
    expect(wrapper.find('[data-test="enterprise-welcome"]').exists()).toBe(true)

    vi.advanceTimersByTime(5100)
    await nextTick()
    expect(wrapper.find('[data-test="enterprise-welcome"]').exists()).toBe(false)
    expect(store.welcomePending).toBe(false)
    expect(store.isEnterprise).toBe(true)
  })

  it('点关闭立即收起；没有用户名时用邮箱前缀', async () => {
    getEnterpriseStatus.mockResolvedValue(status(true))
    authState.user = { email: 'yu@example.com' }
    const wrapper = mount(EnterpriseWelcome)
    await useEnterpriseStore().fetch()
    await nextTick()
    expect(wrapper.text()).toContain('"name":"yu"')
    await wrapper.get('[data-test="enterprise-welcome-close"]').trigger('click')
    expect(wrapper.find('[data-test="enterprise-welcome"]').exists()).toBe(false)
  })

  it('登出后清空状态，下次登录重新提示', async () => {
    getEnterpriseStatus.mockResolvedValue(status(true))
    const wrapper = mount(EnterpriseWelcome)
    const store = useEnterpriseStore()
    await store.fetch()
    await nextTick()
    store.reset()
    await nextTick()
    expect(wrapper.find('[data-test="enterprise-welcome"]').exists()).toBe(false)
    expect(store.isEnterprise).toBe(false)

    await store.fetch()
    await nextTick()
    expect(wrapper.find('[data-test="enterprise-welcome"]').exists()).toBe(true)
  })

  it('拉取失败不打扰用户', async () => {
    getEnterpriseStatus.mockRejectedValue(new Error('boom'))
    const spy = vi.spyOn(console, 'error').mockImplementation(() => {})
    const wrapper = mount(EnterpriseWelcome)
    await useEnterpriseStore().fetch()
    await nextTick()
    expect(wrapper.find('[data-test="enterprise-welcome"]').exists()).toBe(false)
    spy.mockRestore()
  })

  it('首页不弹，进入控制台才弹，之后在控制台切换页面不再重复', async () => {
    getEnterpriseStatus.mockResolvedValue(status(true))
    goHome()
    const wrapper = mount(EnterpriseWelcome)
    const store = useEnterpriseStore()
    await store.fetch()
    await nextTick()
    expect(wrapper.find('[data-test="enterprise-welcome"]').exists()).toBe(false)
    expect(store.welcomePending).toBe(true)

    goConsole()
    await nextTick()
    expect(wrapper.find('[data-test="enterprise-welcome"]').exists()).toBe(true)
    expect(store.welcomePending).toBe(false)

    await wrapper.get('[data-test="enterprise-welcome-close"]').trigger('click')
    routeState.meta = { title: 'other console page' }
    await nextTick()
    expect(wrapper.find('[data-test="enterprise-welcome"]').exists()).toBe(false)
  })

  it('应用刚启动、路由还没解析时不弹', async () => {
    getEnterpriseStatus.mockResolvedValue(status(true))
    Object.assign(routeState, { matched: [], meta: {} })
    const wrapper = mount(EnterpriseWelcome)
    await useEnterpriseStore().fetch()
    await nextTick()
    expect(wrapper.find('[data-test="enterprise-welcome"]').exists()).toBe(false)
  })
})
