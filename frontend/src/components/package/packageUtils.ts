/**
 * 套餐页面的纯逻辑：档位样式、购买须知解析、月历网格、时长拆分。
 * 不依赖组件实例，便于单测。
 */

import type { PackageCalendarDay, PackageCycle, PackagePlan, PackageTier } from '@/api/packages'

export type PackageHue = 'violet' | 'orange' | 'green' | 'rose'
export type PackageBadge = 'recommend' | 'hot' | 'value' | 'best'
/** 取自 components/icons/Icon.vue 的图标名 */
export type PackageIcon = 'sparkles' | 'fire' | 'gift' | 'trophy'

export interface PackagePlanStyle {
  hue: PackageHue
  badge: PackageBadge
  icon: PackageIcon
  /** 高亮描边（周卡 2x） */
  hot: boolean
  /** 角落「新上架」标签（月卡） */
  ribbon: boolean
}

/** 周期 × 额度档位 → 卡片样式。后台新增套餐只选周期与档位，样式由这里统一决定。 */
const PLAN_STYLES: Record<string, PackagePlanStyle> = {
  'week-1': { hue: 'violet', badge: 'recommend', icon: 'sparkles', hot: false, ribbon: false },
  'week-2': { hue: 'orange', badge: 'hot', icon: 'fire', hot: true, ribbon: false },
  'month-1': { hue: 'green', badge: 'value', icon: 'gift', hot: false, ribbon: true },
  'month-2': { hue: 'rose', badge: 'best', icon: 'trophy', hot: false, ribbon: true },
}

export function packagePlanStyle(cycle: PackageCycle, tier: PackageTier): PackagePlanStyle {
  return PLAN_STYLES[`${cycle}-${tier}`] ?? PLAN_STYLES['week-1']
}

/** 商店展示顺序：周卡在前，同周期 1x 在前。 */
export function packagePlanRank(plan: { cycle: PackageCycle; tier: PackageTier }): number {
  return (plan.cycle === 'month' ? 10 : 0) + plan.tier
}

// ---------- 购买须知 ----------

/** 购买须知弹窗的购买上下文：有值时必须读到底并勾选同意才能去支付。 */
export interface PackagePurchaseContext {
  plan: PackagePlan
  groupName: string
}

export interface NoticeSegment {
  text: string
  highlight: boolean
}

export interface NoticeLine {
  title: string
  segments: NoticeSegment[]
  /** 使用规则、退款等需要醒目提示的条目 */
  warn: boolean
}

export interface NoticeVars {
  concurrency: number
  maxFreezeDays: number
}

const NOTICE_WARN_TITLES = ['使用规则', '退款']

/**
 * 解析购买须知：每行「标题：内容」，〔〕内文字高亮，{并发}、{冻结上限} 替换为实际值。
 * 返回结构化片段，由模板逐段渲染，不使用 v-html。
 */
export function parsePackageNotice(text: string, vars: NoticeVars): NoticeLine[] {
  return text
    .split('\n')
    .map((line) => line.trim())
    .filter(Boolean)
    .map((line) => {
      const filled = line
        .replace(/\{并发\}/g, String(vars.concurrency))
        .replace(/\{冻结上限\}/g, String(vars.maxFreezeDays))
      const sep = filled.indexOf('：')
      const title = sep > 0 ? filled.slice(0, sep) : ''
      const body = sep > 0 ? filled.slice(sep + 1) : filled
      return {
        title,
        segments: splitHighlights(body),
        warn: NOTICE_WARN_TITLES.some((key) => title.includes(key)),
      }
    })
}

function splitHighlights(body: string): NoticeSegment[] {
  const segments: NoticeSegment[] = []
  const pattern = /〔(.+?)〕/g
  let last = 0
  for (const match of body.matchAll(pattern)) {
    const index = match.index ?? 0
    if (index > last) segments.push({ text: body.slice(last, index), highlight: false })
    segments.push({ text: match[1], highlight: true })
    last = index + match[0].length
  }
  if (last < body.length) segments.push({ text: body.slice(last), highlight: false })
  return segments
}

// ---------- 月历 ----------

export interface CalendarCell {
  day: PackageCalendarDay | null
  isToday: boolean
}

/**
 * 把某月每天的冻结信息排成周一开头的网格，首尾不足一周用空格补齐。
 * today 为 YYYY-MM-DD。
 */
export function buildCalendarGrid(days: PackageCalendarDay[], today: string): CalendarCell[] {
  if (days.length === 0) return []
  const leading = (days[0].weekday + 6) % 7
  const cells: CalendarCell[] = Array.from({ length: leading }, () => ({ day: null, isToday: false }))
  for (const day of days) cells.push({ day, isToday: day.date === today })
  while (cells.length % 7 !== 0) cells.push({ day: null, isToday: false })
  return cells
}

/** 本地日期的 YYYY-MM 与 YYYY-MM-DD。 */
export function monthKey(date: Date): string {
  return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}`
}

export function dayKey(date: Date): string {
  return `${monthKey(date)}-${String(date.getDate()).padStart(2, '0')}`
}

export function shiftMonth(month: string, delta: number): string {
  const [y, m] = month.split('-').map(Number)
  return monthKey(new Date(y, m - 1 + delta, 1))
}

// ---------- 数值 ----------

/** 秒数拆成天 + 小时（向下取整到小时）。 */
export function splitDuration(seconds: number): { days: number; hours: number } {
  const totalHours = Math.max(0, Math.floor(seconds / 3600))
  return { days: Math.floor(totalHours / 24), hours: totalHours % 24 }
}

export function formatUSD(value: number): string {
  return `$${(Number.isFinite(value) ? value : 0).toFixed(2)}`
}

export function formatCNY(value: number): string {
  return `¥${(Number.isFinite(value) ? value : 0).toFixed(2)}`
}

/** 已用比例（0~100），用于进度条。 */
export function usedPercent(used: number, quota: number): number {
  if (!(quota > 0)) return 0
  return Math.min(100, Math.max(0, Math.round((used / quota) * 100)))
}
