<template>
  <div v-if="hasHomeContent" class="min-h-screen">
    <iframe v-if="isHomeContentUrl" :src="homeContent.trim()" class="h-screen w-full border-0" allowfullscreen></iframe>
    <div v-else v-html="homeContent"></div>
  </div>

  <div
    v-else-if="compactHomeEnabled"
    data-testid="compact-home"
    class="flex min-h-screen flex-col bg-gray-50 text-gray-900 dark:bg-dark-950 dark:text-white"
  >
    <header class="border-b border-gray-200 px-4 py-4 sm:px-6 dark:border-dark-800">
      <nav class="mx-auto flex max-w-5xl flex-wrap items-center justify-between gap-3 sm:gap-4">
        <div class="flex min-w-0 flex-1 items-center gap-3">
          <img :src="siteLogo || '/logo.svg'" alt="Logo" class="h-9 w-9 shrink-0 rounded-lg object-contain" />
          <span class="min-w-0 truncate text-base font-semibold">{{ siteName }}</span>
        </div>
        <div class="flex max-w-full shrink-0 flex-wrap items-center justify-end gap-2">
          <LocaleSwitcher />
          <a
            v-if="docUrl"
            :href="docUrl"
            target="_blank"
            rel="noopener noreferrer"
            class="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg text-gray-500 hover:bg-gray-100 dark:text-dark-400 dark:hover:bg-dark-800"
            :title="t('home.viewDocs')"
          >
            <Icon name="book" size="md" />
          </a>
          <button
            type="button"
            class="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg text-gray-500 hover:bg-gray-100 dark:text-dark-400 dark:hover:bg-dark-800"
            :title="isDark ? t('home.switchToLight') : t('home.switchToDark')"
            @click="toggleTheme"
          >
            <Icon :name="isDark ? 'sun' : 'moon'" size="md" />
          </button>
          <RouterLink
            :to="isAuthenticated ? dashboardPath : '/login'"
            class="inline-flex min-h-10 shrink-0 items-center justify-center rounded-lg bg-gray-900 px-4 py-2 text-sm font-medium text-white hover:bg-gray-800 dark:bg-white dark:text-gray-900 dark:hover:bg-gray-200"
          >
            {{ isAuthenticated ? t('home.dashboard') : t('home.login') }}
          </RouterLink>
        </div>
      </nav>
    </header>

    <main class="flex min-w-0 flex-1 items-center justify-center px-4 py-16 sm:px-6">
      <div class="min-w-0 max-w-2xl text-center">
        <img :src="siteLogo || '/logo.svg'" alt="Logo" class="mx-auto mb-6 h-20 w-20 rounded-2xl object-contain" />
        <h1 class="[overflow-wrap:anywhere] text-3xl font-bold md:text-4xl">{{ siteName }}</h1>
        <p class="mt-4 whitespace-pre-wrap [overflow-wrap:anywhere] text-base text-gray-600 dark:text-dark-300">{{ siteSubtitle }}</p>
        <RouterLink
          :to="isAuthenticated ? dashboardPath : '/login'"
          class="mt-8 inline-flex min-h-10 items-center justify-center rounded-lg bg-primary-600 px-5 py-2.5 text-sm font-medium text-white hover:bg-primary-700"
        >
          {{ isAuthenticated ? t('home.goToDashboard') : t('home.login') }}
        </RouterLink>
      </div>
    </main>

    <footer class="min-w-0 border-t border-gray-200 px-4 py-5 text-center text-sm text-gray-500 [overflow-wrap:anywhere] sm:px-6 dark:border-dark-800 dark:text-dark-400">
      &copy; {{ currentYear }} {{ siteName }}
    </footer>
  </div>

  <PortalLayout v-else data-testid="default-home">
    <section class="lc-hero">
      <div class="lc-wrap lc-hero-grid">
        <div>
          <div class="lc-eyebrow"><span class="lc-live-dot"></span>AI API GATEWAY</div>
          <h1 class="lc-title">一个网关<br>接通<span class="lc-title-accent">全球大模型</span></h1>
          <p class="lc-lead">{{ siteSubtitle || '兼容 OpenAI、Anthropic 与 Gemini 协议，替换 base_url 即可接入现有应用。' }}</p>
          <div class="lc-hero-actions">
            <RouterLink :to="isAuthenticated ? dashboardPath : '/register'" class="lc-button lc-button-primary">
              {{ isAuthenticated ? '进入控制台' : '免费获取 API Key' }}
              <Icon name="arrowRight" size="sm" />
            </RouterLink>
            <RouterLink to="/portal/pricing" class="lc-button">查看定价方案</RouterLink>
          </div>
        </div>

        <div class="lc-terminal terminal-container" aria-label="API 接入示例">
          <div class="lc-terminal-bar">
            <span class="lc-window-dot" style="background:#ff5f57"></span>
            <span class="lc-window-dot" style="background:#febc2e"></span>
            <span class="lc-window-dot" style="background:#28c840"></span>
            <span class="lc-terminal-name">~/{{ siteName }} · quickstart</span>
            <span class="lc-ready">READY</span>
          </div>
          <pre class="lc-code"><span class="lc-code-key">from</span> openai <span class="lc-code-key">import</span> OpenAI

