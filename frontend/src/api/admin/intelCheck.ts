/**
 * Admin Model Intelligence Check API
 * 模型智力检测的管理端接口：设置、受检分组、题库、轮次与明细。
 *
 * 与公开接口（api/intelCheck.ts）的关键差别有两处，都别混用：
 *   1. 这里的明细带 error_message（上游状态码与报错片段），公开侧换成中性的
 *      status_note。把管理端类型用在公开页上会把上游细节泄出去。
 *   2. 受检分组只有 api_key_masked，没有任何字段承载完整凭据——后端 handler
 *      就不返回它。若哪天需要「回填 key」，正确做法是留空表示不改，而不是把
 *      明文发到浏览器。
 */

import { apiClient } from '../client'

export type IntelCheckKind = 'logic' | 'drawing'
export type IntelCheckStatus = 'pass' | 'fail' | 'request_error' | 'running'
export type IntelCheckAPIMode = 'responses' | 'chat_completions'
export type IntelCheckEffort = 'low' | 'medium' | 'high' | 'xhigh'
export type IntelCheckMatchMode = 'exact' | 'numeric' | 'contains' | 'regex'

// ==================== 设置 ====================

export interface IntelCheckDegradedRule {
  fail_streak: number
  recover_streak: number
}

/** 兼容旧版源码评审设置；新版在线判定不读取模型与凭据配置。 */
export interface IntelCheckDrawingJudge {
  target_id: number
  model: string
  reasoning_effort: string
  /** 评审通过分数线（1-100）。 */
  pass_score: number
  /** 兼容旧配置，新版归一化后固定为 true，不调用模型评审。 */
  skip_review?: boolean
}

export interface IntelCheckSettings {
  enabled: boolean
  interval_minutes: number
  request_timeout_seconds: number
  concurrency: number
  degraded_rule: IntelCheckDegradedRule
  drawing_judge: IntelCheckDrawingJudge
  timeline_points: number
  retention_days: number
  intro_title: string
  intro_text: string
}

// ==================== 受检分组 ====================

export interface IntelCheckTarget {
  id: number
  name: string
  description: string
  base_url: string
  /** 脱敏后的凭据（前 4 位 + ***）。没有承载明文的字段，这是有意的。 */
  api_key_masked: string
  /** true 时后端无法解密存量密文，该分组会被调度跳过，需重新填写。 */
  api_key_decrypt_failed: boolean
  api_mode: IntelCheckAPIMode
  model: string
  reasoning_effort: string
  /** 纯展示用的倍率文案，不参与任何计费。 */
  rate_label: string
  enabled: boolean
  sort_order: number
  created_by: number
  created_at: string
  updated_at: string
}

export interface IntelCheckTargetParams {
  name: string
  description?: string
  base_url: string
  /** 新建必填；编辑时留空表示保留原凭据（管理端拿到的是掩码，回传会把掩码写进库）。 */
  api_key?: string
  api_mode?: IntelCheckAPIMode
  model: string
  reasoning_effort?: IntelCheckEffort
  rate_label?: string
  /** 整块覆盖语义：编辑时必须显式带上，省略会让已停用的分组重新启用。 */
  enabled: boolean
  sort_order?: number
}

// ==================== 题库 ====================

/** 参考稿的结构指标快照，由服务端从 reference_html 现算，不接受传入。 */
export interface IntelCheckReferenceMetrics {
  structure_baseline?: { shapes: number; geometry_values: number }
  standard_count?: number
  shape_count?: number
  animated_targets?: number
  defs_symbols?: number
  path_data_bytes?: number
  html_bytes?: number
  mechanisms?: string[]
  has_svg?: boolean
  has_title?: boolean
  has_desc?: boolean
}

/** 确定性结构验收；min_ratio × 100 为结构综合分的通过线。 */
export interface IntelCheckDrawingRules {
  min_ratio?: number
  required_keywords?: string[]
  max_bytes?: number
  /** 额外标准正样本原文，仅题目详情返回，指标由服务端计算。 */
  standard_sources?: string[]
}

/** 题库列表行：不含标准样本正文，只给体积、结构基准和标准数量。 */
export interface IntelCheckQuestionListItem {
  id: number
  kind: IntelCheckKind
  title: string
  prompt: string
  expected_answer: string
  match_mode: string
  reference_html_bytes: number
  reference_metrics: IntelCheckReferenceMetrics | null
  drawing_rules: IntelCheckDrawingRules | null
  /** 兼容旧接口字段；新版结构验收不使用源码评审。 */
  has_review_rubric: boolean
  enabled: boolean
  created_at: string
  updated_at: string
}

