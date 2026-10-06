import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return { ...actual, useI18n: () => ({ t: (key: string, params?: Record<string, unknown>) => (params ? `${key}:${JSON.stringify(params)}` : key) }) }
})

const { listUserPackages, getUserPackageStats, unfreezeUserPackage, voidUserPackage } = vi.hoisted(() => ({
  listUserPackages: vi.fn(),
  getUserPackageStats: vi.fn(),
  unfreezeUserPackage: vi.fn(),
  voidUserPackage: vi.fn(),
}))
vi.mock('@/api/admin/packages', () => ({
  default: { listUserPackages, getUserPackageStats, unfreezeUserPackage, voidUserPackage },
}))
vi.mock('@/api/admin', () => ({
  adminAPI: { groups: { getAll: vi.fn().mockResolvedValue([{ id: 2, name: 'GPT-Pro', subscription_type: 'standard', is_free: false }]) } },
}))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError: vi.fn(), showSuccess: vi.fn() }) }))

import UserPackagesTab from '../UserPackagesTab.vue'

const item = (over: Record<string, unknown> = {}) => ({
  id: 7, user_id: 9, group_id: 2, plan_id: 1, order_id: 55, name: '摸鱼周卡', cycle: 'week', tier: 1,
  quota_usd: 10, used_usd: 2.5, starts_at: '2026-10-06T00:00:00Z', expires_at: '2026-10-13T00:00:00Z', status: 'active',
  frozen_seconds_total: 0, created_at: '2026-10-06T00:00:00Z', updated_at: '2026-10-06T00:00:00Z',
  user_email: 'a@example.com', username: 'alice', group_name: 'GPT-Pro', paid_amount: 1, remaining_usd: 7.5, frozen_seconds: 86400, max_freeze_days: 7,
  ...over,
})

const stats = {
  total: 10, active: 4, frozen: 2, exhausted: 1, expired: 2, voided: 1,
  live_quota_usd: 300, live_used_usd: 120, recent_sold: 6, recent_revenue: 18, window_days: 30,
}

const stubs = { ConfirmDialog: true, Pagination: true }

async function mountTab() {
  const wrapper = mount(UserPackagesTab, { global: { stubs } })
  await flushPromises()
  return wrapper
}

describe('管理端用户套餐总览', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    listUserPackages.mockResolvedValue({ items: [item(), item({ id: 8, status: 'frozen', order_id: undefined, user_email: '' })], total: 2, page: 1, page_size: 20, pages: 1 })
    getUserPackageStats.mockResolvedValue(stats)
    unfreezeUserPackage.mockResolvedValue({})
    voidUserPackage.mockResolvedValue({})
  })

  it('打开即加载列表与汇总，不需要先指定用户', async () => {
    const wrapper = await mountTab()
    expect(listUserPackages).toHaveBeenCalledWith({ keyword: undefined, status: undefined, cycle: undefined, group_id: undefined, page: 1, page_size: 20 })
    expect(wrapper.findAll('[data-test="package-row"]')).toHaveLength(2)
    expect(wrapper.get('[data-test="package-stats"]').text()).toContain('"frozen":2')
    expect(wrapper.text()).toContain('a@example.com')
    expect(wrapper.text()).toContain('GPT-Pro')
  })

  it('手工发放的套餐没有订单，实付显示「手工发放」；冻结按钮只出现在冻结中的套餐', async () => {
    const wrapper = await mountTab()
    const rows = wrapper.findAll('[data-test="package-row"]')
    expect(rows[0].text()).toContain('¥1.00')
    expect(rows[1].text()).toContain('admin.packages.userPackages.manual')
    expect(rows[0].text()).not.toContain('admin.packages.userPackages.unfreeze')
    expect(rows[1].text()).toContain('admin.packages.userPackages.unfreeze')
  })

  it('状态筛选回到第一页并带上条件', async () => {
    const wrapper = await mountTab()
    listUserPackages.mockClear()
    await wrapper.get('[data-test="status"]').setValue('frozen')
    await flushPromises()
    expect(listUserPackages).toHaveBeenLastCalledWith(expect.objectContaining({ status: 'frozen', page: 1 }))
  })

  it('关键词输入防抖：连续输入只请求一次', async () => {
    vi.useFakeTimers()
    try {
      const wrapper = await mountTab()
      listUserPackages.mockClear()
      const input = wrapper.get('[data-test="keyword"]')
      await input.setValue('al')
      await input.setValue('alice')
      expect(listUserPackages).not.toHaveBeenCalled()
      vi.advanceTimersByTime(350)
      await flushPromises()
      expect(listUserPackages).toHaveBeenCalledTimes(1)
      expect(listUserPackages).toHaveBeenCalledWith(expect.objectContaining({ keyword: 'alice' }))
    } finally {
      vi.useRealTimers()
    }
  })

  it('解冻后重新加载列表和汇总', async () => {
    const wrapper = await mountTab()
    listUserPackages.mockClear()
    getUserPackageStats.mockClear()
    const unfreezeBtn = wrapper.findAll('[data-test="package-row"]')[1].findAll('button').find((b) => b.text().includes('unfreeze'))
    await unfreezeBtn!.trigger('click')
    await flushPromises()
    expect(unfreezeUserPackage).toHaveBeenCalledWith(8)
    expect(listUserPackages).toHaveBeenCalledTimes(1)
    expect(getUserPackageStats).toHaveBeenCalledTimes(1)
  })
})
