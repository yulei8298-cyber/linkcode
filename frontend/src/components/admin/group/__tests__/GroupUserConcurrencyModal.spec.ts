import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import type { AdminGroup } from '@/types'
import GroupUserConcurrencyModal from '../GroupUserConcurrencyModal.vue'

const mocks = vi.hoisted(() => ({ getUserConcurrency: vi.fn(), showError: vi.fn() }))
vi.mock('@/api/admin', () => ({ adminAPI: { groups: { getUserConcurrency: mocks.getUserConcurrency } } }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError: mocks.showError }) }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))

enableAutoUnmount(afterEach)
afterEach(() => vi.useRealTimers())

const group = (id: number, name = `Group ${id}`) => ({ id, name, platform: 'openai' }) as AdminGroup

const summary = (groupId: number, users: unknown[] = []) => ({
  group_id: groupId,
  total: 6,
  api_key_count: 9,
  users,
})

const sampleUsers = [
  {
    user_id: 10,
    email: 'ten@example.com',
    username: 'ten',
    concurrency: 4,
    api_keys: [
      { api_key_id: 1, api_key_name: 'main', concurrency: 3 },
      { api_key_id: 2, api_key_name: 'backup', concurrency: 1 },
    ],
  },
  { user_id: 20, email: 'twenty@example.com', username: '', concurrency: 2, api_keys: [{ api_key_id: 3, api_key_name: 'k', concurrency: 2 }] },
]

const mountModal = (props: { show: boolean; group: AdminGroup | null }) =>
  mount(GroupUserConcurrencyModal, {
    props,
    global: {
      stubs: {
        BaseDialog: { props: ['show'], template: '<div v-if="show"><slot /></div>' },
        Icon: true,
        PlatformIcon: true,
      },
    },
  })

beforeEach(() => {
  vi.clearAllMocks()
  vi.useFakeTimers()
  mocks.getUserConcurrency.mockResolvedValue(summary(1, sampleUsers))
})

describe('GroupUserConcurrencyModal', () => {
  it('does not request anything while closed', async () => {
    mountModal({ show: false, group: group(1) })
    await flushPromises()

    expect(mocks.getUserConcurrency).not.toHaveBeenCalled()
  })

  it('loads the group when opened and lists users with their per-key concurrency', async () => {
    const wrapper = mountModal({ show: false, group: group(1) })
    await wrapper.setProps({ show: true })
    await flushPromises()

    expect(mocks.getUserConcurrency).toHaveBeenCalledWith(1)
    expect(wrapper.get('[data-testid="user-concurrency-total"]').text()).toBe('6')
    const rows = wrapper.findAll('[data-testid="user-concurrency-row"]')
    expect(rows).toHaveLength(2)
    // 有昵称时主标题用昵称、邮箱放在下面；没有昵称时直接用邮箱
    expect(rows[0].text()).toContain('ten')
    expect(rows[0].text()).toContain('ten@example.com')
    expect(rows[0].text()).toContain('main')
    expect(rows[0].text()).toContain('×3')
    expect(rows[0].text()).toContain('×1')
    expect(rows[1].text()).toContain('twenty@example.com')
  })

  it('shows the empty state when nobody has requests in flight', async () => {
    mocks.getUserConcurrency.mockResolvedValue(summary(1, []))
    const wrapper = mountModal({ show: true, group: group(1) })
    await flushPromises()

    expect(wrapper.find('[data-testid="user-concurrency-empty"]').exists()).toBe(true)
    expect(wrapper.findAll('[data-testid="user-concurrency-row"]')).toHaveLength(0)
  })

  it('refreshes every 3 seconds while open and stops after closing', async () => {
    const wrapper = mountModal({ show: true, group: group(1) })
    await flushPromises()
    expect(mocks.getUserConcurrency).toHaveBeenCalledTimes(1)

    await vi.advanceTimersByTimeAsync(3000)
    expect(mocks.getUserConcurrency).toHaveBeenCalledTimes(2)
    await vi.advanceTimersByTimeAsync(3000)
    expect(mocks.getUserConcurrency).toHaveBeenCalledTimes(3)

    await wrapper.setProps({ show: false })
    await vi.advanceTimersByTimeAsync(9000)
    expect(mocks.getUserConcurrency).toHaveBeenCalledTimes(3)
  })

  it('does not stack requests while the previous one is still pending', async () => {
    let resolveFirst: (value: unknown) => void = () => {}
    mocks.getUserConcurrency.mockReturnValueOnce(new Promise((resolve) => { resolveFirst = resolve }))
    mountModal({ show: true, group: group(1) })
    await flushPromises()

    await vi.advanceTimersByTimeAsync(9000)
    expect(mocks.getUserConcurrency).toHaveBeenCalledTimes(1)

    resolveFirst(summary(1, sampleUsers))
    await flushPromises()
    await vi.advanceTimersByTimeAsync(3000)
    expect(mocks.getUserConcurrency).toHaveBeenCalledTimes(2)
  })

  it('switches to the new group and ignores a stale response from the old one', async () => {
    let resolveOld: (value: unknown) => void = () => {}
    mocks.getUserConcurrency.mockImplementation((id: number) =>
      id === 1 ? new Promise((resolve) => { resolveOld = resolve }) : Promise.resolve(summary(2, [sampleUsers[1]])),
    )
    const wrapper = mountModal({ show: true, group: group(1) })
    await flushPromises()

    await wrapper.setProps({ group: group(2) })
    await flushPromises()
    expect(mocks.getUserConcurrency).toHaveBeenLastCalledWith(2)
    expect(wrapper.findAll('[data-testid="user-concurrency-row"]')).toHaveLength(1)

    resolveOld(summary(1, sampleUsers))
    await flushPromises()
    expect(wrapper.findAll('[data-testid="user-concurrency-row"]')).toHaveLength(1)
    expect(wrapper.text()).not.toContain('ten@example.com')
  })

  it('reports a load failure once and keeps polling', async () => {
    mocks.getUserConcurrency.mockRejectedValueOnce(new Error('network'))
    mountModal({ show: true, group: group(1) })
    await flushPromises()

    expect(mocks.showError).toHaveBeenCalledWith('admin.groups.userConcurrencyLoadFailed')
    await vi.advanceTimersByTimeAsync(3000)
    expect(mocks.getUserConcurrency).toHaveBeenCalledTimes(2)
  })
})
