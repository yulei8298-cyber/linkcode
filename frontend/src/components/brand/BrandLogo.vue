<template>
  <span class="brand-logo" :style="{ gap: `${Math.round(size * 0.36)}px` }">
    <img
      v-if="customLogo"
      :src="customLogo"
      alt=""
      class="brand-logo-img"
      :style="{ width: `${size}px`, height: `${size}px` }"
    />
    <BrandMark v-else :size="size" />
    <template v-if="showName">
      <BrandWordmark v-if="isLinkCode" :height="Math.round(size * 0.6)" />
      <span v-else class="brand-logo-text" :style="{ fontSize: `${Math.round(size * 0.58)}px` }">{{ displayName }}</span>
    </template>
  </span>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import BrandMark from './BrandMark.vue'
import BrandWordmark from './BrandWordmark.vue'
import { normalizeSiteName } from '@/utils/branding'

// 站点名为 LinkCode 时使用手绘字标；其他站点名照常显示文字，兼容自部署改名。
// 后台上传了自定义 Logo 时优先使用；未上传或仍是默认 /logo.svg 时用内置图形标志。
const props = withDefaults(
  defineProps<{ name?: string; logoUrl?: string; size?: number; showName?: boolean }>(),
  { name: '', logoUrl: '', size: 28, showName: true },
)

const DEFAULT_LOGO = '/logo.svg'
const displayName = computed(() => normalizeSiteName(props.name))
const isLinkCode = computed(() => displayName.value === 'LinkCode')
const customLogo = computed(() => {
  const url = props.logoUrl.trim()
  return url && url !== DEFAULT_LOGO ? url : ''
})
</script>

<style scoped>
.brand-logo {
  display: inline-flex;
  align-items: center;
  min-width: 0;
  color: var(--lc-ink);
}

.brand-logo-img {
  flex: none;
  border-radius: 22%;
  object-fit: contain;
}

.brand-logo-text {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-weight: 700;
  letter-spacing: -0.01em;
}
</style>
