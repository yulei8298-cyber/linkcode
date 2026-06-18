/**
 * Public (no-auth) API endpoints for the public portal.
 * 公开门户使用的只读接口，无需登录即可访问。
 * 数据结构复用用户侧接口的类型定义（channels.ts / channelMonitor.ts）。
 */

import { apiClient } from './client'
import type { UserAvailableChannel } from './channels'
import type { UserMonitorListResponse, UserMonitorDetail } from './channelMonitor'

/** 公开定价方案：列出所有公开分组（is_exclusive=false）的渠道与模型定价。 */
export async function getPublicPricing(options?: { signal?: AbortSignal }): Promise<UserAvailableChannel[]> {
  const { data } = await apiClient.get<UserAvailableChannel[]>('/public/pricing', {
    signal: options?.signal,
  })
  return data
}

/** 公开渠道可用性监控列表。 */
export async function getPublicMonitors(options?: { signal?: AbortSignal }): Promise<UserMonitorListResponse> {
  const { data } = await apiClient.get<UserMonitorListResponse>('/public/channel-monitors', {
    signal: options?.signal,
  })
  return data
}

/** 单个监控项的多窗口可用率/延迟详情。 */
export async function getPublicMonitorStatus(id: number): Promise<UserMonitorDetail> {
  const { data } = await apiClient.get<UserMonitorDetail>(`/public/channel-monitors/${id}/status`)
  return data
}

export const publicPortalAPI = {
  getPublicPricing,
  getPublicMonitors,
  getPublicMonitorStatus,
}

export default publicPortalAPI
