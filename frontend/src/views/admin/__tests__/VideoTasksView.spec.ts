import { flushPromises, mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

const listMock = vi.hoisted(() => vi.fn())

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})
vi.mock('@/api/admin', () => ({ adminAPI: { videoTasks: { list: listMock, refresh: vi.fn(), retryBilling: vi.fn() } } }))
vi.mock('@/stores', () => ({ useAppStore: () => ({ showError: vi.fn(), showSuccess: vi.fn() }) }))

import VideoTasksView from '../VideoTasksView.vue'

const baseTask = {
  id: 1, user_id: 1, api_key_id: 2, group_id: 3, account_id: 4, protocol: 'openai_videos', source: 'unified',
  model: 'kling-v3', upstream_model: 'kling-v3', upstream_task_id: 'video_x', progress: 100, resolution: '720p',
  aspect_ratio: '16:9', duration_seconds: 5, actual_duration_seconds: null, video_count: 1, generate_audio: false,
  estimated_cost: 0.42, actual_cost: 0.42, completion_tokens: 0, error_code: '', error_message: '', poll_count: 3,
  billing_status: 'billed', billing_attempts: 1, billing_error: '', inbound_endpoint: '/v1/videos',
  created_at: '2026-10-21T14:43:52Z', updated_at: '2026-10-21T14:58:42Z', finished_at: null, billed_at: null,
}

describe('VideoTasksView', () => {
  it('「查看成品」使用网关签名链接，未完成任务不显示', async () => {
    listMock.mockResolvedValueOnce({
      items: [
        { ...baseTask, task_id: 'vt_done', status: 'completed', content_url: '/video-files/vt_done?expires=1&signature=abc' },
        { ...baseTask, task_id: 'vt_run', status: 'in_progress' },
      ],
      total: 2, page: 1, page_size: 20, pages: 1,
    })
    const wrapper = mount(VideoTasksView, {
      global: { stubs: { AppLayout: { template: '<div><slot /></div>' }, Pagination: true } },
    })
    await flushPromises()
    const links = wrapper.findAll('a')
    expect(links).toHaveLength(1)
    expect(links[0].attributes('href')).toBe('/video-files/vt_done?expires=1&signature=abc')
    expect(wrapper.html()).not.toContain('krill')
  })
})
