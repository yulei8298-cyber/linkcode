import { defineConfig, loadEnv, Plugin } from 'vite'
import vue from '@vitejs/plugin-vue'
import checker from 'vite-plugin-checker'
import { resolve } from 'path'

function escapeHtml(value: string): string {
  return value.replace(/[&<>"']/g, (character) => ({
    '&': '&amp;',
    '<': '&lt;',
    '>': '&gt;',
    '"': '&quot;',
    "'": '&#39;',
  })[character] || character)
}

function isSafeImageUrl(value: string): boolean {
  const trimmed = value.trim()
  if ((trimmed.startsWith('/') && !trimmed.startsWith('//')) || /^data:image\//i.test(trimmed)) {
    return true
  }
  try {
    const parsed = new URL(trimmed)
    return parsed.protocol === 'http:' || parsed.protocol === 'https:'
  } catch {
    return false
  }
}

function injectBranding(html: string, config: { site_name?: string; site_logo?: string }): string {
  let brandedHtml = html
  const siteName = config.site_name?.trim()
  if (siteName) {
    brandedHtml = brandedHtml.replace(
      /<title>[^<]*<\/title>/i,
      `<title>${escapeHtml(siteName)} - AI API Gateway</title>`,
    )
  }

  const siteLogo = config.site_logo?.trim()
  if (siteLogo && isSafeImageUrl(siteLogo)) {
    brandedHtml = brandedHtml.replace(
      /<link\s+rel=["']icon["'][^>]*>/i,
      `<link rel="icon" href="${escapeHtml(siteLogo)}" />`,
    )
  }
  return brandedHtml
}

/**
 * Vite 插件：开发模式下注入公开配置到 index.html
 * 与生产模式的后端注入行为保持一致，消除闪烁
 */
function injectPublicSettings(backendUrl: string): Plugin {
  return {
    name: 'inject-public-settings',
    apply: 'serve',
    transformIndexHtml: {
      order: 'pre',
      async handler(html) {
        try {
          const response = await fetch(`${backendUrl}/api/v1/settings/public`, {
            signal: AbortSignal.timeout(2000)
          })
          if (response.ok) {
            const data = await response.json()
            if (data.code === 0 && data.data) {
              const script = `<script>window.__APP_CONFIG__=${JSON.stringify(data.data)};</script>`
              return injectBranding(html, data.data).replace('</head>', `${script}\n</head>`)
            }
          }
        } catch (e) {
          console.warn('[vite] 无法获取公开配置，将回退到 API 调用:', (e as Error).message)
        }
        return html
      }
    }
  }
}

/**
 * 首屏语言包预加载。语言包由 src/i18n 动态 import，默认要等主脚本执行完才开始下载，
 * 首屏多出一轮往返（海外服务器上约 0.25s 以上）。构建时找出中英文语言包分包，在 HTML 里
 * 按与 getDefaultLocale 相同的规则（先看 sub2api_locale，再看浏览器语言是否 zh 开头）
 * 只预加载要用的那一个，和主脚本并行下载。
 * 内联脚本带后端的 CSP nonce 占位符（internal/web 在响应时替换）；被拦截时只是不预加载，页面照常。
 * 脚本插在 <head> 开头（<meta charset> 之后），原因见下方 transformIndexHtml。
 */
const CSP_NONCE_PLACEHOLDER = '__CSP_NONCE_VALUE__'
const LOCALE_ENTRY = /\/src\/i18n\/locales\/(en|zh)\/index\.ts$/

function preloadLocaleChunk(): Plugin {
  let base = '/'
  return {
    name: 'preload-locale-chunk',
    apply: 'build',
    configResolved(config) {
      base = config.base
    },
    transformIndexHtml: {
      order: 'post',
      handler(html, ctx) {
        const files: Record<string, string> = {}
        for (const chunk of Object.values(ctx.bundle ?? {})) {
          const match = chunk.type === 'chunk' && chunk.facadeModuleId ? LOCALE_ENTRY.exec(chunk.facadeModuleId) : null
          if (match) files[match[1]] = base + chunk.fileName
        }
        if (!files.en || !files.zh) return html
        const script =
          `<script nonce="${CSP_NONCE_PLACEHOLDER}">(function(){try{` +
          `var s=localStorage.getItem('sub2api_locale');` +
          `var zh=s==='zh'||s==='en'?s==='zh':(navigator.language||'').toLowerCase().indexOf('zh')===0;` +
          `var l=document.createElement('link');l.rel='modulepreload';` +
          `l.href=zh?${JSON.stringify(files.zh)}:${JSON.stringify(files.en)};` +
          `document.head.appendChild(l)}catch(e){}})()</script>`
        // 必须放在样式表之前：浏览器规定普通脚本要等它前面的样式表加载完才执行，
        // 放在 </head> 前会被 CSS 挡住，预加载就和主脚本自己发起请求同时甚至更晚，等于没做。
        const charset = /<meta\s+charset=[^>]*>/i
        return charset.test(html)
          ? html.replace(charset, (tag) => `${tag}\n    ${script}`)
          : html.replace(/<head[^>]*>/i, (tag) => `${tag}\n    ${script}`)
      }
    }
  }
}

export default defineConfig(({ mode }) => {
  // 加载环境变量
  const env = loadEnv(mode, process.cwd(), '')
  const backendUrl = env.VITE_DEV_PROXY_TARGET || 'http://localhost:8080'
  const devPort = Number(env.VITE_DEV_PORT || 3000)

  return {
    plugins: [
      vue(),
      checker({
        vueTsc: true
      }),
      injectPublicSettings(backendUrl),
      preloadLocaleChunk()
    ],
  resolve: {
    alias: {
      '@': resolve(__dirname, 'src'),
      // 使用 vue-i18n 运行时版本，避免 CSP unsafe-eval 问题
      'vue-i18n': 'vue-i18n/dist/vue-i18n.runtime.esm-bundler.js'
    }
  },
  define: {
    // 启用 vue-i18n JIT 编译，在 CSP 环境下处理消息插值
    // JIT 编译器生成 AST 对象而非 JS 代码，无需 unsafe-eval
    __INTLIFY_JIT_COMPILATION__: true
  },
  build: {
    outDir: '../backend/internal/web/dist',
    emptyOutDir: true,
    rollupOptions: {
      output: {
        /**
         * 手动分包配置
         * 分离第三方库并按功能合并应用代码，避免循环依赖
         */
        manualChunks(id: string) {
          if (id.includes('node_modules')) {
            // Vue 核心库
            if (
              id.includes('/vue/') ||
              id.includes('/vue-router/') ||
              id.includes('/pinia/') ||
              id.includes('/@vue/')
            ) {
              return 'vendor-vue'
            }

            // UI 工具库（较大，单独分离）
            if (id.includes('/@vueuse/') || id.includes('/xlsx/')) {
              return 'vendor-ui'
            }

            // 图表库
            if (id.includes('/chart.js/') || id.includes('/vue-chartjs/')) {
              return 'vendor-chart'
            }

            // 国际化
            if (id.includes('/vue-i18n/') || id.includes('/@intlify/')) {
              return 'vendor-i18n'
            }

            // Stripe 仅在支付流程中按需加载，避免进入首页公共依赖。
            if (id.includes('/@stripe/stripe-js/')) {
              return 'vendor-stripe'
            }

            // Airwallex 同理：它在模块加载时就会向 airwallex.com 拉取两个脚本，
            // 落进 vendor-misc 会让每个页面都发起这两个海外请求。
            if (id.includes('/@airwallex/')) {
              return 'vendor-airwallex'
            }

            // 其他小型第三方库合并
            return 'vendor-misc'
          }

          // 应用代码：按入口点自动分包，不手动干预
          // 这样可以避免循环依赖，同时保持合理的 chunk 数量
        }
      }
    }
  },
    server: {
      host: '0.0.0.0',
      port: devPort,
      proxy: {
        '/api': {
          target: backendUrl,
          changeOrigin: true
        },
        '/v1': {
          target: backendUrl,
          changeOrigin: true
        },
        '/setup': {
          target: backendUrl,
          changeOrigin: true
        }
      }
    }
  }
})
