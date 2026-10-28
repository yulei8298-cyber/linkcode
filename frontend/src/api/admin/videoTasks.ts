/**
 * 视频任务管理接口：查看任务与账务状态、手动刷新、重试结算。
 */
import { apiClient } from '../client'
import type { PaginatedResponse } from '@/types'

export type VideoTaskStatus = 'queued' | 'in_progress' | 'completed' | 'failed' | 'cancelled'
export type VideoBillingStatus = 'pending' | 'billed' | 'skipped' | 'failed'

export interface VideoTask {
  id: number
  task_id: string
  user_id: number
  api_key_id: number
  group_id: number | null
  account_id: number
  protocol: string
  source: 'unified' | 'native'
  model: string
  upstream_model: string
  upstream_task_id: string
  status: VideoTaskStatus
  progress: number
  resolution: string
  aspect_ratio: string
  duration_seconds: number
  actual_duration_seconds: number | null
  video_count: number
  generate_audio: boolean
  estimated_cost: number
  actual_cost: number | null
  completion_tokens: number
  /** 网关签名的成品链接（相对路径，24 小时有效）；未完成时为空。不返回上游原始地址。 */
  content_url?: string
  error_code: string
  error_message: string
  poll_count: number
  billing_status: VideoBillingStatus
  billing_attempts: number
  billing_error: string
  inbound_endpoint: string
  created_at: string
  updated_at: string
  finished_at: string | null
  billed_at: string | null
}

export interface VideoTaskQuery {
  page?: number
  page_size?: number
  status?: string
  billing_status?: string
  protocol?: string
  model?: string
  keyword?: string
  user_id?: number
}

export async function list(params: VideoTaskQuery): Promise<PaginatedResponse<VideoTask>> {
  const { data } = await apiClient.get('/admin/video-tasks', { params })
  return data
}

export async function refresh(taskId: string): Promise<VideoTask> {
  const { data } = await apiClient.post(`/admin/video-tasks/${encodeURIComponent(taskId)}/refresh`)
  return data
}

export async function retryBilling(taskId: string): Promise<VideoTask> {
  const { data } = await apiClient.post(`/admin/video-tasks/${encodeURIComponent(taskId)}/retry-billing`)
  return data
}

export default { list, refresh, retryBilling }
