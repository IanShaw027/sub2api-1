import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import GroupRateMultipliersModal from '../GroupRateMultipliersModal.vue'
import GroupRPMOverridesModal from '../GroupRPMOverridesModal.vue'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string, fallback?: string) => {
      const labels: Record<string, string> = {
        'admin.groups.platforms.anthropic': 'Claude',
        'admin.groups.platforms.openai': 'OpenAI',
      }
      return labels[key] ?? fallback ?? key
    },
  }),
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: vi.fn(),
    showSuccess: vi.fn(),
  }),
}))

const { listUsers } = vi.hoisted(() => ({
  listUsers: vi.fn(),
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    groups: {
      getGroupRateMultipliers: vi.fn(),
      getGroupRPMOverrides: vi.fn(),
      batchSetGroupRateMultipliers: vi.fn(),
      batchSetGroupRPMOverrides: vi.fn(),
      clearGroupRPMOverrides: vi.fn(),
    },
    users: {
      list: listUsers,
    },
  },
}))

const baseGroup = {
  id: 1,
  name: 'Fallback Group',
  rate_multiplier: 1,
  rpm_limit: 100,
}

function mountWithPlatform(component: unknown, platform: string) {
  return mount(component as never, {
    props: {
      show: false,
      group: {
        ...baseGroup,
        platform,
      },
    },
    global: {
      stubs: {
        BaseDialog: {
          template: '<div><slot /></div>',
        },
        PlatformIcon: true,
        Pagination: true,
        Icon: true,
      },
    },
  })
}

describe('group platform fallback labels in group modals', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    listUsers.mockReset()
    listUsers.mockResolvedValue({ items: [], total: 0, page: 1, page_size: 10, pages: 1 })
  })

  it.each([
    ['claude', 'Claude'],
    ['openai', 'OpenAI'],
    ['unknown', 'unknown'],
  ])('renders %s as %s in both modals', (platform, expectedLabel) => {
    const rateWrapper = mountWithPlatform(GroupRateMultipliersModal, platform)
    const rpmWrapper = mountWithPlatform(GroupRPMOverridesModal, platform)

    expect(rateWrapper.text()).toContain(expectedLabel)
    if (expectedLabel !== platform) {
      expect(rateWrapper.text()).not.toContain(platform)
    }
    expect(rpmWrapper.text()).toContain(expectedLabel)
    if (expectedLabel !== platform) {
      expect(rpmWrapper.text()).not.toContain(platform)
    }
  })

  it('uses helper fallback instead of raw platform key when translation is missing', () => {
    const rateWrapper = mountWithPlatform(GroupRateMultipliersModal, 'claude')
    const rpmWrapper = mountWithPlatform(GroupRPMOverridesModal, 'claude')

    expect(rateWrapper.text()).toContain('Claude')
    expect(rateWrapper.text()).not.toContain('claude')
    expect(rpmWrapper.text()).toContain('Claude')
    expect(rpmWrapper.text()).not.toContain('claude')
  })

  it('searches users without subscriptions or usage stats in the rate multiplier modal', async () => {
    const wrapper = mount(GroupRateMultipliersModal, {
      props: {
        show: true,
        group: {
          ...baseGroup,
          platform: 'openai',
        },
      },
      global: {
        stubs: {
          BaseDialog: {
            template: '<div><slot /></div>',
          },
          PlatformIcon: true,
          Pagination: true,
          Icon: true,
        },
      },
    })

    await wrapper.get('input.input').setValue('retro')
    await vi.advanceTimersByTimeAsync(300)
    await flushPromises()

    expect(listUsers).toHaveBeenCalledWith(1, 10, {
      search: 'retro',
      include_subscriptions: false,
      include_usage_stats: false,
    })
  })

  it('searches users without subscriptions or usage stats in the RPM override modal', async () => {
    const wrapper = mount(GroupRPMOverridesModal, {
      props: {
        show: true,
        group: {
          ...baseGroup,
          platform: 'openai',
        },
      },
      global: {
        stubs: {
          BaseDialog: {
            template: '<div><slot /></div>',
          },
          PlatformIcon: true,
          Pagination: true,
          Icon: true,
        },
      },
    })

    await wrapper.get('input.input').setValue('retro')
    await vi.advanceTimersByTimeAsync(300)
    await flushPromises()

    expect(listUsers).toHaveBeenCalledWith(1, 10, {
      search: 'retro',
      include_subscriptions: false,
      include_usage_stats: false,
    })
  })
})
