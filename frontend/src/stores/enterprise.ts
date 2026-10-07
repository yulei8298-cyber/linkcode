import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import { getEnterpriseStatus, type EnterpriseStatus } from '@/api/enterprise'

/**
 * 企业尊享状态。登录态恢复（刷新页面、重新打开网站）或登录后拉取一次；
 * 状态只存内存，所以「每次打开网站提示一次」自然成立：刷新即重置，同一次访问内切换页面不重复提示。
 * 标识与提示只在控制台展示，首页、门户页不展示。
 */
export const useEnterpriseStore = defineStore('enterprise', () => {
  const status = ref<EnterpriseStatus | null>(null)
  // 欢迎提示是否还没展示过：拉到企业用户状态时置为 true，在控制台弹出后置为 false。
  const welcomePending = ref(false)
  let generation = 0

  const isEnterprise = computed(() => status.value?.enterprise === true)

  async function fetch(): Promise<void> {
    const current = ++generation
    try {
      const result = await getEnterpriseStatus()
      if (current !== generation) return // 期间已登出或重新拉取，丢弃过期结果
      status.value = result
      welcomePending.value = result.enterprise
    } catch (error) {
      // 企业尊享只是锦上添花，失败不打扰用户
      console.error('Failed to fetch enterprise status:', error)
    }
  }

  function dismissWelcome(): void {
    welcomePending.value = false
  }

  /** 登出时清空，下次登录会重新提示。 */
  function reset(): void {
    generation++
    status.value = null
    welcomePending.value = false
  }

  return { status, welcomePending, isEnterprise, fetch, dismissWelcome, reset }
})
