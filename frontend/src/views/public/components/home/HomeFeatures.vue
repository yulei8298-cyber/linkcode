<template>
  <section class="hm-sec">
    <div class="lc-wrap">
      <div class="hm-head">
        <p class="hm-kicker">日常开发</p>
        <h2>为写代码这件事准备的</h2>
        <p>适合长时间跑 Agent 和成批改代码。</p>
      </div>
      <div class="hf-grid">
        <article v-for="item in items" :key="item.title" class="hm-card hf-card">
          <Icon :name="item.icon" size="lg" class="hf-icon" />
          <h3>{{ item.title }}</h3>
          <p>{{ item.text }}</p>
        </article>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import Icon from '@/components/icons/Icon.vue'

const props = defineProps<{ hasCommunity: boolean; showIntelCheck: boolean }>()

type IconName = 'cube' | 'globe' | 'key' | 'chart' | 'sparkles' | 'chat'
interface Feature { icon: IconName; title: string; text: string }

const items = computed<Feature[]>(() => {
  const list: Feature[] = [
    { icon: 'cube', title: '主流模型都能调', text: 'Claude、GPT、Grok、DeepSeek、Kimi，同一个账号、同一份余额。新模型上线后尽快接入。' },
    { icon: 'globe', title: '国内直连线路', text: '国内网络换用直连地址即可，其余配置不用动，地址在控制台的 API 密钥页可以复制。' },
    { icon: 'key', title: '密钥单独管控', text: '每个密钥可以设额度、可用模型、有效期和 IP，分给同事或脚本更放心。' },
    { icon: 'chart', title: '可用性公开', text: '定时用真实请求探测各号池，延迟和成败都能在可用性检测页看到。' },
  ]
  if (props.showIntelCheck) {
    list.push({ icon: 'sparkles', title: '降智看得见', text: '定时给模型出逻辑题和绘图题，题目和原始回复全部公开。' })
  }
  if (props.hasCommunity) {
    list.push({ icon: 'chat', title: '有人回你', text: '配置不通、报错看不懂，到 QQ 群或 Telegram 群里问，有人帮你看。' })
  }
  return list
})
</script>

<style scoped>
.hf-grid { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 16px; }
.hf-card { padding: 28px; transition: border-color .16s; }
.hf-card:hover { border-color: var(--lc-line-2); }
.hf-icon { color: var(--lc-accent); }
.hf-card h3 { margin-top: 18px; font-size: 17px; font-weight: 600; }
.hf-card p { margin-top: 8px; color: var(--lc-ink-2); font-size: 14px; line-height: 1.7; }

@media (max-width: 1024px) {
  .hf-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); }
}

@media (max-width: 760px) {
  .hf-grid { grid-template-columns: minmax(0, 1fr); }
  .hf-card { padding: 22px; }
}
</style>
