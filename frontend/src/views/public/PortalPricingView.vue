<template>
  <PortalLayout>
    <div class="lc-wrap pr">
      <header class="pr-head">
        <p class="pr-crumb"><RouterLink to="/home">首页</RouterLink> / 定价方案</p>
        <h1>定价方案</h1>
        <p class="pr-sub">按用量扣费。单价 = 官方价 × 分组倍率。</p>
      </header>

      <!-- 充值：比例 + 档位 -->
      <section class="pr-top">
        <div class="pr-card pr-rate-card">
          <p class="pr-label">{{ pricingConfig.rechargeLabel }}</p>
          <p class="pr-rate">
            <span>¥{{ yuanText }}</span><i>=</i><span>${{ usdText }}</span>
            <small>{{ pricingConfig.creditUnitLabel }}</small>
          </p>
          <p v-if="pricingConfig.activityText" class="pr-activity">
            <b>{{ pricingConfig.activityLabel }}</b>{{ pricingConfig.activityText }}
          </p>
          <p v-if="pricingConfig.highlightText" class="pr-note">{{ pricingConfig.highlightText }}</p>
          <ul class="pr-facts">
            <li><Icon name="check" size="sm" />按实际 token 用量扣费，用多少扣多少</li>
            <li><Icon name="check" size="sm" />余额长期有效，不按月清零</li>
          </ul>
          <div class="pr-actions">
            <a :href="pricingConfig.rechargeButtonUrl || '/payment'" target="_blank" rel="noopener noreferrer" class="lc-button lc-button-primary">
              {{ pricingConfig.rechargeButtonText }}
            </a>
            <RouterLink v-if="!isAuthenticated" to="/register" class="lc-button">注册账号</RouterLink>
          </div>
        </div>

        <div class="pr-card">
          <div class="pr-card-head"><h2>充值档位</h2><span>余额长期有效</span></div>
          <table class="pr-table pr-tiers">
            <thead><tr><th>档位</th><th class="num">充值金额</th><th class="num">档位赠送</th></tr></thead>
            <tbody>
              <tr v-for="tier in rechargeTiers" :key="tier.amount">
                <td>{{ tier.label || '充值' }}</td>
                <td class="num">¥{{ formatNumber(tier.amount) }}</td>
                <td class="num" :class="{ gift: tier.benefitText }">{{ tier.benefitText || '—' }}</td>
              </tr>
            </tbody>
          </table>
          <p v-if="affiliateRate > 0" class="pr-foot">邀请好友注册，好友消费后按 {{ formatNumber(affiliateRate) }}% 返到你的余额。</p>
        </div>
      </section>

      <!-- 分组倍率 -->
      <section class="pr-sec">
        <div class="pr-sec-head">
          <div>
            <h2>分组倍率</h2>
            <p>同一个模型在不同分组里价格不同，倍率越低越便宜。创建密钥时选择分组。</p>
          </div>
          <div v-if="platforms.length > 1" class="pr-seg" role="group" aria-label="按平台筛选">
            <button type="button" :aria-pressed="platformFilter === ''" @click="platformFilter = ''">全部</button>
            <button
              v-for="p in platforms"
              :key="p"
              type="button"
              :aria-pressed="platformFilter === p"
              @click="platformFilter = p"
            >{{ platformLabel(p) }}</button>
          </div>
        </div>

        <div class="pr-card pr-scroll">
          <p v-if="pricingLoading" class="pr-state">正在读取分组倍率…</p>
          <p v-else-if="loadError" class="pr-state">
            读取失败。<button type="button" class="pr-link" @click="loadPricing">重试</button>
          </p>
          <p v-else-if="groupRows.length === 0" class="pr-state">{{ pricingConfig.channelEmptyText }}</p>
          <table v-else class="pr-table">
            <thead>
              <tr><th>分组</th><th>平台</th><th class="num">倍率</th><th class="num">折合人民币</th></tr>
            </thead>
            <tbody>
              <tr v-for="row in visibleRows" :key="`${row.channel}:${row.group.id}`">
                <td class="strong">{{ row.group.name }}</td>
                <td>
                  <span class="pr-platform">
                    <span class="pr-icon"><PlatformIcon :platform="row.platform as GroupPlatform" size="xs" /></span>
                    {{ platformLabel(row.platform) }}
                  </span>
                </td>
                <td class="num"><span class="pr-mult">×{{ formatNumber(row.group.rate_multiplier) }}</span></td>
                <td class="num" :title="describeGroupRate(row.group.rate_multiplier)">
                  ¥{{ formatNumber(row.group.rate_multiplier * pricingConfig.yuanAmount) }} <span class="dim">/ 官方 $1</span>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
        <p class="pr-hint">
          每个模型的具体单价，到
          <RouterLink v-if="showModelPlaza" to="/model-plaza" class="pr-link">模型广场</RouterLink>
          <template v-else>控制台的模型广场</template>
          按分组查看。
        </p>
      </section>

      <!-- 计价方式 -->
      <section class="pr-sec">
        <div class="pr-sec-head"><div><h2>怎么计算</h2><p>以官方输入价 $5.00 / 百万 tokens、倍率 ×0.4 的分组为例。</p></div></div>
        <ol class="pr-steps">
          <li><span>官方价</span><b>$5.00</b><small>每百万输入 tokens</small></li>
          <li><span>乘以分组倍率</span><b>×0.4</b><small>创建密钥时选的分组</small></li>
          <li><span>实际扣除额度</span><b>$2.00</b><small>每百万输入 tokens</small></li>
          <li><span>折合人民币</span><b>¥{{ formatNumber(2 * pricingConfig.yuanAmount / pricingConfig.usdAmount) }}</b><small>按 ¥{{ yuanText }} = ${{ usdText }} 换算</small></li>
        </ol>
      </section>

      <!-- 常见问题 -->
      <section class="pr-sec">
        <div class="pr-sec-head"><div><h2>常见问题</h2></div></div>
        <dl class="pr-faq">
          <div><dt>价格会变吗？</dt><dd>会跟着上游成本调整，调价前会发公告并在群里通知。</dd></div>
          <div><dt>额度能用在哪些模型上？</dt><dd>当前已开放的所有模型，一份余额通用，以本页和控制台显示为准。</dd></div>
          <div><dt>能给密钥设消费上限吗？</dt><dd>可以。每个密钥可以单独设置额度、可用模型和有效期。</dd></div>
          <div v-if="affiliateRate > 0"><dt>邀请返利怎么算？</dt><dd>被邀请人产生符合规则的消费后，按 {{ formatNumber(affiliateRate) }}% 返到你的余额。</dd></div>
        </dl>
      </section>
    </div>
  </PortalLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { RouterLink } from 'vue-router'
