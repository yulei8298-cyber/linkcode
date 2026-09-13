<template>
  <div class="lc-ic-timeline-row">
    <div class="lc-ic-timeline-head">
      <small>{{ label }}</small>
      <b>{{ passRate }}</b>
    </div>
    <div class="lc-ic-timeline" role="list" :aria-label="`${label}检测历史`">
      <button
        v-for="(point, index) in cells"
        :key="point ? point.result_id : `empty-${index}`"
        type="button"
        role="listitem"
        class="lc-ic-cell"
        :class="statusClass(point?.status)"
        :disabled="!point"
        :title="timelineTitle(point)"
        :aria-label="timelineTitle(point)"
        @click="point && emit('select', point.result_id)"
      ></button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { IntelCheckStats, IntelCheckTimelinePoint } from '@/api/intelCheck'
import { formatPassRate, padTimeline, statusClass, timelineTitle } from './intelCheckFormat'

const props = defineProps<{
  label: string
  points: IntelCheckTimelinePoint[]
  /** 固定格数，由后端 timeline_points 决定（默认 48）。 */
  size: number
  stats: IntelCheckStats
}>()

const emit = defineEmits<{
  (e: 'select', resultId: number): void
}>()

// 补齐到固定格数，缺的填在左侧（较旧的一端），保证各分组最右一格是同一时刻。
const cells = computed(() => padTimeline(props.points, props.size))
const passRate = computed(() => formatPassRate(props.stats?.pass_rate, props.stats?.has_data))
</script>

<style scoped>
.lc-ic-timeline-row + .lc-ic-timeline-row {
  margin-top: 14px;
}

.lc-ic-timeline-head {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  margin-bottom: 6px;
}

.lc-ic-timeline-head small {
  font-size: 12px;
  opacity: 0.72;
}

.lc-ic-timeline-head b {
  font-size: 13px;
  font-variant-numeric: tabular-nums;
}

.lc-ic-timeline {
  display: flex;
  gap: 2px;
  align-items: stretch;
}

.lc-ic-cell {
  flex: 1 1 0;
  /* 下限 3px：格子数由管理员配置（最多 200），窄屏上不设下限会压成看不见的细线 */
  min-width: 3px;
  height: 26px;
  border: 0;
  border-radius: 2px;
  padding: 0;
  cursor: pointer;
  background: #22c55e;
  transition: transform 0.12s ease, filter 0.12s ease;
}

.lc-ic-cell:hover:not(:disabled),
.lc-ic-cell:focus-visible {
  transform: scaleY(1.18);
  filter: brightness(1.12);
  outline: none;
}

.lc-ic-cell.bad {
  background: #ef4444;
}

/* 请求失败用黄色，与红色的「未通过」严格区分：
   前者是我们这侧的链路问题，后者才是对受检模型的指控。 */
.lc-ic-cell.degraded {
  background: #f59e0b;
}

.lc-ic-cell.running {
  background: #7dd3fc;
  animation: lc-ic-pulse 1.4s ease-in-out infinite;
}

.lc-ic-cell.unknown {
  background: rgba(148, 163, 184, 0.32);
}

/* 无数据的占位格不可点，也不做悬浮反馈——它背后没有任何明细可看 */
.lc-ic-cell:disabled {
  cursor: default;
}

@keyframes lc-ic-pulse {
  0%,
  100% {
    opacity: 1;
  }
  50% {
    opacity: 0.45;
  }
}

/* 尊重系统的减少动效偏好：这个脉冲是装饰性的，不承载信息 */
@media (prefers-reduced-motion: reduce) {
  .lc-ic-cell.running {
    animation: none;
  }
  .lc-ic-cell:hover:not(:disabled),
  .lc-ic-cell:focus-visible {
    transform: none;
  }
}
</style>
