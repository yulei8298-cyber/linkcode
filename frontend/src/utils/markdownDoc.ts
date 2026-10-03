import { marked } from 'marked'
import DOMPurify from 'dompurify'

// 公开文档页（教程、法律文档）的 Markdown 渲染：净化后为二级标题补锚点并生成目录。
// 锚点在净化之后再写入，id 由本函数生成，不会引入外部可控属性。

export interface DocHeading {
  id: string
  text: string
}

export interface RenderedDoc {
  html: string
  toc: DocHeading[]
}

const DOC_HEADING_ID_PREFIX = 'doc-sec-'

export function renderMarkdownDoc(markdown: string): RenderedDoc {
  const content = markdown.trim()
  if (!content) return { html: '', toc: [] }

  const html = DOMPurify.sanitize(marked.parse(content, { breaks: true, gfm: true, async: false }) as string)
  const container = document.createElement('div')
  container.innerHTML = html

  // 文档一般按二级标题分节；没有二级标题时退到三级，保证目录有意义
  const level = container.querySelector('h2') ? 'h2' : 'h3'
  const toc: DocHeading[] = []
  container.querySelectorAll(level).forEach((heading, index) => {
    const text = heading.textContent?.trim() || ''
    if (!text) return
    const id = `${DOC_HEADING_ID_PREFIX}${index + 1}`
    heading.id = id
    toc.push({ id, text })
  })

  return { html: container.innerHTML, toc }
}