import { useAuthStore, useAppStore } from '@/stores'
import { getPublicPricing } from '@/api/public'
import type { UserAvailableChannel } from '@/api/channels'
import PortalLayout from './components/PortalLayout.vue'
import PlatformIcon from '@/components/common/PlatformIcon.vue'
import Icon from '@/components/icons/Icon.vue'
import { extractRechargeBenefitAmounts, findRechargeBenefit, parsePricingDisplayConfig, type RechargeTierConfig } from '@/utils/pricingDisplayConfig'
import { extractApiErrorMessage } from '@/utils/apiError'
import type { GroupPlatform } from '@/types'

const PLATFORM_ORDER = ['anthropic', 'openai', 'gemini', 'grok', 'deepseek', 'kimi', 'zhipu', 'minimax']
const PLATFORM_LABELS: Record<string, string> = {
  anthropic: 'Claude', openai: 'OpenAI', gemini: 'Gemini', grok: 'Grok', deepseek: 'DeepSeek',
  kimi: 'Kimi', zhipu: 'GLM', minimax: 'MiniMax', antigravity: 'Antigravity',
}

const authStore = useAuthStore()
const appStore = useAppStore()
const channels = ref<UserAvailableChannel[]>([])
const pricingLoading = ref(false)
const loadError = ref(false)
const platformFilter = ref('')
const settings = computed(() => appStore.cachedPublicSettings)
const pricingConfig = computed(() => parsePricingDisplayConfig(settings.value?.pricing_display_config || ''))
const isAuthenticated = computed(() => authStore.isAuthenticated)
const affiliateRate = computed(() => (settings.value?.affiliate_enabled === true ? Number(settings.value?.affiliate_rebate_rate || 0) : 0))
const showModelPlaza = computed(() => settings.value?.model_plaza_enabled === true &&
  (isAuthenticated.value || settings.value?.model_plaza_require_auth !== true))

// 与原定价页一致，只展示基础比例；区间上限由充值页按档位计算
const yuanText = computed(() => formatNumber(pricingConfig.value.yuanAmount))
const usdText = computed(() => formatNumber(pricingConfig.value.usdAmount))

