import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { IntelCheckQuestion, IntelCheckQuestionListItem } from '@/api/admin/intelCheck'
import IntelCheckQuestionDialog from '../IntelCheckQuestionDialog.vue'

const { getQuestion, updateQuestion, createQuestion, showError, showSuccess } = vi.hoisted(() => ({
  getQuestion: vi.fn(), updateQuestion: vi.fn(), createQuestion: vi.fn(),
  showError: vi.fn(), showSuccess: vi.fn(),
}))

vi.mock('@/api/admin', () => ({
  adminAPI: { intelCheck: { getQuestion, updateQuestion, createQuestion } },
}))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError, showSuccess }) }))

const primary = '<svg><circle r="2"/></svg>'
const extra = '<svg><path d="M0 0L1 1"/></svg>'
const fixture: IntelCheckQuestion = {
  id: 1, kind: 'drawing', title: '结构题', prompt: '输出动画', expected_answer: '',
  match_mode: 'exact', reference_html: primary, reference_metrics: {
    structure_baseline: { shapes: 1, geometry_values: 3 }, standard_count: 2,
  },
  drawing_rules: { min_ratio: 0.7, max_bytes: 262144, standard_sources: [extra] },
  review_rubric: '仅兼容保存', enabled: true, created_at: '', updated_at: '',
}
const listItem: IntelCheckQuestionListItem = {
  ...fixture, reference_html_bytes: primary.length, has_review_rubric: true,
  drawing_rules: { min_ratio: 0.7, max_bytes: 262144 },
}

function mountDialog(question: IntelCheckQuestionListItem | null = listItem) {
  getQuestion.mockResolvedValue(fixture)
  updateQuestion.mockResolvedValue(fixture)
  createQuestion.mockResolvedValue(fixture)
  return mount(IntelCheckQuestionDialog, {
    props: { show: true, question },
    global: { stubs: {
      BaseDialog: { props: ['show'], template: '<div v-if="show"><slot/><slot name="footer"/></div>' },
      NumberField: true, Toggle: true,
    } },
  })
}

function fileWithText(name: string, source: string, read = () => Promise.resolve(source)) {
  const file = new File([source], name, { type: 'text/html' })
  Object.defineProperty(file, 'text', { value: read })
  return file
}

async function importFiles(wrapper: ReturnType<typeof mountDialog>, files: File[]) {
  const input = wrapper.get('input[type="file"]')
  Object.defineProperty(input.element, 'files', { configurable: true, value: files })
  await input.trigger('change')
}

describe('绘图题标准集表单', () => {
  beforeEach(() => vi.resetAllMocks())

  it('从详情回填额外标准并原样保存，不上传指标', async () => {
    const wrapper = mountDialog()
    await flushPromises()
    expect(getQuestion).toHaveBeenCalledWith(1)
    const values = wrapper.findAll('textarea').map((node) => node.element.value)
    expect(values).toContain(primary)
    expect(values).toContain(extra)
    await wrapper.get('button.btn-primary').trigger('click')
    await flushPromises()
    expect(updateQuestion).toHaveBeenCalledWith(1, expect.objectContaining({
      reference_html: primary,
      drawing_rules: expect.objectContaining({ standard_sources: [extra], min_ratio: 0.7 }),
    }))
    expect(updateQuestion.mock.calls[0]?.[1]).not.toHaveProperty('reference_metrics')
    expect(wrapper.emitted('saved')).toHaveLength(1)
    wrapper.unmount()
  })

  it('多文件导入替换标准集并保留源码原文', async () => {
    const wrapper = mountDialog()
    await flushPromises()
    const sources = ['  <svg><circle r="1"/></svg>\n', primary, extra]
    await importFiles(wrapper, sources.map((source, i) => fileWithText(`${i}.html`, source)))
    await flushPromises()
    await wrapper.get('button.btn-primary').trigger('click')
    await flushPromises()
    expect(updateQuestion).toHaveBeenCalledWith(1, expect.objectContaining({
      reference_html: sources[0],
      drawing_rules: expect.objectContaining({ standard_sources: sources.slice(1) }),
    }))
    wrapper.unmount()
  })

  it('数量或体积超限时拒绝替换已有标准', async () => {
    const wrapper = mountDialog()
    await flushPromises()
    await importFiles(wrapper, Array.from({ length: 9 }, (_, i) => fileWithText(`${i}.html`, extra)))
    await importFiles(wrapper, [fileWithText('过大.html', 'x'.repeat(512 * 1024 + 1))])
    expect(showError).toHaveBeenCalledTimes(2)
    expect(wrapper.findAll('textarea').map((node) => node.element.value)).toContain(primary)
    wrapper.unmount()
  })

  it.each(['关闭', '切换题型', '切换题目'])('%s后忽略旧导入结果', async (action) => {
    let resolve!: (value: string) => void
    const pending = new Promise<string>((done) => { resolve = done })
    const wrapper = mountDialog()
    await flushPromises()
    await importFiles(wrapper, [fileWithText('延迟.html', extra, () => pending)])
    expect(wrapper.get('button.btn-primary').attributes('disabled')).toBeDefined()
    if (action === '关闭') {
      await wrapper.setProps({ show: false })
      await wrapper.setProps({ show: true, question: null })
    } else if (action === '切换题型') {
      await wrapper.get('select').setValue('logic')
      await wrapper.get('select').setValue('drawing')
    } else {
      await wrapper.setProps({ question: { ...listItem, id: 2 } })
      await flushPromises()
    }
    resolve('不应写入新表单的旧文件')
    await flushPromises()
    expect(wrapper.findAll('textarea').map((node) => node.element.value))
      .not.toContain('不应写入新表单的旧文件')
    expect(wrapper.get('button.btn-primary').attributes('disabled')).toBeUndefined()
    wrapper.unmount()
  })
})
