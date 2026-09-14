<template>
  <article class="ic-card ic-grp" :class="{ bad: isDegraded }">
    <div class="ic-ghead">
      <div class="ic-gname">
        {{ group.name }}
        <!-- 模型名与推理等级是有意公开的：本页要证明的正是「跑的是满血配置」，
             把它们藏起来，页面就失去了可验证性。 -->
        <span class="ic-chip model">{{ group.model }}</span>
        <span class="ic-chip">推理 {{ (group.reasoning_effort || '').toUpperCase() }}</span>
        <span v-if="group.rate_label" class="ic-chip">{{ group.rate_label }}</span>
      </div>
      <span class="ic-pill" :class="pillClass">
        <span class="ic-dot"></span>{{ stateLabel(group.state) }}
      </span>
    </div>

    <div v-if="group.description" class="ic-gdesc">{{ group.description }}</div>

    <div class="ic-gbody">
      <div class="ic-tests">
        <IntelCheckTimeline
          label="逻辑题测试"
          kind="logic"
          :points="group.logic_timeline"
          :size="timelinePoints"
          :stats="group.logic_stats_24h"
          @select="(id) => emit('select', id)"
        />
        <IntelCheckTimeline
          label="绘图测试"
          kind="drawing"
          :points="group.drawing_timeline"
          :size="timelinePoints"
          :stats="group.drawing_stats_24h"
          @select="(id) => emit('select', id)"
        />
      </div>

      <!-- 最新画作。判失败的画作同样展示——让人看到降智时画成什么样，
           恰恰是这张页面最有说服力的部分。 -->
      <div class="ic-art">
        <div class="ic-ah">
          <b>最新画作{{ artworkTitle ? ` · ${artworkTitle}` : '' }}</b>
          <span v-if="artworkMeta">{{ artworkMeta }}</span>
        </div>
        <div class="ic-frame">
          <SvgArtworkPreview
            :html="artworkHtml"
            :height="240"
            :empty-text="artworkEmptyText"
            :title="`${group.name} 的最新画作`"
          />
          <button
            v-if="group.latest_drawing_result_id > 0"
            type="button"
            class="ic-artbtn"
            @click="emit('select', group.latest_drawing_result_id)"
          >
            ▶ 播放动画 · 查看源码
          </button>
        </div>
        <div class="ic-tip">
          <template v-if="isDegraded">
            连续 {{ rule.fail_streak }} 次逻辑题未通过即判定「疑似降智」，恢复
            {{ rule.recover_streak }} 次通过后自动转回正常。
          </template>
          <template v-else>点击任意色块可查看该次检测：题目、判定结果与模型回复。</template>
        </div>
      </div>
    </div>
  </article>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import IntelCheckTimeline from './IntelCheckTimeline.vue'
import SvgArtworkPreview from './SvgArtworkPreview.vue'
import {
  getIntelCheckResult,
  type IntelCheckDegradedRule,
  type IntelCheckGroup,
} from '@/api/intelCheck'
import { formatDateTime, formatLatency, stateLabel } from './intelCheckFormat'

const props = defineProps<{
  group: IntelCheckGroup
  /** 时间线格数，由后端 timeline_points 决定。 */
  timelinePoints: number
  /** 降智判定规则，用于卡片底部那句说明。 */
  rule: IntelCheckDegradedRule
}>()

const emit = defineEmits<{
  (e: 'select', resultId: number): void
}>()

const isDegraded = computed(() => props.group.state === 'degraded')
const pillClass = computed(() => {
  if (props.group.state === 'normal') return 'ok'
  return props.group.state === 'degraded' ? 'bad' : 'unknown'
})

const artworkHtml = ref('')
const artworkTitle = ref('')
const artworkMeta = ref('')
const artworkLoading = ref(false)
// 记住已取过的 id：概览每 30 秒轮询一次，最新画作往往几十分钟才换一张，
// 不去重就会每轮把几百 KB 的产物重新拉一遍 × 分组数。
let loadedResultId = 0
let abortController: AbortController | null = null

