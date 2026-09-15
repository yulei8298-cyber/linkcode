<template>
  <div class="ic-test">
    <div class="ic-thead">
      <b>
        <span class="ic-kind-ico">{{ kind === 'drawing' ? '🎨' : '◻' }}</span>
        {{ label }}
      </b>
      <span v-if="latest" class="ic-last" :class="{ f: latest.status === 'fail', u: latest.status === 'unverified' }">
        ● <b>{{ statusLabel(latest.status) }}</b> · {{ formatRelative(latest.checked_at) }}
      </span>
    </div>

    <div class="ic-stats">
      <span class="ic-big" :class="{ f: lowPassRate }">
        {{ formatPassRate(stats?.pass_rate, stats?.has_data) }}
      </span>
      <span v-if="stats?.has_data">
        {{ stats.pass }}/{{ stats.pass + stats.fail }} {{ kind === 'drawing' ? '画出' : '答对' }}
      </span>
      <span v-if="stats?.error" class="ic-warn">{{ stats.error }} 次请求失败</span>
      <span v-if="stats?.unverified" class="ic-unverified-text">{{ stats.unverified }} 次未验证</span>
      <span v-if="avgLatency != null">平均 {{ formatLatency(avgLatency) }}</span>
    </div>

    <div class="ic-tl" :style="{ gridTemplateColumns: `repeat(${size}, 1fr)` }" role="list">
      <i
        v-for="(point, index) in cells"
        :key="point ? point.result_id : `empty-${index}`"
        role="listitem"
        :class="[statusClass(point?.status), { clickable: !!point }]"
        :title="timelineTitle(point)"
        @click="point && emit('select', point.result_id)"
      ></i>
    </div>

    <div class="ic-tlax">
      <span>{{ startLabel || '—' }}</span>
      <span>现在</span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { IntelCheckKind, IntelCheckStats, IntelCheckTimelinePoint } from '@/api/intelCheck'
import {
  averageLatency,
  formatLatency,
  formatPassRate,
  formatRelative,
  latestPoint,
  padTimeline,
  statusClass,
  statusLabel,
  timelineStart,
  timelineTitle,
} from './intelCheckFormat'

const props = defineProps<{
  label: string
  kind: IntelCheckKind
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
const latest = computed(() => latestPoint(props.points))
const avgLatency = computed(() => averageLatency(props.points))
const startLabel = computed(() => timelineStart(props.points))

// 低于 60% 时大数字转红：这条线是「一眼看出这组不对劲」的主要依据，
// 光靠色块密度读者要数半天。
const lowPassRate = computed(
  () => !!props.stats?.has_data && (props.stats.pass_rate ?? 1) < 0.6,
)
</script>

<style scoped>
.ic-test + .ic-test {
  margin-top: 22px;
}

.ic-thead {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  font-size: 14px;
  margin-bottom: 8px;
}

.ic-thead > b {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  font-weight: 700;
  color: var(--ic-text);
}

.ic-kind-ico {
  font-size: 15px;
}

.ic-last {
  font-size: 13px;
  color: var(--ic-muted);
  white-space: nowrap;
}

.ic-last b {
  color: var(--ic-ok);
  font-weight: 600;
}

.ic-last.f b {
  color: var(--ic-fail);
}

.ic-last.u b {
  color: var(--ic-unverified);
}

.ic-stats {
  display: flex;
  align-items: baseline;
  flex-wrap: wrap;
  gap: 14px;
  margin-bottom: 8px;
  font-size: 13px;
  color: var(--ic-muted);
}

.ic-big {
  font-size: 26px;
  font-weight: 900;
  color: var(--ic-accent);
  font-variant-numeric: tabular-nums;
  line-height: 1.1;
}

.ic-big.f {
  color: var(--ic-fail);
}

.ic-warn {
  color: var(--ic-warn-text);
}

.ic-unverified-text {
  color: var(--ic-unverified);
}

/* 等分网格而非 flex：格数由管理员配置（12–200），网格能保证任意格数下
   总宽度恒等于容器宽度，各分组的色块因此严格上下对齐——
   「同题同刻横向对比」靠的就是这个对齐。 */
.ic-tl {
  display: grid;
  gap: 4px;
  height: 38px;
}

.ic-tl i {
  border-radius: 5px;
  background: var(--ic-ok);
  transition: transform 0.1s ease;
}

.ic-tl i.clickable {
  cursor: pointer;
}

.ic-tl i.clickable:hover {
  transform: scaleY(1.15);
}

.ic-tl i.bad {
  background: var(--ic-fail);
}

/* 请求失败用琥珀色，与红色的「未通过」严格区分：
   前者是我们这侧的链路故障，后者才是对受检模型的指控。 */
.ic-tl i.degraded {
  background: var(--ic-req);
}

.ic-tl i.running {
  background: var(--ic-run);
  animation: ic-pulse 1.4s ease-in-out infinite;
}

.ic-tl i.unverified {
  background: var(--ic-unverified);
}

.ic-tl i.unknown {
  background: var(--ic-none);
}

@keyframes ic-pulse {
  50% {
    opacity: 0.5;
  }
}

@media (prefers-reduced-motion: reduce) {
  .ic-tl i.running {
    animation: none;
  }
  .ic-tl i.clickable:hover {
    transform: none;
  }
}

.ic-tlax {
  display: flex;
  justify-content: space-between;
  margin-top: 6px;
  font-family: ui-monospace, Menlo, Consolas, monospace;
  font-size: 11px;
  color: var(--ic-faint);
}
</style>
