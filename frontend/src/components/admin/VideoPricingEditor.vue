<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import type { VideoPricingSpec, VideoTierPrice } from '@/api/admin/modelBasePricing'
import { estimateVideoCost, VIDEO_TIERS } from '@/utils/videoPricing'

const props = defineProps<{
  modelValue: VideoPricingSpec | null
  /** 后端内置参考价，可一键载入。 */
  defaultSpec?: VideoPricingSpec | null
}>()
const emit = defineEmits<{ 'update:modelValue': [value: VideoPricingSpec | null] }>()

type Unit = VideoPricingSpec['unit']
interface DurationRow { seconds: string; price: string }
interface TierRow { tier: string; perSecond: string; perSecondWithAudio: string; perVideo: DurationRow[] }

const units: Array<{ value: Unit; label: string; hint: string }> = [
  { value: 'per_second', label: '按秒', hint: '费用 = 每秒价格 × 实际时长（可单独设置带音频价）' },
  { value: 'per_video', label: '按次（条）', hint: '按分辨率 × 时长档收费；时长取不小于实际时长的最短档' },
  { value: 'token', label: '按 token', hint: '按上游回报的输出 token 计费（火山方舟 Seedance 官方口径）' },
]

const enabled = ref(false)
const unit = ref<Unit>('per_second')
const rows = ref<TierRow[]>([])
const tokenPrice = ref('')
const defaultResolution = ref('')
const defaultDuration = ref('')
const minDuration = ref('')
const maxDuration = ref('')

const text = (v: number | null | undefined) => (v == null ? '' : String(v))

function load(spec: VideoPricingSpec | null | undefined) {
  enabled.value = !!spec
  unit.value = spec?.unit ?? 'per_second'
  rows.value = VIDEO_TIERS.map(({ value }) => {
    const tier: VideoTierPrice = spec?.prices?.[value] ?? {}
    return {
      tier: value,
      perSecond: text(tier.per_second),
      perSecondWithAudio: text(tier.per_second_with_audio),
      perVideo: Object.entries(tier.per_video ?? {}).map(([seconds, price]) => ({ seconds, price: String(price) })),
    }
  })
  tokenPrice.value = text(spec?.token_price_per_million)
  defaultResolution.value = spec?.default_resolution ?? ''
  defaultDuration.value = text(spec?.default_duration_seconds)
  minDuration.value = text(spec?.min_duration_seconds)
  maxDuration.value = text(spec?.max_duration_seconds)
}

// type="number" 输入框经 v-model 会产生数字，清空后又是空串，统一按字符串判断是否填写。
const filled = (v: unknown) => String(v ?? '').trim() !== ''
const num = (v: unknown): number | undefined => (filled(v) ? Number(v) : undefined)
const int = (v: unknown): number | undefined => (filled(v) ? Math.round(Number(v)) : undefined)

const spec = computed<VideoPricingSpec | null>(() => {
  if (!enabled.value) return null
  const prices: Record<string, VideoTierPrice> = {}
  for (const row of rows.value) {
    if (unit.value === 'per_second' && num(row.perSecond) !== undefined) {
      prices[row.tier] = { per_second: num(row.perSecond), per_second_with_audio: num(row.perSecondWithAudio) }
    }
    if (unit.value === 'per_video') {
      const perVideo = Object.fromEntries(
        row.perVideo.filter(r => filled(r.seconds) && filled(r.price)).map(r => [String(r.seconds).trim(), Number(r.price)])
      )
      if (Object.keys(perVideo).length) prices[row.tier] = { per_video: perVideo }
    }
  }
  return {
    unit: unit.value,
    prices: unit.value === 'token' ? undefined : prices,
    token_price_per_million: unit.value === 'token' ? num(tokenPrice.value) : undefined,
    default_resolution: defaultResolution.value || undefined,
    default_duration_seconds: int(defaultDuration.value),
    min_duration_seconds: int(minDuration.value),
    max_duration_seconds: int(maxDuration.value),
  }
})

const preview = computed(() => {
  if (!spec.value || spec.value.unit === 'token') return ''
  const resolution = spec.value.default_resolution || '720p'
  const seconds = spec.value.default_duration_seconds || 5
  const cost = estimateVideoCost(spec.value, resolution, seconds)
  return cost == null ? '' : `示例：${resolution} · ${seconds} 秒 ≈ $${cost.toFixed(4)}`
})

// 初始化必须无条件载入（即使 modelValue 为 null 也要生成各分辨率档的空行）；
// 之后只在外部值与当前编辑结果不一致时重新载入，避免回写自身输出时打断输入。
load(props.modelValue)
watch(() => props.modelValue, value => {
  if (JSON.stringify(value) !== JSON.stringify(spec.value)) load(value)
})
watch(spec, value => emit('update:modelValue', value), { deep: true })

function addDuration(row: TierRow) {
  row.perVideo.push({ seconds: '', price: '' })
}
</script>

