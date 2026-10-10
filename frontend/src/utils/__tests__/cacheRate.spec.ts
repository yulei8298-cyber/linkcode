import { describe, expect, it } from 'vitest'
import { calcCacheRate, calcRowCacheRate, formatCacheRate } from '../formatters'

describe('calcCacheRate', () => {
  it('divides cache read tokens by total tokens', () => {
    expect(calcCacheRate(25, 100)).toBe(0.25)
    expect(calcCacheRate(0, 100)).toBe(0)
    expect(calcCacheRate(100, 100)).toBe(1)
  })

  it('returns null when there are no tokens at all', () => {
    expect(calcCacheRate(0, 0)).toBeNull()
    expect(calcCacheRate(5, 0)).toBeNull()
    expect(calcCacheRate(5, Number.NaN)).toBeNull()
  })

  it('clamps inconsistent data into 0..1', () => {
    expect(calcCacheRate(150, 100)).toBe(1)
    expect(calcCacheRate(-5, 100)).toBe(0)
  })
})

describe('calcRowCacheRate', () => {
  it('uses input + output + cache creation + cache read as the total', () => {
    // 总计 = 100 + 50 + 12 + 38 = 200，缓存读取 38 → 19%
    expect(
      calcRowCacheRate({ input_tokens: 100, output_tokens: 50, cache_creation_tokens: 12, cache_read_tokens: 38 }),
    ).toBeCloseTo(0.19, 10)
  })

  it('treats missing fields as zero', () => {
    expect(calcRowCacheRate({ input_tokens: 80, cache_read_tokens: 20 })).toBeCloseTo(0.2, 10)
    expect(calcRowCacheRate({})).toBeNull()
    expect(calcRowCacheRate({ input_tokens: null, output_tokens: null })).toBeNull()
  })

  it('is zero (not null) when there are tokens but no cache read', () => {
    expect(calcRowCacheRate({ input_tokens: 10, output_tokens: 5 })).toBe(0)
  })
})

describe('formatCacheRate', () => {
  it('keeps one decimal place', () => {
    expect(formatCacheRate(0.375)).toBe('37.5%')
    expect(formatCacheRate(0)).toBe('0.0%')
    expect(formatCacheRate(1)).toBe('100.0%')
    expect(formatCacheRate(0.11956)).toBe('12.0%')
  })

  it('shows a dash when there is no data', () => {
    expect(formatCacheRate(null)).toBe('-')
  })
})
