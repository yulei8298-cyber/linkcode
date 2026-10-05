import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import type { UserPackageView } from '@/api/packages'
import PackageItemCard from '../mine/PackageItemCard.vue'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})

function view(patch: Partial<UserPackageView> = {}): UserPackageView {
  return {
    id: 1, user_id: 1, group_id: 7, plan_id: 2, name: '爆肝周卡', cycle: 'week', tier: 2,
    quota_usd: 240, used_usd: 187.6, starts_at: '2026-09-30T13:00:00Z', expires_at: '2026-10-07T13:00:00Z',
    status: 'active', frozen_seconds_total: 0, created_at: '', updated_at: '', group_name: 'Claude',
    remaining_usd: 52.4, frozen_seconds: 0, freeze_left_seconds: 7 * 86400, max_freeze_days: 7, deduct_order: 1, ...patch,
  }
}

function render(item: UserPackageView, todayFreezable: boolean, freezeEnabled = true) {
  return mount(PackageItemCard, {
    props: { item, freezeEnabled, todayFreezable, nextFreezableDate: '2026-10-10', busy: false },
    global: { stubs: { Icon: true } },
  })
}

describe('PackageItemCard', () => {
  it('可冻结日：冻结按钮可用并带出套餐', async () => {
    const wrapper = render(view(), true)
    const btn = wrapper.get('[data-test="package-freeze"]')
    expect(btn.attributes('disabled')).toBeUndefined()
    await btn.trigger('click')
    expect(wrapper.emitted('freeze')?.[0]?.[0]).toMatchObject({ id: 1 })
    expect(wrapper.text()).toContain('packages.mine.status.active')
  })

  it('不可冻结日：按钮禁用并提示下一个可冻结日', () => {
    const wrapper = render(view({ deduct_order: 2 }), false)
    expect(wrapper.get('[data-test="package-freeze"]').attributes('disabled')).toBeDefined()
    expect(wrapper.text()).toContain('packages.mine.onlyHoliday')
    expect(wrapper.text()).toContain('packages.mine.status.queued')
  })

  it('冻结额度用完：即使是可冻结日也不能再冻结', () => {
    const wrapper = render(view({ freeze_left_seconds: 0, frozen_seconds: 7 * 86400 }), true)
    expect(wrapper.get('[data-test="package-freeze"]').attributes('disabled')).toBeDefined()
    expect(wrapper.text()).toContain('packages.mine.capUsed')
  })

  it('冻结中：只显示解冻（任何日子都能解冻）', async () => {
    const wrapper = render(view({ status: 'frozen', deduct_order: 0, frozen_at: '2026-10-03T00:12:00Z', frozen_seconds: 100800 }), false)
    expect(wrapper.find('[data-test="package-freeze"]').exists()).toBe(false)
    await wrapper.get('[data-test="package-unfreeze"]').trigger('click')
    expect(wrapper.emitted('unfreeze')).toHaveLength(1)
    expect(wrapper.text()).toContain('packages.mine.status.frozen')
  })

  it('冻结功能关闭：不显示冻结按钮', () => {
    const wrapper = render(view(), true, false)
    expect(wrapper.find('[data-test="package-freeze"]').exists()).toBe(false)
  })
})
