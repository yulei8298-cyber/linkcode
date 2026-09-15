/**
 * 模型智力检测（公开端点，无需登录）。
 *
 * 后端定期用与 Codex CLI 一致的请求方式，向各受检分组发一道逻辑题和一道绘图题，
 * 并把每次的原始回复与判定过程如实公开。本模块的类型与后端
 * service.IntelCheckPublicOverview / IntelCheckPublicResult 一一对应。
 *
 * 这里刻意没有 base_url、api_key、http 状态码之类的字段——后端的公开视图就不返回它们
 * （错误原文统一换成中性的 status_note）。若某天接口开始返回这些，
 * 也不该在前端类型里给它们留位置。
 */

import { apiClient } from './client'

/** 题型。 */
export type IntelCheckKind = 'logic' | 'drawing'

/** 单次检测状态：通过 / 未通过 / 请求失败（不计入判定）/ 检测中。 */
export type IntelCheckStatus = 'pass' | 'fail' | 'request_error' | 'running' | 'unverified'

/** 分组状态：正常 / 疑似降智 / 暂无数据。 */
export type IntelCheckState = 'normal' | 'degraded' | 'unknown'

/** 时间线上的一格色块。按 checked_at 升序（旧 → 新），与页面从左到右一致。 */
export interface IntelCheckTimelinePoint {
  result_id: number
  status: IntelCheckStatus
  latency_ms: number | null
  checked_at: string
}

/** 一段时间窗内的统计。request_error 不进通过率分母——上游抖动不是模型能力问题。 */
export interface IntelCheckStats {
  pass: number
  fail: number
  error: number
  /** 绘图产物存在，但缺少可测量动作证据；不进通过率分母。 */
  unverified: number
  pass_rate: number
  /** false 时应显示「暂无数据」而非 0%，二者含义完全不同。 */
  has_data: boolean
}

/** 降智判定规则，公开出来让读者能自行复核色块的含义。 */
export interface IntelCheckDegradedRule {
  fail_streak: number
  recover_streak: number
}

/** 公开页上的一张分组卡片。 */
export interface IntelCheckGroup {
  id: number
  name: string
  description: string
  /** 模型名与推理等级是有意公开的：本页要证明的正是「跑的是满血配置」。 */
  model: string
  reasoning_effort: string
  rate_label: string
  state: IntelCheckState
  logic_timeline: IntelCheckTimelinePoint[]
  drawing_timeline: IntelCheckTimelinePoint[]
  logic_stats_24h: IntelCheckStats
  drawing_stats_24h: IntelCheckStats
  /** 最近一次有画作产出的明细 id，0 表示暂无。判失败的画作同样会出现在这里。 */
  latest_drawing_result_id: number
}

/** 页面顶部汇总。 */
export interface IntelCheckSummary {
  total_groups: number
  normal_groups: number
  degraded_groups: number
  unknown_groups: number
  stats_24h: IntelCheckStats
  latest_round_seq: number
  last_checked_at: string | null
  /** 按「上一轮开始时间 + 检测周期」推算的预计时刻，不是承诺；未跑过时为 null。 */
  next_check_at: string | null
}

/** 一次请求拿到的全部公开数据。 */
export interface IntelCheckOverview {
  intro_title: string
  intro_text: string
  interval_minutes: number
  timeline_points: number
  degraded_rule: IntelCheckDegradedRule
  summary: IntelCheckSummary
  groups: IntelCheckGroup[]
  /** 服务端生成时刻，供倒计时以服务端时间为基准，避免客户端时钟偏差。 */
  generated_at: string
}

/** 单次检测详情的对外视图。 */
export interface IntelCheckResult {
  id: number
  round_seq: number
  kind: IntelCheckKind
  status: IntelCheckStatus
  target_id: number
  target_name: string
  model: string
  reasoning_effort: string
  question_title: string
  /** 本轮实际发送的题面快照。题目日后被改或被删，历史详情仍能还原当时问了什么。 */
  prompt_snapshot: string
  /** 逻辑题专用：三者合起来才能让读者复核判定。 */
  expected_answer: string
  match_mode: string
  extracted_answer: string
  raw_reply: string
  /** 绘图题专用：已由服务端清洗的 HTML 与逐项判定明细。 */
  html_output: string
  judge_detail: Record<string, unknown> | null
  latency_ms: number | null
  /** 状态的中性说明，取代不对外的 error_message。 */
  status_note: string
  checked_at: string
}

/** 获取公开页数据。功能未开启时后端返回 404。 */
export async function getIntelCheckOverview(
  options?: { signal?: AbortSignal },
): Promise<IntelCheckOverview> {
  const { data } = await apiClient.get<IntelCheckOverview>('/public/intel-check/overview', {
    signal: options?.signal,
  })
  return data
}

/** 获取单次检测详情。 */
export async function getIntelCheckResult(
  id: number,
  options?: { signal?: AbortSignal },
): Promise<IntelCheckResult> {
  const { data } = await apiClient.get<IntelCheckResult>(`/public/intel-check/results/${id}`, {
    signal: options?.signal,
  })
  return data
}

export const intelCheckAPI = {
  getIntelCheckOverview,
  getIntelCheckResult,
}

export default intelCheckAPI
