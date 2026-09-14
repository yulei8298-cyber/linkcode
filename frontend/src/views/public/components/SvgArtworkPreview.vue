<template>
  <div ref="wrapper" class="ic-artwork" :style="{ height: `${height}px` }">
    <!--
      安全边界，改动前务必读完这段。

      sandbox="allow-scripts" 且**不给** allow-same-origin —— 这两个值同时出现
      等于完全解除沙箱（浏览器会给出与无沙箱一致的权限），届时这段由模型生成、
      我们无法预先审阅的代码就能读取父页面、cookie 与登录态。

      为什么必须允许脚本：产物的动画机制包括 requestAnimationFrame、
      pauseAnimations() 这类纯 JS 实现，禁用脚本会让一部分合格产物在预览里
      变成静止图片，而"能不能动"正是判定要点之一。

      ⚠️ 服务端已不再清洗产物（见后端 SanitizeIntelCheckDrawing 的注释：
      清洗会改写 SVG 的自闭合写法与属性大小写，让动画悄悄失效，
      而这张页面的全部价值就在于如实展示模型画了什么）。
      因此**这个 sandbox 属性现在是唯一的防护层**，不是"三层之一"。
      任何人想往里加 allow-same-origin，请先去后端那段注释里看清代价。
    -->
    <iframe
      v-if="html"
      :srcdoc="html"
      sandbox="allow-scripts"
      referrerpolicy="no-referrer"
      class="ic-artwork-frame"
      :style="frameStyle"
      :title="title"
    ></iframe>
    <div v-else class="ic-artwork-empty">
      <span>{{ emptyText }}</span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'

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
 * 产物是一份完整网页，按桌面视口写的（常见 900–1200px 宽，且多半带
 * `min-height:100vh` 与固定 aspect-ratio）。直接塞进一个几百像素的框里，
 * 它会按框的宽度铺开、再按自己的比例算高度，结果高出容器、下半截被裁掉——
 * 表现出来就是"画得不对/动画不对"，而其实产物本身是好的。
 *
 * 所以按缩略图的通行做法：让 iframe 以固定的逻辑尺寸渲染（等于给它一个
 * 桌面视口），再整体缩放到容器里。这样看到的是完整构图，比例也与
 * 用户自己打开这份 HTML 时一致。
 *
 * 逻辑高度取 720 而非 600：产物常把画面包在标题栏 + 页脚里（观察到的样例
 * 就带页头标题与底部按钮条），600 会把页脚挤出视口。宁可上下留白，
 * 也不要裁掉内容——裁掉的那部分往往正是"有没有做完"的证据。
 */
const LOGICAL_WIDTH = 1000
const LOGICAL_HEIGHT = 720

const wrapper = ref<HTMLElement | null>(null)
const scale = ref(1)
let observer: ResizeObserver | null = null

function measure() {
  const width = wrapper.value?.clientWidth ?? 0
  if (width <= 0) return
  // 宽高都要放得下，取较小的那个比例，避免某一边溢出。
  scale.value = Math.min(width / LOGICAL_WIDTH, props.height / LOGICAL_HEIGHT)
}

const frameStyle = computed(() => ({
  width: `${LOGICAL_WIDTH}px`,
  height: `${LOGICAL_HEIGHT}px`,
  transform: `scale(${scale.value})`,
  // 缩放后按容器居中：translate 的百分比是相对元素自身的，
  // 所以先移到中心再缩放（transform 从右往左生效）。
  left: '50%',
  top: '50%',
  marginLeft: `${-LOGICAL_WIDTH / 2}px`,
  marginTop: `${-LOGICAL_HEIGHT / 2}px`,
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
  position: absolute;
  display: block;
  border: 0;
  transform-origin: center center;
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
