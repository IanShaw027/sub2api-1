import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'

const { get, post } = vi.hoisted(() => ({
  get: vi.fn(),
  post: vi.fn(),
}))

const apiSkills = vi.hoisted(() => ({
  createSkill: vi.fn(),
  createSkillVersion: vi.fn(),
  getSkillDetail: vi.fn(),
  getSkillRevenue: vi.fn(),
  installSkill: vi.fn(),
  listMySkills: vi.fn(),
  listSkillMarket: vi.fn(),
  listSkillRuns: vi.fn(),
  listSkillVersions: vi.fn(),
  publishSkillVersion: vi.fn(),
  submitSkillVersion: vi.fn(),
  uninstallSkill: vi.fn(),
  updateSkill: vi.fn(),
  updateSkillVersion: vi.fn(),
}))

vi.mock('@/api/client', () => ({
  apiClient: {
    get,
    post,
  },
}))

vi.mock('@/api/skills', () => apiSkills)

import { useSkillsCenterStore } from '@/stores/skillsCenter'

describe('useSkillsCenterStore', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    get.mockReset()
    post.mockReset()
    get.mockResolvedValue({
      data: {
        id: 42,
        slug: 'demo-skill',
        name: 'Demo skill',
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
        variable_schema: [],
        content: null,
        metadata: {},
        examples: [],
        readme: null,
        install_note: null,
        can_install: true,
        can_run: true,
      },
    })
    apiSkills.getSkillDetail.mockResolvedValue({
      id: 42,
      slug: 'demo-skill',
      name: 'Demo skill',
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
      variable_schema: [],
      content: null,
      metadata: {},
      examples: [],
      readme: null,
      install_note: null,
      can_install: true,
      can_run: true,
    })
    get.mockResolvedValue({ data: { items: [{ id: 7, status: 'active' }] } })
    post.mockResolvedValue({ data: { status: 'dispatched' } })
  })

  it('includes user variables when starting test and use runs', async () => {
    const store = useSkillsCenterStore()
    const parameters = {
      api_key: 'abc123',
      retries: 2,
    }

    await store.testSkillVersion(42, 101, parameters)
    await store.useSkillVersion(42, null, parameters)

    expect(post).toHaveBeenNthCalledWith(
      1,
      '/user/skills/42/test',
      {
        parameters: {
          api_key: 'abc123',
          retries: 2,
        },
        trace: { api_key_id: 7 },
      },
      {
        params: {
          version_id: 101,
        },
      }
    )
    expect(post).toHaveBeenNthCalledWith(
      2,
      '/user/skills/42/use',
      {
        parameters: {
          api_key: 'abc123',
          retries: 2,
        },
        trace: { api_key_id: 7 },
      },
      undefined
    )
  })
})
