/**
 * 套餐（周卡 / 月卡）管理端接口：套餐配置、全局设置、可冻结日期、用户套餐。
 * 用户的套餐并发在「编辑用户」里修改（package_concurrency 字段）。
 */

import { apiClient } from '../client'
import type { PackageCycle, PackagePlan, PackageTier, UserPackage } from '../packages'
import type { EnterpriseMode, EnterpriseStatus } from '../enterprise'

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

/** 管理端「用户套餐」列表项：套餐本身加上用户、分组名与实付金额。 */
export interface AdminUserPackage extends UserPackage {
  user_email: string
  username: string
  group_name: string
  /** 实付金额（元）；管理员手工发放、没有订单时为 0 */
  paid_amount: number
  remaining_usd: number
  frozen_seconds: number
  /** 这张套餐（按周卡 / 月卡）的累计冻结上限（天） */
  max_freeze_days: number
}

export interface AdminUserPackageParams {
  /** 用户邮箱、用户名；纯数字时同时匹配用户 ID */
  keyword?: string
  status?: string
  cycle?: string
  group_id?: number
  page?: number
  page_size?: number
}

export interface AdminUserPackagePage {
  items: AdminUserPackage[]
  total: number
  page: number
  page_size: number
  pages: number
}

export interface AdminUserPackageStats {
  total: number
  active: number
  frozen: number
  exhausted: number
  expired: number
  voided: number
  /** 生效中（active + frozen）套餐的总额度与已用额度 */
  live_quota_usd: number
  live_used_usd: number
  recent_sold: number
  recent_revenue: number
  window_days: number
}

export async function listUserPackages(params: AdminUserPackageParams): Promise<AdminUserPackagePage> {
  const { data } = await apiClient.get<AdminUserPackagePage>('/admin/packages/user-packages', { params })
  return data
}

/** 某分组的企业倍率：企业尊享用户按量使用时取它与分组 / 个人专属倍率中更低的，套餐不受影响。 */
export interface EnterpriseGroupRate {
  group_id: number
  multiplier: number
}

/** 企业尊享全局设置：总开关、自动获得的累计消费门槛、各分组企业倍率。 */
export interface EnterpriseSettings {
  enabled: boolean
  threshold: number
  group_rates: EnterpriseGroupRate[]
}

export async function getEnterpriseSettings(): Promise<EnterpriseSettings> {
  const { data } = await apiClient.get<EnterpriseSettings>('/admin/packages/enterprise/settings')
  return data
}

export async function updateEnterpriseSettings(settings: EnterpriseSettings): Promise<EnterpriseSettings> {
  const { data } = await apiClient.put<EnterpriseSettings>('/admin/packages/enterprise/settings', settings)
  return data
}

export async function getUserEnterprise(userId: number): Promise<EnterpriseStatus> {
  const { data } = await apiClient.get<EnterpriseStatus>(`/admin/packages/enterprise/users/${userId}`)
  return data
}

export async function setUserEnterprise(userId: number, mode: EnterpriseMode): Promise<EnterpriseStatus> {
  const { data } = await apiClient.put<EnterpriseStatus>(`/admin/packages/enterprise/users/${userId}`, { mode })
  return data
}

export async function getUserPackageStats(): Promise<AdminUserPackageStats> {
  const { data } = await apiClient.get<AdminUserPackageStats>('/admin/packages/user-packages/stats')
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
  getUserPackageStats,
  getEnterpriseSettings,
  updateEnterpriseSettings,
  getUserEnterprise,
  setUserEnterprise,
  unfreezeUserPackage,
  voidUserPackage,
}

export default packagesAdminAPI
