<template>
  <section class="hm-sec">
    <div class="lc-wrap hq">
      <div class="hm-head">
        <p class="hm-kicker">常见问题</p>
        <h2>接入前你可能想知道</h2>
      </div>
      <div class="hq-list">
        <details v-for="(item, i) in items" :key="item.q" class="hm-card" :open="i === 0">
          <summary>{{ item.q }}</summary>
          <p>{{ item.a }}</p>
        </details>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
const items = [
  {
    q: '支持哪些客户端？',
    a: 'Claude Code、Codex CLI、OpenCode，以及 Cherry Studio、Cursor、Cline 等能自定义接口地址的工具。自己写的程序直接用 OpenAI 或 Anthropic 官方 SDK。',
  },
  {
    q: '分组和倍率是什么意思？',
    a: '同一个模型背后有不同的上游号池，每个号池是一个分组，各有倍率。实际单价是官方价乘以倍率。创建密钥时选分组，之后也能改。',
  },
  {
    q: 'Codex 连不上怎么办？',
    a: 'Codex 需要单独新建密钥并选择 GPT 分组。对话站福利分组和 Link AI GPT 密钥不能在 Codex 里用。',
  },
  {
    q: '请求返回 401？',
    a: '先确认密钥复制完整。OpenAI 协议的请求头用 Authorization: Bearer，Claude 协议用 x-api-key。',
  },
  {
    q: '余额会过期吗？',
    a: '不会，用多少扣多少。',
  },
]
</script>

<style scoped>
.hq { max-width: 860px; }
.hq-list { display: grid; gap: 10px; }
summary { display: flex; align-items: center; justify-content: space-between; gap: 16px; padding: 18px 22px; cursor: pointer; list-style: none; font-size: 16px; font-weight: 500; }
summary::-webkit-details-marker { display: none; }
summary::after { content: ''; flex: none; width: 8px; height: 8px; margin-top: -4px; border-right: 1.5px solid var(--lc-ink-3); border-bottom: 1.5px solid var(--lc-ink-3); transform: rotate(45deg); transition: transform .16s; }
details[open] summary::after { margin-top: 4px; transform: rotate(-135deg); }
details p { padding: 0 22px 20px; color: var(--lc-ink-2); font-size: 15px; line-height: 1.75; }
</style>
