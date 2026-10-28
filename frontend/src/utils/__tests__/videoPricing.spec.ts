import { describe, expect, it } from 'vitest'
import type { VideoPricingSpec } from '@/api/admin/modelBasePricing'
import { describeVideoPricing, estimateVideoCost } from '../videoPricing'
import { formatVideoUsage, isVideoUsage } from '../billingMode'

const perSecond: VideoPricingSpec = {
  unit: 'per_second',
  prices: { '720p': { per_second: 0.1, per_second_with_audio: 0.15 }, '1080p': { per_second: 0.2 } },
  min_duration_seconds: 2,
  max_duration_seconds: 30,
}

describe('estimateVideoCost', () => {
  it('按秒：分辨率档、带音频价与时长上下限与后端一致', () => {
    expect(estimateVideoCost(perSecond, '720p', 10)).toBeCloseTo(1)
    expect(estimateVideoCost(perSecond, '720p', 10, true)).toBeCloseTo(1.5)
    expect(estimateVideoCost(perSecond, '480p', 1)).toBeCloseTo(0.2) // 取更高的最近档 720p，时长收敛到 2 秒
    expect(estimateVideoCost(perSecond, '4k', 60)).toBeCloseTo(6) // 取最高档 1080p，时长收敛到 30 秒
  })

  it('按次：取不小于实际时长的最短时长档', () => {
    const perVideo: VideoPricingSpec = { unit: 'per_video', prices: { '720p': { per_video: { '6': 0.28, '10': 0.56 } } } }
    expect(estimateVideoCost(perVideo, '720p', 5)).toBeCloseTo(0.28)
    expect(estimateVideoCost(perVideo, '720p', 8)).toBeCloseTo(0.56)
    expect(estimateVideoCost(perVideo, '720p', 15)).toBeCloseTo(0.56)
  })

  it('按 token 不做估算', () => {
    expect(estimateVideoCost({ unit: 'token', token_price_per_million: 10 }, '720p', 5)).toBeNull()
  })
})

describe('describeVideoPricing', () => {
  it('给出最低起价并乘倍率', () => {
    expect(describeVideoPricing(perSecond)).toBe('$0.1/秒 起')
    expect(describeVideoPricing(perSecond, 2)).toBe('$0.2/秒 起')
    expect(describeVideoPricing({ unit: 'token', token_price_per_million: 10 })).toBe('$10.00/百万 token')
  })
})

describe('video usage helpers', () => {
  const t = (key: string, values?: Record<string, unknown>) => `${key}:${values?.n ?? ''}`
  it('识别视频用量并格式化摘要', () => {
    expect(isVideoUsage({ billing_mode: 'video' })).toBe(true)
    expect(isVideoUsage({ video_count: 1, billing_mode: 'per_request' })).toBe(true)
    expect(isVideoUsage({ video_count: 1, billing_mode: 'token' })).toBe(false)
    expect(isVideoUsage(null)).toBe(false)
    expect(formatVideoUsage({ video_count: 1, video_resolution: '720p', video_duration_seconds: 5 }, t))
      .toBe('usage.videoCountValue:1 · 720p · usage.videoSecondsValue:5')
  })
})
