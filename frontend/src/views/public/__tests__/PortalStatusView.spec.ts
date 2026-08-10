import { ref } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

import PortalStatusView from '../PortalStatusView.vue'

const { getPublicMonitors } = vi.hoisted(() => ({
  getPublicMonitors: vi.fn()
}))

vi.mock('@/api/public', () => ({
  getPublicMonitors,
  getPublicMonitorStatus: vi.fn()
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showError: vi.fn() })
}))

vi.mock('@/composables/useAutoRefresh', () => ({
  useAutoRefresh: () => ({
    countdown: ref(30),
    setEnabled: vi.fn()
  })
}))

const monitor = (id: number, status: 'operational' | 'degraded' | 'failed') => ({
  id,
  name: `Monitor ${id}`,
  provider: 'openai' as const,
  group_name: 'default',
  primary_model: 'gpt-5.6-sol',
  primary_status: status,
  primary_latency_ms: 100,
  primary_ping_latency_ms: 20,
  availability_7d: 99,
  extra_models: [],
  timeline: [
    {
      status,
      latency_ms: 100,
      ping_latency_ms: 20,
      checked_at: '2026-08-10T00:00:00Z'
    }
  ]
})

describe('PortalStatusView', () => {
  it('keeps degraded status amber and reserves red for failures', async () => {
    getPublicMonitors.mockResolvedValue({
      items: [monitor(1, 'operational'), monitor(2, 'degraded'), monitor(3, 'failed')]
    })

    const wrapper = mount(PortalStatusView, {
      global: {
        stubs: {
          PortalLayout: { template: '<div><slot /></div>' },
          PortalMonitorDetailDialog: true,
          RouterLink: true,
          Icon: true
        }
      }
    })

    await flushPromises()

    const cards = wrapper.findAll('.lc-monitor-card')
    expect(cards).toHaveLength(3)
    expect(cards[0].find('.lc-status').text()).toBe('正常')
    expect(cards[0].find('.lc-status').classes()).not.toContain('bad')
    expect(cards[1].find('.lc-status').text()).toBe('降级')
    expect(cards[1].find('.lc-status').classes()).toContain('degraded')
    expect(cards[1].find('.lc-status').classes()).not.toContain('bad')
    expect(cards[2].find('.lc-status').text()).toBe('异常')
    expect(cards[2].find('.lc-status').classes()).toContain('bad')

    expect(cards[1].findAll('.lc-timeline i').at(-1)?.classes()).toContain('degraded')
    expect(cards[2].findAll('.lc-timeline i').at(-1)?.classes()).toContain('bad')

    wrapper.unmount()
  })

  it('shows the same latest 60 checks as the console from oldest to newest', async () => {
    const item = monitor(1, 'operational')
    item.timeline = Array.from({ length: 65 }, (_, index) => ({
      status: index === 0 ? 'failed' as const : index === 59 ? 'degraded' as const : 'operational' as const,
      latency_ms: 100 + index,
      ping_latency_ms: 20,
      checked_at: new Date(Date.UTC(2026, 7, 10, 1, 0, 64 - index)).toISOString()
    }))
    getPublicMonitors.mockResolvedValue({ items: [item] })

    const wrapper = mount(PortalStatusView, {
      global: {
        stubs: {
          PortalLayout: { template: '<div><slot /></div>' },
          PortalMonitorDetailDialog: true,
          RouterLink: true,
          Icon: true
        }
      }
    })

    await flushPromises()

    const points = wrapper.findAll('.lc-timeline i')
    expect(points).toHaveLength(60)
    expect(points[0].classes()).toContain('degraded')
    expect(points[59].classes()).toContain('bad')
    expect(points[0].attributes('title')).toContain('降级')
    expect(points[59].attributes('title')).toContain('异常')

    wrapper.unmount()
  })
})
