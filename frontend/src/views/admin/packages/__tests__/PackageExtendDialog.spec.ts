import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return { ...actual, useI18n: () => ({ t: (key: string, params?: Record<string, unknown>) => (params ? `${key}:${JSON.stringify(params)}` : key) }) }
})
// 原样输出时间串，断言顺延结果时不受本机时区影响。
vi.mock('@/utils/format', () => ({ formatDateTimeToMinute: (v: string) => v }))

import PackageExtendDialog from '../PackageExtendDialog.vue'

const BaseDialogStub = {
  name: 'BaseDialog',
  props: ['show', 'title', 'width'],
  emits: ['close'],
  template: '<div v-if="show"><slot /><slot name="footer" /></div>',
}

const item = (over: Record<string, unknown> = {}) => ({
  id: 7, user_id: 9, group_id: 2, plan_id: 1, name: '摸鱼周卡', cycle: 'week', tier: 1,
  quota_usd: 10, used_usd: 2.5, starts_at: '2026-10-06T00:00:00Z', expires_at: '2026-10-13T00:00:00.000Z', status: 'active',
  frozen_seconds_total: 0, created_at: '2026-10-06T00:00:00Z', updated_at: '2026-10-06T00:00:00Z',
  user_email: 'a@example.com', username: 'alice', group_name: 'GPT-Pro', paid_amount: 1, remaining_usd: 7.5, frozen_seconds: 0, max_freeze_days: 7,
  ...over,
})

function mountDialog(props: Record<string, unknown>) {
  return mount(PackageExtendDialog, { props: { busy: false, ...props }, global: { stubs: { BaseDialog: BaseDialogStub } } })
}

describe('延长套餐弹窗', () => {
  it('没有选中套餐时不显示', () => {
    const wrapper = mountDialog({ item: null })
    expect(wrapper.find('form').exists()).toBe(false)
  })

  it('默认延长 7 天，并预览延长后的到期时间', () => {
    const wrapper = mountDialog({ item: item() })
    expect((wrapper.get('[data-test="extend-days"]').element as HTMLInputElement).value).toBe('7')
    expect(wrapper.get('[data-test="extend-preview"]').text()).toContain('2026-10-20T00:00:00.000Z')
  })

  it('快捷天数切换后提交带上选中的天数', async () => {
    const wrapper = mountDialog({ item: item() })
    const presets = wrapper.findAll('[data-test="extend-preset"]')
    expect(presets).toHaveLength(5)
    await presets[4].trigger('click')
    expect(wrapper.get('[data-test="extend-preview"]').text()).toContain('2026-11-12T00:00:00.000Z')
    await wrapper.get('form').trigger('submit')
    expect(wrapper.emitted('confirm')?.[0]).toEqual([expect.objectContaining({ id: 7 }), 30])
  })

  it.each([[0], [-1], [366], [1.5]])('非法天数 %s 禁用确认且不提交', async (days) => {
    const wrapper = mountDialog({ item: item() })
    await wrapper.get('[data-test="extend-days"]').setValue(days)
    expect(wrapper.get('[data-test="extend-confirm"]').attributes('disabled')).toBeDefined()
    expect(wrapper.find('[data-test="extend-preview"]').exists()).toBe(false)
    await wrapper.get('form').trigger('submit')
    expect(wrapper.emitted('confirm')).toBeUndefined()
  })

  it('处理中不重复提交', async () => {
    const wrapper = mountDialog({ item: item(), busy: true })
    await wrapper.get('form').trigger('submit')
    expect(wrapper.emitted('confirm')).toBeUndefined()
  })

  it('冻结中的套餐提示解冻时还会再顺延', () => {
    const wrapper = mountDialog({ item: item({ status: 'frozen' }) })
    expect(wrapper.text()).toContain('admin.packages.userPackages.extend.frozenNote')
    expect(mountDialog({ item: item() }).text()).not.toContain('frozenNote')
  })

  it('换一张套餐时天数回到默认值', async () => {
    const wrapper = mountDialog({ item: item() })
    await wrapper.get('[data-test="extend-days"]').setValue(30)
    await wrapper.setProps({ item: item({ id: 8 }) })
    expect((wrapper.get('[data-test="extend-days"]').element as HTMLInputElement).value).toBe('7')
  })
})
