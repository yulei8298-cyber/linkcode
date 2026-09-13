<template>
  <div>
    <label class="label">{{ label }}</label>
    <input
      :value="modelValue"
      type="number"
      class="input"
      :min="min"
      :max="max"
      @input="onInput"
    />
    <p v-if="hint" class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ hint }}</p>
  </div>
</template>

<script setup lang="ts">
/**
 * 带范围提示的数字输入。
 *
 * 存在的理由只有一个：`v-model.number` 在输入框被清空时会给出空字符串，
 * 而设置对象的字段类型是 number，直接绑上去会让整块 JSON 带着 "" 提交，
 * 后端反序列化即失败。这里统一在边界上兜住——非数字一律忽略，保留上一个有效值。
 *
 * 不在这里做 clamp：后端 Normalize 会把越界值收敛到合法区间，
 * 前端跟着再算一遍只会制造两处可能不一致的规则。min/max 只交给浏览器做提示。
 */
const props = defineProps<{
  modelValue: number
  label: string
  hint?: string
  min?: number
  max?: number
}>()

const emit = defineEmits<{
  (e: 'update:modelValue', value: number): void
}>()

function onInput(event: Event) {
  const raw = (event.target as HTMLInputElement).value
  const parsed = Number(raw)
  if (raw === '' || Number.isNaN(parsed)) {
    // 保留原值：把 0 写回去会让「清空重填」的中间态变成一次合法但错误的提交。
    emit('update:modelValue', props.modelValue)
    return
  }
  emit('update:modelValue', parsed)
}
</script>
