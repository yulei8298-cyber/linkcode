import { describe, expect, it } from 'vitest'
import { parseWechatResumeRoute, stripWechatResumeQuery } from '../paymentWechatResume'

describe('parseWechatResumeRoute', () => {
  it('prefers the opaque resume token over legacy openid query params', () => {
    expect(parseWechatResumeRoute({
      wechat_resume: '1',
      wechat_resume_token: 'resume-token-123',
      openid: 'openid-123',
      payment_type: 'wxpay',
      amount: '12.5',
      order_type: 'subscription',
      plan_id: '7',
    }, [], 88)).toEqual({
      wechatResumeToken: 'resume-token-123',
      paymentType: 'wxpay',
      orderType: 'subscription',
      orderAmount: 0,
      planId: 7,
    })
  })

  it('falls back to legacy openid-based resume when opaque token is absent', () => {
    expect(parseWechatResumeRoute({
      wechat_resume: '1',
      openid: 'openid-123',
      payment_type: 'wxpay',
      amount: '12.5',
      order_type: 'balance',
    }, [], 88)).toEqual({
      openid: 'openid-123',
      paymentType: 'wxpay',
      orderType: 'balance',
      orderAmount: 12.5,
      planId: undefined,
    })
  })
})

describe('parseWechatResumeRoute（套餐订单）', () => {
  it('order_type=package 不会因为带 plan_id 被当成订阅，并带回须知版本', () => {
    expect(parseWechatResumeRoute({
      wechat_resume: '1',
      wechat_resume_token: 'resume-token-pkg',
      payment_type: 'wxpay',
      order_type: 'package',
      plan_id: '9',
      notice_version: '3',
    }, [], 0)).toEqual({
      wechatResumeToken: 'resume-token-pkg',
      paymentType: 'wxpay',
      orderType: 'package',
      orderAmount: 0,
      planId: 9,
      packageNoticeVersion: 3,
    })
  })

  it('非法须知版本视为未同意；非套餐订单忽略 notice_version', () => {
    const pkg = parseWechatResumeRoute({ wechat_resume: '1', openid: 'o1', order_type: 'package', plan_id: '9', amount: '95', notice_version: 'x' }, [], 0)
    expect(pkg?.packageNoticeVersion).toBeUndefined()
    expect(pkg?.orderAmount).toBe(95)
    const sub = parseWechatResumeRoute({ wechat_resume: '1', openid: 'o1', order_type: 'subscription', plan_id: '9', notice_version: '3' }, [], 0)
    expect(sub?.orderType).toBe('subscription')
    expect(sub?.packageNoticeVersion).toBeUndefined()
  })

  it('stripWechatResumeQuery 一并清掉 notice_version', () => {
    expect(stripWechatResumeQuery({ notice_version: '3', package_plan: '9' })).toEqual({ package_plan: '9' })
  })
})

describe('stripWechatResumeQuery', () => {
  it('removes both opaque-token and legacy resume params from the route query', () => {
    expect(stripWechatResumeQuery({
      foo: 'bar',
      wechat_resume: '1',
      wechat_resume_token: 'resume-token-123',
      openid: 'openid-123',
      payment_type: 'wxpay',
      amount: '12.5',
      order_type: 'subscription',
      plan_id: '7',
      state: 'state-123',
      scope: 'snsapi_base',
    })).toEqual({
      foo: 'bar',
    })
  })
})
