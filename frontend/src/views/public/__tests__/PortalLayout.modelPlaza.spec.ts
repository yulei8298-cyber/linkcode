import { reactive } from 'vue'
import { mount } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'
import { describe, expect, it, vi } from 'vitest'
import PortalLayout from '../components/PortalLayout.vue'

const auth = reactive({ isAuthenticated: false, isAdmin: false, checkAuth: vi.fn() })
const app = reactive({ cachedPublicSettings: { model_plaza_enabled: true, model_plaza_require_auth: false, site_name: 'LinkCode', chat_station_url: '' }, fetchPublicSettings: vi.fn() })
vi.mock('@/stores', () => ({ useAuthStore: () => auth, useAppStore: () => app }))
vi.mock('@/api', () => ({ lobeHubSSOAPI: { authorize: vi.fn() } }))

async function mountPortal() {
  const router = createRouter({ history: createMemoryHistory(), routes: ['/home', '/model-plaza', '/portal/pricing', '/portal/status', '/login', '/register', '/dashboard'].map(path => ({ path, component: { template: '<div />' } })) })
  await router.push('/home')
  return mount(PortalLayout, { global: { plugins: [router], stubs: { Icon: true } } })
}

describe('public model plaza entry', () => {
  it('places the model plaza immediately after pricing and closes the mobile menu on navigation', async () => {
    app.cachedPublicSettings.model_plaza_enabled = true
    app.cachedPublicSettings.model_plaza_require_auth = false
    const wrapper = await mountPortal()
    const links = wrapper.findAll('.lc-navlinks a')
    expect(links.map(link => link.text())).toEqual(['首页', '可用性检测', '定价方案', '模型广场'])
    expect(links[3].attributes('href')).toBe('/model-plaza')
    await wrapper.find('.lc-menu-button').trigger('click')
    expect(wrapper.find('.lc-navlinks').classes()).toContain('open')
    await links[3].trigger('click')
    expect(wrapper.find('.lc-navlinks').classes()).not.toContain('open')
    wrapper.unmount()
  })
  it('respects the existing feature and authentication gates', async () => {
    app.cachedPublicSettings.model_plaza_enabled = false
    auth.isAuthenticated = false
    const wrapper = await mountPortal()
    expect(wrapper.find('a[href="/model-plaza"]').exists()).toBe(false)
    app.cachedPublicSettings.model_plaza_enabled = true
    app.cachedPublicSettings.model_plaza_require_auth = true
    await wrapper.vm.$nextTick()
    expect(wrapper.find('a[href="/model-plaza"]').exists()).toBe(false)
    auth.isAuthenticated = true
    await wrapper.vm.$nextTick()
    expect(wrapper.find('a[href="/model-plaza"]').exists()).toBe(true)
    wrapper.unmount()
    auth.isAuthenticated = false
  })
})