const artworkEmptyText = computed(() => {
  if (artworkLoading.value) return '加载中…'
  return props.group.latest_drawing_result_id > 0 ? '画作加载失败' : '暂无画作'
})

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
      artworkTitle.value = result.question_title || ''
      const time = formatDateTime(result.checked_at).slice(-8)
      const latency = result.latency_ms != null ? ` · ${formatLatency(result.latency_ms)}` : ''
      artworkMeta.value = `${time}${latency}`
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
.ic-grp {
  padding: 24px 28px;
  margin-bottom: 18px;
}

/* 疑似降智的卡片整体变红：这是本页唯一需要被一眼看到的状态，
   只挂一个小角标会被 48 格色块的视觉重量盖过去。 */
.ic-grp.bad {
  outline: 1.5px solid var(--ic-bad-outline);
  background: var(--ic-bad-surface);
}

.ic-ghead {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 6px;
}

.ic-gname {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 12px;
  font-size: 17px;
  font-weight: 800;
  color: var(--ic-accent-strong);
  min-width: 0;
}

.ic-grp.bad .ic-gname {
  color: var(--ic-warn-text);
}

.ic-chip {
  font-family: ui-monospace, Menlo, Consolas, monospace;
  font-size: 12px;
  font-weight: 500;
  padding: 3px 8px;
  border-radius: 8px;
  background: var(--ic-chip-bg);
  color: var(--ic-chip-text);
}

.ic-chip.model {
  font-size: 14px;
  font-weight: 600;
  background: transparent;
  color: var(--ic-text);
  padding-left: 0;
  padding-right: 0;
}

.ic-pill {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  flex-shrink: 0;
  font-size: 12px;
  font-weight: 700;
  padding: 6px 12px;
  border-radius: 99px;
}

.ic-pill.ok {
  background: var(--ic-ok-bg);
  color: var(--ic-ok-text);
  outline: 1px solid var(--ic-ok-line);
}

.ic-pill.bad {
  background: var(--ic-fail-bg);
  color: var(--ic-fail-text);
  outline: 1px solid var(--ic-fail-line);
}

.ic-pill.unknown {
  background: var(--ic-chip-bg);
  color: var(--ic-muted);
}

.ic-pill .ic-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: currentColor;
}

.ic-gdesc {
  font-size: 13px;
  color: var(--ic-muted);
  margin-bottom: 18px;
}

.ic-gbody {
  display: grid;
  gap: 28px;
}

/* 画作固定 440px、时间线占剩余空间：窄屏下改为上下堆叠，
   时间线在上——色块是主证据，画作是佐证。 */
@media (min-width: 1100px) {
  .ic-gbody {
    grid-template-columns: minmax(0, 1fr) 440px;
    align-items: start;
  }
}

.ic-tests {
  min-width: 0;
}

.ic-ah {
  display: flex;
  justify-content: space-between;
  align-items: baseline;
  gap: 10px;
  margin-bottom: 10px;
  font-size: 13px;
  color: var(--ic-text-soft);
}

.ic-ah b {
  font-weight: 700;
}

.ic-ah span {
  color: var(--ic-muted);
  font-family: ui-monospace, Menlo, Consolas, monospace;
  font-size: 12px;
  white-space: nowrap;
}

.ic-frame {
  position: relative;
  border-radius: 16px;
  outline: 1px solid var(--ic-line);
  overflow: hidden;
}

.ic-artbtn {
  position: absolute;
  right: 10px;
  bottom: 10px;
  font-size: 12px;
  padding: 5px 10px;
  border: 0;
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.9);
  color: #334155;
  outline: 1px solid rgba(148, 163, 184, 0.4);
  cursor: pointer;
}

.ic-artbtn:hover {
  background: #fff;
}

.ic-tip {
  margin-top: 8px;
  font-size: 12px;
  line-height: 1.6;
  color: var(--ic-faint);
}
</style>
