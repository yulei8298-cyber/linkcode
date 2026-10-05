/**
 * 套餐（周卡 / 月卡）用户端接口。类型与后端 service.PackagePlan / UserPackageView /
 * PackageMine / PackageCalendarDay 一一对应。下单沿用 paymentAPI（order_type=package）。
 */

import { apiClient } from './client'

export type PackageCycle = 'week' | 'month'
export type PackageTier = 1 | 2
export type PackageStatus = 'active' | 'frozen' | 'exhausted' | 'expired' | 'voided'
export type PackageCalendarKind = 'weekend' | 'holiday' | 'makeup' | 'none'

export interface PackagePlan {
  id: number
  group_id: number
  name: string
  cycle: PackageCycle
  tier: PackageTier
  price: number
  quota_usd: number
  validity_days: number
  for_sale: boolean
  created_at?: string
  updated_at?: string
}

export interface PackageShopGroup {
  group_id: number
  group_name: string
  rate_multiplier: number
  plans: PackagePlan[]
}

export interface PackageNotice {
  text: string
  version: number
}

export interface PackageShop {
  groups: PackageShopGroup[]
  notice: PackageNotice
  freeze_enabled: boolean
  max_freeze_days: number
  package_concurrency: number
}

export interface UserPackage {
  id: number
  user_id: number
  group_id: number
  plan_id: number
  order_id?: number
  name: string
  cycle: PackageCycle
  tier: PackageTier
  quota_usd: number
  used_usd: number
  starts_at: string
  expires_at: string
  status: PackageStatus
  frozen_at?: string
  frozen_seconds_total: number
  created_at: string
  updated_at: string
}

export interface UserPackageView extends UserPackage {
  group_name: string
  remaining_usd: number
  frozen_seconds: number
  freeze_left_seconds: number
  /** 同分组内的扣费顺序，冻结或已结束为 0 */
  deduct_order: number
}

export interface PackageCalendarDay {
  date: string
  weekday: number
  freezable: boolean
  kind: PackageCalendarKind
  label: string
}

export interface PackageMine {
  active: UserPackageView[]
  ended: UserPackageView[]
  package_concurrency: number
  freeze_enabled: boolean
  max_freeze_days: number
  today: PackageCalendarDay
  next_freezable?: PackageCalendarDay
}

export async function getShop(): Promise<PackageShop> {
  const { data } = await apiClient.get<PackageShop>('/packages/shop')
  return data
}

export async function getMine(): Promise<PackageMine> {
  const { data } = await apiClient.get<PackageMine>('/packages/mine')
  return data
}

/** month 形如 2026-10 */
export async function getCalendar(month: string): Promise<PackageCalendarDay[]> {
  const { data } = await apiClient.get<PackageCalendarDay[]>('/packages/calendar', { params: { month } })
  return data
}

export async function freezePackage(id: number): Promise<UserPackage> {
  const { data } = await apiClient.post<UserPackage>(`/packages/${id}/freeze`)
  return data
}

export async function unfreezePackage(id: number): Promise<UserPackage> {
  const { data } = await apiClient.post<UserPackage>(`/packages/${id}/unfreeze`)
  return data
}

export const packagesAPI = { getShop, getMine, getCalendar, freezePackage, unfreezePackage }

export default packagesAPI
