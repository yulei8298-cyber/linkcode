/**
 * 套餐（周卡 / 月卡）管理端接口：套餐配置、全局设置、可冻结日期、用户套餐。
 * 用户的套餐并发在「编辑用户」里修改（package_concurrency 字段）。
 */

import { apiClient } from '../client'
import type { PackageCycle, PackagePlan, PackageTier, UserPackage } from '../packages'

export interface PackagePlanInput {
  id?: number
  group_id: number
  cycle: PackageCycle
  tier: PackageTier
  name: string
  price: number
  /** tier=2 时后端忽略，固定为同周期 1x 的两倍 */
  quota_usd: number
  for_sale: boolean
}

export interface PackageHolidaySyncState {
  last_synced_at?: string
  /** 年份 → 同步到的天数；0 表示官方尚未公布 */
  years: Record<string, number>
  last_error?: string
}

export interface PackageSettings {
  freeze_enabled: boolean
  max_freeze_days_week: number
  max_freeze_days_month: number
  holiday_sync_enabled: boolean
  holiday_source_url: string
  notice_text: string
  notice_version: number
}

export interface PackageSettingsResponse extends PackageSettings {
  holiday_sync: PackageHolidaySyncState
}

export interface PackageHolidayRange {
  name: string
  start: string
  end: string
  days: number
  kind: 'off' | 'work'
  source: 'auto' | 'manual'
}

export interface PackageHolidayInput {
  name: string
  start: string
  end: string
}

export async function listPlans(groupId?: number): Promise<PackagePlan[]> {
  const { data } = await apiClient.get<PackagePlan[]>('/admin/packages/plans', {
    params: groupId ? { group_id: groupId } : undefined,
  })
  return data
}

export async function savePlan(input: PackagePlanInput): Promise<PackagePlan> {
  const { data } = await apiClient.post<PackagePlan>('/admin/packages/plans', input)
  return data
}

export async function deletePlan(id: number): Promise<void> {
  await apiClient.delete(`/admin/packages/plans/${id}`)
}

export async function getSettings(): Promise<PackageSettingsResponse> {
  const { data } = await apiClient.get<PackageSettingsResponse>('/admin/packages/settings')
  return data
}

export async function updateSettings(settings: PackageSettings): Promise<PackageSettings> {
  const { data } = await apiClient.put<PackageSettings>('/admin/packages/settings', settings)
  return data
}

export async function listHolidays(year: number): Promise<PackageHolidayRange[]> {
  const { data } = await apiClient.get<PackageHolidayRange[]>('/admin/packages/holidays', { params: { year } })
  return data
}

export async function addHoliday(input: PackageHolidayInput): Promise<void> {
  await apiClient.post('/admin/packages/holidays', input)
}

export async function deleteHoliday(input: PackageHolidayInput): Promise<void> {
  await apiClient.post('/admin/packages/holidays/delete', input)
}

export async function syncHolidays(): Promise<PackageHolidaySyncState> {
  const { data } = await apiClient.post<PackageHolidaySyncState>('/admin/packages/holidays/sync')
  return data
}

export async function listUserPackages(userId: number): Promise<UserPackage[]> {
  const { data } = await apiClient.get<UserPackage[]>('/admin/packages/user-packages', { params: { user_id: userId } })
  return data
}

export async function unfreezeUserPackage(id: number): Promise<UserPackage> {
  const { data } = await apiClient.post<UserPackage>(`/admin/packages/user-packages/${id}/unfreeze`)
  return data
}

export async function voidUserPackage(id: number): Promise<UserPackage> {
  const { data } = await apiClient.post<UserPackage>(`/admin/packages/user-packages/${id}/void`)
  return data
}

export const packagesAdminAPI = {
  listPlans,
  savePlan,
  deletePlan,
  getSettings,
  updateSettings,
  listHolidays,
  addHoliday,
  deleteHoliday,
  syncHolidays,
  listUserPackages,
  unfreezeUserPackage,
  voidUserPackage,
}

export default packagesAdminAPI
