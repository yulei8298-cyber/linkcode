import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import type { PackagePlan } from '@/api/packages'
import PackageNoticeDialog from '../shop/PackageNoticeDialog.vue'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})

const plan: PackagePlan = {
  id: 2, group_id: 7, name: '爆肝周卡', cycle: 'week', tier: 2, price: 190, quota_usd: 240, validity_days: 7, for_sale: true,
}

// jsdom 不做布局：用原型上的 scrollHeight / clientHeight 模拟「内容比可视区长」
const originals = {
  scrollHeight: Object.getOwnPropertyDescriptor(HTMLElement.prototype, 'scrollHeight'),
  clientHeight: Object.getOwnPropertyDescriptor(HTMLElement.prototype, 'clientHeight'),
}

beforeEach(() => {
  Object.defineProperty(HTMLElement.prototype, 'scrollHeight', { configurable: true, get: () => 1000 })
  Object.defineProperty(HTMLElement.prototype, 'clientHeight', { configurable: true, get: () => 300 })
})

afterEach(() => {
  for (const [key, desc] of Object.entries(originals)) {
    if (desc) Object.defineProperty(HTMLElement.prototype, key, desc)
    else delete (HTMLElement.prototype as unknown as Record<string, unknown>)[key]
  }
  document.body.innerHTML = ''
})

async function mountOpen(purchase: { plan: PackagePlan; groupName: string } | null) {
  const wrapper = mount(PackageNoticeDialog, {
    props: { show: false, text: '立即生效：付款后生效\n退款政策：不退款', concurrency: 5, maxFreezeDays: 7, purchase },
    attachTo: document.body,
  })
  await wrapper.setProps({ show: true })
  await flushPromises()
  return wrapper
}

function q<T extends Element>(selector: string): T {
  const el = document.querySelector<T>(selector)
  if (!el) throw new Error(`missing ${selector}`)
  return el
}

describe('PackageNoticeDialog', () => {
  it('购买模式：未读到底不能勾选，勾选前不能支付，确认后带出套餐', async () => {
    const wrapper = await mountOpen({ plan, groupName: 'Claude' })
    const agree = q<HTMLInputElement>('[data-test="package-notice-agree"]')
    const confirm = q<HTMLButtonElement>('[data-test="package-notice-confirm"]')
    expect(agree.disabled).toBe(true)
    expect(confirm.disabled).toBe(true)

    const body = q<HTMLElement>('[data-test="package-notice-body"]')
    body.scrollTop = 700
    body.dispatchEvent(new Event('scroll'))
    await flushPromises()
    expect(agree.disabled).toBe(false)
    expect(confirm.disabled).toBe(true)

    agree.checked = true
    agree.dispatchEvent(new Event('change'))
    await flushPromises()
    expect(confirm.disabled).toBe(false)

    confirm.click()
    expect(wrapper.emitted('confirm')?.[0]).toEqual([plan])
    wrapper.unmount()
  })

  it('重新打开时重置已读与勾选状态', async () => {
    const wrapper = await mountOpen({ plan, groupName: 'Claude' })
    const body = q<HTMLElement>('[data-test="package-notice-body"]')
    body.scrollTop = 700
    body.dispatchEvent(new Event('scroll'))
    await flushPromises()

    await wrapper.setProps({ show: false })
    await wrapper.setProps({ show: true })
    await flushPromises()
    expect(q<HTMLInputElement>('[data-test="package-notice-agree"]').disabled).toBe(true)
    wrapper.unmount()
  })

  it('查看模式没有勾选与支付按钮', async () => {
    const wrapper = await mountOpen(null)
    expect(document.querySelector('[data-test="package-notice-agree"]')).toBeNull()
    expect(document.querySelector('[data-test="package-notice-confirm"]')).toBeNull()
    expect(document.body.textContent).toContain('退款政策')
    wrapper.unmount()
  })
})
