import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import SkillCard from '@/components/skills/SkillCard.vue'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (_key: string, fallback?: string) => fallback ?? _key,
  }),
}))

vi.mock('vue-router', () => ({
  RouterLink: {
    name: 'RouterLinkStub',
    props: ['to'],
    template: '<a :href="typeof to === `string` ? to : (to?.path ?? ``)"><slot /></a>',
  },
}))

vi.mock('@/components/icons/Icon.vue', () => ({
  default: {
    name: 'IconStub',
    template: '<span />',
  },
}))

describe('SkillCard', () => {
  it('hides the install CTA when the backend disallows installation', () => {
    const wrapper = mount(SkillCard, {
      props: {
        skill: {
          id: 42,
          slug: 'locked-skill',
          name: 'Locked skill',
          tagline: '',
          description: '',
          type: 'prompt_chat',
          visibility: 'public',
          status: 'published',
          category: null,
          tags: [],
          cover_image_url: null,
          pricing: { mode: 'free', amount: 0, currency: 'USD' },
          source_locked: false,
          can_view_source: true,
          installed: false,
          owned: false,
          editable: false,
          author: { id: 1, name: 'Author', avatar_url: null },
          stats: { installs: 0, runs: 0, revenue: 0, rating: null, versions: 1 },
          latest_version: null,
          current_version: null,
          created_at: '2026-01-01T00:00:00Z',
          updated_at: '2026-01-01T00:00:00Z',
          can_install: false,
        } as never,
      },
    })

    const buttonLabels = wrapper.findAll('button').map((item) => item.text())

    expect(wrapper.text()).toContain('技能详情')
    expect(buttonLabels).not.toContain('安装')
    expect(wrapper.find('button').exists()).toBe(false)
  })
})
