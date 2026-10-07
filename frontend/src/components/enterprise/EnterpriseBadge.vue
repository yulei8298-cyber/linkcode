<template>
  <span class="enterprise-badge" data-test="enterprise-badge" :title="t('enterprise.badgeTitle')">
    <svg class="enterprise-badge-gem" viewBox="0 0 16 16" width="11" height="11" aria-hidden="true" focusable="false">
      <path d="M3.2 2.5h9.6L15 6.2 8 14 1 6.2z" fill="currentColor" opacity="0.9" />
      <path d="M1 6.2h14M5.4 6.2 8 14l2.6-7.8M5.4 6.2 3.2 2.5M10.6 6.2l2.2-3.7" fill="none" stroke="var(--lc-surface, #ffffff)" stroke-width="0.9" stroke-linejoin="round" />
    </svg>
    {{ t('enterprise.badge') }}
  </span>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'

const { t } = useI18n()
</script>

<style scoped>
/* 企业尊享标识：香槟金小胶囊，与品牌橙同为暖色；浅色、深色主题各自校准对比度 */
.enterprise-badge {
  --eb-fg: #8a5a12;
  --eb-bg: linear-gradient(135deg, #fdf3d8, #f6e2ac);
  --eb-line: rgba(176, 124, 30, 0.45);
  display: inline-flex;
  flex: none;
  align-items: center;
  gap: 4px;
  padding: 2px 8px;
  border: 1px solid var(--eb-line);
  border-radius: 999px;
  background: var(--eb-bg);
  color: var(--eb-fg);
  font-size: 11px;
  font-weight: 600;
  line-height: 1.5;
  letter-spacing: 0.04em;
  white-space: nowrap;
}

</style>

<!-- 深色主题：不带作用域，才能用 html.dark 后代选择器覆盖上面的变量 -->
<style>
html.dark .enterprise-badge {
  --eb-fg: #f2c46d;
  --eb-bg: rgba(242, 196, 109, 0.12);
  --eb-line: rgba(242, 196, 109, 0.45);
}
</style>