<template>
  <div class="rounded-lg border border-fuchsia-200 p-4 dark:border-fuchsia-900/50" data-testid="video-pricing-editor">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <label class="flex items-center gap-2 text-sm font-medium text-gray-800 dark:text-gray-100">
        <input v-model="enabled" type="checkbox" class="rounded border-gray-300 text-fuchsia-600" data-testid="video-pricing-enabled" />
        视频计费（视频平台模型）
      </label>
      <button v-if="defaultSpec" type="button" class="btn btn-secondary btn-sm" data-testid="video-pricing-load-default" @click="load(defaultSpec)">载入内置参考价</button>
    </div>
    <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">未配置时使用内置参考价；内置也没有的模型网关会拒绝生成，避免未计价使用。价格单位 USD，最终实付再乘分组倍率（或视频独立倍率）。</p>

    <div v-if="enabled" class="mt-4 space-y-4">
      <div class="flex flex-wrap gap-2">
        <button
          v-for="item in units" :key="item.value" type="button"
          :class="['rounded-md border px-3 py-1.5 text-sm', unit === item.value ? 'border-fuchsia-500 bg-fuchsia-50 text-fuchsia-700 dark:bg-fuchsia-900/20 dark:text-fuchsia-300' : 'border-gray-200 text-gray-600 dark:border-dark-600 dark:text-gray-300']"
          :data-testid="`video-unit-${item.value}`" @click="unit = item.value"
        >{{ item.label }}</button>
      </div>
      <p class="text-xs text-gray-500 dark:text-gray-400">{{ units.find(u => u.value === unit)?.hint }}</p>

      <table v-if="unit === 'per_second'" class="w-full text-sm">
        <thead><tr class="text-left text-xs text-gray-500"><th class="py-1">分辨率</th><th>每秒价格</th><th>带音频每秒价格（可选）</th></tr></thead>
        <tbody>
          <tr v-for="row in rows" :key="row.tier">
            <td class="py-1 pr-3 font-mono">{{ VIDEO_TIERS.find(t => t.value === row.tier)?.label }}</td>
            <td class="pr-3"><input v-model="row.perSecond" type="number" min="0" step="any" class="input" placeholder="不设置" :data-testid="`video-per-second-${row.tier}`" /></td>
            <td><input v-model="row.perSecondWithAudio" type="number" min="0" step="any" class="input" placeholder="同每秒价格" /></td>
          </tr>
        </tbody>
      </table>

      <div v-else-if="unit === 'per_video'" class="space-y-3">
        <div v-for="row in rows" :key="row.tier" class="rounded border border-gray-100 p-3 dark:border-dark-600">
          <div class="flex items-center justify-between">
            <span class="font-mono text-sm">{{ VIDEO_TIERS.find(t => t.value === row.tier)?.label }}</span>
            <button type="button" class="text-xs text-fuchsia-600 hover:underline" :data-testid="`video-add-duration-${row.tier}`" @click="addDuration(row)">+ 时长档</button>
          </div>
          <div v-for="(item, index) in row.perVideo" :key="index" class="mt-2 flex items-center gap-2">
            <input v-model="item.seconds" class="input w-28" placeholder="秒数或 *" />
            <span class="text-xs text-gray-500">秒 → $</span>
            <input v-model="item.price" type="number" min="0" step="any" class="input w-32" placeholder="每条价格" />
            <button type="button" class="text-xs text-red-500" @click="row.perVideo.splice(index, 1)">删除</button>
          </div>
        </div>
      </div>

      <label v-else class="block text-sm text-gray-600 dark:text-gray-300">每百万输出 token 价格
        <input v-model="tokenPrice" type="number" min="0" step="any" class="input mt-1" placeholder="例如 46" data-testid="video-token-price" />
      </label>

      <div class="grid gap-3 sm:grid-cols-4">
        <label class="text-xs text-gray-600 dark:text-gray-300">默认分辨率
          <select v-model="defaultResolution" class="input mt-1"><option value="">720p</option><option v-for="t in VIDEO_TIERS.filter(t => t.value !== '*')" :key="t.value" :value="t.value">{{ t.label }}</option></select>
        </label>
        <label class="text-xs text-gray-600 dark:text-gray-300">默认时长（秒）<input v-model="defaultDuration" type="number" min="1" class="input mt-1" placeholder="5" /></label>
        <label class="text-xs text-gray-600 dark:text-gray-300">最短计费时长<input v-model="minDuration" type="number" min="0" class="input mt-1" placeholder="不限" /></label>
        <label class="text-xs text-gray-600 dark:text-gray-300">最长计费时长<input v-model="maxDuration" type="number" min="0" class="input mt-1" placeholder="不限" /></label>
      </div>
      <p v-if="preview" class="text-xs text-fuchsia-700 dark:text-fuchsia-300" data-testid="video-pricing-preview">{{ preview }}</p>
    </div>
  </div>
</template>