/** 题库详情：编辑表单需要全部正文原样回填。 */
export interface IntelCheckQuestion {
  id: number
  kind: IntelCheckKind
  title: string
  prompt: string
  expected_answer: string
  match_mode: string
  reference_html: string
  reference_metrics: IntelCheckReferenceMetrics | null
  drawing_rules: IntelCheckDrawingRules | null
  review_rubric: string
  enabled: boolean
  created_at: string
  updated_at: string
}

export interface IntelCheckQuestionParams {
  kind: IntelCheckKind
  title: string
  prompt: string
  /** 逻辑题必填。 */
  expected_answer?: string
  match_mode?: IntelCheckMatchMode
  /** 绘图题专用。reference_metrics 不在入参里：它是门禁相对项的分母，可写等于把阈值交给调用方。 */
  reference_html?: string
  drawing_rules?: IntelCheckDrawingRules
  review_rubric?: string
  enabled: boolean
}

// ==================== 轮次与明细 ====================

export interface IntelCheckRound {
  id: number
  seq: number
  started_at: string
  /** null 表示这一轮仍在执行。 */
  finished_at: string | null
  logic_question_id: number | null
  drawing_question_id: number | null
  /** cron | manual */
  trigger_source: string
}

export interface IntelCheckResultListItem {
  id: number
  round_id: number
  target_id: number
  kind: IntelCheckKind
  status: IntelCheckStatus
  latency_ms: number | null
  extracted_answer: string
  /** 管理端相对公开详情多出来的那一项：上游状态码与报错片段，排障靠它。 */
  error_message: string
  raw_reply_bytes: number
  html_output_bytes: number
  input_tokens: number | null
  output_tokens: number | null
  checked_at: string
}

/** 试跑与阈值标定的结果。没有 round_id：它不落库、不占轮次号。 */
export interface IntelCheckTrialResult {
  question_id: number
  question_kind: IntelCheckKind
  question_title: string
  target_id: number
  target_name: string
  model: string
  reasoning_effort: string
  status: IntelCheckStatus
  latency_ms: number | null
  raw_reply: string
  extracted_answer: string
  html_output: string
  judge_detail: Record<string, unknown> | null
  error_message: string
  input_tokens: number | null
  output_tokens: number | null
}

export interface PaginatedResponse<T> {
  items: T[]
  total: number
  page: number
  page_size: number
  pages: number
}

export interface ListQuestionParams {
  page?: number
  page_size?: number
  kind?: IntelCheckKind
  enabled?: boolean
}

export interface ListResultParams {
  page?: number
  page_size?: number
  round_id?: number
  target_id?: number
  kind?: IntelCheckKind
  status?: IntelCheckStatus
}

// ==================== 设置 ====================

export async function getSettings(): Promise<IntelCheckSettings> {
  const { data } = await apiClient.get<IntelCheckSettings>('/admin/intel-check/settings')
  return data
}

/** 整块覆盖。缺失字段会被后端 Normalize 补默认值，表单必须提交全部字段。 */
export async function updateSettings(params: IntelCheckSettings): Promise<IntelCheckSettings> {
  const { data } = await apiClient.put<IntelCheckSettings>('/admin/intel-check/settings', params)
  return data
}

/** 立即触发一轮。立即返回，不等轮次跑完；已有一轮在飞时返回 409。 */
export async function runNow(): Promise<{ started: boolean }> {
  const { data } = await apiClient.post<{ started: boolean }>('/admin/intel-check/run-now')
  return data
}

// ==================== 受检分组 ====================

export async function listTargets(
  params: { page?: number; page_size?: number; enabled?: boolean; search?: string } = {},
  options?: { signal?: AbortSignal },
): Promise<PaginatedResponse<IntelCheckTarget>> {
  const { data } = await apiClient.get<PaginatedResponse<IntelCheckTarget>>(
    '/admin/intel-check/targets',
    { params, signal: options?.signal },
  )
  return data
}

export async function getTarget(id: number): Promise<IntelCheckTarget> {
  const { data } = await apiClient.get<IntelCheckTarget>(`/admin/intel-check/targets/${id}`)
  return data
}

