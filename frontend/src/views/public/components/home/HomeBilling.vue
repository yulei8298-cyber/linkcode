<template>
  <section class="hm-sec">
    <div class="lc-wrap">
      <div class="hm-head">
        <p class="hm-kicker">计费</p>
        <h2>用多少，付多少</h2>
        <p>充值得到美元额度，每次调用按实际用量扣费。</p>
      </div>
      <div class="hm-card hb">
        <div class="hb-main">
          <h3>余额按量计费</h3>
          <p>适合多个模型混着用，用量时多时少也不浪费。</p>
          <div class="hb-rate">
            <span class="lc-mono">¥{{ yuan }}</span><i>=</i><span class="lc-mono">${{ usd }}</span><small>{{ unitLabel }}</small>
          </div>
          <div class="hb-models">
            <span v-for="p in providers" :key="p.platform" class="hm-chip">
              <span class="hm-pm" :class="`is-${p.platform}`"><PlatformIcon :platform="p.platform" size="xs" /></span>{{ p.name }}
            </span>
          </div>
          <RouterLink to="/portal/pricing" class="lc-button lc-button-primary lc-button-xl hb-full">查看实时费率</RouterLink>
        </div>
        <ul class="hb-checks">
          <li v-for="item in checks" :key="item.title">
            <Icon name="check" size="md" class="hb-check" />
            <div><b>{{ item.title }}</b><span>{{ item.text }}</span></div>
          </li>
        </ul>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { RouterLink } from 'vue-router'
import Icon from '@/components/icons/Icon.vue'
import PlatformIcon from '@/components/common/PlatformIcon.vue'
import type { GroupPlatform } from '@/types'

const props = defineProps<{
  yuan: string
  usd: string
  unitLabel: string
  affiliateRate: number
}>()

const providers: { platform: GroupPlatform; name: string }[] = [
  { platform: 'anthropic', name: 'Claude' },
  { platform: 'openai', name: 'GPT' },
  { platform: 'grok', name: 'Grok' },
  { platform: 'deepseek', name: 'DeepSeek' },
  { platform: 'kimi', name: 'Kimi' },
]

const checks = computed(() => {
  const list = [
    { title: '余额长期有效', text: '不按月清零，什么时候用都行。' },
    { title: '价格公开可算', text: '官方价乘以分组倍率，没有其他附加费用。' },
    { title: '随时切换模型', text: '一份余额通用所有已开放的模型。' },
  ]
  if (props.affiliateRate > 0) {
    list.push({ title: `邀请返利 ${props.affiliateRate}%`, text: '好友消费后，按比例返到你的余额。' })
  }
  return list
})
</script>

<style scoped>
.hb { display: grid; grid-template-columns: minmax(0, 1fr) minmax(0, 1fr); overflow: hidden; border-radius: 16px; }
.hb-main { display: grid; align-content: start; gap: 14px; padding: 36px; border-right: 1px solid var(--lc-line); }
.hb-main h3 { font-size: 22px; font-weight: 600; }
.hb-main > p { color: var(--lc-ink-2); font-size: 15px; }
.hb-rate { display: flex; align-items: baseline; flex-wrap: wrap; gap: 12px; margin: 10px 0 4px; }
.hb-rate .lc-mono { font-size: 44px; font-weight: 600; letter-spacing: -0.02em; }
.hb-rate i { color: var(--lc-ink-3); font-size: 28px; font-style: normal; }
.hb-rate small { color: var(--lc-ink-2); font-size: 15px; }
.hb-models { display: flex; flex-wrap: wrap; gap: 8px; }
.hb-full { width: 100%; margin-top: 10px; }
.hb-checks { display: grid; align-content: center; gap: 22px; padding: 36px; background: var(--lc-bg-2); }
.hb-checks li { display: flex; gap: 14px; }
.hb-check { flex: none; margin-top: 2px; color: var(--lc-accent); }
.hb-checks div { display: grid; gap: 2px; }
.hb-checks b { font-size: 15px; font-weight: 600; }
.hb-checks span { color: var(--lc-ink-2); font-size: 14px; }

@media (max-width: 1024px) {
  .hb { grid-template-columns: minmax(0, 1fr); }
  .hb-main { border-right: 0; border-bottom: 1px solid var(--lc-line); }
}

@media (max-width: 760px) {
  .hb-main, .hb-checks { padding: 24px; }
  .hb-rate .lc-mono { font-size: 34px; }
}
</style>
