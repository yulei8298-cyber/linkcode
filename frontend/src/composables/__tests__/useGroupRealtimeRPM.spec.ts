import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { defineComponent, h } from 'vue'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import { GROUP_REALTIME_RPM_REFRESH_MS, useGroupRealtimeRPM } from '../useGroupRealtimeRPM'

const mocks = vi.hoisted(() => ({ getRealtimeRPM: vi.fn() }))
vi.mock('@/api/admin', () => ({ adminAPI: { groups: { getRealtimeRPM: mocks.getRealtimeRPM } } }))

enableAutoUnmount(afterEach)

const setHidden = (hidden: boolean) =>
  Object.defineProperty(document, 'hidden', { configurable: true, get: () => hidden })

let api: ReturnType<typeof useGroupRealtimeRPM>

const mountHost = (enabled?: () => boolean) =>
  mount(
    defineComponent({
      setup() {
        api = useGroupRealtimeRPM({ enabled })
        return () => h('div')
      },
    }),
  )

beforeEach(() => {
  vi.clearAllMocks()
  vi.useFakeTimers()
  setHidden(false)
  mocks.getRealtimeRPM.mockResolvedValue({ window_seconds: 60, total: 42, items: [{ group_id: 2, rpm: 30 }, { group_id: 38, rpm: 12 }] })
})

afterEach(() => {
  vi.useRealTimers()
  setHidden(false)
})

describe('useGroupRealtimeRPM', () => {
  it('loads on mount and exposes per-group RPM, defaulting missing groups to 0', async () => {
    mountHost()
    expect(api.loaded.value).toBe(false)
    await flushPromises()

    expect(api.loaded.value).toBe(true)
    expect(api.rpmOf(2)).toBe(30)
    expect(api.rpmOf(38)).toBe(12)
    expect(api.rpmOf(999)).toBe(0)
    expect(api.totalRpm.value).toBe(42)
  })

  it('refreshes on the fixed interval', async () => {
    mountHost()
    await flushPromises()
    expect(mocks.getRealtimeRPM).toHaveBeenCalledTimes(1)

    await vi.advanceTimersByTimeAsync(GROUP_REALTIME_RPM_REFRESH_MS)
    expect(mocks.getRealtimeRPM).toHaveBeenCalledTimes(2)

    mocks.getRealtimeRPM.mockResolvedValue({ window_seconds: 60, total: 5, items: [{ group_id: 2, rpm: 5 }] })
    await vi.advanceTimersByTimeAsync(GROUP_REALTIME_RPM_REFRESH_MS)
    expect(api.rpmOf(2)).toBe(5)
    expect(api.rpmOf(38)).toBe(0)
  })

  it('does not stack requests while one is still pending', async () => {
    let resolveFirst: (value: unknown) => void = () => {}
    mocks.getRealtimeRPM.mockReturnValueOnce(new Promise((resolve) => { resolveFirst = resolve }))
    mountHost()

    await vi.advanceTimersByTimeAsync(GROUP_REALTIME_RPM_REFRESH_MS * 3)
    expect(mocks.getRealtimeRPM).toHaveBeenCalledTimes(1)

    resolveFirst({ window_seconds: 60, total: 0, items: [] })
    await flushPromises()
    await vi.advanceTimersByTimeAsync(GROUP_REALTIME_RPM_REFRESH_MS)
    expect(mocks.getRealtimeRPM).toHaveBeenCalledTimes(2)
  })

  it('skips requests while disabled', async () => {
    let enabled = false
    mountHost(() => enabled)
    await flushPromises()
    await vi.advanceTimersByTimeAsync(GROUP_REALTIME_RPM_REFRESH_MS * 2)
    expect(mocks.getRealtimeRPM).not.toHaveBeenCalled()

    enabled = true
    await vi.advanceTimersByTimeAsync(GROUP_REALTIME_RPM_REFRESH_MS)
    expect(mocks.getRealtimeRPM).toHaveBeenCalledTimes(1)
  })

  it('pauses while the tab is hidden and refreshes immediately when it comes back', async () => {
    mountHost()
    await flushPromises()
    expect(mocks.getRealtimeRPM).toHaveBeenCalledTimes(1)

    setHidden(true)
    await vi.advanceTimersByTimeAsync(GROUP_REALTIME_RPM_REFRESH_MS * 3)
    expect(mocks.getRealtimeRPM).toHaveBeenCalledTimes(1)

    setHidden(false)
    document.dispatchEvent(new Event('visibilitychange'))
    await flushPromises()
    expect(mocks.getRealtimeRPM).toHaveBeenCalledTimes(2)
  })

  it('keeps the last good numbers when a refresh fails', async () => {
    mountHost()
    await flushPromises()

    mocks.getRealtimeRPM.mockRejectedValue(new Error('down'))
    await vi.advanceTimersByTimeAsync(GROUP_REALTIME_RPM_REFRESH_MS)
    expect(api.loaded.value).toBe(true)
    expect(api.rpmOf(2)).toBe(30)
  })
})
