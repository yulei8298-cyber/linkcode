import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import PlazaVideoPricing from '../PlazaVideoPricing.vue'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return { ...actual, useI18n: () => ({ t: (key: string, values?: Record<string, unknown>) => (values ? `${key}(${JSON.stringify(values)})` : key) }) }
})

describe('PlazaVideoPricing', () => {
  it('按秒价按倍率换算并列出各分辨率', () => {
    const wrapper = mount(PlazaVideoPricing, {
      props: { rate: 2, spec: { unit: 'per_second', prices: { '1080p': { per_second: 0.17 }, '480p': { per_second: 0.0425 } }, min_duration_seconds: 2, max_duration_seconds: 30 } },
    })
    const items = wrapper.findAll('li').map(li => li.text())
    expect(items[0]).toContain('480p')
    expect(items[0]).toContain('$0.085')
    expect(items[1]).toContain('$0.34')
    expect(wrapper.text()).toContain('modelPlaza.video.rateNote')
  })

  it('价格至少保留两位小数', () => {
    const wrapper = mount(PlazaVideoPricing, { props: { rate: 1, spec: { unit: 'per_second', prices: { '720p': { per_second: 1.6 }, '1080p': { per_second: 0.416 }, '4k': { per_second: 2.2 } } } } })
    const text = wrapper.text()
    expect(text).toContain('$1.60')
    expect(text).toContain('$0.416')
    expect(text).toContain('$2.20')
  })

  it('按条价列出分辨率与时长档', () => {
    const wrapper = mount(PlazaVideoPricing, { props: { rate: 1, spec: { unit: 'per_video', prices: { '720p': { per_video: { '6': 0.28, '10': 0.56 } } } } } })
    expect(wrapper.findAll('li')).toHaveLength(2)
    expect(wrapper.text()).toContain('$0.28')
    expect(wrapper.text()).not.toContain('rateNote')
  })
})
