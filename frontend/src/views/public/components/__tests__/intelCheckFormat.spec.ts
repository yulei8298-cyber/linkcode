import { describe, expect, it } from 'vitest'
import { padTimeline, statusClass } from '../intelCheckFormat'

describe('intel check timeline formatting', () => {
  it.each([
    ['pass', ''],
    ['fail', 'bad'],
    ['request_error', 'degraded'],
    ['running', 'running'],
    ['unverified', 'unverified'],
    ['unknown', 'unknown'],
  ])('maps %s to the timeline class %s', (status, expected) => {
    expect(statusClass(status)).toBe(expected)
  })

  it('pads older empty cells on the left and keeps the newest points aligned', () => {
    const points = [
      { result_id: 1, status: 'pass' as const, latency_ms: 10, checked_at: '2026-09-13T10:00:00Z' },
      { result_id: 2, status: 'fail' as const, latency_ms: 20, checked_at: '2026-09-13T10:30:00Z' },
    ]

    expect(padTimeline(points, 4)).toEqual([
      null,
      null,
      points[0],
      points[1],
    ])
  })

  it('keeps only the newest cells when history exceeds the configured size', () => {
    const points = Array.from({ length: 3 }, (_, index) => ({
      result_id: index + 1,
      status: 'pass' as const,
      latency_ms: null,
      checked_at: `2026-09-13T1${index}:00:00Z`,
    }))

    expect(padTimeline(points, 2).map((point) => point?.result_id ?? null)).toEqual([2, 3])
  })
})
