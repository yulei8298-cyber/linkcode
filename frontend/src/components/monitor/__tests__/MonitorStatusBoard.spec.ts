import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

import type { UserMonitorView } from '@/api/channelMonitor'
import MonitorStatusBoard from '../MonitorStatusBoard.vue'

const { isQuotaVisible } = vi.hoisted(() => ({
  isQuotaVisible: vi.fn(() => false),
}))

vi.mock('@/utils/featureFlags', () => ({
  isChannelMonitorQuotaVisible: () => isQuotaVisible(),
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) => (params ? `${key}:${JSON.stringify(params)}` : key),
      te: () => true,
      locale: { value: 'zh' },
    }),
  }
})

function makeItem(overrides: Partial<UserMonitorView> = {}): UserMonitorView {
  return {
    id: 1,
    name: 'claude-main',
    provider: 'kimi',
    group_name: '',
    primary_model: 'quota',
    primary_status: 'operational',
    primary_latency_ms: null,
    primary_ping_latency_ms: null,
    availability_7d: 100,
    extra_models: [],
    timeline: [],
    ...overrides,
  }
}

function mountBoard(items: UserMonitorView[], showQuota = true) {
  return mount(MonitorStatusBoard, {
    props: { items, window: '7d', windowLabel: '7 天', detailCache: {}, showQuota },
    global: { stubs: { PlatformIcon: true } },
  })
}

const quotaSnapshot = {
  source: 'cn_quota',
  success: true,
  plan_level: 'kimi-plus',
  tiers: [{ window: 'daily', label: 'requests', used_percent: 60 }],
  fetched_at: '2026-08-18T00:00:00Z',
} as UserMonitorView['latest_quota']

describe('MonitorStatusBoard', () => {
  it('groups monitors by provider and summarises statuses', () => {
    const wrapper = mountBoard([
      makeItem({ id: 1, provider: 'openai', primary_model: 'gpt-5', primary_status: 'operational' }),
      makeItem({ id: 2, provider: 'anthropic', primary_model: 'claude-opus-5', primary_status: 'degraded' }),
      makeItem({ id: 3, provider: 'anthropic', primary_model: 'claude-opus-5', primary_status: 'failed' }),
    ])

    expect(wrapper.findAll('.st-group h2').map(h => h.text())).toEqual(['Claude', 'GPT'])
    expect(wrapper.findAll('.st-group')[0].get('h2 .st-dot').classes()).toContain('bad')
    expect(wrapper.get('.st-summary').text()).toContain('monitorBoard.total:{"n":3}')
  })

  it('renders 72 timeline cells with the oldest point on the left', () => {
    const wrapper = mountBoard([makeItem({
      primary_model: 'gpt-5',
      timeline: [
        { status: 'failed', latency_ms: 10, ping_latency_ms: 1, checked_at: '2026-08-10T00:01:00Z' },
        { status: 'degraded', latency_ms: 10, ping_latency_ms: 1, checked_at: '2026-08-10T00:00:00Z' },
      ],
    })])

    const cells = wrapper.findAll('.st-bar i')
    expect(cells).toHaveLength(72)
    expect(cells[70].classes()).toContain('degraded')
    expect(cells[71].classes()).toContain('bad')
  })

  it('emits detail with the clicked monitor', async () => {
    const item = makeItem({ primary_model: 'gpt-5' })
    const wrapper = mountBoard([item])
    await wrapper.get('.st-detail').trigger('click')
    expect(wrapper.emitted('detail')?.[0]).toEqual([item])
  })

  it('hides the quota block when the system switch is off even if data exists', () => {
    isQuotaVisible.mockReturnValue(false)
    const wrapper = mountBoard([makeItem({ latest_quota: quotaSnapshot })])
    expect(wrapper.find('[data-testid="monitor-quota-view"]').exists()).toBe(false)
  })

  it('renders the quota block when the switch is on and a snapshot exists', () => {
    isQuotaVisible.mockReturnValue(true)
    const wrapper = mountBoard([makeItem({ latest_quota: quotaSnapshot })])
    expect(wrapper.find('[data-testid="monitor-quota-view"]').exists()).toBe(true)
    expect(wrapper.text()).toContain('kimi-plus')
  })

  it('never renders the quota block on pages that do not opt in', () => {
    isQuotaVisible.mockReturnValue(true)
    const wrapper = mountBoard([makeItem({ latest_quota: quotaSnapshot })], false)
    expect(wrapper.find('[data-testid="monitor-quota-view"]').exists()).toBe(false)
  })

  // 占位符 "quota" 是主模型存储值，不得作为假模型名直接透出到用户端。
  it('shows the localized quota label instead of the raw placeholder model', () => {
    const wrapper = mountBoard([makeItem()])
    expect(wrapper.get('.st-name span').text()).toBe('monitorCommon.checkMode.quota')
  })

  it('keeps the real model name for probe monitors', () => {
    const wrapper = mountBoard([makeItem({ primary_model: 'claude-sonnet-4-5' })])
    expect(wrapper.get('.st-name span').text()).toBe('claude-sonnet-4-5')
  })
})