client = OpenAI(
    base_url=<span class="lc-code-value">"{{ normalizedApiBaseUrl }}"</span>,
    api_key=<span class="lc-code-value">"sk-linkcode-••••••••"</span>,
)

response = client.responses.create(
    model=<span class="lc-code-value">"gpt-5.6"</span>,
    input=<span class="lc-code-value">"你好"</span>,
)</pre>
          <div class="lc-endpoints">
            <div class="lc-endpoint"><span>Anthropic 协议</span><code>/v1/messages</code></div>
            <div class="lc-endpoint"><span>OpenAI 协议</span><code>/v1/responses</code></div>
            <div class="lc-endpoint"><span>Gemini 协议</span><code>/v1beta/models</code></div>
          </div>
        </div>
      </div>
    </section>

    <div class="lc-brands" aria-hidden="true">
      <div class="lc-brands-track">
        <span>CLAUDE <b>◆</b> OPENAI <b>◆</b> GEMINI <b>◆</b> GROK <b>◆</b></span>
        <span>CLAUDE <b>◆</b> OPENAI <b>◆</b> GEMINI <b>◆</b> GROK <b>◆</b></span>
      </div>
    </div>

    <div class="lc-wrap">
      <section class="lc-section">
        <div class="lc-section-head">
          <div class="lc-section-tag">Capabilities</div>
          <h2 class="lc-section-title">一个 Key，打通全部模型</h2>
          <p class="lc-section-copy">协议差异、密钥管理与账单对账，统一收敛到同一个网关。</p>
        </div>
        <div class="lc-grid lc-bento">
          <article class="lc-card lc-card-pad lc-bento-main">
            <div class="lc-icon-box"><Icon name="grid" /></div>
            <h3>全系模型矩阵</h3>
            <p>旗舰与轻量模型统一开放，同一 API Key 可以直接切换。</p>
            <div class="lc-model-groups">
              <div v-for="group in modelMatrix" :key="group.brand">
                <div class="lc-model-brand">{{ group.brand }}</div>
                <div class="lc-tags"><span v-for="model in group.models" :key="model" class="lc-tag">{{ model }}</span></div>
              </div>
            </div>
          </article>
          <article v-for="capability in capabilities" :key="capability.title" class="lc-card lc-card-pad">
            <div class="lc-icon-box"><Icon :name="capability.icon" /></div>
            <h3>{{ capability.title }}</h3>
            <p>{{ capability.description }}</p>
            <div class="lc-list">
              <div v-for="row in capability.rows" :key="row[0]" class="lc-list-row"><span>{{ row[0] }}</span><b>{{ row[1] }}</b></div>
            </div>
          </article>
        </div>
      </section>

      <section class="lc-section">
        <div class="lc-section-head"><div class="lc-section-tag">Quick start</div><h2 class="lc-section-title">三步接入，五分钟上手</h2></div>
        <div class="lc-grid lc-grid-3">
          <article v-for="(step, index) in steps" :key="step.title" class="lc-card lc-card-pad">
            <div class="lc-icon-box lc-mono">{{ index + 1 }}</div>
            <h3>{{ step.title }}</h3><p>{{ step.description }}</p>
            <div class="lc-list-row lc-mono" style="margin-top:14px"><span>{{ step.code }}</span></div>
          </article>
        </div>
      </section>

      <section class="lc-section">
        <div class="lc-section-head">
          <div class="lc-section-tag">Pricing</div>
          <h2 class="lc-section-title">{{ pricingConfig.title }}</h2>
          <p class="lc-section-copy">{{ pricingConfig.subtitle }}</p>
        </div>
        <div class="lc-pricing-summary">
          <article class="lc-card lc-card-pad"><div class="lc-kicker">{{ pricingConfig.rechargeLabel }}</div><div class="lc-pricing-value">{{ pricingRange }}</div><p>{{ pricingConfig.exampleText }}</p></article>
          <article class="lc-card lc-card-pad"><div class="lc-kicker">计费方式</div><div class="lc-pricing-value">用多少 · 扣多少</div><p>无月租、无最低消费，请求明细随时可查。</p></article>
          <article class="lc-card lc-card-pad"><div class="lc-kicker">余额有效期</div><div class="lc-pricing-value">永久</div><p>不按月清零，充值额度长期可用。</p></article>
        </div>
        <div class="lc-hero-actions" style="justify-content:center"><RouterLink to="/portal/pricing" class="lc-button">查看完整定价与充值方案 <Icon name="arrowRight" size="sm" /></RouterLink></div>
      </section>

      <section v-if="qqGroup || telegramGroupUrl" class="lc-section">
        <div class="lc-section-head"><div class="lc-section-tag">Community</div><h2 class="lc-section-title">加入社群，随时找到我们</h2></div>
        <div class="lc-community">
          <a v-if="qqGroup" class="lc-card lc-community-card" href="#"><span class="lc-community-mark">QQ</span><div><h4>QQ群</h4><p>{{ qqGroup }}</p></div><span class="lc-go">查看群号</span></a>
          <a v-if="telegramGroupUrl" class="lc-card lc-community-card" :href="telegramGroupUrl" target="_blank" rel="noopener noreferrer"><span class="lc-community-mark">TG</span><div><h4>Telegram 群</h4><p>海外用户与实时公告</p></div><span class="lc-go">立即加入</span></a>
        </div>
      </section>

      <section class="lc-section">
        <div class="lc-section-head"><div class="lc-section-tag">FAQ</div><h2 class="lc-section-title">常见问题</h2></div>
        <div class="lc-faq">
          <article class="lc-card lc-faq-item"><h4><i>Q1</i>余额会过期吗？</h4><p>不会。充值余额永久有效，没有月租，用多少扣多少。</p></article>
          <article class="lc-card lc-faq-item"><h4><i>Q2</i>支持 Claude Code / Cursor 吗？</h4><p>支持。兼容 OpenAI 或 Anthropic 协议的客户端可以直接接入。</p></article>
        </div>
      </section>
    </div>

    <section class="lc-cta"><div class="lc-wrap lc-cta-inner"><h2 class="lc-section-title">现在，把 <span class="lc-title-accent">全球算力</span> 接入你的应用</h2><div class="lc-hero-actions" style="justify-content:center"><RouterLink to="/register" class="lc-button lc-button-primary">立即注册 <Icon name="arrowRight" size="sm" /></RouterLink><RouterLink to="/portal/status" class="lc-button">查看渠道状态</RouterLink></div></div></section>
  </PortalLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { RouterLink } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useAuthStore, useAppStore } from '@/stores'
