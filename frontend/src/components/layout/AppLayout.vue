<template>
  <div class="min-h-screen bg-[var(--lc-bg)] dark:bg-dark-950">
    <!-- Sidebar -->
    <AppSidebar />

    <!-- Main Content Area -->
    <div
      class="relative min-h-screen transition-[margin] duration-200"
      :class="[sidebarCollapsed ? 'lg:ml-[72px]' : 'lg:ml-64']"
    >
      <!-- Header -->
      <AppHeader />

      <!-- Main Content -->
      <main class="p-4 md:p-6 lg:p-8">
        <slot />
      </main>
    </div>
  </div>
</template>

<script lang="ts">
// 控制台观感开关（字体平滑、页面底色）挂在 <html> 上（弹窗会 teleport 到 body，挂在布局根节点上管不到）。
// 切换页面时新旧布局的挂载与卸载可能交错，用计数保证仍有布局在时不被提前移除。
const CONSOLE_CLASS = 'lc-console'
let mountedConsoleLayouts = 0
</script>

<script setup lang="ts">
import '@/styles/onboarding.css'
import { computed, onBeforeUnmount, onMounted } from 'vue'
import { useAppStore } from '@/stores'
import { useAuthStore } from '@/stores/auth'
import { useOnboardingTour } from '@/composables/useOnboardingTour'
import { useOnboardingStore } from '@/stores/onboarding'
import AppSidebar from './AppSidebar.vue'
import AppHeader from './AppHeader.vue'

const appStore = useAppStore()
const authStore = useAuthStore()
const sidebarCollapsed = computed(() => appStore.sidebarCollapsed)
const isAdmin = computed(() => authStore.user?.role === 'admin')

const { replayTour } = useOnboardingTour({
  storageKey: isAdmin.value ? 'admin_guide' : 'user_guide',
  autoStart: true
})

const onboardingStore = useOnboardingStore()

onMounted(() => {
  onboardingStore.setReplayCallback(replayTour)
  mountedConsoleLayouts++
  document.documentElement.classList.add(CONSOLE_CLASS)
})

onBeforeUnmount(() => {
  mountedConsoleLayouts = Math.max(0, mountedConsoleLayouts - 1)
  if (mountedConsoleLayouts === 0) document.documentElement.classList.remove(CONSOLE_CLASS)
})

defineExpose({ replayTour })
</script>
