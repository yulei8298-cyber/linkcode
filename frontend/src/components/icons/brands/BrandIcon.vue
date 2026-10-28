<template>
  <svg
    v-if="icon"
    :class="sizeClass"
    :viewBox="icon.viewBox"
    fill="currentColor"
    :fill-rule="icon.fillRule"
    aria-hidden="true"
  >
    <path
      v-for="(path, index) in icon.paths"
      :key="index"
      :d="path.d"
      :fill-rule="path.fillRule"
      :fill-opacity="path.opacity"
    />
  </svg>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { BRAND_ICONS } from './brandIcons'

interface Props {
  /** 品牌标识，见 brandIcons.ts 的键（如 kling、hailuo、gemini）。 */
  brand: string
  size?: 'xs' | 'sm' | 'md' | 'lg'
}

const props = withDefaults(defineProps<Props>(), { size: 'sm' })

const icon = computed(() => BRAND_ICONS[props.brand])

const sizeClass = computed(() => {
  const sizes = { xs: 'w-3 h-3', sm: 'w-3.5 h-3.5', md: 'w-4 h-4', lg: 'w-5 h-5' }
  return sizes[props.size] + ' flex-shrink-0'
})
</script>
