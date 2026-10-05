/**
 * 支付页的套餐购买上下文：从套餐商店带来的 package_plan / notice_version 解析出
 * 套餐与分组信息，供确认面板展示与下单使用。下单与支付仍走支付页原有流程。
 */

import { ref } from 'vue'
import packagesAPI, { type PackagePlan } from '@/api/packages'
import { extractApiErrorMessage } from '@/utils/apiError'

export interface PackagePurchaseState {
  plan: PackagePlan
  groupName: string
  rateMultiplier: number
  noticeVersion: number
}

export function usePackagePurchase(messages: { notFound: string }) {
  const purchase = ref<PackagePurchaseState | null>(null)
  const error = ref('')

  /** 读取套餐信息；noticeVersion 为 0 时由后端以「须知已更新」拒绝下单。 */
  async function load(planId: number, noticeVersion: number): Promise<boolean> {
    error.value = ''
    try {
      const shop = await packagesAPI.getShop()
      for (const group of shop.groups) {
        const plan = group.plans.find((p) => p.id === planId)
        if (plan) {
          purchase.value = { plan, groupName: group.group_name, rateMultiplier: group.rate_multiplier, noticeVersion }
          return true
        }
      }
      error.value = messages.notFound
    } catch (err: unknown) {
      error.value = extractApiErrorMessage(err, messages.notFound)
    }
    purchase.value = null
    return false
  }

  function clear() {
    purchase.value = null
    error.value = ''
  }

  return { purchase, error, load, clear }
}

/** 解析路由里的正整数参数，非法时返回 0。 */
export function positiveIntQuery(value: unknown): number {
  const raw = Array.isArray(value) ? value[0] : value
  const n = Number.parseInt(typeof raw === 'string' ? raw : '', 10)
  return Number.isFinite(n) && n > 0 ? n : 0
}