export async function createTarget(params: IntelCheckTargetParams): Promise<IntelCheckTarget> {
  const { data } = await apiClient.post<IntelCheckTarget>('/admin/intel-check/targets', params)
  return data
}

export async function updateTarget(
  id: number,
  params: IntelCheckTargetParams,
): Promise<IntelCheckTarget> {
  const { data } = await apiClient.put<IntelCheckTarget>(
    `/admin/intel-check/targets/${id}`,
    params,
  )
  return data
}

/** 删除分组会级联删掉它的历史明细（外键 ON DELETE CASCADE）。 */
export async function deleteTarget(id: number): Promise<void> {
  await apiClient.delete(`/admin/intel-check/targets/${id}`)
}

// ==================== 题库 ====================

export async function listQuestions(
  params: ListQuestionParams = {},
  options?: { signal?: AbortSignal },
): Promise<PaginatedResponse<IntelCheckQuestionListItem>> {
  const { data } = await apiClient.get<PaginatedResponse<IntelCheckQuestionListItem>>(
    '/admin/intel-check/questions',
    { params, signal: options?.signal },
  )
  return data
}

export async function getQuestion(id: number): Promise<IntelCheckQuestion> {
  const { data } = await apiClient.get<IntelCheckQuestion>(`/admin/intel-check/questions/${id}`)
  return data
}

/** 新建。绘图题会在响应里带上服务端算出的 reference_metrics，供管理员核对。 */
export async function createQuestion(
  params: IntelCheckQuestionParams,
): Promise<IntelCheckQuestion> {
  const { data } = await apiClient.post<IntelCheckQuestion>(
    '/admin/intel-check/questions',
    params,
  )
  return data
}

export async function updateQuestion(
  id: number,
  params: IntelCheckQuestionParams,
): Promise<IntelCheckQuestion> {
  const { data } = await apiClient.put<IntelCheckQuestion>(
    `/admin/intel-check/questions/${id}`,
    params,
  )
  return data
}

/** 删题不影响历史明细：题面已快照在 results.prompt_snapshot 里。 */
export async function deleteQuestion(id: number): Promise<void> {
  await apiClient.delete(`/admin/intel-check/questions/${id}`)
}

/**
 * 阈值标定：只判定粘贴进来的产物，不向受检分组发绘图请求。
 * 用于调 min_ratio 与 pass_score——省掉绘图那次请求，一次标定从几分钟降到一次评审调用。
 */
export async function evaluateQuestion(
  id: number,
  source: string,
): Promise<IntelCheckTrialResult> {
  const { data } = await apiClient.post<IntelCheckTrialResult>(
    `/admin/intel-check/questions/${id}/evaluate`,
    { source },
  )
  return data
}

/**
 * 试跑：对单组真发一次请求并判定，结果不落库。
 * 与真实检测共用同一条判定路径，所以这里看到的结论就是线上会得出的结论。
 */
export async function dryRunQuestion(
  id: number,
  targetId: number,
): Promise<IntelCheckTrialResult> {
  const { data } = await apiClient.post<IntelCheckTrialResult>(
    `/admin/intel-check/questions/${id}/dry-run`,
    undefined,
    { params: { target_id: targetId } },
  )
  return data
}

// ==================== 轮次与明细 ====================

export async function listRounds(
  params: { page?: number; page_size?: number } = {},
  options?: { signal?: AbortSignal },
): Promise<PaginatedResponse<IntelCheckRound>> {
  const { data } = await apiClient.get<PaginatedResponse<IntelCheckRound>>(
    '/admin/intel-check/rounds',
    { params, signal: options?.signal },
  )
  return data
}

/** 明细列表。后端把每页上限收敛到 20（行里带大字段的字节数，不带正文）。 */
export async function listResults(
  params: ListResultParams = {},
  options?: { signal?: AbortSignal },
): Promise<PaginatedResponse<IntelCheckResultListItem>> {
  const { data } = await apiClient.get<PaginatedResponse<IntelCheckResultListItem>>(
    '/admin/intel-check/results',
    { params, signal: options?.signal },
  )
  return data
}

export const intelCheckAdminAPI = {
  getSettings,
  updateSettings,
  runNow,
  listTargets,
  getTarget,
  createTarget,
  updateTarget,
  deleteTarget,
  listQuestions,
  getQuestion,
  createQuestion,
  updateQuestion,
  deleteQuestion,
  evaluateQuestion,
  dryRunQuestion,
  listRounds,
  listResults,
}

export default intelCheckAdminAPI
