<template>
  <!--
    LinkCode 字标：与图形同笔画的几何线条，不依赖字体文件。
    i 上的点是品牌橙节点，与图形终点呼应——这是品牌固定元素，任何场景都保留。
  -->
  <svg
    class="brand-wordmark"
    :height="height"
    :width="(height * 139) / 28"
    viewBox="0 0 139 28"
    role="img"
    aria-label="LinkCode"
  >
    <g
      fill="none"
      stroke="currentColor"
      stroke-width="3"
      stroke-linecap="round"
      stroke-linejoin="round"
    >
      <path d="M1.5,4 V18 a6,6 0 0 0 6,6 H13" />
      <path d="M19,10 V24" />
      <circle cx="19" cy="4.6" r="2.15" fill="var(--lc-accent)" stroke="none" />
      <path d="M25,10 V24 M25,16.5 a6.5,6.5 0 0 1 13,0 V24" />
      <path d="M44,4 V24 M54,10 L44,19.5 M47.6,16.2 L54.5,24" />
      <path d="M77.07,6.93 A10,10 0 1 0 77.07,21.07" />
      <circle cx="89" cy="17" r="7" />
      <circle cx="109" cy="17" r="7" />
      <path d="M116,4 V24" />
      <path d="M122,17 H136 A7,7 0 1 0 133.95,21.95" />
    </g>
  </svg>
</template>

<script setup lang="ts">
withDefaults(defineProps<{ height?: number }>(), { height: 17 })
</script>

<style scoped>
.brand-wordmark {
  flex: none;
  display: block;
}
</style>
