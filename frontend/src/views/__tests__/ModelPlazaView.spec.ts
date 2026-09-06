import { reactive } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import ModelPlazaView from '../ModelPlazaView.vue'

const mocks = vi.hoisted(() => ({ getModelPlaza: vi.fn(), auth: { user: null as { id: number } | null, isAuthenticated: false }, query: {} as Record<string, string> }))
const auth = reactive(mocks.auth)
const route = reactive({ query: mocks.query })
vi.mock('@/api/modelPlaza', () => ({ getModelPlaza: mocks.getModelPlaza }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => auth }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ fetchPublicSettings: vi.fn() }) }))
vi.mock('vue-router', () => ({ useRoute: () => route }))
vi.mock('@/components/layout/AppLayout.vue', () => ({ default: { template: '<div class="console"><slot /></div>' } }))
vi.mock('@/views/public/components/PortalLayout.vue', () => ({ default: { template: '<div class="portal"><slot /></div>' } }))
vi.mock('@/components/modelPlaza/ModelPlazaContent.vue', () => ({ default: { name: 'ModelPlazaContent', props: ['response', 'loading', 'error', 'portal'], emits: ['retry'], template: '<div />' } }))

const response = { description: '', groups: [] }
beforeEach(() => { mocks.getModelPlaza.mockReset(); auth.user = null; auth.isAuthenticated = false; route.query = {} })

describe('ModelPlazaView', () => {
  it('uses the public portal and retains embedded console links', async () => {
    mocks.getModelPlaza.mockResolvedValue(response)
    const wrapper = mount(ModelPlazaView)
    await flushPromises()
    expect(wrapper.find('.portal').exists()).toBe(true)
    auth.user = { id: 1 }; auth.isAuthenticated = true; route.query = { embedded: '1' }
    await flushPromises()
    expect(wrapper.find('.console').exists()).toBe(true)
    wrapper.unmount()
  })
  it('retries an API failure', async () => {
    mocks.getModelPlaza.mockRejectedValueOnce(new Error('offline')).mockResolvedValueOnce(response)
    const wrapper = mount(ModelPlazaView)
    await flushPromises()
    const child = wrapper.findComponent({ name: 'ModelPlazaContent' })
    expect(child.props('error')).toBe(true)
    child.vm.$emit('retry')
    await flushPromises()
    expect(child.props('error')).toBe(false)
    expect(child.props('response')).toEqual(response)
    wrapper.unmount()
  })
  it('aborts and discards stale authenticated prices after sign-out', async () => {
    let resolveOld!: (value: typeof response) => void
    mocks.getModelPlaza.mockImplementationOnce(() => new Promise(resolve => { resolveOld = resolve })).mockResolvedValueOnce(response)
    auth.user = { id: 1 }; auth.isAuthenticated = true
    const wrapper = mount(ModelPlazaView)
    const signal = mocks.getModelPlaza.mock.calls[0][0].signal as AbortSignal
    auth.user = null; auth.isAuthenticated = false
    await flushPromises()
    expect(signal.aborted).toBe(true)
    resolveOld({ description: 'private-price', groups: [] })
    await flushPromises()
    expect(wrapper.findComponent({ name: 'ModelPlazaContent' }).props('response')).toEqual(response)
    wrapper.unmount()
  })
})
