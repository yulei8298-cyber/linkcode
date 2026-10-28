import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import PlatformTypeBadge from '../PlatformTypeBadge.vue'
import PlatformIcon from '../PlatformIcon.vue'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})

describe('PlatformTypeBadge video accounts', () => {
  it('视频账号显示上游厂商标志与名称', () => {
    const wrapper = mount(PlatformTypeBadge, { props: { platform: 'video', type: 'apikey', videoProtocol: 'kling' } })
    expect(wrapper.text()).toContain('可灵 Kling')
    expect(wrapper.find('[data-testid="video-vendor-icon"]').exists()).toBe(true)
    expect(wrapper.findComponent(PlatformIcon).exists()).toBe(false)
  })

  it('未知协议回落通用视频平台图标', () => {
    const wrapper = mount(PlatformTypeBadge, { props: { platform: 'video', type: 'apikey', videoProtocol: '' } })
    expect(wrapper.text()).toContain('Video')
    expect(wrapper.findComponent(PlatformIcon).exists()).toBe(true)
  })

  it('非视频平台不受影响', () => {
    const wrapper = mount(PlatformTypeBadge, { props: { platform: 'gemini', type: 'apikey', videoProtocol: 'kling' } })
    expect(wrapper.text()).toContain('Gemini')
    expect(wrapper.find('[data-testid="video-vendor-icon"]').exists()).toBe(false)
  })
})