import LocaleSwitcher from '@/components/common/LocaleSwitcher.vue'
import PortalLayout from '@/views/public/components/PortalLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import { parsePricingDisplayConfig } from '@/utils/pricingDisplayConfig'
import { sanitizeUrl } from '@/utils/url'
import { normalizeSiteName } from '@/utils/branding'

const authStore = useAuthStore()
const appStore = useAppStore()
const { t } = useI18n()
const settings = computed(() => appStore.cachedPublicSettings)
const siteName = computed(() => normalizeSiteName(settings.value?.site_name || appStore.siteName))
const siteLogo = computed(() => sanitizeUrl(settings.value?.site_logo || appStore.siteLogo || '', { allowRelative: true, allowDataUrl: true }))
const siteSubtitle = computed(() => settings.value?.site_subtitle || '')
const homeContent = computed(() => settings.value?.home_content || '')
const hasHomeContent = computed(() => homeContent.value.trim().length > 0)
const compactHomeEnabled = computed(() => settings.value?.compact_home_enabled === true)
const isHomeContentUrl = computed(() => /^https?:\/\//.test(homeContent.value.trim()))
const isAuthenticated = computed(() => authStore.isAuthenticated)
const dashboardPath = computed(() => (authStore.isAdmin ? '/admin/dashboard' : '/dashboard'))
const apiBaseUrl = computed(() => settings.value?.api_base_url?.trim() || '')
const normalizedApiBaseUrl = computed(() => {
  const configuredUrl = apiBaseUrl.value || window.location.origin
  return configuredUrl.replace(/\/+$/, '').replace(/\/v1$/i, '')
})
const telegramGroupUrl = computed(() => sanitizeUrl(settings.value?.telegram_group_url || ''))
const qqGroup = computed(() => settings.value?.qq_group?.trim() || '')
const docUrl = computed(() => sanitizeUrl(settings.value?.doc_url || appStore.docUrl || ''))
const pricingConfig = computed(() => parsePricingDisplayConfig(settings.value?.pricing_display_config || ''))
const pricingRange = computed(() => `¥${formatNumber(pricingConfig.value.yuanAmount)} = $${formatNumber(pricingConfig.value.usdAmount)} ${pricingConfig.value.creditUnitLabel}`)
const isDark = ref(document.documentElement.classList.contains('dark'))
const currentYear = new Date().getFullYear()

const modelMatrix = [
  { brand: 'Anthropic', models: ['Claude Opus 5', 'Claude Sonnet 5', 'Claude Haiku 4.5'] },
  { brand: 'OpenAI', models: ['gpt-5.6-luna', 'gpt-5.6-terra', 'gpt-5.6-sol'] },
  { brand: 'xAI', models: ['Grok 4.5'] },
]
const capabilities = [
  { icon: 'swap' as const, title: '三套协议兼容', description: 'OpenAI / Anthropic / Gemini 格式可直接调用。', rows: [['OpenAI', '/v1/responses'], ['Anthropic', '/v1/messages']] },
  { icon: 'chart' as const, title: '用量透明可查', description: '按 Key、模型和时间查看调用与费用明细。', rows: [['调用明细', '逐条可查'], ['余额有效期', '永久']] },
  { icon: 'terminal' as const, title: '客户端开箱即用', description: '常用编程工具与 SDK 无需重写即可接入。', rows: [['Claude Code', '支持'], ['Cursor / Cline', '支持']] },
  { icon: 'key' as const, title: '密钥粒度管控', description: '每个 Key 可限制模型范围与消费配额。', rows: [['模型白名单', '可设'], ['消费上限', '可设']] },
]
const steps = computed(() => [
  { title: '注册并充值', description: '注册后按需充值，余额长期有效。', code: '用多少 · 扣多少' },
  { title: '创建 API Key', description: '在控制台生成密钥，并配置模型范围。', code: 'sk-linkcode-xxxxxxxx' },
  { title: '替换 base_url', description: `将现有 SDK 的接口地址指向 ${siteName.value}。`, code: normalizedApiBaseUrl.value },
])

function formatNumber(value: number) {
  return Number.isInteger(value) ? String(value) : value.toFixed(2).replace(/\.?0+$/, '')
}

function toggleTheme() {
  isDark.value = !isDark.value
  document.documentElement.classList.toggle('dark', isDark.value)
  localStorage.setItem('theme', isDark.value ? 'dark' : 'light')
}

function initTheme() {
  const savedTheme = localStorage.getItem('theme')
  if (savedTheme === 'dark' || (!savedTheme && window.matchMedia('(prefers-color-scheme: dark)').matches)) {
    isDark.value = true
    document.documentElement.classList.add('dark')
  }
}

onMounted(() => {
  initTheme()
  if (!appStore.publicSettingsLoaded) void appStore.fetchPublicSettings()
  void authStore.checkAuth()
})
</script>
