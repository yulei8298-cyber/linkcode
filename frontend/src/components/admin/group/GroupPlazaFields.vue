<script setup lang="ts">
import { computed } from 'vue'
import type { ModelsListState } from '@/views/admin/groupsModelsList'

const props = defineProps<{ modelValue: ModelsListState }>()
const emit = defineEmits<{ 'update:modelValue': [ModelsListState] }>()
const update = (patch: Partial<ModelsListState>) => emit('update:modelValue', { ...props.modelValue, ...patch, plazaConfigured: true })
const selected = computed(() => props.modelValue.plazaModels ?? [])
const candidates = computed(() => [...new Set([...props.modelValue.items.map(item => item.id), ...props.modelValue.savedModels, ...selected.value])])
const modelText = computed({
  get: () => selected.value.join('\n'),
  set: value => update({ plazaModels: [...new Set(value.split(/[\n,，]/).map(name => name.trim()).filter(Boolean))] })
})
function toggle(name: string) {
  update({ plazaModels: selected.value.includes(name) ? selected.value.filter(value => value !== name) : [...selected.value, name] })
}
</script>

<template>
  <section class="mt-4 space-y-3 rounded-xl border border-orange-200 bg-orange-50/50 p-4 dark:border-orange-900/60 dark:bg-orange-950/10" aria-label="模型广场展示设置">
    <label class="flex items-center gap-2 text-sm font-medium text-gray-800 dark:text-gray-200">
      <input type="checkbox" :checked="modelValue.plazaEnabled ?? true" @change="update({ plazaEnabled: ($event.target as HTMLInputElement).checked })" />
      展示到模型广场
    </label>
    <p class="text-xs text-gray-500">仅控制模型广场展示。分组状态、调用权限和 API 模型列表由原有设置控制。</p>
    <div v-if="modelValue.plazaEnabled ?? true" class="space-y-3">
      <label class="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300">
        <input type="checkbox" :checked="modelValue.plazaCustomModels" @change="update({ plazaCustomModels: ($event.target as HTMLInputElement).checked })" />
        指定模型广场展示模型
      </label>
      <p v-if="!modelValue.plazaCustomModels" class="text-xs text-gray-500">自动展示分组模型列表与渠道定价中配置的模型。</p>
      <template v-else>
        <div class="flex flex-wrap gap-3 text-xs">
          <button type="button" class="text-primary-600" @click="update({ plazaModels: [...candidates] })">全选</button>
          <button type="button" class="text-primary-600" @click="update({ plazaModels: [] })">清空</button>
          <span class="text-gray-500">已选择 {{ selected.length }} 个模型</span>
        </div>
        <div class="flex max-h-40 flex-wrap gap-2 overflow-y-auto">
          <button v-for="name in candidates" :key="name" type="button" :aria-pressed="selected.includes(name)"
            class="rounded-md border px-2 py-1 text-xs"
            :class="selected.includes(name) ? 'border-orange-400 bg-orange-100 text-orange-900 dark:bg-orange-900/40 dark:text-orange-200' : 'border-gray-200 text-gray-500 dark:border-dark-500'"
            @click="toggle(name)">{{ name }}</button>
        </div>
        <label class="block text-xs text-gray-600 dark:text-gray-300">展示模型（每行一个完整模型名称）
          <textarea v-model="modelText" rows="4" class="input mt-1 font-mono text-xs" placeholder="gpt-5.5&#10;gpt-5.4" />
        </label>
        <p class="text-xs text-gray-500">仅展示所选模型；清空后该分组不在广场显示。配置完成后点击分组弹窗底部“保存”。</p>
      </template>
    </div>
  </section>
</template>
