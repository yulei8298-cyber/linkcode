import { beforeAll, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import ModelPlazaContent from '../ModelPlazaContent.vue'
import type { ModelPlazaGroup, PlazaModel } from '@/api/modelPlaza'
import { plazaHasExclusiveRate } from '@/utils/modelPlazaPricing'

vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => ({ isAuthenticated: true }) }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ cachedPublicSettings: {} }) }))

// jsdom 没有实现 <dialog> 的 showModal / close
beforeAll(() => {
  HTMLDialogElement.prototype.showModal = function () { this.setAttribute('open', '') }
  HTMLDialogElement.prototype.close = function () { this.removeAttribute('open'); this.dispatchEvent(new Event('close')) }
})

function model(name: string): PlazaModel {
  return {
    name, platform: 'openai',
    pricing: { billing_mode: 'token', input_price: 2e-6, output_price: 8e-6, cache_write_price: null, cache_read_price: null, image_input_price: null, image_output_price: null, per_request_price: null, intervals: [] },
    official_pricing: { input_price: 2e-6, output_price: 8e-6, cache_read_price: null, cache_write_price: null },
  }
}
function group(overrides: Partial<ModelPlazaGroup>): ModelPlazaGroup {
  return { id: 1, name: '普通', description: '', platform: 'openai', subscription_type: 'standard', rate_multiplier: 0.5,
    peak_rate_enabled: false, peak_start: '', peak_end: '', peak_rate_multiplier: 1, is_exclusive: false,
    image_rate_independent: false, image_rate_multiplier: 1, video_rate_independent: false, video_rate_multiplier: 1,
    long_context_pricing_enabled: true, models: [model('gpt-normal')], ...overrides } as ModelPlazaGroup
}
const plain = group({ id: 1 })
const enterprise = group({ id: 3, name: '企业', rate_multiplier: 0.3, user_rate_multiplier: 0.28, enterprise_rate: true, models: [model('gpt-ent')] })

function mountContent(groups: ModelPlazaGroup[]) {
  return mount(ModelPlazaContent, { props: { response: { description: '', groups }, loading: false }, global: { stubs: { Icon: true, PlatformIcon: true, PlazaGroupSection: true } } })
}

describe('模型广场专属价格', () => {
  it('只有比分组默认倍率更低的专属 / 企业倍率才算专属价格', () => {
    expect(plazaHasExclusiveRate(enterprise)).toBe(true)
    expect(plazaHasExclusiveRate(plain)).toBe(false)
    expect(plazaHasExclusiveRate(group({ user_rate_multiplier: 0.8 }))).toBe(false)
    expect(plazaHasExclusiveRate(group({ user_rate_multiplier: 0.5 }))).toBe(false)
  })

  it('企业倍率分组的卡片显示「企业专属价」，并说明套餐仍按原倍率', () => {
    const wrapper = mountContent([plain, enterprise])
    const tag = wrapper.get('[data-test="exclusive-price"]')
    expect(tag.text()).toBe('modelPlaza.cards.enterprisePrice')
    expect(tag.classes()).toContain('is-enterprise')
    expect(wrapper.text()).toContain('modelPlaza.cards.enterpriseRate')
  })

  it('个人专属倍率显示「专属价格」', () => {
    const wrapper = mountContent([group({ id: 5, user_rate_multiplier: 0.4 })])
    const tag = wrapper.get('[data-test="exclusive-price"]')
    expect(tag.text()).toBe('modelPlaza.cards.exclusivePrice')
    expect(tag.classes()).not.toContain('is-enterprise')
  })

  it('「仅看专属价格」只保留专属价格的模型；没有专属价格时不显示开关', async () => {
    const wrapper = mountContent([plain, enterprise])
    const allTab = wrapper.findAll('.plaza-group-tabs button').find((b) => b.text().includes('modelPlaza.cards.allGroups'))!
    await allTab.trigger('click')
    expect(wrapper.text()).toContain('gpt-normal')

    const toggle = wrapper.get('[data-test="only-exclusive"]')
    expect(toggle.text()).toContain('1')
    await toggle.get('input').setValue(true)
    expect(wrapper.text()).toContain('gpt-ent')
    expect(wrapper.text()).not.toContain('gpt-normal')

    expect(mountContent([plain]).find('[data-test="only-exclusive"]').exists()).toBe(false)
  })
})
