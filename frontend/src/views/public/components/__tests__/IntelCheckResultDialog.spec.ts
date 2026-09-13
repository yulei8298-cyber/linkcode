import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import IntelCheckResultDialog from '../IntelCheckResultDialog.vue'

const { getIntelCheckResult } = vi.hoisted(() => ({
  getIntelCheckResult: vi.fn(),
}))

vi.mock('@/api/intelCheck', () => ({
  getIntelCheckResult,
}))

const BaseDialogStub = {
  props: ['show', 'title'],
  template: '<div v-if="show"><h2>{{ title }}</h2><slot /><slot name="footer" /></div>',
}

const SvgArtworkPreviewStub = {
  props: ['html', 'title'],
  template: '<div data-test="artwork-preview">{{ title }} {{ html }}</div>',
}

const logicResult = {
  id: 1,
  round_seq: 12,
  kind: 'logic' as const,
  status: 'pass' as const,
  target_id: 3,
  target_name: '满血组',
  model: 'gpt-test',
  reasoning_effort: 'high',
  question_title: '数字题',
  prompt_snapshot: '1 + 1 等于多少？',
  expected_answer: '2',
  match_mode: 'numeric',
  extracted_answer: '2',
  raw_reply: '答案是 2',
  html_output: '',
  judge_detail: { matched: true, reason: '答案匹配' },
  latency_ms: 120,
  status_note: '判定完成',
  checked_at: '2026-09-13T10:00:00Z',
}

const drawingResult = {
  ...logicResult,
  id: 2,
  kind: 'drawing' as const,
  status: 'fail' as const,
  question_title: '绘图题',
  prompt_snapshot: '画一只会动的鸟',
  expected_answer: '',
  match_mode: '',
  extracted_answer: '',
  raw_reply: '<svg>raw</svg>',
  html_output: '<html><svg><title>bird</title></svg></html>',
  judge_detail: {
    gate_pass: false,
    reason: '结构门禁未通过',
    gate_items: [{ item: '存在动画机制', pass: false, detail: 'mechanisms=无' }],
    review_items: [{ item: '主体完整度', score: 10, max_score: 25, comment: '缺少主体部件' }],
  },
}

function mountDialog(result: typeof logicResult) {
  getIntelCheckResult.mockResolvedValue(result)
  return mount(IntelCheckResultDialog, {
    props: { show: true, resultId: result.id },
    global: {
      stubs: {
        BaseDialog: BaseDialogStub,
        SvgArtworkPreview: SvgArtworkPreviewStub,
      },
    },
  })
}

describe('IntelCheckResultDialog', () => {
  beforeEach(() => {
    getIntelCheckResult.mockReset()
  })

  it('renders logic details and loads one request when opened', async () => {
    const wrapper = mountDialog(logicResult)
    await flushPromises()

    expect(getIntelCheckResult).toHaveBeenCalledOnce()
    expect(wrapper.text()).toContain('标准答案（数值比较）')
    expect(wrapper.text()).toContain('答案是 2')
    expect(wrapper.text()).toContain('答案匹配')
    expect(wrapper.find('[data-test="artwork-preview"]').exists()).toBe(false)
    wrapper.unmount()
  })

  it('renders drawing preview and exposes gate/review branches', async () => {
    const wrapper = mountDialog(drawingResult)
    await flushPromises()

    expect(wrapper.find('[data-test="artwork-preview"]').exists()).toBe(true)
    expect(wrapper.text()).toContain('结构门禁未通过')
    expect(wrapper.text()).toContain('存在动画机制')
    expect(wrapper.text()).toContain('主体完整度')

    const sourceTab = wrapper.findAll('button').find((button) => button.text() === '源码')
    expect(sourceTab).toBeDefined()
    await sourceTab!.trigger('click')
    expect(wrapper.text()).toContain('<html><svg><title>bird</title></svg></html>')
    wrapper.unmount()
  })
})
