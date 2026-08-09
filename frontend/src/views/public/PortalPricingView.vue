<template>
  <PortalLayout>
    <section class="lc-page-head">
      <div class="lc-wrap lc-page-head-inner">
        <div class="lc-crumb"><RouterLink to="/home">首页</RouterLink> / 定价方案</div>
        <h1 class="lc-page-title">透明定价<span>按量计费</span></h1>
        <p class="lc-lead">{{ pricingConfig.subtitle }}</p>
      </div>
    </section>

    <div class="lc-wrap">
      <section class="lc-section" style="padding-top:54px">
        <div class="lc-card lc-card-pad">
          <div class="lc-grid lc-grid-2" style="margin-top:0;align-items:center">
            <div>
              <div class="lc-kicker">{{ pricingConfig.rechargeLabel }}</div>
              <div class="lc-pricing-value" style="font-size:clamp(30px,5vw,50px)">{{ pricingRange }}</div>
              <p style="margin-top:14px">{{ pricingConfig.highlightText }}</p>
              <div v-if="pricingConfig.activityText" class="lc-list-row" style="margin-top:15px"><span>{{ pricingConfig.activityLabel }}</span><b>{{ pricingConfig.activityText }}</b></div>
            </div>
            <div class="lc-hero-actions" style="justify-content:flex-end;margin-top:0">
              <a
                :href="pricingConfig.rechargeButtonUrl || '/payment'"
                target="_blank"
                rel="noopener noreferrer"
                class="lc-button lc-button-primary"
              >{{ pricingConfig.rechargeButtonText }} <Icon name="arrowRight" size="sm" /></a>
              <RouterLink v-if="!isAuthenticated" to="/register" class="lc-button">先注册</RouterLink>
            </div>
          </div>
        </div>

        <div class="lc-grid lc-grid-2" style="margin-top:16px">
          <article v-for="benefit in pricingConfig.benefits" :key="benefit" class="lc-card lc-list-row"><span>{{ benefit }}</span><b>支持</b></article>
          <article v-if="affiliateEnabled && affiliateRate > 0" class="lc-card lc-list-row"><span>邀请返利，好友消费返 {{ formatNumber(affiliateRate) }}%</span><b>已开启</b></article>
        </div>
      </section>

      <section class="lc-section">
        <div class="lc-section-head"><div class="lc-section-tag">Available models</div><h2 class="lc-section-title">已开放的模型</h2><p class="lc-section-copy">仅展示 Claude 与 OpenAI 的最新系列，实际可用性以后台渠道配置为准。</p></div>
        <div class="lc-card lc-table-wrap">
          <div class="lc-table-scroll">
            <table class="lc-table">
              <thead><tr><th>模型</th><th>平台</th><th>输入价格</th><th>输出价格</th><th>状态</th></tr></thead>
              <tbody>
                <tr v-for="model in latestModels" :key="`${model.platform}:${model.name}`">
                  <td><b>{{ displayModelName(model.name) }}</b></td><td>{{ platformLabel(model.platform) }}</td>
                  <td class="lc-rate">{{ formatTokenPrice(model.pricing?.input_price) }}</td><td class="lc-rate">{{ formatTokenPrice(model.pricing?.output_price) }}</td>
                  <td><span class="lc-status">已开放</span></td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
      </section>

      <section class="lc-section">
        <div class="lc-section-head"><div class="lc-section-tag">Channel rates</div><h2 class="lc-section-title">公开渠道与倍率</h2><p class="lc-section-copy">以后台当前渠道、分组与倍率配置为准。</p></div>
        <div v-if="pricingLoading" class="lc-card lc-empty animate-pulse">正在读取定价配置...</div>
        <div v-else-if="channelRows.length === 0" class="lc-card lc-empty"><h3>暂无公开渠道</h3><p>{{ pricingConfig.channelEmptyText }}</p></div>
        <div v-else>
          <div v-for="section in channelRateSections" :key="section.platform" class="lc-rate-platform">
            <div class="lc-rate-platform-head">
              <b>{{ section.heading }}</b>
              <span>({{ section.rows.length }})</span>
              <span class="lc-rate-platform-line"></span>
            </div>
            <div class="lc-rate-grid">
              <article v-for="row in section.rows" :key="`${row.channel}:${row.platform}:${row.group.id}`" class="lc-card lc-channel-rate-card">
                <div class="lc-rate-platform-icon" :class="{ 'is-openai': row.platform === 'openai' }">
                  <PlatformIcon :platform="row.platform as GroupPlatform" size="lg" />
                </div>
                <div class="lc-channel-rate-body">
                  <div class="lc-channel-rate-title">
                    <h3>{{ row.group.name }}</h3>
                    <span class="lc-platform-chip" :class="{ 'is-openai': row.platform === 'openai' }">{{ platformLabel(row.platform) }}</span>
                  </div>
                  <div class="lc-channel-rate-meta">
                    <span class="lc-rate-badge">{{ formatNumber(row.group.rate_multiplier) }}x</span>
                    <span>{{ describeGroupRate(row.group.rate_multiplier) }}</span>
                  </div>
                  <div class="lc-channel-source">渠道：{{ row.channel }}</div>
                </div>
              </article>
            </div>
          </div>
        </div>
      </section>

      <section class="lc-section">
        <div class="lc-section-head"><div class="lc-section-tag">Recharge</div><h2 class="lc-section-title">充值档位</h2><p class="lc-section-copy">档位和赠送比例来自后台定价配置。</p></div>
        <div class="lc-tier-grid">
          <article v-for="tier in rechargeTiers" :key="tier.amount" class="lc-card lc-tier">
            <div class="lc-kicker">{{ tier.label || '充值档位' }}</div>
            <div class="lc-list">
              <div class="lc-list-row"><span>充值金额</span><b>¥{{ formatNumber(tier.amount) }}</b></div>
              <div class="lc-list-row"><span>可用模型</span><b>已开放模型</b></div>
            </div>
            <div class="lc-tier-benefit" :class="{ 'has-benefit': tier.benefitText }">
              <small>档位赠送</small>
              <b>{{ tier.benefitText || '无额外赠送' }}</b>
            </div>
          </article>
        </div>
      </section>

      <section class="lc-section">
        <div class="lc-section-head"><div class="lc-section-tag">FAQ</div><h2 class="lc-section-title">计费相关问题</h2></div>
        <div class="lc-faq">
          <article class="lc-card lc-faq-item"><h4><i>Q1</i>价格会变动吗？</h4><p>价格会随上游成本调整，调价信息会通过公告与社群同步。</p></article>
          <article class="lc-card lc-faq-item"><h4><i>Q2</i>额度能用在哪些模型上？</h4><p>额度可用于当前后台已开放的模型，请以本页与控制台显示为准。</p></article>
          <article v-if="affiliateEnabled && affiliateRate > 0" class="lc-card lc-faq-item"><h4><i>Q3</i>邀请返利怎么结算？</h4><p>被邀请人产生符合规则的消费后，邀请人按当前配置的 {{ formatNumber(affiliateRate) }}% 比例获得返利。</p></article>
          <article class="lc-card lc-faq-item"><h4><i>{{ affiliateEnabled && affiliateRate > 0 ? 'Q4' : 'Q3' }}</i>能设置消费上限吗？</h4><p>可以。每个 API Key 可单独设置消费配额与模型白名单。</p></article>
        </div>
      </section>
    </div>

    <section class="lc-cta"><div class="lc-wrap lc-cta-inner"><h2 class="lc-section-title">价格写在<span class="lc-title-accent">明面上</span>，用着才安心</h2><div class="lc-hero-actions" style="justify-content:center"><RouterLink to="/register" class="lc-button lc-button-primary">立即注册</RouterLink><RouterLink to="/portal/status" class="lc-button">查看渠道可用性</RouterLink></div></div></section>
  </PortalLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { RouterLink } from 'vue-router'
