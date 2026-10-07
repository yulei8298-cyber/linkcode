/**
 * 企业尊享：当前用户的状态。类型与后端 service.EnterpriseStatus 对应。
 * 管理端的设置与手动开通接口在 api/admin/packages.ts。
 */

import { apiClient } from './client'

/** 单个用户的开通方式：auto 按累计消费自动，on 强制开通，off 强制关闭。 */
export type EnterpriseMode = 'auto' | 'on' | 'off'

export interface EnterpriseStatus {
  enterprise: boolean
  mode: EnterpriseMode
  /** 当前累计消费 */
  total: number
  /** 自动获得的门槛 */
  threshold: number
  /** 全局总开关是否打开 */
  enabled: boolean
}

export async function getEnterpriseStatus(): Promise<EnterpriseStatus> {
  const { data } = await apiClient.get<EnterpriseStatus>('/user/enterprise')
  return data
}

export default { getEnterpriseStatus }
