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
})
