import { describe, expect, it } from 'vitest'
import {
  defaultPricingDisplayConfig,
  extractRechargeBenefitAmounts,
  findRechargeBenefit,
  parsePricingDisplayConfig,
} from '../pricingDisplayConfig'

describe('findRechargeBenefit', () => {
  const benefits = ['¥100 赠送 $10', '¥200 赠送 $25', '¥500 赠送 $65']

  it('does not invent a benefit for an unconfigured amount', () => {
    expect(findRechargeBenefit(benefits, 20)).toBeNull()
    expect(findRechargeBenefit(benefits, 50)).toBeNull()
  })

  it('uses ¥20 as the baseline recharge tier', () => {
    expect(defaultPricingDisplayConfig.recommendedAmounts).toEqual([20, 50, 100, 200])
  })

  it('keeps the configured recharge button destination', () => {
    const config = parsePricingDisplayConfig(JSON.stringify({
      rechargeButtonText: '立即充值',
      rechargeButtonUrl: 'https://pay.example.com/recharge',
    }))

    expect(config.rechargeButtonText).toBe('立即充值')
    expect(config.rechargeButtonUrl).toBe('https://pay.example.com/recharge')
  })

  it.each([
    [100, '¥100 赠送 $10'],
    [200, '¥200 赠送 $25'],
    [500, '¥500 赠送 $65'],
  ])('returns the exact configured benefit for ¥%s', (amount, expected) => {
    expect(findRechargeBenefit(benefits, amount)).toBe(expected)
  })

  it('extracts benefit-backed recharge tiers', () => {
    expect(extractRechargeBenefitAmounts(benefits)).toEqual([100, 200, 500])
  })
})
