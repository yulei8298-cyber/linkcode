<template>
  <div class="lc-ic-artwork" :style="{ height: `${height}px` }">
    <!--
      安全边界，改动前务必读完这段。

      sandbox="allow-scripts" 且**不给** allow-same-origin —— 这两个值同时出现
      等于完全解除沙箱（浏览器会给出与无沙箱一致的权限），届时这段由模型生成、
      我们无法预先审阅的代码就能读取父页面、cookie 与登录态。

      为什么必须允许脚本：参考稿与候选稿的动画机制包括 requestAnimationFrame 这类
      纯 JS 实现（见设计文档 §5.2 的实测——三份样例分别用了 script / smil / css），
      禁用脚本会让一部分合格产物在预览里变成静止图片，而"能不能动"正是判定要点之一。

      不给同源权限后，脚本跑在一个不透明源里：DOM 隔离、无 cookie、无 storage。
      再叠加服务端清洗（剔除外链与逃逸 API）与 srcdoc 里注入的 CSP
      (default-src 'none')，共三层。三层缺一层都不要放行这个组件。

      referrerpolicy 与 loading 不是安全措施，只是避免预览把 referer 带出去、
      以及列表里多个预览同时抢带宽。
    -->
    <iframe
      v-if="html"
      :srcdoc="html"
      sandbox="allow-scripts"
      referrerpolicy="no-referrer"
      loading="lazy"
      class="lc-ic-artwork-frame"
      :title="title"
    ></iframe>
    <div v-else class="lc-ic-artwork-empty">
      <span>{{ emptyText }}</span>
    </div>
  </div>
</template>

<script setup lang="ts">
withDefaults(
  defineProps<{
    /** 服务端已清洗的完整 HTML 文档（含 CSP meta）。空串表示本次没有产出。 */
    html: string
    title?: string
    height?: number
    emptyText?: string
  }>(),
  {
    title: '模型绘图产物预览',
    height: 260,
    emptyText: '本次没有产出可展示的画作',
  },
)
</script>

<style scoped>
.lc-ic-artwork {
  position: relative;
  width: 100%;
  overflow: hidden;
  border-radius: 12px;
  border: 1px solid rgba(148, 163, 184, 0.28);
  /* 浅色棋盘底：模型产出的 SVG 常带透明背景，纯白底会让白色造型整个消失 */
  background-color: #fbfcfe;
  background-image:
    linear-gradient(45deg, rgba(148, 163, 184, 0.09) 25%, transparent 25%),
    linear-gradient(-45deg, rgba(148, 163, 184, 0.09) 25%, transparent 25%),
    linear-gradient(45deg, transparent 75%, rgba(148, 163, 184, 0.09) 75%),
    linear-gradient(-45deg, transparent 75%, rgba(148, 163, 184, 0.09) 75%);
  background-size: 16px 16px;
  background-position: 0 0, 0 8px, 8px -8px, -8px 0;
}

.lc-ic-artwork-frame {
  display: block;
  width: 100%;
  height: 100%;
  border: 0;
}

.lc-ic-artwork-empty {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 100%;
  font-size: 13px;
  opacity: 0.6;
}
</style>
