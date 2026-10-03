<template>
  <section id="how" class="hm-sec">
    <div class="lc-wrap">
      <div class="hm-head">
        <p class="hm-kicker">接入</p>
        <h2>改一个地址，就能用</h2>
        <p>不用改代码，也不用换工具。把接口地址指向 {{ siteName }}，填上控制台里创建的密钥。</p>
      </div>
      <div class="hc">
        <div class="hc-steps" role="tablist" aria-label="接入说明" @keydown="onStepKey">
          <button
            v-for="(step, i) in steps"
            :id="`hc-tab-${i}`"
            :key="step.title"
            role="tab"
            type="button"
            :aria-selected="active === i"
            :aria-controls="`hc-pane-${i}`"
            :tabindex="active === i ? 0 : -1"
            @click="active = i"
          >
            <b>{{ step.title }}</b>
            <span>{{ step.text }}</span>
          </button>
        </div>

        <div class="hc-stage">
          <div v-show="active === 0" id="hc-pane-0" role="tabpanel" aria-labelledby="hc-tab-0" class="hc-pane">
            <div class="hc-pills" role="tablist" aria-label="配置示例">
              <button
                v-for="snippet in snippets"
                :key="snippet.id"
                role="tab"
                type="button"
                :aria-selected="snippetId === snippet.id"
                @click="snippetId = snippet.id"
              >{{ snippet.label }}</button>
              <button type="button" class="hc-copy" @click="copySnippet">
                <Icon name="copy" size="xs" />复制
              </button>
            </div>
            <pre class="hm-code"><template v-for="(line, li) in currentSnippet.lines" :key="li"><span v-for="(seg, si) in line" :key="si" :class="seg[0]">{{ seg[1] }}</span>
</template></pre>
          </div>

          <div v-show="active === 1" id="hc-pane-1" role="tabpanel" aria-labelledby="hc-tab-1" class="hc-pane">
            <p class="hc-cap">计价方式</p>
            <div class="hc-formula"><span>官方价</span><i>×</i><span>分组倍率</span><i>=</i><b>实际单价</b></div>
            <div class="hc-rows">
              <div><span>官方价（输入）</span><span class="lc-mono">$5.00 / 百万 tokens</span></div>
              <div><span>分组倍率</span><span class="lc-mono">×0.4</span></div>
              <div class="sum"><span>你付的价格</span><span class="lc-mono">$2.00 / 百万 tokens</span></div>
            </div>
            <p class="hc-cap">示例。各分组倍率见 <RouterLink to="/portal/pricing">定价方案</RouterLink>。</p>
          </div>

          <div v-show="active === 2" id="hc-pane-2" role="tabpanel" aria-labelledby="hc-tab-2" class="hc-pane">
            <p class="hc-cap">使用记录 · 示例</p>
            <div class="hc-log">
              <div><span>时间</span><span>模型</span><span class="r">Token</span><span class="r">费用</span></div>
              <div v-for="row in sampleLogs" :key="row[0]">
                <span class="lc-mono">{{ row[0] }}</span><span class="lc-mono">{{ row[1] }}</span>
                <span class="lc-mono r">{{ row[2] }}</span><span class="lc-mono r">{{ row[3] }}</span>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { RouterLink } from 'vue-router'
import Icon from '@/components/icons/Icon.vue'
import { useClipboard } from '@/composables/useClipboard'

const props = defineProps<{ siteName: string; apiBase: string }>()

const steps = [
  { title: '替换接口地址和密钥', text: '控制台「使用密钥」会按你的系统生成整套配置，复制粘贴即可。' },
  { title: '按分组挑价格', text: '同一个模型有不同号池，每个分组标明倍率，倍率低的分组更便宜。' },
  { title: '每次调用都有记录', text: '模型、Token、费用和耗时逐条可查，扣了多少一清二楚。' },
]
const active = ref(0)