import { useAuthStore, useAppStore } from '@/stores'
import { getPublicPricing } from '@/api/public'
import type { UserAvailableChannel, UserSupportedModel } from '@/api/channels'
import PortalLayout from './components/PortalLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import PlatformIcon from '@/components/common/PlatformIcon.vue'
import { extractRechargeBenefitAmounts, findRechargeBenefit, parsePricingDisplayConfig, type RechargeTierConfig } from '@/utils/pricingDisplayConfig'
import { extractApiErrorMessage } from '@/utils/apiError'
import type { GroupPlatform } from '@/types'

type DisplayModel = UserSupportedModel & { platform: string }
const authStore = useAuthStore()
const appStore = useAppStore()
const channels = ref<UserAvailableChannel[]>([])
const pricingLoading = ref(false)
const settings = computed(() => appStore.cachedPublicSettings)
const pricingConfig = computed(() => parsePricingDisplayConfig(settings.value?.pricing_display_config || ''))
const isAuthenticated = computed(() => authStore.isAuthenticated)
const affiliateEnabled = computed(() => settings.value?.affiliate_enabled === true)
const affiliateRate = computed(() => Number(settings.value?.affiliate_rebate_rate || 0))
const pricingRange = computed(() => `¥${formatNumber(pricingConfig.value.yuanAmount)} = $${formatNumber(pricingConfig.value.usdAmount)} ${pricingConfig.value.creditUnitLabel}`)
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
    benefitText: findRechargeBenefit(pricingConfig.value.benefits, tier.amount),
  }))
})

