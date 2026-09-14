<template>
  <div ref="wrapper" class="ic-artwork" :style="{ height: `${height}px` }">
    <!--
      安全边界，改动前务必读完这段。

      sandbox="allow-scripts" 且**不给** allow-same-origin —— 这两个值同时出现
      等于完全解除沙箱（浏览器会给出与无沙箱一致的权限），届时这段由模型生成、
      我们无法预先审阅的代码就能读取父页面、cookie 与登录态。

      为什么必须允许脚本：产物的动画机制包括 requestAnimationFrame 驱动的
      逆向运动学（观察到的样例用它实时求解膝关节位置），禁用脚本会让一部分
      合格产物在预览里变成静止图片，而"能不能动"正是判定要点之一。

      ⚠️ 服务端已不再清洗产物（见后端 SanitizeIntelCheckDrawing 的注释）。
      外层改为独立 HTTP 预览外壳，避免 srcdoc 继承主站 nonce CSP 把动画拦掉。
      外壳的响应头与内层 iframe 也施加沙箱，不给任何一层 allow-same-origin。
      任何人想往里加 allow-same-origin，请先去后端那段注释里看清代价。
    -->
    <iframe
      v-if="html"
      ref="frame"
      :src="previewURL"
      sandbox="allow-scripts"
      referrerpolicy="no-referrer"
      class="ic-artwork-frame"
      :style="frameStyle"
      :title="title"
      @load="sendArtwork"
    ></iframe>
    <div v-else class="ic-artwork-empty">
      <span>{{ emptyText }}</span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { buildApiUrl } from '@/api/url'

const props = withDefaults(
  defineProps<{
    /** 模型产出的完整 HTML 文档（未经改写）。空串表示本次没有产出。 */
    html: string
    title?: string
    height?: number
    emptyText?: string
  }>(),
  {
    title: '模型绘图产物预览',
    height: 240,
    emptyText: '本次没有产出可展示的画作',
  },
)

/**
 * 预览用「宽视口 + 等比缩放」呈现，有两处坑是踩过才知道的：
 *
 * 1. **不能只给 iframe 一个窄宽度**。产物普遍带响应式分支
 *    （观察到的样例写着 `matchMedia("(max-width: 600px)")`，窄屏时会把
 *    viewBox 裁成中间特写）。iframe 内部视口若落进窄屏区间，我们看到的
 *    就不是作品本身，而是它的移动端裁切版。故逻辑宽度固定在桌面区间。
 *
 * 2. **不能用 transform: scale() 缩放 iframe**。产物的动画常靠脚本实时改写
 *    `<defs>` 里 path 的 `d` 属性、再由 `<use>` 引用渲染（逆向运动学求解
 *    膝关节就是这么做的）。iframe 一旦进入被 scale 的合成层，Chrome 对
 *    shadow DOM 内 `d` 属性变更的重绘会漏帧——表现为「脚在动、腿不动」，
 *    也就是"脚不踩单车"。而同一份产物单独打开却完全正常。
 *
 * 所以改用 CSS `zoom`：它参与布局而非合成变换，不会把 iframe 推进独立的
 * 合成层，因此上述重绘问题不出现；视觉效果与 scale 等价。
 * Chrome/Safari 长期支持，Firefox 126+ 起标准化支持，旧版 Firefox 上退化为
 * 不缩放（画面被容器裁切，仍可读），不影响判定。
 */
const LOGICAL_WIDTH = 1100
const previewURL = buildApiUrl('/public/intel-check/preview')

const wrapper = ref<HTMLElement | null>(null)
const frame = ref<HTMLIFrameElement | null>(null)
const zoom = ref(1)
let observer: ResizeObserver | null = null

function sendArtwork() {
  // 外壳为不透明源，只能用 *；目标窗口来自固定外壳，不广播，不传登录态。
  frame.value?.contentWindow?.postMessage({
    type: 'intel-check-preview', html: props.html, title: props.title,
  }, '*')
}

watch(() => [props.html, props.title], sendArtwork, { flush: 'post' })

function measure() {
  const width = wrapper.value?.clientWidth ?? 0
  if (width <= 0) return
  // 只按宽度算：高度方向由容器 overflow 裁切。产物的页高不可预知
  // （有的带页头页脚、有的 100vh），按高度缩放会让不同产物的画面大小不一致，
  // 横向对比时反而更难看出差别。
  zoom.value = Math.min(1, width / LOGICAL_WIDTH)
}

const frameStyle = computed(() => ({
  width: `${LOGICAL_WIDTH}px`,
  // 高度按缩放反推，让 iframe 缩放后正好填满容器高度，
  // 使产物内部的 100vh 布局拿到一个合理的视口高度。
  height: `${Math.round(props.height / zoom.value)}px`,
  zoom: zoom.value,
}))

onMounted(() => {
  measure()
  if (typeof ResizeObserver !== 'undefined' && wrapper.value) {
    observer = new ResizeObserver(() => measure())
    observer.observe(wrapper.value)
  }
})

onBeforeUnmount(() => {
  observer?.disconnect()
  observer = null
})
</script>

<style scoped>
.ic-artwork {
  position: relative;
  width: 100%;
  overflow: hidden;
  /* 天空渐变底：模型产出常带透明背景，纯白底会让白色造型整个消失，
     纯深色底又会吃掉深色线条。 */
  background: linear-gradient(180deg, #e0f2fe, #fef3c7);
}

.ic-artwork-frame {
  display: block;
  border: 0;
}

.ic-artwork-empty {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 100%;
  font-size: 13px;
  /* 底色是浅色天空渐变，文字色必须跟着定死为深色——
     若留给继承，深色主题下会变成浅字压浅底。 */
  color: #64748b;
}
</style>