function onStepKey(event: KeyboardEvent) {
  const delta = event.key === 'ArrowDown' ? 1 : event.key === 'ArrowUp' ? -1 : 0
  if (!delta) return
  event.preventDefault()
  active.value = (active.value + delta + steps.length) % steps.length
  document.getElementById(`hc-tab-${active.value}`)?.focus()
}

// 代码片段按 [样式类, 文本] 分段，避免 v-html；c=注释 k=关键字 s=字符串
type Segment = [string, string]
const snippets = computed(() => {
  const base = props.apiBase
  return [
    {
      id: 'cc',
      label: 'Claude Code',
      lines: [
        [['c', '# 写进 ~/.zshrc 或 ~/.bashrc，重开终端后运行 claude']],
        [['k', 'export'], ['', ' ANTHROPIC_BASE_URL='], ['s', `"${base}"`]],
        [['k', 'export'], ['', ' ANTHROPIC_API_KEY='], ['s', '"sk-你的密钥"']],
        [],
        [['', 'claude']],
      ] as Segment[][],
    },
    {
      id: 'codex',
      label: 'Codex CLI',
      lines: [
        [['c', '# ~/.codex/config.toml']],
        [['', 'model_provider = '], ['s', '"OpenAI"']],
        [['', 'model = '], ['s', '"gpt-5.5"']],
        [],
        [['', '[model_providers.OpenAI]']],
        [['', 'name = '], ['s', '"OpenAI"']],
        [['', 'base_url = '], ['s', `"${base}"`]],
        [['', 'wire_api = '], ['s', '"responses"']],
        [['', 'requires_openai_auth = '], ['k', 'true']],
        [],
        [['c', '# ~/.codex/auth.json']],
        [['', '{ '], ['s', '"OPENAI_API_KEY"'], ['', ': '], ['s', '"sk-你的密钥"'], ['', ' }']],
      ] as Segment[][],
    },
    {
      id: 'py',
      label: 'Python',
      lines: [
        [['k', 'from'], ['', ' openai '], ['k', 'import'], ['', ' OpenAI']],
        [],
        [['', 'client = OpenAI(']],
        [['', '    base_url='], ['s', `"${base}/v1"`], ['', ',']],
        [['', '    api_key='], ['s', '"sk-你的密钥"'], ['', ',']],
        [['', ')']],
        [['', 'resp = client.chat.completions.create(']],
        [['', '    model='], ['s', '"gpt-5.5"'], ['', ',']],
        [['', '    messages=[{'], ['s', '"role"'], ['', ': '], ['s', '"user"'], ['', ', '], ['s', '"content"'], ['', ': '], ['s', '"你好"'], ['', '}],']],
        [['', ')']],
      ] as Segment[][],
    },
  ]
})
const snippetId = ref('cc')
const currentSnippet = computed(() => snippets.value.find(s => s.id === snippetId.value) ?? snippets.value[0])

function copySnippet() {
  const text = currentSnippet.value.lines.map(line => line.map(seg => seg[1]).join('')).join('\n')
  void useClipboard().copyToClipboard(text, '已复制配置')
}

// 示意数据，仅用于说明使用记录的样子
const sampleLogs = [
  ['14:32:08', 'claude-sonnet-4-6', '49,414', '$0.0651'],
  ['14:11:02', 'gpt-5.6-sol', '14,790', '$0.0132'],
  ['13:58:44', 'deepseek-v4-flash', '4,542', '$0.0010'],
  ['13:40:17', 'claude-opus-5', '24,762', '$0.0745'],
]
</script>

