import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import AccountQuotaInfo from '../AccountQuotaInfo.vue'
import type { Account } from '@/types'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key
    })
  }
})

function makeAccount(overrides: Partial<Account>): Account {
  return {
    id: 1,
    name: 'gemini-account',
    platform: 'gemini',
    type: 'oauth',
    proxy_id: null,
    concurrency: 1,
    priority: 1,
    status: 'active',
    error_message: null,
    last_used_at: null,
    expires_at: null,
    auto_pause_on_expired: true,
    created_at: '2026-05-03T00:00:00Z',
    updated_at: '2026-05-03T00:00:00Z',
    schedulable: true,
    rate_limited_at: null,
    rate_limit_reset_at: null,
    overload_until: null,
    temp_unschedulable_until: null,
    temp_unschedulable_reason: null,
    session_window_start: null,
    session_window_end: null,
    session_window_status: null,
    credentials: {},
    ...overrides
  }
}

describe('AccountQuotaInfo', () => {
  it('distinguishes code assist standard and enterprise tiers', () => {
    const standardWrapper = mount(AccountQuotaInfo, {
      props: {
        account: makeAccount({
          credentials: {
            oauth_type: 'code_assist',
            tier_id: 'gcp_standard'
          }
        })
      }
    })

    const enterpriseWrapper = mount(AccountQuotaInfo, {
      props: {
        account: makeAccount({
          credentials: {
            oauth_type: 'code_assist',
            tier_id: 'gcp_enterprise'
          }
        })
      }
    })

    expect(standardWrapper.text()).toContain('GCP Standard')
    expect(standardWrapper.text()).not.toContain('GCP Enterprise')
    expect(enterpriseWrapper.text()).toContain('GCP Enterprise')
  })

  it('normalizes google one pro metadata without leaking verbose detail rows', () => {
    const wrapper = mount(AccountQuotaInfo, {
      props: {
        account: makeAccount({
          credentials: {
            oauth_type: 'google_one',
            tier_id: 'g1-pro-tier',
            email: 'user@example.com',
            project_id: 'refreshing-center-hnmwg',
            scope: 'openid https://www.googleapis.com/auth/userinfo.email https://www.googleapis.com/auth/cloud-platform'
          },
          extra: {
            plan_name: 'Gemini Code Assist in Google One AI Pro',
            gemini_paid_tier_name: 'Gemini Code Assist in Google One AI Pro',
            gemini_available_credits: [
              {
                creditType: 'GOOGLE_ONE_AI',
                creditAmount: '100'
              }
            ]
          }
        })
      }
    })

    expect(wrapper.text()).toContain('Google One Pro')
    expect(wrapper.text()).toContain('admin.accounts.gemini.quotaPolicy.rows.googleOne.limitsPro')
    expect(wrapper.text()).toContain('admin.accounts.gemini.rateLimit.ok')
    expect(wrapper.text()).not.toContain('admin.accounts.gemini.quotaPolicy.rows.googleOne.limitsFree')
    expect(wrapper.text()).not.toContain('user@example.com')
    expect(wrapper.text()).not.toContain('refreshing-center-hnmwg')
    expect(wrapper.text()).not.toContain('Gemini Code Assist in Google One AI Pro')
  })

  it('shows Gemini rate limit countdown when account is limited', () => {
    const future = new Date(Date.now() + 90_000).toISOString()
    const wrapper = mount(AccountQuotaInfo, {
      props: {
        account: makeAccount({
          credentials: {
            oauth_type: 'code_assist',
            tier_id: 'gcp_enterprise'
          },
          rate_limit_reset_at: future
        })
      }
    })

    expect(wrapper.text()).toContain('GCP Enterprise')
    expect(wrapper.text()).not.toContain('GCP Standard')
    expect(wrapper.text()).toContain('admin.accounts.gemini.rateLimit.limited')
  })

  it('falls back to ai studio badge for api key accounts without oauth metadata', () => {
    const wrapper = mount(AccountQuotaInfo, {
      props: {
        account: makeAccount({
          type: 'apikey',
          credentials: {
            tier_id: 'aistudio_free'
          }
        })
      }
    })

    expect(wrapper.text()).toContain('AI Studio Free Tier')
    expect(wrapper.text()).not.toContain('user@example.com')
  })

  it('shows Vertex AI badge for Gemini service accounts', () => {
    const wrapper = mount(AccountQuotaInfo, {
      props: {
        account: makeAccount({
          type: 'service_account',
          credentials: {
            tier_id: 'vertex',
            project_id: 'vertex-proj',
            client_email: 'svc@vertex-proj.iam.gserviceaccount.com',
            location: 'global'
          }
        })
      }
    })

    expect(wrapper.text()).toContain('Vertex AI')
    expect(wrapper.text()).toContain('admin.accounts.gemini.quotaPolicy.rows.vertex.channel')
    expect(wrapper.text()).toContain('admin.accounts.gemini.quotaPolicy.rows.vertex.limits')
    expect(wrapper.text()).not.toContain('AI Studio')
  })
})
