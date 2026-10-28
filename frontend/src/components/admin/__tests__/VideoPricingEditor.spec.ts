import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import VideoPricingEditor from '../VideoPricingEditor.vue'
import type { VideoPricingSpec } from '@/api/admin/modelBasePricing'

const lastEmit = (wrapper: ReturnType<typeof mount>) => {
  const events = wrapper.emitted('update:modelValue') as Array<[VideoPricingSpec | null]> | undefined
  return events?.[events.length - 1]?.[0]
}

describe('VideoPricingEditor', () => {
  it('按秒：只输出已填写的分辨率档并显示估价示例', async () => {
    const wrapper = mount(VideoPricingEditor, { props: { modelValue: null } })
    await wrapper.find('[data-testid="video-pricing-enabled"]').setValue(true)
    await wrapper.find('[data-testid="video-per-second-720p"]').setValue('0.1')
    expect(lastEmit(wrapper)).toMatchObject({ unit: 'per_second', prices: { '720p': { per_second: 0.1 } } })
    expect(Object.keys(lastEmit(wrapper)!.prices!)).toEqual(['720p'])
    expect(wrapper.find('[data-testid="video-pricing-preview"]').text()).toContain('$0.5000')
  })

  it('按 token 与关闭视频计费', async () => {
    const wrapper = mount(VideoPricingEditor, { props: { modelValue: { unit: 'per_second', prices: { '*': { per_second: 0.05 } } } } })
    await wrapper.find('[data-testid="video-unit-token"]').trigger('click')
    await wrapper.find('[data-testid="video-token-price"]').setValue('46')
    expect(lastEmit(wrapper)).toMatchObject({ unit: 'token', token_price_per_million: 46 })
    expect(lastEmit(wrapper)!.prices).toBeUndefined()
    await wrapper.find('[data-testid="video-pricing-enabled"]').setValue(false)
    expect(lastEmit(wrapper)).toBeNull()
  })

  it('载入内置参考价', async () => {
    const defaultSpec: VideoPricingSpec = { unit: 'per_video', prices: { '720p': { per_video: { '6': 0.28 } } }, default_duration_seconds: 6 }
    const wrapper = mount(VideoPricingEditor, { props: { modelValue: null, defaultSpec } })
    await wrapper.find('[data-testid="video-pricing-load-default"]').trigger('click')
    expect(lastEmit(wrapper)).toMatchObject({ unit: 'per_video', prices: { '720p': { per_video: { '6': 0.28 } } }, default_duration_seconds: 6 })
  })
})
