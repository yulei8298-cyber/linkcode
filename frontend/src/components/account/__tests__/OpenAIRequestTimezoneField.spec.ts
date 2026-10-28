import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import OpenAIRequestTimezoneField from '../OpenAIRequestTimezoneField.vue'
import Select from '@/components/common/Select.vue'
const { getTimezones } = vi.hoisted(() => ({ getTimezones: vi.fn() }))
vi.mock('@/api/admin', () => ({ adminAPI: { accounts: { getOpenAIRequestTimezones: getTimezones } } }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
beforeEach(() => { getTimezones.mockResolvedValue({ default: 'America/Los_Angeles', timezones: ['America/Los_Angeles', 'America/New_York'] }) })
describe('account request timezone', () => {
  it('loads the catalog and preserves the saved timezone', async () => {
    const wrapper = mount(OpenAIRequestTimezoneField, { props: { modelValue: 'America/New_York' } })
    await flushPromises()
    expect(wrapper.emitted('update:modelValue')).toBeUndefined()
    wrapper.findComponent(Select).vm.$emit('update:modelValue', 'America/Los_Angeles')
    expect(wrapper.emitted('update:modelValue')).toEqual([['America/Los_Angeles']])
  })
  it('uses the server default for an unsupported saved timezone', async () => {
    const wrapper = mount(OpenAIRequestTimezoneField, { props: { modelValue: 'Unknown/Zone' } })
    await flushPromises()
    expect(wrapper.emitted('update:modelValue')).toEqual([['America/Los_Angeles']])
  })
  it('does not overwrite the saved timezone when the catalog fails', async () => {
    getTimezones.mockRejectedValue(new Error('offline'))
    const wrapper = mount(OpenAIRequestTimezoneField, { props: { modelValue: 'America/New_York' } })
    await flushPromises()
    expect(wrapper.text()).toContain('requestTimezoneLoadFailed')
    expect(wrapper.emitted('update:modelValue')).toBeUndefined()
  })
})
