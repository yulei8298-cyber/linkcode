import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { JSDOM } from 'jsdom'
import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import SvgArtworkPreview from '../SvgArtworkPreview.vue'

const source = '<svg viewBox="0 0 10 10"><script>requestAnimationFrame(frame)</script></svg>'
const shellHTML = readFileSync(resolve(process.cwd(), '../backend/internal/handler/templates/intel_check_preview.html'), 'utf8')

describe('SvgArtworkPreview', () => {
  it('使用独立外壳，原文只发送到指定沙箱窗口', async () => {
    const wrapper = mount(SvgArtworkPreview, { props: { html: source, title: '原稿' }, attachTo: document.body })
    const frame = wrapper.get('iframe')
    expect(frame.attributes('src')).toBe('/api/v1/public/intel-check/preview')
    expect(frame.attributes('srcdoc')).toBeUndefined()
    expect(frame.attributes('sandbox')).toBe('allow-scripts')
    expect(frame.attributes('referrerpolicy')).toBe('no-referrer')
    const post = vi.spyOn((frame.element as HTMLIFrameElement).contentWindow!, 'postMessage')
    await frame.trigger('load')
    expect(post).toHaveBeenLastCalledWith({ type: 'intel-check-preview', html: source, title: '原稿' }, '*')
    await wrapper.setProps({ html: source + '<!-- 新版本 -->' })
    expect(post).toHaveBeenLastCalledWith({ type: 'intel-check-preview', html: source + '<!-- 新版本 -->', title: '原稿' }, '*')
    await wrapper.setProps({ html: '' })
    expect(wrapper.find('iframe').exists()).toBe(false)
    expect(wrapper.text()).toContain('本次没有产出可展示的画作')
    wrapper.unmount()
  })

  it('无产物时不创建预览外壳', () => {
    const wrapper = mount(SvgArtworkPreview, { props: { html: '' } })
    expect(wrapper.find('iframe').exists()).toBe(false)
    wrapper.unmount()
  })
})

describe('绘图预览外壳消息协议', () => {
  it('只接受直接父窗口消息，保留源码并拒绝无效或超限消息', () => {
    // 仅执行仓库内的固定外壳脚本；jsdom 不运行 srcdoc 中的模型代码。
    const dom = new JSDOM(shellHTML, { runScripts: 'outside-only' })
    const parent = {} as Window
    Object.defineProperty(dom.window, 'parent', { value: parent })
    dom.window.eval(dom.window.document.querySelector('script')!.textContent!)
    const frame = dom.window.document.querySelector('iframe')!
    const write = vi.spyOn(frame, 'srcdoc', 'set')
    const message = (from: Window, data: unknown) => dom.window.dispatchEvent(new dom.window.MessageEvent('message', {
      source: from, data,
    }))
    message({} as Window, { type: 'intel-check-preview', html: source })
    message(parent, { type: '其他协议', html: source })
    message(parent, { type: 'intel-check-preview', html: {} })
    message(parent, { type: 'intel-check-preview', html: 'x'.repeat(2 * 1024 * 1024 + 1) })
    expect(write).not.toHaveBeenCalled()
    message(parent, { type: 'intel-check-preview', html: source, title: '原稿' })
    expect(frame.srcdoc).toBe(source)
    expect(frame.title).toBe('原稿')
    expect(frame.getAttribute('sandbox')).toBe('allow-scripts')
    message(parent, { type: 'intel-check-preview', html: source, title: '新标题' })
    expect(frame.title).toBe('新标题')
    expect(write).toHaveBeenCalledTimes(1)
    message(parent, { type: 'intel-check-preview', html: '' })
    expect(frame.srcdoc).toBe('')
    dom.window.close()
  })
})
