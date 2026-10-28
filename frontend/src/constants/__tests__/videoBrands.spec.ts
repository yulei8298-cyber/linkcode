import { describe, expect, it } from 'vitest'
import { getVideoProtocol, inferVideoVendor, listVideoProtocols, videoVendorStyle } from '../videoBrands'
import { BRAND_ICONS } from '@/components/icons/brands/brandIcons'

describe('video protocol catalog', () => {
  it('来自后端平台清单，覆盖主流视频厂商且都有可用品牌标志', () => {
    const protocols = listVideoProtocols().map(p => p.protocol)
    expect(protocols).toEqual(expect.arrayContaining(['ark', 'kling', 'minimax', 'dashscope', 'vidu', 'veo', 'runway', 'luma', 'pixverse', 'openai_videos', 'newapi']))
    for (const protocol of protocols) {
      const style = videoVendorStyle(protocol)
      expect(style.label, protocol).not.toBe('Video')
      expect(BRAND_ICONS[style.icon], protocol).toBeDefined()
    }
    expect(getVideoProtocol('kling')?.supports_secret_key).toBe(true)
    expect(getVideoProtocol('dashscope')?.example_models).toContain('wan3.0-video')
  })

  it('未知协议回落通用视频样式', () => {
    expect(videoVendorStyle('unknown').label).toBe('Video')
    expect(videoVendorStyle(undefined).icon).toBe('')
  })

  it('按模型名推断厂商', () => {
    expect(inferVideoVendor('doubao-seedance-2-0-260128')).toBe('seedance')
    expect(inferVideoVendor('MiniMax-Hailuo-2.3')).toBe('hailuo')
    expect(inferVideoVendor('wan3.0-video')).toBe('wan')
    expect(inferVideoVendor('veo-3.1-generate-preview')).toBe('veo')
    expect(inferVideoVendor('kling-v3')).toBe('kling')
    expect(inferVideoVendor('gpt-5.5')).toBe('')
  })
})
