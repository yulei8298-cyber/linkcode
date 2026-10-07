import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})

const { getEnterpriseSettings, updateEnterpriseSettings, showError, showSuccess } = vi.hoisted(() => ({
  getEnterpriseSettings: vi.fn(),
  updateEnterpriseSettings: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn(),
}))
vi.mock('@/api/admin/packages', () => ({ default: { getEnterpriseSettings, updateEnterpriseSettings } }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError, showSuccess }) }))

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
    expect(updateEnterpriseSettings).toHaveBeenCalledWith({ enabled: true, threshold: 5000 })
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
})
