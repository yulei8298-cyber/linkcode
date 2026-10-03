<template>
  <div class="table-page-layout" :class="{ 'mobile-mode': isMobile }">
    <!-- 固定区域：操作按钮 + 搜索过滤，合并为一张工具栏卡片；筛选项换行时也落在卡片内 -->
    <div v-if="$slots.actions || $slots.filters" class="layout-section-fixed table-toolbar">
      <div v-if="$slots.actions" class="table-toolbar-row">
        <slot name="actions" />
      </div>
      <div v-if="$slots.filters" class="table-toolbar-row">
        <slot name="filters" />
      </div>
    </div>

    <!-- 滚动区域：表格 -->
    <div class="layout-section-scrollable">
      <div class="card table-scroll-container">
        <slot name="table" />
      </div>
    </div>

    <!-- 固定区域：分页器 -->
    <div v-if="$slots.pagination" class="layout-section-fixed">
      <slot name="pagination" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'

const isMobile = ref(false)

const checkMobile = () => {
  isMobile.value = window.innerWidth < 1024
}

onMounted(() => {
  checkMobile()
  window.addEventListener('resize', checkMobile)
})

onUnmounted(() => {
  window.removeEventListener('resize', checkMobile)
})
</script>

<style scoped>
/* 桌面端：Flexbox 布局 */
.table-page-layout {
  @apply flex flex-col gap-4;
  height: calc(100vh - 64px - 4rem); /* 减去 header + lg:p-8 的上下padding */
}

.layout-section-fixed {
  @apply flex-shrink-0;
}

.table-toolbar {
  display: grid;
  gap: 12px;
  padding: 12px 14px;
  border: 1px solid var(--lc-line);
  border-radius: 12px;
  background: var(--lc-surface);
}

.table-toolbar-row + .table-toolbar-row {
  padding-top: 12px;
  border-top: 1px solid var(--lc-line);
}

.layout-section-scrollable {
  @apply flex-1 min-h-0 flex flex-col;
}

/* 表格滚动容器 - 增强版表体滚动方案 */
.table-scroll-container {
  @apply flex flex-col overflow-hidden h-full bg-white dark:bg-dark-800 rounded-xl border border-gray-200 dark:border-dark-700;
}

.table-scroll-container :deep(.table-wrapper) {
  @apply flex-1 overflow-x-auto overflow-y-auto;
  /* 确保横向滚动条显示在最底部 */
  scrollbar-gutter: stable;
}

.table-scroll-container :deep(table) {
  @apply w-full;
  min-width: max-content; /* 关键：确保表格宽度根据内容撑开，从而触发横向滚动 */
  display: table; /* 使用标准 table 布局以支持 sticky 列 */
}

.table-scroll-container :deep(thead) {
  background: var(--lc-surface-2);
}

.table-scroll-container :deep(tbody) {
  /* 保持默认 table-row-group 显示，不使用 block */
}

.table-scroll-container :deep(th) {
  @apply px-4 py-3 text-left text-[12.5px] font-medium normal-case tracking-normal text-gray-500 dark:text-dark-400 border-b border-gray-200 dark:border-dark-700;
}

.table-scroll-container :deep(td) {
  @apply px-4 py-3 text-sm text-gray-700 dark:text-gray-300 border-b border-gray-100 dark:border-dark-700/60;
}

/* 移动端：恢复正常滚动 */
.table-page-layout.mobile-mode .table-toolbar {
  padding: 12px;
}

.table-page-layout.mobile-mode .table-scroll-container {
  @apply h-auto overflow-visible border-none shadow-none bg-transparent;
}

.table-page-layout.mobile-mode .layout-section-scrollable {
  @apply flex-none min-h-fit;
}

.table-page-layout.mobile-mode .table-scroll-container :deep(table) {
  @apply flex-none;
  display: table;
  min-width: 100%;
}
</style>
