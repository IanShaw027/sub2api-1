import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get, post, put } = vi.hoisted(() => ({
  get: vi.fn(),
  post: vi.fn(),
  put: vi.fn(),
}))

vi.mock('@/api/client', () => ({
  apiClient: {
    get,
    post,
    put,
  },
}))

import {
  createSkillVersion,
  getSkillDetail,
  getSkillRevenue,
  installSkill,
  listMySkills,
  listSkillMarket,
  listSkillVersions,
  uninstallSkill,
  updateSkillVersion,
} from '@/api/skills'
import { createDefaultSkillContent } from '@/types/skills'

describe('skills api', () => {
  beforeEach(() => {
    get.mockReset()
    post.mockReset()
    put.mockReset()
  })

  it('lists market skills from /user/skills with market filters', async () => {
    const signal = new AbortController().signal

    get.mockResolvedValueOnce({
      data: {
        items: [],
        total: 0,
        page: 2,
        page_size: 12,
        pages: 1,
      },
    })

    await expect(
      listSkillMarket(
        2,
        12,
        {
          search: 'poster',
          type: 'script',
          installed: 'installed',
          category: 'automation',
          price_mode: 'paid',
          sort: 'popular',
        },
        { signal },
      ),
    ).resolves.toMatchObject({
      items: [],
      total: 0,
      page: 2,
      page_size: 12,
    })

    expect(get).toHaveBeenCalledWith(
      '/user/skills',
      expect.objectContaining({
        params: {
          page: 2,
          page_size: 12,
          scope: 'market',
          search: 'poster',
          type: 'script',
          installed: 'installed',
          category: 'automation',
          price_mode: 'paid',
          sort: 'popular',
        },
        signal,
      }),
    )
  })

  it('lists owned skills from /user/skills with mine filters', async () => {
    get.mockResolvedValueOnce({
      data: {
        items: [],
        total: 0,
        page: 1,
        page_size: 18,
        pages: 1,
      },
    })

    await expect(
      listMySkills(1, 18, {
        search: 'assistant',
        type: 'prompt_chat',
        category: 'workflow',
        status: 'published',
        visibility: 'public',
        sort: 'latest',
      }),
    ).resolves.toMatchObject({
      items: [],
      total: 0,
      page: 1,
      page_size: 18,
    })

    expect(get).toHaveBeenCalledWith('/user/skills', {
      params: {
        page: 1,
        page_size: 18,
        scope: 'mine',
        search: 'assistant',
        type: 'prompt_chat',
        category: 'workflow',
        status: 'published',
        visibility: 'public',
        sort: 'latest',
      },
      signal: undefined,
    })
  })

  it('forwards free price mode when listing market skills', async () => {
    get.mockResolvedValueOnce({
      data: {
        items: [],
        total: 0,
        page: 1,
        page_size: 18,
        pages: 1,
      },
    })

    await listSkillMarket(1, 18, {
      price_mode: 'free',
    })

    expect(get).toHaveBeenCalledWith(
      '/user/skills',
      expect.objectContaining({
        params: expect.objectContaining({
          scope: 'market',
          price_mode: 'free',
        }),
      }),
    )
  })

  it('normalizes redacted skill detail payloads without synthesizing empty source content', async () => {
    get.mockResolvedValueOnce({
      data: {
        id: 77,
        name: 'Paid Skill',
        type: 'script',
        visibility: 'public',
        pricing: {
          mode: 'paid',
          amount: 19.9,
          currency: 'CNY',
        },
        source_locked: true,
        can_view_source: false,
        variable_schema: [
          { key: 'topic', type: 'string', label: 'Topic' },
        ],
        readme: 'safe readme',
        install_note: 'safe install note',
      },
    })

    await expect(getSkillDetail(77)).resolves.toMatchObject({
      id: 77,
      can_view_source: false,
      content: null,
      metadata: {},
      variable_schema: [
        {
          key: 'topic',
          type: 'string',
        },
      ],
      readme: 'safe readme',
      install_note: 'safe install note',
    })
  })

  it('normalizes redacted skill version payloads without source metadata', async () => {
    get.mockResolvedValueOnce({
      data: {
        items: [
          {
            id: 91,
            skill_id: 77,
            version: 'v1',
            status: 'published',
            created_at: '2026-05-01T00:00:00Z',
          },
        ],
        total: 1,
        page: 1,
        page_size: 20,
        pages: 1,
      },
    })

    await expect(listSkillVersions(77)).resolves.toMatchObject({
      items: [
        {
          id: 91,
          skill_id: 77,
          version: 'v1',
          content: null,
          metadata: {},
        },
      ],
    })
  })

  it('posts install and uninstall requests to skill endpoints', async () => {
    post.mockResolvedValueOnce({ data: { message: 'ok' } })
    post.mockResolvedValueOnce({ data: { message: 'ok' } })

    await expect(installSkill(33)).resolves.toEqual({ message: 'ok' })
    await expect(uninstallSkill(33)).resolves.toEqual({ message: 'ok' })

    expect(post).toHaveBeenNthCalledWith(1, '/user/skills/33/install')
    expect(post).toHaveBeenNthCalledWith(2, '/user/skills/33/uninstall')
  })

  it('normalizes revenue order statuses without defaulting unknown values to paid', async () => {
    get.mockResolvedValueOnce({
      data: {
        summary: {
          total_revenue: 80,
          total_sales: 4,
          total_runs: 5,
          pending_amount: 20,
          settled_amount: 80,
          refunded_amount: 0,
          currency: 'CNY',
        },
        trend: [],
        orders: [
          { id: 1, status: 'transferred', amount: 80, created_at: '2026-06-01T00:00:00Z' },
          { id: 2, status: 'canceled', amount: 20, created_at: '2026-06-02T00:00:00Z' },
          { id: 3, status: 'mystery', amount: 10, created_at: '2026-06-03T00:00:00Z' },
        ],
      },
    })

    await expect(getSkillRevenue(9)).resolves.toMatchObject({
      orders: [
        { id: 1, status: 'paid' },
        { id: 2, status: 'cancelled' },
        { id: 3, status: 'pending' },
      ],
    })

    expect(get).toHaveBeenCalledWith('/user/skills/9/revenue', {
      signal: undefined,
    })
  })

  it('creates and updates skill versions on the version endpoints', async () => {
    const createPayload = {
      version: 'v2',
      changelog: 'ship',
      source_locked: true,
      content: {
        type: 'prompt_chat',
        system_prompt: 'You are helpful',
        user_prompt_template: '{{input}}',
      },
    }
    const updatePayload = {
      changelog: 'hotfix',
    }

    post.mockResolvedValueOnce({
      data: {
        id: 301,
        skill_id: 9,
        version: 'v2',
        status: 'published',
      },
    })
    put.mockResolvedValueOnce({
      data: {
        id: 302,
        skill_id: 9,
        version: 'v2.1',
        status: 'draft',
      },
    })

    await expect(createSkillVersion(9, createPayload)).resolves.toMatchObject({
      id: 301,
      skill_id: 9,
      version: 'v2',
      status: 'published',
    })
    await expect(updateSkillVersion(9, 302, updatePayload)).resolves.toMatchObject({
      id: 302,
      skill_id: 9,
      version: 'v2.1',
      status: 'draft',
    })

    expect(post).toHaveBeenCalledWith('/user/skills/9/versions', createPayload)
    expect(put).toHaveBeenCalledWith('/user/skills/9/versions/302', updatePayload)
  })

  it('defaults script skills to an executable runtime and entrypoint', () => {
    expect(createDefaultSkillContent('script')).toMatchObject({
      type: 'script',
      runtime: 'node20',
      entrypoint: 'main.mjs',
      dependencies: [],
      timeout_seconds: 30,
    })
  })
})
