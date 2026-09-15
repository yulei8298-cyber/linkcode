import { chromium } from 'playwright'
import { analyzeMotionSamples, REQUIRED_PARTS, unverified } from './motion.mjs'
import { readJSONStdin } from './stdin.mjs'

async function collectSample(page) {
  return page.evaluate((requiredParts) => {
    const parts = {}
    for (const name of requiredParts) {
      const element = document.querySelector(`[data-intel-part="${name}"]`)
      const rect = element.getBoundingClientRect()
      const matrix = element.getCTM?.() || new DOMMatrix()
      const paths = element.matches('path') ? [element] : [...element.querySelectorAll('path')]
      const endpoints = paths.flatMap((path) => {
        try {
          const length = path.getTotalLength()
          const screen = path.getScreenCTM()
          if (!screen) return []
          return [path.getPointAtLength(0), path.getPointAtLength(length)].map((point) => ({
            x: screen.a * point.x + screen.c * point.y + screen.e,
            y: screen.b * point.x + screen.d * point.y + screen.f,
          }))
        } catch {
          return []
        }
      })
      parts[name] = {
        center: { x: rect.left + rect.width / 2, y: rect.top + rect.height / 2 },
        matrix: { a: matrix.a, b: matrix.b, c: matrix.c, d: matrix.d },
        endpoints,
      }
    }
    return { parts }
  }, REQUIRED_PARTS)
}

async function evaluateHTML(html) {
  // 浏览器沙箱在 no-new-privileges 的容器中不可用；这里以无凭据、只读、
  // 独立内网且有限资源的 sidecar 容器作为进程隔离边界。
  const browser = await chromium.launch({
    headless: true,
    chromiumSandbox: false,
    args: ['--disable-dev-shm-usage', '--no-sandbox'],
  })
  const context = await browser.newContext({
    viewport: { width: 1200, height: 800 },
    javaScriptEnabled: true,
    serviceWorkers: 'block',
  })
  try {
    await context.route('**/*', (route) => route.abort())
    if (typeof context.routeWebSocket === 'function') {
      await context.routeWebSocket('**/*', (socket) => socket.close())
    }
    const page = await context.newPage()
    page.on('dialog', (dialog) => dialog.dismiss())
    await page.emulateMedia({ reducedMotion: 'no-preference' })
    await page.setContent(html, { waitUntil: 'domcontentloaded', timeout: 2500 })

    const contract = await page.evaluate((requiredParts) => {
      const mainSVG = [...document.querySelectorAll('svg')]
        .sort((a, b) => b.getBoundingClientRect().width * b.getBoundingClientRect().height - a.getBoundingClientRect().width * a.getBoundingClientRect().height)[0]
      if (!mainSVG) return { reason: '未找到主 SVG' }
      const isVisible = (element) => {
        const rect = element.getBoundingClientRect()
        if (rect.width < 2 || rect.height < 2 || rect.right <= 0 || rect.bottom <= 0 || rect.left >= innerWidth || rect.top >= innerHeight) return false
        for (let current = element; current && current !== mainSVG.parentElement; current = current.parentElement) {
          const style = getComputedStyle(current)
          if (style.display === 'none' || style.visibility === 'hidden' || Number.parseFloat(style.opacity || '1') <= 0.05 || current.hasAttribute('hidden')) return false
        }
        return true
      }
      for (const name of requiredParts) {
        const matches = [...document.querySelectorAll(`[data-intel-part="${name}"]`)]
        if (matches.length !== 1) return { reason: `部件标记 ${name} 数量为 ${matches.length}，要求恰好 1 个` }
        const element = matches[0]
        if (!(element instanceof SVGElement) || !mainSVG.contains(element) || !isVisible(element)) {
          return { reason: `部件标记 ${name} 没有挂在主 SVG 的实际可见部件上` }
        }
        const rect = element.getBoundingClientRect()
        if (name === 'crank-center' && (rect.width > 60 || rect.height > 60)) {
          return { reason: 'crank-center 必须标记曲柄轴心的小型可见部件，不能标记整车或大分组' }
        }
        if (name.startsWith('leg-')) {
          const paths = element.matches('path') ? [element] : [...element.querySelectorAll('path')]
          if (paths.length === 0) return { reason: `${name} 没有可测量的 SVG path` }
        }
      }
      return { reason: '' }
    }, REQUIRED_PARTS)
    if (contract.reason) return unverified(contract.reason)

    const samples = []
    await page.waitForTimeout(250)
    for (let index = 0; index < 16; index += 1) {
      samples.push(await collectSample(page))
      await page.waitForTimeout(120)
    }
    return analyzeMotionSamples(samples)
  } finally {
    await context.close()
    await browser.close()
  }
}

const input = await readJSONStdin()
process.stdout.write(JSON.stringify(await evaluateHTML(input.html)))
