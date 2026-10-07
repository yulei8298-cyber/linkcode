import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return { ...actual, useI18n: () => ({ t: (key: string, params?: Record<string, unknown>) => (params ? `${key}:${JSON.stringify(params)}` : key) }) }
})

const { getEnterpriseSettings, updateEnterpriseSettings, showError, showSuccess } = vi.hoisted(() => ({
  getEnterpriseSettings: vi.fn(),
  updateEnterpriseSettings: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn(),
}))
vi.mock('@/api/admin/packages', () => ({ default: { getEnterpriseSettings, updateEnterpriseSettings } }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError, showSuccess }) }))
vi.mock('@/api/admin', () => ({
  adminAPI: { groups: { getAll: vi.fn().mockResolvedValue([
    { id: 7, name: 'Claude-稳定', rate_multiplier: 0.3, subscription_type: 'standard' },
    { id: 9, name: '订阅', rate_multiplier: 1, subscription_type: 'subscription' },
  ]) } },
}))

import EnterpriseTab from '../EnterpriseTab.vue'

describe('管理端企业尊享设置', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    getEnterpriseSettings.mockResolvedValue({ enabled: true, threshold: 3000 })
    updateEnterpriseSettings.mockImplementation(async (s) => s)
  })

  it('加载并显示当前门槛，保存时带上修改后的值', async () => {
    const wrapper = mount(EnterpriseTab, { global: { stubs: { Toggle: true } } })
    await flushPromises()
    const input = wrapper.get('[data-test="threshold"]')
    expect((input.element as HTMLInputElement).value).toBe('3000')

    await input.setValue('5000')
    await wrapper.get('[data-test="save"]').trigger('click')
    await flushPromises()
    expect(updateEnterpriseSettings).toHaveBeenCalledWith({ enabled: true, threshold: 5000, group_rates: [] })
    expect(showSuccess).toHaveBeenCalled()
  })

  it('门槛不大于 0 时不提交', async () => {
    const wrapper = mount(EnterpriseTab, { global: { stubs: { Toggle: true } } })
    await flushPromises()
    await wrapper.get('[data-test="threshold"]').setValue('0')
    await wrapper.get('[data-test="save"]').trigger('click')
    await flushPromises()
    expect(updateEnterpriseSettings).not.toHaveBeenCalled()
    expect(showError).toHaveBeenCalledWith('admin.packages.enterprise.invalidThreshold')
  })

  it('为分组添加企业倍率并保存；只能选普通分组', async () => {
    const wrapper = mount(EnterpriseTab, { global: { stubs: { Toggle: true } } })
    await flushPromises()
    await wrapper.get('[data-test="add-group-rate"]').trigger('click')
    const row = wrapper.get('[data-test="group-rate-row"]')
    const options = row.findAll('option').map((o) => o.text())
    expect(options).toContain('Claude-稳定')
    expect(options).not.toContain('订阅')

    await row.get('select').setValue(7)
    await row.get('[data-test="group-rate-input"]').setValue('0.28')
    expect(row.text()).toContain('"rate":0.3')
    await wrapper.get('[data-test="save"]').trigger('click')
    await flushPromises()
    expect(updateEnterpriseSettings).toHaveBeenCalledWith({ enabled: true, threshold: 3000, group_rates: [{ group_id: 7, multiplier: 0.28 }] })
  })

  it('没选分组或倍率不合法时不提交', async () => {
    const wrapper = mount(EnterpriseTab, { global: { stubs: { Toggle: true } } })
    await flushPromises()
    await wrapper.get('[data-test="add-group-rate"]').trigger('click')
    await wrapper.get('[data-test="save"]').trigger('click')
    expect(showError).toHaveBeenLastCalledWith('admin.packages.enterprise.invalidGroup')

    await wrapper.get('[data-test="group-rate-row"] select').setValue(7)
    await wrapper.get('[data-test="group-rate-input"]').setValue('0')
    await wrapper.get('[data-test="save"]').trigger('click')
    expect(showError).toHaveBeenLastCalledWith('admin.packages.enterprise.invalidRate')
    expect(updateEnterpriseSettings).not.toHaveBeenCalled()
  })
})

