<template>
  <!-- LinkCode 图形标志：一条 L 形路由线连接两个节点，终点为品牌橙。颜色随主题变量切换。 -->
  <svg
    class="brand-mark"
    :width="size"
    :height="size"
    viewBox="0 0 32 32"
    aria-hidden="true"
    focusable="false"
  >
    <rect width="32" height="32" rx="9" fill="var(--lc-logo-bg)" />
    <path
      d="M10,8 V15.5 a7,7 0 0 0 7,7 H22.5"
      fill="none"
      stroke="var(--lc-logo-fg)"
      stroke-width="3"
      stroke-linecap="round"
    />
    <circle cx="10" cy="8" r="3.1" fill="var(--lc-logo-fg)" />
    <circle cx="22.8" cy="22.5" r="3.5" fill="var(--lc-logo-node)" />
  </svg>
</template>

<script setup lang="ts">
withDefaults(defineProps<{ size?: number }>(), { size: 28 })
</script>

<style scoped>
.brand-mark {
  flex: none;
  display: block;
}
</style>
