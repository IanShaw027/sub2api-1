import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get, post } = vi.hoisted(() => ({
  get: vi.fn(),
  post: vi.fn(),
}))

vi.mock('@/api/client', () => ({
  apiClient: {
    get,
    post,
  },
}))

import {
  approveReview,
  disableSkill,
  forceSkillPrivate,
  getRuntimeOverview,
  listGovernanceSkills,
  listReviews,
  listSettlements,
  rejectReview,
} from '@/api/admin/skills'

describe('admin skills api', () => {
  beforeEach(() => {
    get.mockReset()
    post.mockReset()
  })

  it('lists pending skill reviews from /admin/skills/reviews', async () => {
    const signal = new AbortController().signal

    get.mockResolvedValueOnce({
      data: {
        items: [],
        total: 0,
        page: 3,
        page_size: 15,
        pages: 1,
        summary: {
          pending_count: 0,
          approved_count: 0,
          rejected_count: 0,
          high_risk_count: 0,
        },
      },
    })

    await expect(
      listReviews(
        3,
        15,
        {
          search: 'script',
          review_status: 'pending',
          risk_level: 'high',
          visibility: 'public',
        },
        { signal },
      ),
    ).resolves.toMatchObject({
      items: [],
      total: 0,
      page: 3,
      page_size: 15,
      summary: {
        pending_count: 0,
      },
    })

    expect(get).toHaveBeenCalledWith(
      '/admin/skills/reviews',
      expect.objectContaining({
        params: {
          page: 3,
          page_size: 15,
          search: 'script',
          review_status: 'pending',
          risk_level: 'high',
          visibility: 'public',
        },
        signal,
      }),
    )
  })

  it('posts review moderation actions to the review endpoints', async () => {
    const approvePayload = { note: 'looks good' }
    const rejectPayload = { reason: 'missing audit note' }

    post.mockResolvedValueOnce({
      data: {
        message: 'approved',
        review_status: 'approved',
        operated_at: '2026-05-01T00:00:00Z',
      },
    })
    post.mockResolvedValueOnce({
      data: {
        message: 'rejected',
        review_status: 'rejected',
        operated_at: '2026-05-01T00:05:00Z',
      },
    })

    await expect(approveReview(18, approvePayload)).resolves.toMatchObject({
      action: 'approve',
      status: 'approved',
      message: 'approved',
    })
    await expect(rejectReview(18, rejectPayload)).resolves.toMatchObject({
      action: 'reject',
      status: 'rejected',
      message: 'rejected',
    })

    expect(post).toHaveBeenNthCalledWith(1, '/admin/skills/reviews/18/approve', approvePayload)
    expect(post).toHaveBeenNthCalledWith(2, '/admin/skills/reviews/18/reject', rejectPayload)
  })

  it('uses governance, runtime, settlement and governance action endpoints', async () => {
    get.mockResolvedValueOnce({
      data: {
        items: [],
        total: 0,
        page: 1,
        page_size: 20,
        pages: 1,
        summary: {
          total_count: 0,
          online_count: 0,
          force_private_count: 0,
          disabled_count: 0,
          pending_versions_count: 0,
        },
      },
    })
    get.mockResolvedValueOnce({
      data: {
        items: [],
        total: 0,
        page: 1,
        page_size: 20,
        pages: 1,
        summary: {
          total_skills: 0,
          active_skills: 0,
          requests_24h: 0,
          success_rate: 0,
          p95_latency_ms: 0,
          warning_count: 0,
          critical_count: 0,
        },
        events: [],
      },
    })
    get.mockResolvedValueOnce({
      data: {
        items: [],
        total: 0,
        page: 1,
        page_size: 20,
        pages: 1,
        summary: {
          pending_amount: 0,
          settled_amount: 0,
          frozen_amount: 0,
          pending_skill_count: 0,
          currency: 'CNY',
        },
      },
    })
    post.mockResolvedValueOnce({
      data: {
        message: 'disabled',
        governance_status: 'disabled',
        operated_at: '2026-05-01T00:10:00Z',
      },
    })
    post.mockResolvedValueOnce({
      data: {
        message: 'forced private',
        governance_status: 'force_private',
        operated_at: '2026-05-01T00:11:00Z',
      },
    })

    await expect(
      listGovernanceSkills(1, 20, {
        search: 'market',
        governance_status: 'disabled',
        review_status: 'approved',
        visibility: 'public',
      }),
    ).resolves.toMatchObject({
      items: [],
      summary: {
        total_count: 0,
      },
    })
    await expect(
      getRuntimeOverview(1, 20, {
        search: 'market',
        health_status: 'warning',
      }),
    ).resolves.toMatchObject({
      items: [],
      summary: {
        total_skills: 0,
      },
      events: [],
    })
    await expect(
      listSettlements(1, 20, {
        search: 'market',
        settlement_status: 'pending',
      }),
    ).resolves.toMatchObject({
      items: [],
      summary: {
        currency: 'CNY',
      },
    })
    await expect(disableSkill(55, { reason: 'abuse' })).resolves.toMatchObject({
      action: 'disable',
      status: 'disabled',
    })
    await expect(forceSkillPrivate(55, { reason: 'license' })).resolves.toMatchObject({
      action: 'force-private',
      status: 'force_private',
    })

    expect(get).toHaveBeenNthCalledWith(1, '/admin/skills/governance', {
      params: {
        page: 1,
        page_size: 20,
        search: 'market',
        governance_status: 'disabled',
        review_status: 'approved',
        visibility: 'public',
      },
      signal: undefined,
    })
    expect(get).toHaveBeenNthCalledWith(2, '/admin/skills/runtime', {
      params: {
        page: 1,
        page_size: 20,
        search: 'market',
        health_status: 'warning',
      },
      signal: undefined,
    })
    expect(get).toHaveBeenNthCalledWith(3, '/admin/skills/settlements', {
      params: {
        page: 1,
        page_size: 20,
        search: 'market',
        settlement_status: 'pending',
      },
      signal: undefined,
    })
    expect(post).toHaveBeenNthCalledWith(1, '/admin/skills/55/disable', { reason: 'abuse' })
    expect(post).toHaveBeenNthCalledWith(2, '/admin/skills/55/force-private', { reason: 'license' })
  })
})
