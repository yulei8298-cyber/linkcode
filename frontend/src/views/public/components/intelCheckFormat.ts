/**
 * 智力检测页的展示格式化。
 *
 * 集中放在一处，是因为同一套状态文案要同时出现在色块 tooltip、分组徽章与详情弹窗里，
 * 三处各写一份迟早会出现「时间线说通过、弹窗说失败」这种自相矛盾的画面。
 */

import type {
  IntelCheckKind,
  IntelCheckState,
  IntelCheckStatus,
  IntelCheckTimelinePoint,
} from '@/api/intelCheck'

/** 单次检测状态的中文文案。 */
export function statusLabel(status?: IntelCheckStatus | string): string {
  switch (status) {
    case 'pass':
      return '通过'
    case 'fail':
      return '未通过'
    case 'request_error':
      return '请求失败'
    case 'running':
      return '检测中'
    default:
      return '暂无数据'
  }
}

/**
 * 色块的样式类。
 *
 * 沿用门户页 lc-timeline 的既有语义类（可用性检测页同款），新增两个本页专用状态：
 *   pass          → 默认（绿）
 *   fail          → bad（红）
 *   request_error → degraded（黄）：链路故障，不是模型不合格，颜色必须与 fail 区分
 *   running       → running（浅蓝，脉冲）
 *   无数据        → unknown（灰）
 */
export function statusClass(status?: IntelCheckStatus | string): string {
  switch (status) {
    case 'pass':
      return ''
    case 'fail':
      return 'bad'
    case 'request_error':
      return 'degraded'
    case 'running':
      return 'running'
    default:
      return 'unknown'
  }
}

/** 分组状态的中文文案。 */
export function stateLabel(state?: IntelCheckState | string): string {
  switch (state) {
    case 'normal':
      return '正常'
    case 'degraded':
      return '疑似降智'
    default:
      return '暂无数据'
  }
}

/** 分组状态徽章的样式类，与色块共用一套语义。 */
export function stateClass(state?: IntelCheckState | string): string {
  switch (state) {
    case 'normal':
      return ''
    case 'degraded':
      return 'bad'
    default:
      return 'unknown'
  }
}

/** 题型文案。 */
export function kindLabel(kind?: IntelCheckKind | string): string {
  return kind === 'drawing' ? '绘图题' : '逻辑题'
}

/** 答案匹配模式文案。 */
export function matchModeLabel(mode?: string): string {
  switch (mode) {
    case 'exact':
      return '完全匹配'
    case 'numeric':
      return '数值比较'
    case 'contains':
      return '包含匹配'
    case 'regex':
      return '正则匹配'
    default:
      return mode || '--'
  }
}

/** 耗时。超过一分钟改用分秒，避免出现「318000 ms」这种读不出量级的数字。 */
export function formatLatency(value?: number | null): string {
  if (value == null) return '--'
  if (value < 1000) return `${Math.round(value)} ms`
  if (value < 60_000) return `${(value / 1000).toFixed(1)} s`
  const minutes = Math.floor(value / 60_000)
  const seconds = Math.round((value % 60_000) / 1000)
  return `${minutes} 分 ${seconds} 秒`
}

/**
 * 通过率。
 *
 * hasData 为 false 时返回「暂无数据」而不是 0%：一个还没跑过的分组显示 0%，
 * 看起来和「每次都失败」一模一样，而这正是本页最不该造成的误解。
 */
export function formatPassRate(rate?: number | null, hasData?: boolean): string {
  if (!hasData || rate == null) return '暂无数据'
  return `${(rate * 100).toFixed(1)}%`
}

/** 绝对时刻，按浏览器本地时区展示。 */
export function formatDateTime(value?: string | null): string {
  if (!value) return '--'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString('zh-CN', { hour12: false })
}

/** 相对时刻（「3 分钟前」）。 */
export function formatRelative(value?: string | null): string {
  if (!value) return '尚未开始检测'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value

  const diffMs = Date.now() - date.getTime()
  if (diffMs < 0) return '刚刚'
  const minutes = Math.floor(diffMs / 60_000)
  if (minutes < 1) return '刚刚'
  if (minutes < 60) return `${minutes} 分钟前`
  const hours = Math.floor(minutes / 60)
  if (hours < 24) return `${hours} 小时前`
  return `${Math.floor(hours / 24)} 天前`
}

/**
 * 距离下次检测的倒计时文案。
 *
 * 预计时刻已经过去时如实说「即将开始」而不做美化：那往往说明调度停了，
 * 藏起来只会让故障更难被发现（与后端 next_check_at 的口径一致）。
 */
export function formatCountdown(nextCheckAt?: string | null): string {
  if (!nextCheckAt) return '尚未开始检测'
  const target = new Date(nextCheckAt)
  if (Number.isNaN(target.getTime())) return '--'

  const diffMs = target.getTime() - Date.now()
  if (diffMs <= 0) return '即将开始'
  const minutes = Math.ceil(diffMs / 60_000)
  if (minutes < 60) return `约 ${minutes} 分钟后`
  const hours = Math.floor(minutes / 60)
  return `约 ${hours} 小时 ${minutes % 60} 分钟后`
}

/**
 * 时间线上有耗时记录的那些格子的平均耗时。
 *
 * 客户端算而不是让后端给：概览接口本就把每格的 latency_ms 传过来了，
 * 再加一个聚合字段等于同一份数据传两遍。running 与请求失败的格子没有耗时，
 * 自然被排除在外——它们本来也不该拉低"正常作答要多久"这个观感。
 */
export function averageLatency(points?: IntelCheckTimelinePoint[]): number | null {
  const values = (points || [])
    .map((point) => point?.latency_ms)
    .filter((value): value is number => typeof value === 'number' && value > 0)
  if (values.length === 0) return null
  return values.reduce((sum, value) => sum + value, 0) / values.length
}

/** 时间线最早一格的时刻，用作时间轴左端刻度。 */
export function timelineStart(points?: IntelCheckTimelinePoint[]): string {
  const first = (points || [])[0]
  if (!first) return ''
  const date = new Date(first.checked_at)
  if (Number.isNaN(date.getTime())) return ''
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}`
}

/** 最近一格（用于「● 通过 · 17 分钟前」）。 */
export function latestPoint(
  points?: IntelCheckTimelinePoint[],
): IntelCheckTimelinePoint | null {
  const list = points || []
  return list.length ? list[list.length - 1] : null
}

/** 色块的悬浮提示：时刻 · 状态 · 耗时。 */
export function timelineTitle(point: IntelCheckTimelinePoint | null): string {
  if (!point) return '暂无数据'
  return `${formatDateTime(point.checked_at)} · ${statusLabel(point.status)} · ${formatLatency(point.latency_ms)}`
}

/**
 * 把时间线补齐到固定格数，不足的部分用 null 填在左侧（较旧的一端）。
 *
 * 不左对齐：各分组的历史长短不一，若都从左边开始画，最右一格就不再是同一时刻，
 * 而「同题同刻横向对比」正是这张页面的说服力来源。
 */
export function padTimeline(
  points: IntelCheckTimelinePoint[] | undefined,
  size: number,
): Array<IntelCheckTimelinePoint | null> {
  const recent = (points || []).slice(-size)
  const missing = Array.from({ length: Math.max(0, size - recent.length) }, () => null)
  return [...missing, ...recent]
}
