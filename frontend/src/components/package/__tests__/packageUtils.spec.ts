import { describe, expect, it } from 'vitest'
import type { PackageCalendarDay } from '@/api/packages'
import {
  buildCalendarGrid,
  formatCNY,
  formatUSD,
  packagePlanRank,
  packagePlanStyle,
  parsePackageNotice,
  shiftMonth,
  splitDuration,
  usedPercent,
} from '../packageUtils'

describe('packagePlanStyle', () => {
  it('按周期 × 档位固定颜色、角标与角落标签', () => {
    expect(packagePlanStyle('week', 1)).toMatchObject({ hue: 'violet', badge: 'recommend', hot: false, ribbon: false })
    expect(packagePlanStyle('week', 2)).toMatchObject({ hue: 'orange', badge: 'hot', hot: true, ribbon: false })
    expect(packagePlanStyle('month', 1)).toMatchObject({ hue: 'green', badge: 'value', ribbon: true })
    expect(packagePlanStyle('month', 2)).toMatchObject({ hue: 'rose', badge: 'best', ribbon: true })
  })

  it('商店排序：周卡在前，同周期 1x 在前', () => {
    const plans = [
      { cycle: 'month' as const, tier: 2 as const },
      { cycle: 'week' as const, tier: 2 as const },
      { cycle: 'month' as const, tier: 1 as const },
      { cycle: 'week' as const, tier: 1 as const },
    ]
    const sorted = [...plans].sort((a, b) => packagePlanRank(a) - packagePlanRank(b))
    expect(sorted.map((p) => `${p.cycle}-${p.tier}`)).toEqual(['week-1', 'week-2', 'month-1', 'month-2'])
  })
})

describe('parsePackageNotice', () => {
  const text = [
    '并发说明：套餐不限 RPM，〔同时不超过 {并发} 个〕，超出拒绝。',
    '冻结规则：周卡最多冻结 {周卡冻结上限} 天，月卡最多冻结 {月卡冻结上限} 天。',
    '',
    '退款政策：〔付款后不退款〕',
    '没有标题的一行',
  ].join('\n')

  it('替换占位符、拆出高亮片段并标记需要醒目的条目', () => {
    const lines = parsePackageNotice(text, { concurrency: 5, maxFreezeDaysWeek: 7, maxFreezeDaysMonth: 15 })
    expect(lines).toHaveLength(4)
    expect(lines[0].title).toBe('并发说明')
    expect(lines[0].segments).toEqual([
      { text: '套餐不限 RPM，', highlight: false },
      { text: '同时不超过 5 个', highlight: true },
      { text: '，超出拒绝。', highlight: false },
    ])
    expect(lines[1].segments[0].text).toBe('周卡最多冻结 7 天，月卡最多冻结 15 天。')
    expect(lines[2]).toMatchObject({ title: '退款政策', warn: true })
    expect(lines[2].segments).toEqual([{ text: '付款后不退款', highlight: true }])
    expect(lines[3]).toMatchObject({ title: '', warn: false })
  })

  it('正文里的尖括号原样作为文本，不会被当成 HTML', () => {
    const [line] = parsePackageNotice('提示：<img src=x onerror=alert(1)>', { concurrency: 5, maxFreezeDaysWeek: 7, maxFreezeDaysMonth: 15 })
    expect(line.segments).toEqual([{ text: '<img src=x onerror=alert(1)>', highlight: false }])
  })
})

describe('buildCalendarGrid', () => {
  function day(date: string, weekday: number): PackageCalendarDay {
    return { date, weekday, freezable: false, kind: 'none', label: '' }
  }

  it('周一开头补齐首尾，并标出今天', () => {
    // 2026-10-01 是周四：前面补 3 个空格（周一~周三）
    const days = Array.from({ length: 31 }, (_, i) => day(`2026-10-${String(i + 1).padStart(2, '0')}`, (4 + i) % 7))
    const cells = buildCalendarGrid(days, '2026-10-04')
    expect(cells.length % 7).toBe(0)
    expect(cells.slice(0, 3).every((c) => c.day === null)).toBe(true)
    expect(cells[3].day?.date).toBe('2026-10-01')
    expect(cells.filter((c) => c.isToday).map((c) => c.day?.date)).toEqual(['2026-10-04'])
    expect(cells.length).toBe(35)
  })

  it('周日开头的月份前面补 6 格；空数据返回空网格', () => {
    const cells = buildCalendarGrid([day('2026-11-01', 0)], '')
    expect(cells.findIndex((c) => c.day !== null)).toBe(6)
    expect(buildCalendarGrid([], '')).toEqual([])
  })
})

describe('数值工具', () => {
  it('shiftMonth 跨年', () => {
    expect(shiftMonth('2026-12', 1)).toBe('2027-01')
    expect(shiftMonth('2026-01', -1)).toBe('2025-12')
  })

  it('splitDuration 拆成天和小时，负数归零', () => {
    expect(splitDuration(7 * 86400)).toEqual({ days: 7, hours: 0 })
    expect(splitDuration(27 * 3600 + 59)).toEqual({ days: 1, hours: 3 })
    expect(splitDuration(-10)).toEqual({ days: 0, hours: 0 })
  })

  it('usedPercent 边界', () => {
    expect(usedPercent(187.6, 240)).toBe(78)
    expect(usedPercent(300, 240)).toBe(100)
    expect(usedPercent(1, 0)).toBe(0)
  })

  it('金额格式', () => {
    expect(formatUSD(52.4)).toBe('$52.40')
    expect(formatCNY(95)).toBe('¥95.00')
    expect(formatUSD(Number.NaN)).toBe('$0.00')
  })
})
