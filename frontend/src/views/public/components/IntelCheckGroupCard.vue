<template>
  <article class="lc-card lc-ic-card">
    <header class="lc-ic-card-head">
      <div class="lc-ic-card-title">
        <b>{{ group.name }}</b>
        <p v-if="group.description" class="lc-ic-card-desc">{{ group.description }}</p>
      </div>
      <span class="lc-status" :class="stateClass(group.state)">{{ stateLabel(group.state) }}</span>
    </header>

    <!-- 模型名与推理等级是有意公开的：本页要证明的正是「跑的是满血配置」，
         把它们藏起来，页面就失去了可验证性。 -->
    <div class="lc-ic-chips">
      <span class="lc-ic-chip">{{ group.model }}</span>
      <span class="lc-ic-chip">推理 {{ group.reasoning_effort }}</span>
      <span v-if="group.rate_label" class="lc-ic-chip lc-ic-chip-rate">{{ group.rate_label }}</span>
    </div>

    <div class="lc-ic-body">
      <div class="lc-ic-timelines">
        <IntelCheckTimeline
          label="逻辑题"
          :points="group.logic_timeline"
          :size="timelinePoints"
          :stats="group.logic_stats_24h"
          @select="(id) => emit('select', id)"
        />
        <IntelCheckTimeline
          label="绘图题"
          :points="group.drawing_timeline"
          :size="timelinePoints"
          :stats="group.drawing_stats_24h"
          @select="(id) => emit('select', id)"
        />
      </div>

      <!-- 最新画作。判失败的画作同样展示——让人看到降智时画成什么样，
           恰恰是这张页面最有说服力的部分。 -->
      <div v-if="group.latest_drawing_result_id > 0" class="lc-ic-artwork-slot">
        <div class="lc-ic-artwork-head">
          <small>最新画作</small>
          <button type="button" class="lc-ic-link" @click="emit('select', group.latest_drawing_result_id)">
            查看详情
          </button>
        </div>
        <SvgArtworkPreview
          :html="artworkHtml"
          :height="180"
          :empty-text="artworkLoading ? '加载中…' : '暂无画作'"
          :title="`${group.name} 的最新画作`"
        />
      </div>
    </div>
  </article>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import IntelCheckTimeline from './IntelCheckTimeline.vue'
import SvgArtworkPreview from './SvgArtworkPreview.vue'
import { getIntelCheckResult, type IntelCheckGroup } from '@/api/intelCheck'
import { stateClass, stateLabel } from './intelCheckFormat'

const props = defineProps<{
  group: IntelCheckGroup
  /** 时间线格数，由后端 timeline_points 决定。 */
  timelinePoints: number
}>()

const emit = defineEmits<{
  (e: 'select', resultId: number): void
}>()

const artworkHtml = ref('')
const artworkLoading = ref(false)
// 记住已取过的 id：概览每 30 秒轮询一次，最新画作往往几十分钟才换一张，
// 不去重就会每轮把几百 KB 的产物重新拉一遍。
let loadedResultId = 0
let abortController: AbortController | null = null

async function loadArtwork(resultId: number) {
  if (resultId <= 0 || resultId === loadedResultId) return

  abortController?.abort()
  const controller = new AbortController()
  abortController = controller
  artworkLoading.value = true
  try {
    const result = await getIntelCheckResult(resultId, { signal: controller.signal })
    if (!controller.signal.aborted) {
      artworkHtml.value = result.html_output || ''
      loadedResultId = resultId
    }
  } catch {
    // 画作是锦上添花：取不到就留空位，不打断整页渲染，也不弹错误提示。
    if (!controller.signal.aborted) artworkHtml.value = ''
  } finally {
    if (abortController === controller) {
      artworkLoading.value = false
      abortController = null
    }
  }
}

watch(
  () => props.group.latest_drawing_result_id,
  (id) => void loadArtwork(id),
  { immediate: true },
)
</script>

<style scoped>
.lc-ic-card {
  display: flex;
  flex-direction: column;
  gap: 14px;
  padding: 18px;
}

.lc-ic-card-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
}

.lc-ic-card-title b {
  font-size: 16px;
}

.lc-ic-card-desc {
  margin: 4px 0 0;
  font-size: 12px;
  line-height: 1.5;
  opacity: 0.7;
}

.lc-ic-chips {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.lc-ic-chip {
  border-radius: 999px;
  border: 1px solid rgba(148, 163, 184, 0.3);
  padding: 2px 9px;
  font-size: 12px;
  opacity: 0.85;
}

.lc-ic-chip-rate {
  border-color: rgba(20, 184, 166, 0.4);
  color: #0d9488;
}

.lc-ic-body {
  display: grid;
  gap: 16px;
}

/* 宽屏时把最新画作放到右侧，与两条时间线并排 */
@media (min-width: 900px) {
  .lc-ic-body {
    grid-template-columns: minmax(0, 1fr) 260px;
    align-items: start;
  }
}

.lc-ic-timelines {
  min-width: 0;
}

.lc-ic-artwork-head {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  margin-bottom: 6px;
}

.lc-ic-artwork-head small {
  font-size: 12px;
  opacity: 0.72;
}

.lc-ic-link {
  border: 0;
  background: none;
  padding: 0;
  font-size: 12px;
  color: #0d9488;
  cursor: pointer;
}

.lc-ic-link:hover {
  text-decoration: underline;
}
</style>
