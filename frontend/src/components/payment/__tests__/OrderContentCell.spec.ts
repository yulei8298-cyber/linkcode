import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return { ...actual, useI18n: () => ({ t: (key: string, params?: Record<string, unknown>) => (params ? `${key}:${JSON.stringify(params)}` : key) }) }
})

import OrderContentCell from '../OrderContentCell.vue'
import type { PaymentOrder } from '@/types/payment'

function order(over: Partial<PaymentOrder> = {}): PaymentOrder {
  return {
    id: 2, user_id: 1, amount: 1, pay_amount: 1, fee_rate: 0, payment_type: 'alipay', out_trade_no: 'x', status: 'COMPLETED',
    order_type: 'package', created_at: '2026-10-06T00:00:00Z', expires_at: '2026-10-06T00:30:00Z', refund_amount: 0, ...over,
  } as PaymentOrder
}

const detail = {
  plan_name: '摸鱼周卡', cycle: 'week' as const, tier: 1, quota_usd: 10, group_name: 'GPT-Pro',
  user_package: {
    id: 1, user_id: 1, group_id: 2, plan_id: 1, name: '摸鱼周卡', cycle: 'week' as const, tier: 1, quota_usd: 10, used_usd: 4,
    starts_at: '2026-10-06T00:00:00Z', expires_at: '2026-10-13T00:00:00Z', status: 'frozen' as const, frozen_seconds_total: 0,
    created_at: '2026-10-06T00:00:00Z', updated_at: '2026-10-06T00:00:00Z', group_name: 'GPT-Pro', remaining_usd: 6,
    frozen_seconds: 86400 * 1.5, freeze_left_seconds: 0, max_freeze_days: 7, deduct_order: 0,
  },
}

describe('OrderContentCell', () => {
  it('套餐订单显示买了什么，以及套餐当前状态、使用量、有效期和冻结天数', () => {
    const text = mount(OrderContentCell, { props: { order: order({ package: detail as never }) } }).text()
    expect(text).toContain('摸鱼周卡')
    expect(text).toContain('GPT-Pro')
    expect(text).toContain('packages.mine.status.frozen')
    expect(text).toContain('$4.00 / $10.00')
    expect(text).toContain('"used":"1.5"')
    expect(text).toContain('"max":7')
  })

  it('没有发货的套餐订单（已取消）只显示套餐信息，不显示套餐状态', () => {
    const w = mount(OrderContentCell, { props: { order: order({ status: 'CANCELLED', package: { ...detail, user_package: undefined } as never }) } })
    expect(w.text()).toContain('摸鱼周卡')
    expect(w.find('[data-test="package-status"]').exists()).toBe(false)
    expect(w.text()).not.toContain('payment.orders.content.shipping')
  })

  it('已付款但套餐还没发放时提示稍后刷新', () => {
    const w = mount(OrderContentCell, { props: { order: order({ package: { ...detail, user_package: undefined } as never }) } })
    expect(w.text()).toContain('payment.orders.content.shipping')
  })

  it('套餐配置已删除、没有详情时仍能显示是套餐订单', () => {
    expect(mount(OrderContentCell, { props: { order: order() } }).text()).toContain('payment.orders.content.package')
  })

  it('余额充值与订阅订单各自显示类型', () => {
    expect(mount(OrderContentCell, { props: { order: order({ order_type: 'balance', amount: 10 }) } }).text()).toContain('payment.orders.content.balance')
    expect(mount(OrderContentCell, { props: { order: order({ order_type: 'subscription' }) } }).text()).toContain('payment.orders.content.subscription')
  })
})
