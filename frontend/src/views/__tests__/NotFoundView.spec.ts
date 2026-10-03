import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import NotFoundView from '../NotFoundView.vue'

const back = vi.fn()

vi.mock('vue-router', () => ({
  useRoute: () => ({ fullPath: '/not/here?x=1' }),
  useRouter: () => ({ back }),
  RouterLink: { props: ['to'], template: '<a :href="to"><slot /></a>' }
}))

vi.mock('vue-i18n', () => ({
  useI18n: () => ({ t: (key: string) => key })
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ cachedPublicSettings: { qq_group: 'QQ群：1025176993' } })
}))

describe('NotFoundView', () => {
  it('shows the missing path, translated copy, next steps and the support group', async () => {
    const wrapper = mount(NotFoundView, { global: { stubs: { Icon: true } } })

    expect(wrapper.text()).toContain('errors.pageNotFound')
    expect(wrapper.text()).toContain('/not/here?x=1')
    expect(wrapper.text()).not.toContain('Go Back')
    expect(wrapper.get('a').attributes('href')).toBe('/dashboard')
    expect(wrapper.text()).toContain('QQ 群 1025176993')

    await wrapper.get('button').trigger('click')
    expect(back).toHaveBeenCalledTimes(1)
  })
})