<style scoped>
.hc { display: grid; grid-template-columns: minmax(0, 5fr) minmax(0, 7fr); gap: 40px; align-items: stretch; }
.hc-steps { display: grid; align-content: center; gap: 4px; }
.hc-steps [role='tab'] { position: relative; display: grid; gap: 6px; overflow: hidden; padding: 20px 22px; border: 1px solid transparent; border-radius: 12px; color: var(--lc-ink-3); text-align: left; }
.hc-steps [role='tab'] b { color: var(--lc-ink-2); font-size: 17px; font-weight: 600; }
.hc-steps [role='tab'] span { font-size: 14px; line-height: 1.65; }
.hc-steps [role='tab']:hover b { color: var(--lc-ink); }
.hc-steps [aria-selected='true'] { border-color: var(--lc-line); background: var(--lc-surface); box-shadow: inset 0 1px 0 var(--lc-hi); color: var(--lc-ink-2); }
.hc-steps [aria-selected='true'] b { color: var(--lc-ink); }
.hc-steps [aria-selected='true']::after { content: ''; position: absolute; left: 22px; right: 22px; bottom: 0; height: 2px; background: var(--lc-accent); }
.hc-stage { display: grid; align-items: center; min-width: 0; min-height: 380px; padding: 28px; border: 1px solid var(--lc-line); border-radius: 16px; background-color: var(--lc-surface); background-image: radial-gradient(var(--lc-dot) 1px, transparent 1px); background-size: 18px 18px; box-shadow: inset 0 1px 0 var(--lc-hi); }
.hc-pane { display: grid; gap: 16px; min-width: 0; }
.hc-pills { display: flex; flex-wrap: wrap; align-items: center; gap: 4px; }
.hc-pills [role='tab'] { height: 30px; padding: 0 12px; border: 1px solid var(--lc-line); border-radius: 999px; background: var(--lc-bg-2); color: var(--lc-ink-2); font: 500 12px var(--lc-font-mono); }
.hc-pills [aria-selected='true'] { border-color: var(--lc-ink); background: var(--lc-ink); color: var(--lc-bg); }
.hc-copy { display: inline-flex; align-items: center; gap: 5px; height: 28px; margin-left: auto; padding: 0 8px; border: 1px solid transparent; border-radius: 6px; color: var(--lc-ink-3); font-size: 12px; }
.hc-copy:hover { border-color: var(--lc-line-2); color: var(--lc-ink); }
.hc-cap { color: var(--lc-ink-3); font: 500 12px var(--lc-font-mono); }
.hc-cap a { text-decoration: underline; text-underline-offset: 3px; }
.hc-formula { display: flex; flex-wrap: wrap; align-items: center; gap: 12px; font-size: 18px; }
.hc-formula span { padding: 10px 16px; border: 1px solid var(--lc-line); border-radius: 10px; background: var(--lc-bg-2); }
.hc-formula i { color: var(--lc-ink-3); font-style: normal; }
.hc-formula b { padding: 10px 16px; border-radius: 10px; background: var(--lc-accent); color: #fff; font-weight: 600; }
.hc-rows, .hc-log { overflow: hidden; border: 1px solid var(--lc-line); border-radius: 10px; background: var(--lc-bg-2); }
.hc-rows div { display: flex; justify-content: space-between; gap: 12px; padding: 12px 16px; border-top: 1px solid var(--lc-line); font-size: 14px; }
.hc-rows div:first-child, .hc-log > div:first-child { border-top: 0; }
.hc-rows span:first-child { color: var(--lc-ink-3); }
.hc-rows .sum { font-weight: 600; }
.hc-rows .sum span:first-child { color: var(--lc-ink); }
.hc-log { font-size: 13px; }
.hc-log > div { display: grid; grid-template-columns: 76px minmax(0, 1fr) 70px 70px; gap: 10px; padding: 11px 16px; border-top: 1px solid var(--lc-line); }
.hc-log > div:first-child { color: var(--lc-ink-3); font-size: 12px; }
.hc-log span { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.hc-log .r { text-align: right; }

@media (max-width: 1024px) {
  .hc { grid-template-columns: minmax(0, 1fr); gap: 24px; }
}

@media (max-width: 760px) {
  .hc-stage { min-height: 0; padding: 16px; }
  .hc-steps [role='tab'] { padding: 16px; }
  .hc-log > div { grid-template-columns: 64px minmax(0, 1fr) 62px; }
  .hc-log > div > :nth-child(3) { display: none; }
}
</style>
