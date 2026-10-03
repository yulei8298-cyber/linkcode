<template>
  <section class="ht">
    <div class="lc-wrap ht-inner">
      <p class="ht-cap">兼容你已经在用的工具</p>
      <div class="ht-grid">
        <div v-for="tool in tools" :key="tool.name" class="ht-cell">
          <span class="hm-pm ht-icon" :class="`is-${tool.platform}`">
            <PlatformIcon v-if="tool.platform" :platform="tool.platform" size="md" />
            <template v-else>{{ tool.mark }}</template>
          </span>
          <b>{{ tool.name }}</b>
          <span>{{ tool.note }}</span>
        </div>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import PlatformIcon from '@/components/common/PlatformIcon.vue'
import type { GroupPlatform } from '@/types'

// 只列能自定义接口地址、确实可以接入的客户端；没有官方图标的用缩写占位
const tools: { name: string; note: string; platform: GroupPlatform | ''; mark?: string }[] = [
  { name: 'Claude Code', note: 'Anthropic 协议', platform: 'anthropic' },
  { name: 'Codex CLI', note: 'Responses 协议', platform: 'openai' },
  { name: 'OpenCode', note: 'OpenAI 协议', platform: 'opencode_go' as GroupPlatform },
  { name: 'Cherry Studio', note: 'OpenAI 协议', platform: '', mark: 'CS' },
  { name: 'Cursor', note: '自定义接口地址', platform: '', mark: 'Cu' },
  { name: 'Cline', note: 'OpenAI / Anthropic', platform: '', mark: 'Cl' },
  { name: 'OpenAI SDK', note: 'Python · Node', platform: 'openai' },
  { name: 'Anthropic SDK', note: 'Python · Node', platform: 'anthropic' },
]
</script>

<style scoped>
.ht { border-block: 1px solid var(--lc-line); background: var(--lc-bg-2); }
.ht-inner { padding: 40px 0 56px; }
.ht-cap { margin-bottom: 24px; text-align: center; color: var(--lc-ink-3); font-size: 13px; }
.ht-grid { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); border-top: 1px solid var(--lc-line); border-left: 1px solid var(--lc-line); }
.ht-cell { display: grid; justify-items: center; gap: 6px; padding: 26px 12px; border-right: 1px solid var(--lc-line); border-bottom: 1px solid var(--lc-line); text-align: center; transition: background-color .16s; }
.ht-cell:hover { background: var(--lc-surface); }
.ht-icon { width: 36px; height: 36px; margin-bottom: 4px; border-radius: 9px; font-size: 11px; }
.ht-cell b { font-size: 14px; font-weight: 600; }
.ht-cell > span:last-child { color: var(--lc-ink-3); font-size: 12px; }

@media (max-width: 760px) {
  .ht-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); }
}
</style>
