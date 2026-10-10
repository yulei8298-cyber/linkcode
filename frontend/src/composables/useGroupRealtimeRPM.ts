import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useIntervalFn } from '@vueuse/core'
import { adminAPI } from '@/api/admin'

/** 分组实时 RPM 的刷新间隔：后端统计窗口是 60 秒滚动，5 秒一刷既实时又不给数据库添负担。 */
export const GROUP_REALTIME_RPM_REFRESH_MS = 5000

interface Options {
  /** 返回 false 时不请求（例如简易模式、该列被隐藏）。 */
  enabled?: () => boolean
}

/**
 * 管理端分组页的实时 RPM：挂载后立即加载并按固定间隔刷新。
 * 标签页在后台时暂停，切回前台立即补刷一次；上一次请求没回来时不叠加新请求。
 */
export function useGroupRealtimeRPM(options: Options = {}) {
  const rpmByGroup = ref<Map<number, number>>(new Map())
  const loaded = ref(false)
  const totalRpm = ref(0)
  let inflight = false

  const rpmOf = (groupId: number): number => rpmByGroup.value.get(groupId) ?? 0

  async function refresh(): Promise<void> {
    if (inflight || document.hidden) return
    if (options.enabled && !options.enabled()) return
    inflight = true
    try {
      const summary = await adminAPI.groups.getRealtimeRPM()
      rpmByGroup.value = new Map(summary.items.map((item) => [item.group_id, item.rpm]))
      totalRpm.value = summary.total
      loaded.value = true
    } catch (error) {
      console.error('Error loading group realtime RPM:', error)
    } finally {
      inflight = false
    }
  }

  const { resume } = useIntervalFn(() => void refresh(), GROUP_REALTIME_RPM_REFRESH_MS, { immediate: false })

  const onVisibilityChange = () => {
    if (!document.hidden) void refresh()
  }

  onMounted(() => {
    void refresh()
    resume()
    document.addEventListener('visibilitychange', onVisibilityChange)
  })
  onUnmounted(() => document.removeEventListener('visibilitychange', onVisibilityChange))

  return { rpmByGroup, loaded: computed(() => loaded.value), totalRpm: computed(() => totalRpm.value), rpmOf, refresh }
}
