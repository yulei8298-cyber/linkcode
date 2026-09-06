import { beforeAll, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import ModelPlazaContent from '../ModelPlazaContent.vue'
import PlazaGroupSection from '../PlazaGroupSection.vue'
import type { ModelPlazaGroup, PlazaModel } from '@/api/modelPlaza'
import { plazaCardPricing } from '@/utils/modelPlazaPricing'

vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => ({ isAuthenticated: true }) }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ cachedPublicSettings: {} }) }))

beforeAll(() => {
  HTMLDialogElement.prototype.showModal = function () { this.setAttribute('open', '') }
  HTMLDialogElement.prototype.close = function () { this.removeAttribute('open'); this.dispatchEvent(new Event('close')) }
})

function model(overrides: Partial<PlazaModel> = {}): PlazaModel {
  return {
    name: 'gpt-test', platform: 'openai',
    pricing: { billing_mode: 'token', input_price: 2e-6, output_price: 8e-6, cache_write_price: null, cache_read_price: 0.5e-6, image_input_price: null, image_output_price: null, per_request_price: null, intervals: [] },
    official_pricing: { input_price: 2e-6, output_price: 8e-6, cache_read_price: 0.5e-6, cache_write_price: null },
    ...overrides
  }
}
function group(overrides: Partial<ModelPlazaGroup> = {}): ModelPlazaGroup {
  return { id: 1, name: '优选', description: '', platform: 'openai', subscription_type: 'standard', rate_multiplier: 0.5,
    peak_rate_enabled: false, peak_start: '', peak_end: '', peak_rate_multiplier: 1, is_exclusive: false,
    image_rate_independent: false, image_rate_multiplier: 1, long_context_pricing_enabled: true,
    models: [model()], ...overrides }
}
function mountContent(groups = [group(), group({ id: 2, name: '标准', rate_multiplier: 1 })]) {
  return mount(ModelPlazaContent, { props: { response: { description: '', groups }, loading: false }, global: { stubs: { Icon: true, PlatformIcon: true, PlazaGroupSection: true } } })
}

describe('model plaza price semantics', () => {
  it('multiplies the official base once, leaving the official reference unchanged', () => {
    const quote = plazaCardPricing(model(), group())
    expect(quote.metrics.map(m => m.price)).toEqual(['$1.00', '$4.00'])
    expect(quote.metrics[0]).toMatchObject({ base: '$2.00', official: '$2.00', isOfficial: true })
  })
  it('honors a free personal rate and distinguishes missing pricing from free pricing', () => {
    expect(plazaCardPricing(model(), group({ user_rate_multiplier: 0 })).metrics[0].price).toBe('$0.00')
    const unknown = plazaCardPricing(model({ pricing: null }), group())
    expect(unknown.metrics[0]).toMatchObject({ price: '-', available: false, isOfficial: false })
    const free = model(); free.pricing!.input_price = 0
    expect(plazaCardPricing(free, group()).metrics[0]).toMatchObject({ price: '$0.00', available: true })
  })
  it('keeps configured billing prices and never labels them as official', () => {
    const custom = model(); custom.pricing!.input_price = 4e-6
    expect(plazaCardPricing(custom, group()).metrics[0]).toMatchObject({ price: '$2.00', base: '$4.00', official: '$2.00', isOfficial: false })
  })
  it('uses the first context tier regardless of API ordering and retains tiny prices', () => {
    const tiered = model()
    const base = { min_tokens: 0, max_tokens: 200000, input_price: 1.25e-8, output_price: 8e-6, cache_read_price: null, cache_write_price: null, per_request_price: null }
    tiered.pricing!.intervals = [{ ...base, min_tokens: 200000, max_tokens: null, input_price: 4e-6 }, base]
    expect(plazaCardPricing(tiered, group())).toMatchObject({ tiered: true })
    expect(plazaCardPricing(tiered, group()).metrics[0].price).toBe('$0.00625')
    expect(tiered.pricing!.intervals[0].min_tokens).toBe(200000)
  })
  it('uses independent image rates without scaling per-image prices to tokens', () => {
    const image = model(); image.pricing!.billing_mode = 'image'; image.pricing!.per_request_price = 0.02
    const quote = plazaCardPricing(image, group({ user_rate_multiplier: 0.1, image_rate_independent: true, image_rate_multiplier: 2 }))
    expect(quote).toMatchObject({ rate: 2, token: false })
    expect(quote.metrics[0]).toMatchObject({ price: '$0.04', base: '$0.02', official: '-', isOfficial: false })
  })
  it('ignores unpriced per-request intervals when selecting the displayed base tier', () => {
    const image = model(); image.pricing!.billing_mode = 'image'
    const tier = { min_tokens: 0, max_tokens: null, input_price: null, output_price: null, cache_read_price: null, cache_write_price: null, per_request_price: null }
    image.pricing!.intervals = [tier, { ...tier, min_tokens: 1024, per_request_price: 0.02 }]
    expect(plazaCardPricing(image, group()).metrics[0].price).toBe('$0.01')
  })
})

