import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import AuthCallbackStatus from '@/components/auth/AuthCallbackStatus.vue'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({ t: (key: string) => key })
}))

const RouterLinkStub = { props: ['to'], template: '<a :href="to"><slot /></a>' }

function mountStatus(props: { processing: boolean; message: string }) {
  return mount(AuthCallbackStatus, { props, global: { stubs: { RouterLink: RouterLinkStub } } })
}

describe('AuthCallbackStatus', () => {
  it('keeps the failure reason on the page with next steps', () => {
    const wrapper = mountStatus({ processing: false, message: 'pending auth session not found' })

    const failed = wrapper.get('[data-testid="auth-callback-failed"]')
    expect(failed.attributes('role')).toBe('alert')
    expect(failed.text()).toContain('auth.callbackStatus.failedTitle')
    expect(failed.text()).toContain('pending auth session not found')
    expect(failed.findAll('a').map(a => a.attributes('href'))).toEqual(['/login', '/home'])
  })

  it('shows only a spinner while the callback is still processing', () => {
    const wrapper = mountStatus({ processing: true, message: 'stale error' })

    expect(wrapper.find('[data-testid="auth-callback-failed"]').exists()).toBe(false)
    expect(wrapper.find('.cb-spinner').exists()).toBe(true)
  })

  it('renders nothing once processing finished without an error', () => {
    const wrapper = mountStatus({ processing: false, message: '' })

    expect(wrapper.find('.cb-status').exists()).toBe(false)
  })
})