const allConfiguredModels = computed<DisplayModel[]>(() => {
  const unique = new Map<string, DisplayModel>()
  for (const channel of channels.value) for (const section of channel.platforms) {
    if (!['anthropic', 'openai'].includes(section.platform)) continue
    for (const model of section.supported_models) unique.set(`${section.platform}:${model.name}`, { ...model, platform: section.platform })
  }
  return [...unique.values()]
})
const latestModels = computed<DisplayModel[]>(() => {
  const anthropic = selectLatestClaude(allConfiguredModels.value.filter(item => item.platform === 'anthropic'))
  const openai = allConfiguredModels.value.filter(item => item.platform === 'openai' && /gpt[-_. ]?5[-_. ]?6/i.test(item.name)).sort((a,b) => a.name.localeCompare(b.name)).slice(0, 3)
  return [...(anthropic.length ? anthropic : fallbackClaudeModels), ...(openai.length ? openai : fallbackOpenAIModels)]
})
const channelRows = computed(() => channels.value.flatMap(channel => channel.platforms.filter(section => ['anthropic', 'openai'].includes(section.platform)).flatMap(section => section.groups.filter(group => !group.is_exclusive).map(group => ({ channel: channel.name, platform: section.platform, group })))))
const channelRateSections = computed(() => ['anthropic', 'openai'].map(platform => {
  const rows = channelRows.value.filter(row => row.platform === platform)
  return { platform, heading: platform === 'anthropic' ? 'Anthropic' : 'OpenAI', rows }
}).filter(section => section.rows.length > 0))

const fallbackClaudeModels: DisplayModel[] = ['claude-opus-5', 'claude-sonnet-5', 'claude-haiku-4.5'].map(name => ({ name, platform: 'anthropic', pricing: null }))
const fallbackOpenAIModels: DisplayModel[] = ['gpt-5.6', 'gpt-5.6-codex', 'gpt-5.6-mini'].map(name => ({ name, platform: 'openai', pricing: null }))

function selectLatestClaude(models: DisplayModel[]) {
  return ['opus', 'sonnet', 'haiku'].map(family => models.filter(model => model.name.toLowerCase().includes(family)).sort((a,b) => b.name.localeCompare(a.name, undefined, { numeric: true }))[0]).filter((model): model is DisplayModel => Boolean(model))
}
function formatNumber(value: number) { return Number.isInteger(value) ? String(value) : value.toFixed(2).replace(/\.?0+$/, '') }
function displayModelName(name: string) { return name.split(/[-_]/).map(part => ['gpt','claude'].includes(part.toLowerCase()) ? part.toUpperCase() : part.charAt(0).toUpperCase() + part.slice(1)).join(' ') }
function platformLabel(platform: string) { return platform === 'anthropic' ? 'Claude / Anthropic' : platform === 'openai' ? 'OpenAI' : platform }
function formatTokenPrice(value: number | null | undefined) { return value == null ? '按渠道配置' : `$${formatNumber(value * 1_000_000)} / 1M tokens` }
function describeGroupRate(multiplier: number) { return pricingConfig.value.channelGroupDescriptionTemplate.replace('{multiplier}', formatNumber(multiplier)).replace('{price}', formatNumber(multiplier * pricingConfig.value.yuanAmount)) }

async function loadPricing() {
  pricingLoading.value = true
  try { channels.value = await getPublicPricing() }
  catch (error) { appStore.showError(extractApiErrorMessage(error, '加载定价失败')) }
  finally { pricingLoading.value = false }
}

onMounted(() => { void appStore.fetchPublicSettings(true); void authStore.checkAuth(); void loadPricing() })
</script>