describe('ModelPlazaContent cards', () => {
  it('selects the cheapest group, switches rates, and supports all-group browsing', async () => {
    const wrapper = mountContent()
    expect(wrapper.find('.plaza-price-value').text()).toBe('$1.00')
    await wrapper.findAll('.plaza-group-tabs button')[2].trigger('click')
    expect(wrapper.find('.plaza-price-value').text()).toBe('$2.00')
    await wrapper.findAll('.plaza-group-tabs button')[0].trigger('click')
    expect(wrapper.findAll('.plaza-model-card')).toHaveLength(2)
    wrapper.unmount()
  })
  it('searches providers inside composite groups and recovers from no results', async () => {
    const wrapper = mountContent([group({ platform: 'composite', models: [model(), model({ name: 'sonnet', platform: 'anthropic' })] })])
    await wrapper.find('input').setValue('Anthropic')
    expect(wrapper.findAll('.plaza-model-card')).toHaveLength(1)
    expect(wrapper.find('.plaza-model-identity').text()).toContain('sonnet')
    await wrapper.find('input').setValue('not-present')
    expect(wrapper.text()).toContain('modelPlaza.noSearchResult')
    await wrapper.find('input').setValue('')
    expect(wrapper.findAll('.plaza-model-card')).toHaveLength(2)
    wrapper.unmount()
  })
  it('resets unavailable group selections when authenticated data is replaced', async () => {
    const wrapper = mountContent([group({ id: 3, name: '专属', user_rate_multiplier: 0 })])
    expect(wrapper.find('.plaza-price-value').text()).toBe('$0.00')
    await wrapper.setProps({ response: { description: '', groups: [group({ id: 5 })] } })
    expect(wrapper.find('.plaza-price-value').text()).toBe('$1.00')
    expect(wrapper.find('[aria-pressed="true"]').text()).toContain('优选')
    wrapper.unmount()
  })
  it('passes the entire billing configuration to the price detail dialog', async () => {
    const m = model({ time_pricing: { timezone: 'Asia/Shanghai', periods: [{ start_time: '01:00', end_time: '08:00', multiplier: 0.5 }] } })
    const g = group({ models: [m], peak_rate_enabled: true, user_rate_multiplier: 0.8 })
    const wrapper = mountContent([g])
    expect(wrapper.text()).toContain('modelPlaza.cards.timePricing')
    await wrapper.find('.plaza-card-footer button').trigger('click')
    expect(wrapper.find('dialog').attributes()).toHaveProperty('open')
    expect(wrapper.findComponent(PlazaGroupSection).props('group')).toEqual(g)
    await wrapper.find('.plaza-dialog-heading button').trigger('click')
    expect(wrapper.find('dialog').attributes()).not.toHaveProperty('open')
    wrapper.unmount()
  })
  it('offers a retry for errors and sanitizes administrator price descriptions', async () => {
    const wrapper = mountContent()
    await wrapper.setProps({ error: true })
    await wrapper.find('.plaza-retry').trigger('click')
    expect(wrapper.emitted('retry')).toHaveLength(1)
    await wrapper.setProps({ error: false, response: { groups: [group()], description: '<img src=x onerror="alert(1)"><script>alert(1)</script>**notice**' } })
    expect(wrapper.find('.plaza-description').html()).not.toContain('onerror')
    expect(wrapper.find('.plaza-description').find('script').exists()).toBe(false)
    wrapper.unmount()
  })
})
