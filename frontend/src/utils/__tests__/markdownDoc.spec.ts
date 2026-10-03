import { describe, expect, it } from 'vitest'
import { renderMarkdownDoc } from '../markdownDoc'

describe('renderMarkdownDoc', () => {
  it('anchors second-level headings and builds the table of contents', () => {
    const doc = renderMarkdownDoc('# 标题\n\n## 一、登录\n\n正文\n\n## 二、充值\n\n### 小节')

    expect(doc.toc).toEqual([
      { id: 'doc-sec-1', text: '一、登录' },
      { id: 'doc-sec-2', text: '二、充值' }
    ])
    expect(doc.html).toContain('<h2 id="doc-sec-1">一、登录</h2>')
    expect(doc.html).not.toContain('<h3 id=')
  })

  it('falls back to third-level headings when there is no second level', () => {
    const doc = renderMarkdownDoc('### 第一条\n\n### 第二条')

    expect(doc.toc.map(item => item.text)).toEqual(['第一条', '第二条'])
  })

  it('sanitizes unsafe markup before rendering', () => {
    const doc = renderMarkdownDoc('## 安全\n\n<img src=x onerror="alert(1)"><script>alert(1)</script>')

    expect(doc.html).not.toContain('onerror')
    expect(doc.html).not.toContain('<script')
  })

  it('returns an empty document for blank content', () => {
    expect(renderMarkdownDoc('  \n ')).toEqual({ html: '', toc: [] })
  })
})