type DisplayRechargeTier = RechargeTierConfig & { benefitText: string | null }
const rechargeTiers = computed<DisplayRechargeTier[]>(() => {
  const configuredTiers = pricingConfig.value.rechargeTiers.length > 0
    ? pricingConfig.value.rechargeTiers
    : [...new Set([
        20,
        ...pricingConfig.value.recommendedAmounts,
        ...extractRechargeBenefitAmounts(pricingConfig.value.benefits),
      ])].sort((a, b) => a - b).map((amount, index, amounts) => ({
        amount,
        bonusPercent: 0,
        label: index === 0 ? '体验起步' : index === amounts.length - 1 ? '高频调用' : '日常开发',
      }))
  return configuredTiers.map(tier => ({
    ...tier,
    // 福利文案形如「¥200 赠送 $5」，表格已有金额列，这里去掉开头的金额
    benefitText: findRechargeBenefit(pricingConfig.value.benefits, tier.amount)?.replace(/^¥\s*\d+(?:\.\d+)?\s*/, '')
      ?? (tier.bonusPercent > 0 ? `赠送 ${formatNumber(tier.bonusPercent)}%` : null),
  }))
})

// 公开分组（排除专属分组），按平台、倍率排序
const groupRows = computed(() => {
  const rank = (p: string) => (PLATFORM_ORDER.indexOf(p) === -1 ? 99 : PLATFORM_ORDER.indexOf(p))
  return channels.value
    .flatMap(channel => channel.platforms.flatMap(section => section.groups
      .filter(group => !group.is_exclusive)
      .map(group => ({ channel: channel.name, platform: section.platform, group }))))
    .sort((a, b) => rank(a.platform) - rank(b.platform) || a.group.rate_multiplier - b.group.rate_multiplier)
})
const platforms = computed(() => [...new Set(groupRows.value.map(row => row.platform))])
const visibleRows = computed(() => (platformFilter.value ? groupRows.value.filter(row => row.platform === platformFilter.value) : groupRows.value))

function formatNumber(value: number) { return Number.isInteger(value) ? String(value) : value.toFixed(2).replace(/\.?0+$/, '') }
function platformLabel(platform: string) { return PLATFORM_LABELS[platform] || platform }
function describeGroupRate(multiplier: number) {
  return pricingConfig.value.channelGroupDescriptionTemplate
    .replace('{multiplier}', formatNumber(multiplier))
    .replace('{price}', formatNumber(multiplier * pricingConfig.value.yuanAmount))
}

async function loadPricing() {
  pricingLoading.value = true
  loadError.value = false
  try {
    channels.value = await getPublicPricing()
  } catch (error) {
    loadError.value = true
    appStore.showError(extractApiErrorMessage(error, '加载定价失败'))
  } finally {
    pricingLoading.value = false
  }
}

onMounted(() => { void appStore.fetchPublicSettings(true); void authStore.checkAuth(); void loadPricing() })
</script>

<style scoped>
.pr { padding: 48px 0 72px; }
.pr-crumb { color: var(--lc-ink-3); font: 12.5px var(--lc-font-mono); }
.pr-crumb a:hover { color: var(--lc-ink); }
.pr-head h1 { margin-top: 10px; font-size: clamp(28px, 3.4vw, 40px); font-weight: 700; letter-spacing: -0.02em; }
.pr-sub { margin-top: 8px; color: var(--lc-ink-2); font-size: 15px; }

.pr-card { min-width: 0; border: 1px solid var(--lc-line); border-radius: 14px; background: var(--lc-surface); box-shadow: inset 0 1px 0 var(--lc-hi); }
.pr-top { display: grid; grid-template-columns: minmax(0, 1fr) minmax(0, 1fr); gap: 16px; margin-top: 32px; }
.pr-rate-card { display: flex; flex-direction: column; gap: 12px; padding: 28px; background: radial-gradient(120% 140% at 100% 0%, var(--lc-accent-soft), transparent 55%), var(--lc-surface); }
.pr-label { color: var(--lc-ink-3); font-size: 13px; }
.pr-rate { display: flex; flex-wrap: wrap; align-items: baseline; gap: 12px; }
.pr-rate span { font: 600 44px/1.1 var(--lc-font-mono); letter-spacing: -0.02em; }
.pr-rate i { color: var(--lc-ink-3); font-size: 28px; font-style: normal; }
.pr-rate small { color: var(--lc-ink-2); font-size: 15px; }
.pr-activity { display: flex; flex-wrap: wrap; align-items: center; gap: 8px; color: var(--lc-ink); font-size: 14px; }
.pr-activity b { padding: 2px 8px; border-radius: 999px; background: var(--lc-accent-soft); color: var(--lc-accent); font-size: 12px; font-weight: 600; }
.pr-note { color: var(--lc-ink-2); font-size: 13.5px; }
.pr-actions { display: flex; flex-wrap: wrap; gap: 8px; margin-top: auto; padding-top: 6px; }
.pr-facts { display: grid; gap: 8px; margin-top: 4px; padding-top: 14px; border-top: 1px solid var(--lc-line); color: var(--lc-ink-2); font-size: 13.5px; }
.pr-facts li { display: flex; align-items: center; gap: 8px; }
.pr-facts svg { color: var(--lc-accent); }
.pr-card-head { display: flex; align-items: baseline; justify-content: space-between; gap: 12px; padding: 16px 20px; border-bottom: 1px solid var(--lc-line); }
.pr-card-head h2 { font-size: 15px; font-weight: 600; }
.pr-card-head span { color: var(--lc-ink-3); font-size: 12.5px; }
.pr-foot { padding: 12px 20px; border-top: 1px solid var(--lc-line); color: var(--lc-ink-2); font-size: 13px; }

