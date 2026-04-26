import { beforeEach, describe, expect, it } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useAppStore } from '@/stores/app'
import type { PublicSettings } from '@/types'

function publicSettings(overrides: Partial<PublicSettings> = {}): PublicSettings {
  return {
    registration_enabled: true,
    email_verify_enabled: false,
    force_email_on_third_party_signup: false,
    registration_email_suffix_whitelist: [],
    promo_code_enabled: false,
    password_reset_enabled: true,
    invitation_code_enabled: false,
    turnstile_enabled: false,
    turnstile_site_key: '',
    site_name: 'Sub2API',
    site_logo: '',
    site_subtitle: '',
    api_base_url: '',
    contact_info: '',
    support_qr_codes: [],
    doc_url: '',
    home_content: '',
    hide_ccs_import_button: false,
    payment_enabled: true,
    affiliate_enabled: false,
    ticket_enabled: false,
    table_default_page_size: 20,
    table_page_size_options: [20, 50, 100],
    custom_menu_items: [],
    custom_endpoints: [],
    linuxdo_oauth_enabled: false,
    wechat_oauth_enabled: false,
    oidc_oauth_enabled: false,
    oidc_oauth_provider_name: '',
    backend_mode_enabled: false,
    version: 'test',
    balance_low_notify_enabled: false,
    account_quota_notify_enabled: false,
    balance_low_notify_threshold: 0,
    channel_monitor_enabled: true,
    channel_monitor_default_interval_seconds: 60,
    available_channels_enabled: false,
    ...overrides,
  }
}

describe('channel monitor feature flag helpers', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('disables direct monitor routes when public settings disable channel monitor', async () => {
    const appStore = useAppStore()
    appStore.cachedPublicSettings = publicSettings({ channel_monitor_enabled: false })
    const { isChannelMonitorRouteEnabled } = await import('@/utils/featureFlags')

    expect(isChannelMonitorRouteEnabled()).toBe(false)
  })

  it('uses channel monitor public interval instead of hardcoded 60 seconds', async () => {
    const appStore = useAppStore()
    appStore.cachedPublicSettings = publicSettings({ channel_monitor_default_interval_seconds: 120 })
    const { getChannelMonitorRefreshIntervalSeconds } = await import('@/utils/featureFlags')

    expect(getChannelMonitorRefreshIntervalSeconds()).toBe(120)
  })
})
