import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import VideoAccountFields from '../VideoAccountFields.vue'
import { getVideoProtocol } from '@/constants/videoBrands'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})

const ark = getVideoProtocol('ark')!
const kling = getVideoProtocol('kling')!

describe('VideoAccountFields', () => {
  it('切换协议时，官方地址自动换成新协议的官方地址', async () => {
    const wrapper = mount(VideoAccountFields, { props: { protocol: 'ark', secretKey: '', baseUrl: ark.default_base_url } })
    await wrapper.find('[data-testid="video-protocol-kling"]').trigger('click')
    expect(wrapper.emitted('update:protocol')?.[0]).toEqual(['kling'])
    expect(wrapper.emitted('update:baseUrl')?.[0]).toEqual([kling.default_base_url])
  })

  it('第三方中转地址在切换协议时保持不变', async () => {
    const wrapper = mount(VideoAccountFields, { props: { protocol: 'ark', secretKey: '', baseUrl: 'https://relay.example.com' } })
    await wrapper.find('[data-testid="video-protocol-dashscope"]').trigger('click')
    expect(wrapper.emitted('update:baseUrl')).toBeUndefined()
  })

  it('只有可灵显示 Secret Key，并可填入示例模型', async () => {
    const wrapper = mount(VideoAccountFields, { props: { protocol: 'ark', secretKey: '', baseUrl: '' } })
    expect(wrapper.find('[data-testid="video-secret-key"]').exists()).toBe(false)
    await wrapper.setProps({ protocol: 'kling' })
    const input = wrapper.find('[data-testid="video-secret-key"]')
    expect(input.exists()).toBe(true)
    await input.setValue('sk-secret')
    expect(wrapper.emitted('update:secretKey')?.[0]).toEqual(['sk-secret'])
    await wrapper.find('[data-testid="video-apply-models"]').trigger('click')
    expect(wrapper.emitted('apply-models')?.[0]).toEqual([kling.example_models])
  })
})