.pr-table { width: 100%; border-collapse: collapse; font-size: 13.5px; }
.pr-table th { padding: 10px 20px; border-bottom: 1px solid var(--lc-line); color: var(--lc-ink-3); font-size: 12px; font-weight: 500; text-align: left; white-space: nowrap; }
.pr-table td { height: 48px; padding: 0 20px; border-bottom: 1px solid var(--lc-line); color: var(--lc-ink-2); white-space: nowrap; }
.pr-table tbody tr:last-child td { border-bottom: 0; }
.pr-table tbody tr:hover td { background: var(--lc-surface-2); }
.pr-table .num { text-align: right; font-family: var(--lc-font-mono); font-variant-numeric: tabular-nums; }
.pr-table .strong { color: var(--lc-ink); font-weight: 600; }
.pr-table .dim { color: var(--lc-ink-3); }
.pr-tiers td { height: 44px; }
.pr-tiers .gift { color: var(--lc-accent); font-weight: 600; }
.pr-platform { display: inline-flex; align-items: center; gap: 8px; color: var(--lc-ink); }
.pr-icon { display: grid; place-items: center; width: 22px; height: 22px; border-radius: 6px; background: var(--lc-pm-bg); }
.pr-mult { padding: 2px 8px; border-radius: 999px; background: var(--lc-accent-soft); color: var(--lc-accent); font-weight: 600; }

.pr-sec { margin-top: 64px; }
.pr-sec-head { display: flex; flex-wrap: wrap; align-items: flex-end; justify-content: space-between; gap: 16px; margin-bottom: 16px; }
.pr-sec-head h2 { font-size: 22px; font-weight: 700; letter-spacing: -0.01em; }
.pr-sec-head p { margin-top: 6px; color: var(--lc-ink-2); font-size: 14px; }
.pr-seg { display: inline-flex; flex-wrap: wrap; gap: 2px; padding: 3px; border: 1px solid var(--lc-line); border-radius: 8px; background: var(--lc-bg-2); }
.pr-seg button { height: 28px; padding: 0 12px; border-radius: 6px; color: var(--lc-ink-2); font-size: 13px; }
.pr-seg button[aria-pressed='true'] { background: var(--lc-surface-3); color: var(--lc-ink); box-shadow: inset 0 1px 0 var(--lc-hi); }
.pr-scroll { overflow-x: auto; }
.pr-state { padding: 40px 20px; text-align: center; color: var(--lc-ink-2); font-size: 14px; }
.pr-link { color: var(--lc-accent); font-weight: 500; }
.pr-link:hover { text-decoration: underline; }
.pr-hint { margin-top: 12px; color: var(--lc-ink-3); font-size: 13px; }

.pr-steps { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); border: 1px solid var(--lc-line); border-radius: 14px; background: var(--lc-surface); }
.pr-steps li { display: grid; gap: 4px; padding: 20px; border-left: 1px solid var(--lc-line); }
.pr-steps li:first-child { border-left: 0; }
.pr-steps span { color: var(--lc-ink-3); font-size: 13px; }
.pr-steps b { font: 600 24px/1.3 var(--lc-font-mono); }
.pr-steps li:nth-child(3) b { color: var(--lc-accent); }
.pr-steps small { color: var(--lc-ink-3); font-size: 12px; }

.pr-faq { border-top: 1px solid var(--lc-line); }
.pr-faq div { display: grid; grid-template-columns: minmax(0, 280px) minmax(0, 1fr); gap: 24px; padding: 16px 0; border-bottom: 1px solid var(--lc-line); }
.pr-faq dt { font-weight: 600; }
.pr-faq dd { color: var(--lc-ink-2); }

@media (max-width: 900px) {
  .pr-top { grid-template-columns: minmax(0, 1fr); }
  .pr-steps { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .pr-steps li:nth-child(3) { border-left: 0; }
  .pr-steps li:nth-child(n + 3) { border-top: 1px solid var(--lc-line); }
}

@media (max-width: 640px) {
  .pr { padding: 32px 0 56px; }
  .pr-rate-card { padding: 22px; }
  .pr-rate span { font-size: 34px; }
  .pr-faq div { grid-template-columns: minmax(0, 1fr); gap: 4px; }
}
</style>
