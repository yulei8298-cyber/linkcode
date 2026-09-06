<script setup lang="ts">
import { computed, ref } from 'vue'
import { adminAPI } from '@/api/admin'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import { extractApiErrorMessage } from '@/utils/apiError'

const model = ref('')
const loading = ref(false)
const saving = ref(false)
const message = ref('')
const error = ref('')
const edited = ref(false)
const values = ref<Record<string, string>>({})
const fields = [
  ['input_price', '输入 token'], ['output_price', '输出 token'],
  ['cache_write_price', '缓存写入（5m）'], ['cache_write_1h_price', '缓存写入（1h）'],
  ['cache_read_price', '缓存读取'], ['image_input_price', '图片输入 token'],
  ['image_output_price', '图片输出 token'], ['per_image_price', '每张图片']
] as const

const hasModel = computed(() => model.value.trim().length > 0)

async function load() {
  if (!hasModel.value) return
  loading.value = true; error.value = ''; message.value = ''
  try {
    const result = await adminAPI.modelBasePricing.getModelBasePricing(model.value.trim())
    edited.value = result.edited
    const catalog = result.catalog as Record<string, number> | null
    const override = result.override as Record<string, number | null> | undefined
    values.value = Object.fromEntries(fields.map(([key]) => {
      const overrideKey = key
      const catalogKey = ({
        input_price: 'input_cost_per_token', output_price: 'output_cost_per_token',
        cache_write_price: 'cache_creation_input_token_cost', cache_write_1h_price: 'cache_creation_input_token_cost_above_1hr',
        cache_read_price: 'cache_read_input_token_cost', image_input_price: 'input_cost_per_image_token',
        image_output_price: 'output_cost_per_image_token', per_image_price: 'output_cost_per_image'
      } as Record<string, string>)[key]
      const value = override?.[overrideKey] ?? catalog?.[catalogKey]
      return [key, value == null ? '' : String(value)]
    }))
  } catch (err: unknown) { error.value = extractApiErrorMessage(err) }
  finally { loading.value = false }
}

async function save() {
  if (!hasModel.value) return
  saving.value = true; error.value = ''; message.value = ''
  try {
    const price = Object.fromEntries(Object.entries(values.value).filter(([, value]) => value.trim() !== '').map(([key, value]) => [key, Number(value)]))
    await adminAPI.modelBasePricing.updateModelBasePricing(model.value.trim(), price)
    edited.value = true; message.value = '已保存，模型广场会立即使用新的基准价。'
  } catch (err: unknown) { error.value = extractApiErrorMessage(err) }
  finally { saving.value = false }
}

async function restore() {
  if (!hasModel.value) return
  saving.value = true; error.value = ''; message.value = ''
  try {
    await adminAPI.modelBasePricing.updateModelBasePricing(model.value.trim(), null)
    edited.value = false; message.value = '已恢复官方目录价格。'; await load()
  } catch (err: unknown) { error.value = extractApiErrorMessage(err) }
  finally { saving.value = false }
}
</script>

<template>
  <AppLayout>
    <div class="mx-auto max-w-5xl space-y-6 p-6">
      <div>
        <p class="text-xs font-semibold uppercase tracking-[0.18em] text-primary-600">Pricing control</p>
        <h1 class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">模型基准价</h1>
        <p class="mt-2 text-sm text-gray-500 dark:text-gray-400">官方目录价格作为默认基准，修改后所有真实分组按各自倍率计算实付价格。留空字段会继续继承官方目录，0 可以表示免费。</p>
      </div>

      <div class="rounded-xl border border-gray-200 bg-white p-5 shadow-sm dark:border-dark-500 dark:bg-dark-800">
        <label class="text-sm font-medium text-gray-700 dark:text-gray-200">模型名称</label>
        <div class="mt-2 flex flex-col gap-3 sm:flex-row">
          <input v-model="model" class="input flex-1" placeholder="例如 gpt-5.5、claude-sonnet-4-6" @keyup.enter="load" />
          <button class="btn btn-primary sm:min-w-28" :disabled="loading || !hasModel" @click="load"><Icon name="search" size="sm" class="mr-2" />{{ loading ? '读取中…' : '读取价格' }}</button>
        </div>
      </div>

      <div v-if="hasModel && (Object.keys(values).length || loading)" class="rounded-xl border border-gray-200 bg-white p-5 shadow-sm dark:border-dark-500 dark:bg-dark-800">
        <div class="flex flex-wrap items-center justify-between gap-3">
          <div><h2 class="font-medium text-gray-900 dark:text-white">{{ model }}</h2><p class="mt-1 text-xs text-gray-500">单位：USD / token；每张图片按 USD / image。</p></div>
          <span v-if="edited" class="rounded-full bg-amber-100 px-3 py-1 text-xs font-medium text-amber-800 dark:bg-amber-900/30 dark:text-amber-300">已覆盖官方价</span>
        </div>
        <div class="mt-5 grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
          <label v-for="[key, label] in fields" :key="key" class="text-sm text-gray-600 dark:text-gray-300">{{ label }}<input v-model="values[key]" type="number" min="0" step="any" class="input mt-1" placeholder="继承官方" /></label>
        </div>
        <div class="mt-5 flex flex-wrap items-center gap-3"><button class="btn btn-primary" :disabled="saving" @click="save">保存基准价</button><button class="btn btn-secondary" :disabled="saving || !edited" @click="restore">恢复官方目录</button><span v-if="message" class="text-sm text-emerald-600">{{ message }}</span><span v-if="error" class="text-sm text-red-600">{{ error }}</span></div>
      </div>
    </div>
  </AppLayout>
</template>
