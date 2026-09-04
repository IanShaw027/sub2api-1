'use strict'
/*
 * Sub2API standalone mock backend (no deps).
 * Usage: node server.js   (listens on PORT || 8091)
 * Point the Vite dev server at it: VITE_DEV_PROXY_TARGET=http://localhost:8091 npm run dev
 */
const http = require('http')
const { URL } = require('url')

const PORT = Number(process.env.PORT || 8091)
const NOW = () => new Date()
const iso = (d) => new Date(d).toISOString()
const minutesAgo = (m) => iso(Date.now() - m * 60_000)
const hoursAgo = (h) => iso(Date.now() - h * 3_600_000)
const daysAgo = (d) => iso(Date.now() - d * 86_400_000)
const inMinutes = (m) => iso(Date.now() + m * 60_000)
const inHours = (h) => iso(Date.now() + h * 3_600_000)
const inDays = (d) => iso(Date.now() + d * 86_400_000)
const dateStr = (offsetDays) => {
  const d = new Date(Date.now() + offsetDays * 86_400_000)
  return d.toISOString().slice(0, 10)
}
const round2 = (n) => Math.round(n * 100) / 100
// deterministic pseudo-random so series look organic but stable between reloads
function seeded(seed) {
  let s = seed >>> 0 || 1
  return () => {
    s = (s * 1664525 + 1013904223) >>> 0
    return s / 0xffffffff
  }
}

// ==================== Users ====================
const baseUserFields = {
  avatar_url: null,
  auth_bindings: { email: true, github: true, linuxdo: false },
  email_bound: true,
  linuxdo_bound: false,
  oidc_bound: false,
  wechat_bound: false,
  frozen_balance: 0,
  rpm_limit: 0,
  allowed_groups: null,
  balance_notify_enabled: true,
  balance_notify_threshold: 10,
  balance_notify_extra_emails: [],
  subscriptions: [],
  deleted_at: null
}

const ADMIN_USER = {
  ...baseUserFields,
  id: 1,
  username: 'Admin',
  email: 'admin@sub2api.dev',
  role: 'admin',
  balance: 142.6,
  concurrency: 20,
  status: 'active',
  last_active_at: minutesAgo(2),
  created_at: '2025-03-12T08:30:00Z',
  updated_at: hoursAgo(3)
}

const NORMAL_USER = {
  ...baseUserFields,
  id: 42,
  username: '林小雨',
  email: 'xiaoyu.lin@example.com',
  role: 'user',
  balance: 38.25,
  concurrency: 5,
  status: 'active',
  last_active_at: minutesAgo(11),
  created_at: '2025-11-02T02:14:00Z',
  updated_at: hoursAgo(1)
}

function currentUser(req) {
  // Allow the frontend to pretend to be a normal user by sending "mock-user-token".
  const auth = String(req.headers.authorization || '')
  if (auth.includes('mock-user-token')) return NORMAL_USER
  return ADMIN_USER
}

// ==================== Groups ====================
function makeGroup(overrides) {
  return {
    id: 0,
    name: '',
    description: null,
    platform: 'anthropic',
    rate_multiplier: 1,
    rpm_limit: 0,
    max_reasoning_effort: '',
    max_reasoning_effort_over_limit: 'downgrade',
    reasoning_effort_mappings: [],
    is_exclusive: false,
    status: 'active',
    subscription_type: 'standard',
    daily_limit_usd: null,
    weekly_limit_usd: null,
    monthly_limit_usd: null,
    long_context_pricing_enabled: false,
    allow_image_generation: false,
    allow_batch_image_generation: false,
    image_rate_independent: false,
    image_rate_multiplier: 1,
    batch_image_discount_multiplier: 0.5,
    batch_image_hold_multiplier: 1.2,
    image_price_1k: null,
    image_price_2k: null,
    image_price_4k: null,
    video_rate_independent: false,
    video_rate_multiplier: 1,
    video_price_480p: null,
    video_price_720p: null,
    video_price_1080p: null,
    web_search_price_per_call: null,
    search_price_per_1k: null,
    audio_realtime_price_per_min: null,
    audio_tts_price_per_million_chars: null,
    audio_stt_price_per_hour: null,
    peak_rate_enabled: false,
    peak_start: '09:00',
    peak_end: '18:00',
    peak_rate_multiplier: 1.2,
    claude_code_only: false,
    fallback_group_id: null,
    fallback_group_id_on_invalid_request: null,
    allow_messages_dispatch: false,
    allow_live: false,
    require_oauth_only: false,
    require_privacy_set: false,
    created_at: '2025-03-12T08:31:00Z',
    updated_at: daysAgo(4),
    // AdminGroup extras
    force_openai_fast: false,
    free_openai_fast: false,
    model_pricing: [],
    profit_control_enabled: false,
    profit_min_margin: 0.1,
    profit_safety_buffer: 0.05,
    model_routing: null,
    model_routing_enabled: false,
    mcp_xml_inject: false,
    supported_model_scopes: [],
    account_count: 0,
    active_account_count: 0,
    rate_limited_account_count: 0,
    sort_order: 0,
    ...overrides
  }
}

const GROUPS = [
  makeGroup({ id: 1, name: '默认分组', description: '所有用户可用的标准 Claude 分组', platform: 'anthropic', rate_multiplier: 1, account_count: 31, active_account_count: 29, rate_limited_account_count: 2, sort_order: 0 }),
  makeGroup({ id: 2, name: 'Claude Max', description: 'Claude Max 订阅池，高并发 / 长上下文', platform: 'anthropic', rate_multiplier: 1.2, is_exclusive: true, subscription_type: 'subscription', daily_limit_usd: 40, weekly_limit_usd: 200, monthly_limit_usd: 600, long_context_pricing_enabled: true, claude_code_only: true, account_count: 24, active_account_count: 23, rate_limited_account_count: 1, sort_order: 1 }),
  makeGroup({ id: 3, name: 'OpenAI Codex', description: 'ChatGPT Codex OAuth 账号池', platform: 'openai', rate_multiplier: 0.9, max_reasoning_effort: 'high', allow_messages_dispatch: true, default_mapped_model: 'gpt-5-codex', web_search_price_per_call: 0.01, account_count: 18, active_account_count: 17, rate_limited_account_count: 0, sort_order: 2 }),
  makeGroup({ id: 4, name: 'Gemini', description: 'Gemini 2.5 Pro / Flash，支持图片生成', platform: 'gemini', rate_multiplier: 0.8, allow_image_generation: true, image_price_1k: 0.04, image_price_2k: 0.08, image_price_4k: 0.16, account_count: 13, active_account_count: 10, rate_limited_account_count: 0, sort_order: 3 })
]
const groupById = (id) => GROUPS.find((g) => g.id === id) || null
// The non-admin Group shape (strip admin-only keys)
function publicGroup(g) {
  const {
    force_openai_fast, free_openai_fast, model_pricing, profit_control_enabled, profit_min_margin,
    profit_safety_buffer, model_routing, model_routing_enabled, mcp_xml_inject, supported_model_scopes,
    account_count, active_account_count, rate_limited_account_count, sort_order, ...rest
  } = g
  return rest
}

// ==================== Accounts ====================
function progress(util, resetsInMinutes, stats) {
  return {
    utilization: util,
    resets_at: inMinutes(resetsInMinutes),
    remaining_seconds: resetsInMinutes * 60,
    window_stats: stats || null,
    predicted_total_cost: stats ? round2(stats.cost * (100 / Math.max(util, 1))) : null
  }
}

function makeAccount(o) {
  return {
    id: 0,
    name: '',
    notes: null,
    platform: 'anthropic',
    type: 'oauth',
    credentials: {},
    credentials_status: { has_access_token: true, has_refresh_token: true },
    extra: {},
    proxy_id: null,
    proxy_fallback_origin_id: null,
    proxy_fallback_origin_name: null,
    concurrency: 5,
    load_factor: 1,
    current_concurrency: 0,
    scheduler_score: null,
    scheduler_scores: null,
    cyber_count: 0,
    cyber_latest_at: null,
    priority: 50,
    rate_multiplier: 1,
    status: 'active',
    error_message: null,
    last_used_at: minutesAgo(3),
    expires_at: Math.floor(Date.now() / 1000) + 86_400 * 20,
    auto_pause_on_expired: true,
    created_at: daysAgo(60),
    updated_at: hoursAgo(1),
    group_ids: [1],
    groups: [publicGroup(GROUPS[0])],
    schedulable: true,
    rate_limited_at: null,
    rate_limit_reset_at: null,
    overload_until: null,
    temp_unschedulable_until: null,
    temp_unschedulable_reason: null,
    session_window_start: hoursAgo(2),
    session_window_end: inHours(3),
    session_window_status: 'allowed',
    window_cost_limit: null,
    window_cost_sticky_reserve: null,
    max_sessions: null,
    session_idle_timeout_minutes: null,
    base_rpm: null,
    rpm_strategy: null,
    rpm_sticky_buffer: null,
    user_msg_queue_mode: null,
    enable_tls_fingerprint: false,
    quota_limit: null,
    quota_used: null,
    quota_daily_limit: null,
    quota_daily_used: null,
    quota_weekly_limit: null,
    quota_weekly_used: null,
    current_window_cost: null,
    active_sessions: null,
    current_rpm: null,
    ...o
  }
}

const ACCOUNTS = [
  makeAccount({
    id: 101, name: 'claude-max-01', platform: 'anthropic', type: 'oauth', priority: 10, concurrency: 8, current_concurrency: 3,
    group_ids: [1, 2], groups: [publicGroup(GROUPS[0]), publicGroup(GROUPS[1])], last_used_at: minutesAgo(1),
    extra: { plan_type: 'max', subscription_tier: 'max_20x', email: 'ops-01@sub2api.dev' },
    window_cost_limit: 25, current_window_cost: 11.42, max_sessions: 6, active_sessions: 3, base_rpm: 60, current_rpm: 14,
    scheduler_score: { base_score: 92, sticky_score: 18, sticky_weighted_enabled: true }, notes: '主力账号，勿动'
  }),
  makeAccount({
    id: 102, name: 'claude-max-02', platform: 'anthropic', type: 'oauth', priority: 10, concurrency: 8, current_concurrency: 1,
    group_ids: [2], groups: [publicGroup(GROUPS[1])], last_used_at: minutesAgo(6),
    status: 'active', rate_limited_at: minutesAgo(12), rate_limit_reset_at: inMinutes(48),
    extra: { plan_type: 'max', subscription_tier: 'max_5x', email: 'ops-02@sub2api.dev' },
    window_cost_limit: 25, current_window_cost: 24.8, max_sessions: 6, active_sessions: 5
  }),
  makeAccount({
    id: 103, name: 'claude-console-apikey', platform: 'anthropic', type: 'apikey', priority: 30, concurrency: 20, current_concurrency: 4,
    credentials_status: { has_api_key: true }, group_ids: [1], groups: [publicGroup(GROUPS[0])], last_used_at: minutesAgo(2),
    expires_at: null, session_window_start: null, session_window_end: null, session_window_status: null,
    quota_limit: 500, quota_used: 318.6, quota_daily_limit: 60, quota_daily_used: 41.2, quota_weekly_limit: 300, quota_weekly_used: 188.9,
    quota_daily_reset_mode: 'fixed', quota_daily_reset_hour: 0, quota_reset_timezone: 'Asia/Shanghai', rate_multiplier: 0.85
  }),
  makeAccount({
    id: 104, name: 'claude-setup-token-03', platform: 'anthropic', type: 'setup-token', priority: 40, concurrency: 4, current_concurrency: 0,
    group_ids: [1], groups: [publicGroup(GROUPS[0])], status: 'error', schedulable: false, last_used_at: hoursAgo(5),
    error_message: 'OAuth token refresh failed: invalid_grant (refresh token revoked)',
    extra: { plan_type: 'pro', subscription_tier: 'pro' }, expires_at: Math.floor(Date.now() / 1000) - 3600
  }),
  makeAccount({
    id: 105, name: 'codex-team-01', platform: 'openai', type: 'oauth', priority: 20, concurrency: 6, current_concurrency: 2,
    group_ids: [3], groups: [publicGroup(GROUPS[2])], last_used_at: minutesAgo(1),
    credentials_status: { has_access_token: true, has_refresh_token: true, has_id_token: true },
    extra: {
      plan_type: 'team', email: 'codex-01@sub2api.dev', chatgpt_account_id: 'acct_7fA2…9c',
      codex_primary_used_percent: 46, codex_primary_reset_after_seconds: 9_800, codex_primary_window_minutes: 300,
      codex_secondary_used_percent: 71, codex_secondary_reset_after_seconds: 302_400, codex_secondary_window_minutes: 10_080,
      codex_usage_updated_at: minutesAgo(4), openai_compact_mode: 'auto', privacy_mode: 'set'
    },
    session_window_start: null, session_window_end: null, session_window_status: null, cyber_count: 2, cyber_latest_at: hoursAgo(9)
  }),
  makeAccount({
    id: 106, name: 'gemini-workspace-01', platform: 'gemini', type: 'oauth', priority: 25, concurrency: 10, current_concurrency: 1,
    group_ids: [4], groups: [publicGroup(GROUPS[3])], last_used_at: minutesAgo(15),
    extra: { project_id: 'sub2api-prod-4711', tier: 'paid' },
    session_window_start: null, session_window_end: null, session_window_status: null,
    quota_daily_limit: 40, quota_daily_used: 9.6
  }),
  makeAccount({
    id: 107, name: 'antigravity-lab', platform: 'antigravity', type: 'oauth', priority: 60, concurrency: 3, current_concurrency: 0,
    group_ids: [1], groups: [publicGroup(GROUPS[0])], last_used_at: hoursAgo(1), schedulable: true,
    temp_unschedulable_until: inMinutes(25), temp_unschedulable_reason: 'stream_timeout',
    session_window_start: null, session_window_end: null, session_window_status: null,
    extra: { email: 'lab@sub2api.dev', subscription_tier: 'ultra' }
  }),
  makeAccount({
    id: 108, name: 'grok-heavy-01', platform: 'grok', type: 'apikey', priority: 70, concurrency: 5, current_concurrency: 0,
    credentials_status: { has_api_key: true }, group_ids: [], groups: [], last_used_at: daysAgo(2), schedulable: false,
    expires_at: null, session_window_start: null, session_window_end: null, session_window_status: null,
    extra: { base_url_mode: 'default' }, notes: '灰度测试，暂停调度'
  })
]

function accountUsage(a) {
  switch (a.platform) {
    case 'anthropic': {
      const base = a.id === 102 ? 98 : a.id === 104 ? 0 : 42 + (a.id % 5) * 7
      return {
        source: 'passive',
        updated_at: minutesAgo(3),
        five_hour: progress(base, 168, { requests: 214, tokens: 5_820_000, cost: 11.42, standard_cost: 11.42, user_cost: 13.7 }),
        seven_day: progress(Math.min(100, base + 14), 4_120, { requests: 2_840, tokens: 92_400_000, cost: 168.3, standard_cost: 168.3, user_cost: 201.9 }),
        seven_day_sonnet: progress(Math.max(0, base - 22), 4_120, { requests: 910, tokens: 21_100_000, cost: 22.7 }),
        seven_day_fable: progress(Math.min(100, base + 3), 4_120, { requests: 1_930, tokens: 71_300_000, cost: 145.6 }),
        subscription_tier: a.extra && a.extra.subscription_tier ? a.extra.subscription_tier : 'pro',
        ...(a.status === 'error' ? { needs_reauth: true, error_code: 'unauthenticated', error: 'refresh token revoked' } : {})
      }
    }
    case 'openai':
      return {
        source: 'passive', updated_at: minutesAgo(4),
        five_hour: progress(46, 163, { requests: 96, tokens: 3_100_000, cost: 4.8 }),
        seven_day: progress(71, 5_040, { requests: 1_420, tokens: 38_000_000, cost: 61.2 }),
        seven_day_sonnet: null, subscription_tier: 'team'
      }
    case 'gemini':
      return {
        source: 'active', updated_at: minutesAgo(9),
        five_hour: null, seven_day: null, seven_day_sonnet: null,
        gemini_shared_daily: progress(24, 610, { requests: 1_020, tokens: 12_400_000, cost: 9.6 }),
        gemini_pro_daily: progress(31, 610), gemini_flash_daily: progress(12, 610),
        gemini_shared_minute: progress(8, 1), gemini_pro_minute: progress(11, 1), gemini_flash_minute: progress(3, 1)
      }
    case 'antigravity':
      return {
        source: 'active', updated_at: minutesAgo(30),
        five_hour: null, seven_day: null, seven_day_sonnet: null,
        antigravity_quota: {
          'claude-sonnet-4-5': { utilization: 37, reset_time: inHours(4) },
          'gemini-2.5-pro': { utilization: 12, reset_time: inHours(4) },
          'gpt-5': { utilization: 64, reset_time: inHours(4) }
        },
        subscription_tier: 'ultra'
      }
    case 'grok':
      return {
        source: 'passive', updated_at: hoursAgo(2),
        five_hour: null, seven_day: null, seven_day_sonnet: null,
        grok_request_quota: { limit: 500, remaining: 412, reset_at: inHours(20) },
        grok_token_quota: { limit: 10_000_000, remaining: 7_260_000, reset_at: inHours(20) },
        grok_local_usage: { requests: 88, tokens: 2_740_000, cost: 3.9 },
        grok_billing: { period_type: 'monthly', usage_percent: 27, prepaid_balance: 73.4, monthly_limit: 100, monthly_used: 26.6, plan: 'SuperGrok Heavy' }
      }
    default:
      return { source: 'passive', updated_at: minutesAgo(5), five_hour: null, seven_day: null, seven_day_sonnet: null }
  }
}

function accountTodayStats(a) {
  const rnd = seeded(a.id * 13)
  const requests = a.status === 'error' ? 0 : Math.round(120 + rnd() * 1800)
  const tokens = requests * Math.round(9_000 + rnd() * 20_000)
  const cost = round2(tokens / 1_000_000 * (2.4 + rnd() * 3))
  return { requests, tokens, cost, standard_cost: cost, user_cost: round2(cost * 1.18) }
}

function accountStats(a, days) {
  const rnd = seeded(a.id * 31)
  const history = []
  let totalCost = 0, totalReq = 0, totalTok = 0, totalUser = 0
  for (let i = days - 1; i >= 0; i--) {
    const requests = Math.round(300 + rnd() * 1500)
    const tokens = requests * Math.round(12_000 + rnd() * 10_000)
    const cost = round2(tokens / 1_000_000 * 3.1)
    const user_cost = round2(cost * 1.2)
    totalCost += cost; totalReq += requests; totalTok += tokens; totalUser += user_cost
    history.push({ date: dateStr(-i), label: dateStr(-i).slice(5), requests, tokens, cost, actual_cost: cost, user_cost })
  }
  const today = history[history.length - 1]
  const highest = history.reduce((m, h) => (h.cost > m.cost ? h : m), history[0])
  const highestReq = history.reduce((m, h) => (h.requests > m.requests ? h : m), history[0])
  return {
    history,
    summary: {
      days, actual_days_used: days, total_cost: round2(totalCost), total_user_cost: round2(totalUser), total_standard_cost: round2(totalCost),
      total_requests: totalReq, total_tokens: totalTok, avg_daily_cost: round2(totalCost / days), avg_daily_user_cost: round2(totalUser / days),
      avg_daily_requests: Math.round(totalReq / days), avg_daily_tokens: Math.round(totalTok / days), avg_duration_ms: 1_740,
      today: { date: today.date, cost: today.cost, user_cost: today.user_cost, requests: today.requests, tokens: today.tokens },
      highest_cost_day: { date: highest.date, label: highest.label, cost: highest.cost, user_cost: highest.user_cost, requests: highest.requests },
      highest_request_day: { date: highestReq.date, label: highestReq.label, requests: highestReq.requests, cost: highestReq.cost, user_cost: highestReq.user_cost }
    },
    models: MODEL_STATS.slice(0, 4),
    endpoints: ENDPOINT_STATS,
    upstream_endpoints: ENDPOINT_STATS
  }
}

// ==================== API Keys ====================
function makeKey(o) {
  return {
    id: 0, user_id: 42, key: '', name: '', group_id: 1, status: 'active',
    ip_whitelist: [], ip_blacklist: [], last_used_at: minutesAgo(4), last_used_ip: '203.0.113.42',
    quota: 0, quota_used: 0, expires_at: null, created_at: daysAgo(30), updated_at: hoursAgo(2),
    current_concurrency: 0, group: publicGroup(GROUPS[0]),
    rate_limit_5h: 0, rate_limit_1d: 0, rate_limit_7d: 0, usage_5h: 0, usage_1d: 0, usage_7d: 0,
    window_5h_start: null, window_1d_start: null, window_7d_start: null, reset_5h_at: null, reset_1d_at: null, reset_7d_at: null,
    ...o
  }
}
const API_KEYS = [
  makeKey({ id: 501, name: 'production', key: 'sk-s2a-a91f3b7c0d2e4f6a8b1c3d5e7f9a0b2c3f2a', group_id: 2, group: publicGroup(GROUPS[1]), quota: 500, quota_used: 212.35, current_concurrency: 2,
    rate_limit_5h: 20, rate_limit_1d: 80, rate_limit_7d: 400, usage_5h: 8.42, usage_1d: 31.7, usage_7d: 186.2,
    window_5h_start: hoursAgo(2), window_1d_start: hoursAgo(9), window_7d_start: daysAgo(4), reset_5h_at: inHours(3), reset_1d_at: inHours(15), reset_7d_at: inDays(3),
    last_used_at: minutesAgo(1), created_at: daysAgo(120) }),
  makeKey({ id: 502, name: 'claude-code-laptop', key: 'sk-s2a-4c8e2a1f9b7d3e5c6a0f2b4d8e1c7a9b5d3e', group_id: 2, group: publicGroup(GROUPS[1]), quota: 100, quota_used: 37.8, ip_whitelist: ['10.0.0.0/8'],
    rate_limit_5h: 10, usage_5h: 6.1, window_5h_start: hoursAgo(1), reset_5h_at: inHours(4), last_used_at: minutesAgo(12), created_at: daysAgo(45), expires_at: inDays(200) }),
  makeKey({ id: 503, name: 'cursor-team', key: 'sk-s2a-7d2b9e4a1c6f8b3d5e0a2c4f6b8d1e3a7c9f', group_id: 3, group: publicGroup(GROUPS[2]), quota: 0, quota_used: 88.12, current_concurrency: 1,
    last_used_at: minutesAgo(3), created_at: daysAgo(80), expires_at: inDays(40) }),
  makeKey({ id: 504, name: 'ci-runner', key: 'sk-s2a-e5a1c7f3b9d2e8a4c6f0b1d3e5a7c9f2b4d6', group_id: 4, group: publicGroup(GROUPS[3]), status: 'expired', quota: 50, quota_used: 49.96,
    last_used_at: daysAgo(9), last_used_ip: '198.51.100.7', created_at: daysAgo(200), expires_at: daysAgo(7) }),
  makeKey({ id: 505, name: 'temp-demo', key: 'sk-s2a-1b3d5f7a9c2e4a6c8e0b2d4f6a8c1e3b5d7f', group_id: null, group: undefined, status: 'disabled', quota: 10, quota_used: 2.4,
    last_used_at: daysAgo(3), last_used_ip: null, created_at: daysAgo(12), expires_at: inDays(2) })
]

// ==================== Dashboard series ====================
const MODEL_STATS = [
  { model: 'claude-sonnet-4-5', requests: 61_240, input_tokens: 412_000_000, output_tokens: 48_200_000, cache_creation_tokens: 96_000_000, cache_read_tokens: 620_000_000, total_tokens: 1_176_200_000, cost: 684.2, actual_cost: 702.9, account_cost: 610.4 },
  { model: 'claude-fable-4-1', requests: 12_310, input_tokens: 96_000_000, output_tokens: 14_800_000, cache_creation_tokens: 21_000_000, cache_read_tokens: 188_000_000, total_tokens: 319_800_000, cost: 412.5, actual_cost: 431.8, account_cost: 380.1 },
  { model: 'gpt-5-codex', requests: 33_420, input_tokens: 188_000_000, output_tokens: 22_400_000, cache_creation_tokens: 0, cache_read_tokens: 92_000_000, total_tokens: 302_400_000, cost: 128.6, actual_cost: 118.4, account_cost: 96.2 },
  { model: 'gemini-2.5-pro', requests: 14_860, input_tokens: 74_000_000, output_tokens: 9_100_000, cache_creation_tokens: 0, cache_read_tokens: 12_000_000, total_tokens: 95_100_000, cost: 42.1, actual_cost: 35.6, account_cost: 28.8 },
  { model: 'claude-haiku-4-5', requests: 6_600, input_tokens: 31_000_000, output_tokens: 4_400_000, cache_creation_tokens: 2_000_000, cache_read_tokens: 9_000_000, total_tokens: 46_400_000, cost: 16.8, actual_cost: 16.8, account_cost: 14.1 }
]
const ENDPOINT_STATS = [
  { endpoint: '/v1/messages', requests: 80_150, total_tokens: 1_542_400_000, cost: 1_113.5, actual_cost: 1_151.5 },
  { endpoint: '/v1/responses', requests: 33_420, total_tokens: 302_400_000, cost: 128.6, actual_cost: 118.4 },
  { endpoint: '/v1/chat/completions', requests: 9_800, total_tokens: 64_000_000, cost: 26.2, actual_cost: 24.9 },
  { endpoint: '/v1beta/models:generateContent', requests: 5_060, total_tokens: 31_100_000, cost: 15.9, actual_cost: 10.7 }
]
const GROUP_STATS = [
  { group_id: 2, group_name: 'Claude Max', requests: 58_400, total_tokens: 1_100_000_000, cost: 812.4, actual_cost: 851.3, account_cost: 702.5 },
  { group_id: 1, group_name: '默认分组', requests: 21_750, total_tokens: 442_000_000, cost: 301.1, actual_cost: 301.1, account_cost: 260.9 },
  { group_id: 3, group_name: 'OpenAI Codex', requests: 33_420, total_tokens: 302_400_000, cost: 128.6, actual_cost: 118.4, account_cost: 96.2 },
  { group_id: 4, group_name: 'Gemini', requests: 14_860, total_tokens: 95_100_000, cost: 42.1, actual_cost: 35.6, account_cost: 28.8 }
]

function trendSeries(days, scale = 1) {
  const rnd = seeded(7)
  const out = []
  for (let i = days - 1; i >= 0; i--) {
    const weekday = new Date(Date.now() - i * 86_400_000).getUTCDay()
    const weekendDip = weekday === 0 || weekday === 6 ? 0.62 : 1
    const requests = Math.round((96_000 + rnd() * 48_000 + (days - i) * 1_100) * weekendDip * scale)
    const input = requests * Math.round(4_200 + rnd() * 1_500)
    const output = requests * Math.round(560 + rnd() * 200)
    const cacheC = Math.round(input * 0.22)
    const cacheR = Math.round(input * 1.4)
    const total = input + output + cacheC + cacheR
    const cost = round2(total / 1_000_000 * 0.62)
    out.push({ date: dateStr(-i), requests, input_tokens: input, output_tokens: output, cache_creation_tokens: cacheC, cache_read_tokens: cacheR, total_tokens: total, cost, actual_cost: round2(cost * 1.04) })
  }
  return out
}

// Hourly series for the "today / last 24h" window (dashboard uses granularity=hour when the range is ≤1 day).
function trendSeriesHourly(hours, scale = 1) {
  const rnd = seeded(11)
  const out = []
  const now = new Date(Date.now())
  now.setUTCMinutes(0, 0, 0)
  for (let i = hours - 1; i >= 0; i--) {
    const at = new Date(now.getTime() - i * 3_600_000)
    const h = at.getUTCHours()
    const daytime = h >= 1 && h <= 14 ? 1 : 0.45 // UTC 01–14 ≈ 09–22 CST
    const requests = Math.round((4_200 + rnd() * 2_400) * daytime * scale)
    const input = requests * Math.round(4_200 + rnd() * 1_500)
    const output = requests * Math.round(560 + rnd() * 200)
    const cacheC = Math.round(input * 0.22)
    const cacheR = Math.round(input * 1.4)
    const total = input + output + cacheC + cacheR
    const cost = round2(total / 1_000_000 * 0.62)
    out.push({ date: at.toISOString().slice(0, 13) + ':00', requests, input_tokens: input, output_tokens: output, cache_creation_tokens: cacheC, cache_read_tokens: cacheR, total_tokens: total, cost, actual_cost: round2(cost * 1.04) })
  }
  return out
}
const trendFor = (query, scale = 1) => (query.get('granularity') === 'hour' ? trendSeriesHourly(24, scale) : trendSeries(rangeDays(query), scale))

// Ops: per-platform account availability (dashboard "platform health") and latest error events.
const ACCOUNT_AVAILABILITY = {
  enabled: true,
  platform: {
    anthropic: { platform: 'anthropic', total_accounts: 34, available_count: 32, rate_limit_count: 2, error_count: 0 },
    openai: { platform: 'openai', total_accounts: 22, available_count: 21, rate_limit_count: 0, error_count: 1 },
    gemini: { platform: 'gemini', total_accounts: 17, available_count: 17, rate_limit_count: 0, error_count: 0 },
    antigravity: { platform: 'antigravity', total_accounts: 8, available_count: 7, rate_limit_count: 1, error_count: 0 },
    grok: { platform: 'grok', total_accounts: 5, available_count: 5, rate_limit_count: 0, error_count: 0 }
  },
  group: {},
  account: {}
}
const OPS_ERROR_LOGS = [
  { id: 9001, created_at: minutesAgo(18), phase: 'upstream', type: 'rate_limited', error_owner: 'provider', error_source: 'upstream_http', severity: 'P2', status_code: 429, platform: 'anthropic', model: 'claude-sonnet-4-5', resolved: false, client_request_id: 'req_9001', request_id: 'up_9001', account_id: 102, account_name: 'claude-pro-team', message: '触发 429，默认分组自动回避 5 分钟' },
  { id: 9002, created_at: minutesAgo(41), phase: 'upstream', type: 'auth_failed', error_owner: 'provider', error_source: 'upstream_http', severity: 'P1', status_code: 401, platform: 'openai', model: 'gpt-5-codex', resolved: false, client_request_id: 'req_9002', request_id: 'up_9002', account_id: 105, account_name: 'codex-team-01', message: 'OAuth 刷新失败，账号已标记异常' },
  { id: 9003, created_at: minutesAgo(73), phase: 'upstream', type: 'timeout', error_owner: 'provider', error_source: 'upstream_http', severity: 'P3', status_code: 504, platform: 'gemini', model: 'gemini-2.5-pro', resolved: true, client_request_id: 'req_9003', request_id: 'up_9003', account_id: 106, account_name: 'gemini-workspace-01', message: '上游响应超时 30s，已自动重试成功' },
  { id: 9004, created_at: minutesAgo(126), phase: 'gateway', type: 'quota_exceeded', error_owner: 'platform', error_source: 'gateway', severity: 'P3', status_code: 402, platform: 'anthropic', model: 'claude-opus-4-1', resolved: true, client_request_id: 'req_9004', request_id: '', account_id: null, account_name: '', message: '用户 林小雨 触发日配额上限' },
  { id: 9005, created_at: minutesAgo(203), phase: 'upstream', type: 'rate_limited', error_owner: 'provider', error_source: 'upstream_http', severity: 'P3', status_code: 429, platform: 'antigravity', model: 'antigravity-pro', resolved: true, client_request_id: 'req_9005', request_id: 'up_9005', account_id: 107, account_name: 'antigravity-lab', message: '限流窗口 5h 用尽，已切换备用账号' },
  { id: 9006, created_at: minutesAgo(318), phase: 'client', type: 'invalid_request', error_owner: 'client', error_source: 'client_request', severity: 'P4', status_code: 400, platform: 'openai', model: 'gpt-5', resolved: true, client_request_id: 'req_9006', request_id: '', account_id: null, account_name: '', message: '请求体缺少 messages 字段' }
]

const RANKED_USERS = [
  { user_id: 7, email: 'dev-team@quantleap.io', username: '量跃科技' },
  { user_id: 42, email: 'xiaoyu.lin@example.com', username: '林小雨' },
  { user_id: 18, email: 'ml@northwind.ai', username: 'Northwind ML' },
  { user_id: 55, email: 'zhangwei@outlook.com', username: '张伟' },
  { user_id: 61, email: 'agents@orbitlabs.dev', username: 'Orbit Labs' },
  { user_id: 93, email: 'kenji.sato@example.jp', username: 'Kenji' },
  { user_id: 120, email: 'research@lumen.edu', username: 'Lumen Research' },
  { user_id: 134, email: 'wang.fang@163.com', username: '王芳' }
]

function usersTrend(days, limit = 5) {
  const rnd = seeded(99)
  const out = []
  for (let i = days - 1; i >= 0; i--) {
    RANKED_USERS.slice(0, limit).forEach((u, idx) => {
      const requests = Math.round((14_000 - idx * 2_100) * (0.7 + rnd() * 0.6))
      const tokens = requests * Math.round(11_000 + rnd() * 6_000)
      const cost = round2(tokens / 1_000_000 * 0.64)
      out.push({ date: dateStr(-i), user_id: u.user_id, email: u.email, username: u.username, requests, tokens, cost, actual_cost: round2(cost * 1.03) })
    })
  }
  return out
}

function usersRanking(limit = 10) {
  const ranking = RANKED_USERS.slice(0, limit).map((u, idx) => {
    const requests = 118_000 - idx * 13_500
    const tokens = requests * 12_400
    return { ...u, actual_cost: round2(tokens / 1_000_000 * 0.66), requests, tokens }
  })
  return {
    ranking,
    total_actual_cost: round2(ranking.reduce((s, r) => s + r.actual_cost, 0)),
    total_requests: ranking.reduce((s, r) => s + r.requests, 0),
    total_tokens: ranking.reduce((s, r) => s + r.tokens, 0),
    start_date: dateStr(-13), end_date: dateStr(0)
  }
}

function apiKeysTrend(days) {
  const rnd = seeded(5)
  const out = []
  for (let i = days - 1; i >= 0; i--) {
    API_KEYS.slice(0, 3).forEach((k, idx) => {
      const requests = Math.round((3_200 - idx * 700) * (0.7 + rnd() * 0.6))
      out.push({ date: dateStr(-i), api_key_id: k.id, key_name: k.name, requests, tokens: requests * 12_800 })
    })
  }
  return out
}

const ADMIN_DASHBOARD_STATS = {
  total_users: 1284, today_new_users: 24, active_users: 312, hourly_active_users: 87,
  stats_updated_at: minutesAgo(1), stats_stale: false,
  total_api_keys: 3902, active_api_keys: 1140,
  total_accounts: 86, normal_accounts: 79, error_accounts: 1, ratelimit_accounts: 3, overload_accounts: 0,
  total_requests: 18_642_310, total_input_tokens: 68_400_000_000, total_output_tokens: 9_120_000_000,
  total_cache_creation_tokens: 14_600_000_000, total_cache_read_tokens: 121_000_000_000, total_tokens: 213_120_000_000,
  total_cost: 168_420.55, total_actual_cost: 172_910.3, total_account_cost: 141_260.8,
  today_requests: 128_430, today_input_tokens: 540_000_000, today_output_tokens: 72_600_000,
  today_cache_creation_tokens: 118_000_000, today_cache_read_tokens: 1_090_000_000, today_tokens: 1_820_600_000,
  today_cost: 1_284.2, today_actual_cost: 1_312.7, today_account_cost: 1_078.4,
  total_balance_actual_cost: 121_040.1, today_balance_actual_cost: 902.3,
  total_subscription_actual_cost: 51_870.2, today_subscription_actual_cost: 410.4,
  total_recharge_amount: 186_300, today_recharge_amount: 1_640, total_refund_amount: 1_120, today_refund_amount: 0,
  average_duration_ms: 1_800, average_first_token_ms: 620, uptime: 37 * 86_400 + 5 * 3_600 + 912,
  rpm: 842, tpm: 12_400_000
}

const USER_DASHBOARD_STATS = {
  total_api_keys: 5, active_api_keys: 3,
  total_requests: 48_210, total_input_tokens: 182_000_000, total_output_tokens: 21_400_000,
  total_cache_creation_tokens: 39_000_000, total_cache_read_tokens: 310_000_000, total_tokens: 552_400_000,
  total_cost: 412.36, total_actual_cost: 428.9,
  today_requests: 1_284, today_input_tokens: 5_200_000, today_output_tokens: 610_000,
  today_cache_creation_tokens: 1_100_000, today_cache_read_tokens: 9_400_000, today_tokens: 16_310_000,
  today_cost: 12.84, today_actual_cost: 13.21, average_duration_ms: 1_620, rpm: 9, tpm: 118_000,
  by_platform: [
    { platform: 'anthropic', total_requests: 36_900, total_tokens: 448_000_000, total_actual_cost: 361.2, today_requests: 1_010, today_tokens: 13_500_000, today_actual_cost: 11.02 },
    { platform: 'openai', total_requests: 9_100, total_tokens: 88_000_000, total_actual_cost: 54.1, today_requests: 214, today_tokens: 2_300_000, today_actual_cost: 1.84 },
    { platform: 'gemini', total_requests: 2_210, total_tokens: 16_400_000, total_actual_cost: 13.6, today_requests: 60, today_tokens: 510_000, today_actual_cost: 0.35 }
  ]
}

function usageLogs(count = 12) {
  const rnd = seeded(2024)
  const models = ['claude-sonnet-4-5', 'claude-fable-4-1', 'gpt-5-codex', 'gemini-2.5-pro']
  const out = []
  for (let i = 0; i < count; i++) {
    const model = models[i % models.length]
    const key = API_KEYS[i % 3]
    const input = Math.round(2_000 + rnd() * 30_000)
    const output = Math.round(200 + rnd() * 4_000)
    const cacheR = Math.round(input * 1.6)
    const cost = round2((input * 3 + output * 15 + cacheR * 0.3) / 1_000_000)
    out.push({
      id: 900_000 - i, user_id: 42, api_key_id: key.id, account_id: 101 + (i % 3), request_id: `req_${(0xabc000 + i * 7919).toString(16)}`,
      model, service_tier: null, reasoning_effort: null, inbound_endpoint: '/v1/messages', upstream_endpoint: '/v1/messages',
      group_id: key.group_id, subscription_id: null,
      input_tokens: input, output_tokens: output, cache_creation_tokens: Math.round(input * 0.2), cache_read_tokens: cacheR,
      cache_creation_5m_tokens: Math.round(input * 0.2), cache_creation_1h_tokens: 0,
      input_cost: round2(input * 3 / 1e6), output_cost: round2(output * 15 / 1e6), cache_creation_cost: round2(input * 0.2 * 3.75 / 1e6), cache_read_cost: round2(cacheR * 0.3 / 1e6),
      total_cost: cost, actual_cost: round2(cost * 1.2), rate_multiplier: 1.2, long_context_billing_applied: false, billing_type: 0,
      request_type: 'stream', stream: true, native_compaction_v2: false, duration_ms: Math.round(900 + rnd() * 6_000), first_token_ms: Math.round(300 + rnd() * 900),
      image_count: 0, image_size: null, image_input_size: null, image_output_size: null, image_size_source: null, image_size_breakdown: null,
      image_input_tokens: 0, image_input_cost: 0, image_output_tokens: 0, image_output_cost: 0,
      user_agent: i % 2 ? 'claude-cli/2.1.4 (external, cli)' : 'Cursor/1.6.2', ip_address: '203.0.113.42', cache_ttl_overridden: false, billing_mode: 'balance',
      created_at: minutesAgo(i * 7 + 1), api_key: { id: key.id, name: key.name }, group: key.group
    })
  }
  return out
}

// ==================== Settings ====================
const PUBLIC_SETTINGS = {
  registration_enabled: true,
  email_verify_enabled: false,
  force_email_on_third_party_signup: false,
  registration_email_suffix_whitelist: [],
  registration_email_domain_quota_enabled: false,
  promo_code_enabled: true,
  password_reset_enabled: true,
  invitation_code_enabled: false,
  login_agreement_enabled: false, // prototype state: footer links only, no consent gate (flip to true to preview the modal)
  login_agreement_mode: 'inline',
  login_agreement_updated_at: '2026-08-01T00:00:00Z',
  login_agreement_revision: 'r3',
  login_agreement_documents: [
    { id: 'terms', title: '服务条款', content_md: '# 服务条款\n\n欢迎使用 Sub2API。使用本服务即表示您同意以下条款。\n\n## 1. 服务内容\n\nSub2API 提供统一的 AI 模型接入网关，您可通过一个 API 密钥调用已接入的模型。\n\n## 2. 账号与安全\n\n- 妥善保管您的 API 密钥，勿在公开仓库中泄露。\n- 对通过您账号发生的一切请求负责。\n\n## 3. 计费\n\n按实际用量计费，详见控制台用量页面。\n\n```bash\nexport ANTHROPIC_BASE_URL="https://api.sub2api.dev"\n```' },
    { id: 'privacy', title: '隐私政策', content_md: '# 隐私政策\n\n我们仅收集提供服务所必需的信息。\n\n## 收集的信息\n\n1. 注册邮箱\n2. 请求元数据（模型、token 数、时间）\n\n## 信息用途\n\n用于计费、限流与故障排查，不会出售给第三方。' }
  ],
  turnstile_enabled: false,
  turnstile_site_key: '',
  tencent_captcha_enabled: false,
  tencent_captcha_app_id: '',
  tencent_captcha_region: '',
  aliyun_captcha_enabled: false,
  aliyun_captcha_scene_id: '',
  aliyun_captcha_prefix: '',
  aliyun_captcha_region: '',
  passkey_enabled: true,
  site_name: 'Sub2API',
  site_logo: '',
  site_subtitle: '', // empty = default heroDescription copy, as in the prototype
  api_base_url: 'https://api.sub2api.dev',
  contact_info: 'support@sub2api.dev',
  doc_url: 'https://docs.example.com',
  home_content: '',
  compact_home_enabled: false,
  hide_ccs_import_button: false,
  payment_enabled: true,
  risk_control_enabled: false,
  table_default_page_size: 20,
  table_page_size_options: [10, 20, 50, 100],
  custom_menu_items: [],
  custom_endpoints: [
    { name: 'OpenAI Compatible', endpoint: 'https://api.sub2api.dev/v1', description: '支持 OpenAI 格式请求' }
  ],
  linuxdo_oauth_enabled: true,
  dingtalk_oauth_enabled: true,
  wechat_oauth_enabled: false,
  wechat_oauth_open_enabled: false,
  wechat_oauth_mp_enabled: false,
  wechat_oauth_mobile_enabled: false,
  oidc_oauth_enabled: false,
  oidc_oauth_provider_name: '',
  github_oauth_enabled: false,
  google_oauth_enabled: false,
  backend_mode_enabled: false,
  version: '1.8.2',
  server_timezone: 'Asia/Shanghai',
  server_utc_offset: '+08:00',
  balance_low_notify_enabled: true,
  account_quota_notify_enabled: true,
  balance_low_notify_threshold: 10,
  channel_monitor_enabled: true,
  channel_monitor_mode: 'v2',
  channel_monitor_default_interval_seconds: 300,
  channel_monitor_hide_throughput: false,
  channel_monitor_show_quota: true,
  available_channels_enabled: true,
  model_plaza_enabled: true,
  model_plaza_require_auth: false,
  plugin_management_enabled: true,
  service_quota_enabled: true,
  affiliate_enabled: true,
  allow_user_view_error_requests: true,
  ticket_enabled: true,
  creation_center_enabled: true,
  support_qr_codes: [],
  download_tools_url: ''
}

const noQuota = { daily_limit_usd: null, weekly_limit_usd: null, monthly_limit_usd: null }
const ADMIN_SETTINGS = {
  registration_enabled: true, email_verify_enabled: false, registration_email_suffix_whitelist: [], registration_email_domain_quota_enabled: false,
  promo_code_enabled: true, password_reset_enabled: true, frontend_url: 'https://console.sub2api.dev', invitation_code_enabled: false,
  totp_enabled: true, totp_encryption_key_configured: true, passkey_enabled: true, passkey_configured: true, passkey_rp_id: 'console.sub2api.dev', passkey_rp_origins: ['https://console.sub2api.dev'],
  session_binding_enabled: false, step_up_enabled: true, audit_log_retention_days: 90,
  login_agreement_enabled: false, login_agreement_mode: 'modal', login_agreement_updated_at: '', login_agreement_documents: [],
  default_balance: 5, affiliate_rebate_rate: 0.1, affiliate_rebate_freeze_hours: 72, affiliate_rebate_duration_days: 365, affiliate_rebate_per_invitee_cap: 50, affiliate_rebate_cap: 500, affiliate_rebate_invitee_limit: 100, affiliate_signup_bonus: 1, affiliate_admin_recharge_enabled: false,
  default_concurrency: 5, default_user_rpm_limit: 0, default_subscriptions: [],
  auth_source_default_email_balance: 5, auth_source_default_email_concurrency: 5, auth_source_default_email_subscriptions: [], auth_source_default_email_grant_on_signup: true, auth_source_default_email_grant_on_first_bind: false,
  auth_source_default_linuxdo_balance: 5, auth_source_default_linuxdo_concurrency: 5, auth_source_default_linuxdo_subscriptions: [], auth_source_default_linuxdo_grant_on_signup: true, auth_source_default_linuxdo_grant_on_first_bind: false,
  auth_source_default_oidc_balance: 5, auth_source_default_oidc_concurrency: 5, auth_source_default_oidc_subscriptions: [], auth_source_default_oidc_grant_on_signup: true, auth_source_default_oidc_grant_on_first_bind: false,
  auth_source_default_wechat_balance: 5, auth_source_default_wechat_concurrency: 5, auth_source_default_wechat_subscriptions: [], auth_source_default_wechat_grant_on_signup: true, auth_source_default_wechat_grant_on_first_bind: false,
  auth_source_default_dingtalk_balance: 5, auth_source_default_dingtalk_concurrency: 5, auth_source_default_dingtalk_subscriptions: [], auth_source_default_dingtalk_grant_on_signup: true, auth_source_default_dingtalk_grant_on_first_bind: false,
  auth_source_default_github_balance: 5, auth_source_default_github_concurrency: 5, auth_source_default_github_subscriptions: [], auth_source_default_github_grant_on_signup: true, auth_source_default_github_grant_on_first_bind: false,
  auth_source_default_google_balance: 5, auth_source_default_google_concurrency: 5, auth_source_default_google_subscriptions: [], auth_source_default_google_grant_on_signup: true, auth_source_default_google_grant_on_first_bind: false,
  force_email_on_third_party_signup: false,
  default_platform_quotas: { anthropic: { ...noQuota }, openai: { ...noQuota }, gemini: { ...noQuota } },
  auth_source_default_email_platform_quotas: {}, auth_source_default_linuxdo_platform_quotas: {}, auth_source_default_oidc_platform_quotas: {}, auth_source_default_wechat_platform_quotas: {}, auth_source_default_github_platform_quotas: {}, auth_source_default_google_platform_quotas: {}, auth_source_default_dingtalk_platform_quotas: {},
  site_name: PUBLIC_SETTINGS.site_name, site_logo: '', site_subtitle: PUBLIC_SETTINGS.site_subtitle, api_base_url: PUBLIC_SETTINGS.api_base_url, contact_info: PUBLIC_SETTINGS.contact_info,
  support_qr_codes: [], doc_url: PUBLIC_SETTINGS.doc_url, download_tools_url: '', home_content: '', compact_home_enabled: false, hide_ccs_import_button: false,
  table_default_page_size: 20, table_page_size_options: [10, 20, 50, 100], backend_mode_enabled: false, custom_menu_items: [], custom_endpoints: PUBLIC_SETTINGS.custom_endpoints,
  smtp_host: 'smtp.sendgrid.net', smtp_port: 587, smtp_username: 'apikey', smtp_password_configured: true, smtp_from_email: 'noreply@sub2api.dev', smtp_from_name: 'Sub2API', smtp_use_tls: true,
  turnstile_enabled: false, turnstile_site_key: '', turnstile_secret_key_configured: false,
  tencent_captcha_enabled: false, tencent_captcha_app_id: '', tencent_captcha_app_secret_key_configured: false, tencent_captcha_cloud_secret_id_configured: false, tencent_captcha_cloud_secret_key_configured: false, tencent_captcha_region: '',
  aliyun_captcha_enabled: false, aliyun_captcha_access_key_id: '', aliyun_captcha_access_key_secret_configured: false, aliyun_captcha_scene_id: '', aliyun_captcha_prefix: '', aliyun_captcha_region: '',
  api_key_acl_trust_forwarded_ip: false, forwarded_client_ip_headers: ['X-Forwarded-For', 'X-Real-IP'],
  linuxdo_connect_enabled: true, linuxdo_connect_client_id: 'ldc_9f2a8b', linuxdo_connect_client_secret_configured: true, linuxdo_connect_redirect_url: 'https://console.sub2api.dev/api/v1/auth/oauth/linuxdo/callback',
  dingtalk_connect_enabled: true, dingtalk_connect_client_id: 'dingzx7a1b', dingtalk_connect_client_secret_configured: true, dingtalk_connect_redirect_url: 'https://console.sub2api.dev/api/v1/auth/oauth/dingtalk/callback',
  dingtalk_connect_corp_restriction_policy: 'none', dingtalk_connect_internal_corp_id: '', dingtalk_connect_bypass_registration: false,
  dingtalk_connect_sync_corp_email: false, dingtalk_connect_sync_display_name: true, dingtalk_connect_sync_dept: false,
  dingtalk_connect_sync_corp_email_attr_key: '', dingtalk_connect_sync_display_name_attr_key: '', dingtalk_connect_sync_dept_attr_key: '',
  dingtalk_connect_sync_corp_email_attr_name: '', dingtalk_connect_sync_display_name_attr_name: '', dingtalk_connect_sync_dept_attr_name: '',
  wechat_connect_enabled: false, wechat_connect_app_id: '', wechat_connect_app_secret_configured: false, wechat_connect_open_app_id: '', wechat_connect_open_app_secret_configured: false,
  wechat_connect_mp_app_id: '', wechat_connect_mp_app_secret_configured: false, wechat_connect_mobile_app_id: '', wechat_connect_mobile_app_secret_configured: false,
  wechat_connect_open_enabled: false, wechat_connect_mp_enabled: false, wechat_connect_mobile_enabled: false, wechat_connect_mode: 'open', wechat_connect_scopes: 'snsapi_login', wechat_connect_redirect_url: '', wechat_connect_frontend_redirect_url: '',
  oidc_connect_enabled: false, oidc_connect_provider_name: '', oidc_connect_client_id: '', oidc_connect_client_secret_configured: false, oidc_connect_issuer_url: '', oidc_connect_discovery_url: '', oidc_connect_authorize_url: '', oidc_connect_token_url: '', oidc_connect_userinfo_url: '', oidc_connect_jwks_url: '',
  oidc_connect_scopes: 'openid profile email', oidc_connect_redirect_url: '', oidc_connect_frontend_redirect_url: '', oidc_connect_token_auth_method: 'client_secret_post', oidc_connect_use_pkce: true, oidc_connect_validate_id_token: true, oidc_connect_allowed_signing_algs: 'RS256', oidc_connect_clock_skew_seconds: 60, oidc_connect_require_email_verified: false,
  oidc_connect_userinfo_email_path: 'email', oidc_connect_userinfo_id_path: 'sub', oidc_connect_userinfo_username_path: 'preferred_username',
  github_oauth_enabled: true, github_oauth_client_id: 'Iv1.8a7b6c5d4e3f', github_oauth_client_secret_configured: true, github_oauth_redirect_url: 'https://console.sub2api.dev/api/v1/auth/oauth/github/callback', github_oauth_frontend_redirect_url: '',
  google_oauth_enabled: false, google_oauth_client_id: '', google_oauth_client_secret_configured: false, google_oauth_redirect_url: '', google_oauth_frontend_redirect_url: '',
  enable_model_fallback: true, fallback_model_anthropic: 'claude-sonnet-4-5', fallback_model_openai: 'gpt-5', fallback_model_gemini: 'gemini-2.5-flash', fallback_model_antigravity: 'claude-sonnet-4-5',
  grok_default_text_model: 'grok-4', grok_cross_client_model_map_enabled: false, grok_default_base_url_mode: 'default',
  account_scheduling_thresholds: { anthropic: 95, openai: 95, gemini: 100, antigravity: 100, grok: 100, kiro: 100 },
  enable_identity_patch: false, identity_patch_prompt: '',
  ops_monitoring_enabled: true, ops_realtime_monitoring_enabled: true, ops_query_mode_default: 'auto', ops_metrics_interval_seconds: 60,
  min_claude_code_version: '', max_claude_code_version: '',
  kiro_version: '0.6.12', kiro_commit: '', system_version: 'darwin', node_version: '22.11.0', kiro_code_execution_sandbox_command: '',
  cache_hit_rate_scale: null, cache_min_block_tokens: null, cache_independent_ttl_seconds: null, cache_prefix_ttl_seconds: null,
  allow_ungrouped_key_scheduling: true,
  openai_ttft_mode: 'auto', enable_fingerprint_unification: true, enable_metadata_passthrough: false, enable_cch_signing: true,
  enable_claude_oauth_system_prompt_injection: false, claude_oauth_system_prompt: '', claude_oauth_system_prompt_blocks: '',
  enable_anthropic_cache_ttl_1h_injection: false, rewrite_message_cache_control: false, enable_client_dateline_normalization: true,
  antigravity_user_agent_version: '1.12.3', openai_codex_user_agent: 'codex_cli_rs/0.42.0', openai_codex_client_version: '0.42.0', openai_codex_client_version_synced: '0.42.0', openai_codex_version_auto_sync_enabled: true,
  min_codex_version: '', max_codex_version: '', codex_cli_only_blacklist: '', codex_cli_only_whitelist: '', codex_cli_only_allow_app_server_clients: true, codex_cli_only_engine_fingerprint_signals: '',
  web_search_emulation_enabled: false,
  payment_enabled: true, risk_control_enabled: false, cyber_session_block_enabled: false, cyber_session_block_ttl_seconds: 900,
  payment_min_amount: 10, payment_max_amount: 5000, payment_daily_limit: 20000, payment_order_timeout_minutes: 15, payment_max_pending_orders: 3, payment_enabled_types: ['alipay', 'wxpay', 'stripe'],
  payment_balance_disabled: false, payment_balance_recharge_multiplier: 1, payment_subscription_usd_to_cny_rate: 7.2, payment_recharge_fee_rate: 0, payment_load_balance_strategy: 'round_robin',
  payment_product_name_prefix: 'Sub2API', payment_product_name_suffix: '充值', payment_help_image_url: '', payment_help_text: '',
  payment_cancel_rate_limit_enabled: true, payment_cancel_rate_limit_max: 5, payment_cancel_rate_limit_window: 1, payment_cancel_rate_limit_unit: 'hour', payment_cancel_rate_limit_window_mode: 'sliding',
  payment_alipay_force_qrcode: false, payment_alipay_mobile_precreate_deep_link: false, payment_visible_method_alipay_source: 'auto', payment_visible_method_wxpay_source: 'auto', payment_visible_method_alipay_enabled: true, payment_visible_method_wxpay_enabled: true,
  openai_low_upstream_rate_priority_enabled: false, openai_oauth_scheduling_rate_multiplier: 1, openai_advanced_scheduler_enabled: false, openai_advanced_scheduler_sticky_weighted_enabled: false, openai_advanced_scheduler_subscription_priority_enabled: false,
  openai_advanced_scheduler_lb_top_k: '3', openai_advanced_scheduler_weight_priority: '1', openai_advanced_scheduler_weight_load: '1', openai_advanced_scheduler_weight_queue: '1', openai_advanced_scheduler_weight_error_rate: '1', openai_advanced_scheduler_weight_ttft: '1', openai_advanced_scheduler_weight_reset: '1', openai_advanced_scheduler_weight_quota_headroom: '1', openai_advanced_scheduler_weight_upstream_cost: '1', openai_advanced_scheduler_weight_previous_response: '1', openai_advanced_scheduler_weight_session_sticky: '1',
  balance_low_notify_enabled: true, balance_low_notify_threshold: 10, balance_low_notify_recharge_url: 'https://console.sub2api.dev/payment', subscription_expiry_notify_enabled: true,
  account_quota_notify_enabled: true, account_quota_notify_emails: [{ email: '', disabled: false, verified: true }],
  channel_monitor_enabled: true, channel_monitor_mode: 'v2', channel_monitor_default_interval_seconds: 300, channel_monitor_hide_throughput: false, channel_monitor_show_quota: true,
  available_channels_enabled: true, model_plaza_enabled: true, model_plaza_require_auth: false, model_plaza_description: '', plugin_management_enabled: true,
  affiliate_enabled: true, ticket_enabled: true, creation_center_enabled: true,
  ip_multi_account_ban_enabled: false, ip_multi_account_ban_window_minutes: 60, ip_multi_account_ban_threshold: 5, ip_multi_account_ban_window2_minutes: 1440, ip_multi_account_ban_threshold2: 10, ip_multi_account_ban_learning_until: '', registration_block_datacenter_ip: false,
  openai_fast_policy_settings: { rules: [] },
  allow_user_view_error_requests: true
}

const ADMIN_PAYMENT_CONFIG = {
  enabled: true, min_amount: 10, max_amount: 5000, daily_limit: 20000, order_timeout_minutes: 15, max_pending_orders: 3,
  enabled_payment_types: ['alipay', 'wxpay', 'stripe'], balance_disabled: false, balance_recharge_multiplier: 1, subscription_usd_to_cny_rate: 7.2,
  recharge_fee_rate: 0, load_balance_strategy: 'round_robin', product_name_prefix: 'Sub2API', product_name_suffix: '充值', help_image_url: '', help_text: ''
}
const USER_PAYMENT_CONFIG = {
  payment_enabled: true, min_amount: 10, max_amount: 5000, daily_limit: 20000, max_pending_orders: 3, order_timeout_minutes: 15,
  balance_disabled: false, balance_recharge_multiplier: 1, subscription_usd_to_cny_rate: 7.2, enabled_payment_types: ['alipay', 'wxpay', 'stripe'],
  help_image_url: '', help_text: '', stripe_publishable_key: ''
}

// ==================== Admin users (bonus for UsersView) ====================
const ADMIN_USERS = [
  { ...ADMIN_USER, notes: '', last_used_at: minutesAgo(2), current_concurrency: 0, current_rpm: 0, today_actual_cost: 0.42, total_actual_cost: 118.6 },
  { ...NORMAL_USER, notes: 'VIP 客户', last_used_at: minutesAgo(11), current_concurrency: 1, current_rpm: 3, today_actual_cost: 13.21, total_actual_cost: 428.9, allowed_groups: [1, 2, 3] },
  ...RANKED_USERS.filter((u) => u.user_id !== 42).map((u, i) => ({
    ...baseUserFields, id: u.user_id, username: u.username, email: u.email, role: 'user', balance: round2(12 + i * 37.3), concurrency: 5 + (i % 3) * 5,
    status: i === 5 ? 'disabled' : 'active', last_active_at: hoursAgo(i + 1), created_at: daysAgo(30 + i * 17), updated_at: hoursAgo(i + 1),
    notes: '', last_used_at: hoursAgo(i + 1), current_concurrency: 0, current_rpm: 0, today_actual_cost: round2(30 - i * 3.4), total_actual_cost: round2(2_400 - i * 260)
  }))
]

// ==================== Router ====================
const tokens = (user) => ({
  access_token: user.role === 'admin' ? 'mock-token' : 'mock-user-token',
  refresh_token: user.role === 'admin' ? 'mock-refresh-token' : 'mock-user-refresh-token',
  expires_in: 86_400,
  token_type: 'Bearer'
})

function paginate(items, query, defaultSize = 20) {
  const page = Math.max(1, Number(query.get('page') || 1))
  const page_size = Math.max(1, Number(query.get('page_size') || defaultSize))
  const start = (page - 1) * page_size
  return { items: items.slice(start, start + page_size), total: items.length, page, page_size, pages: Math.max(1, Math.ceil(items.length / page_size)) }
}
const pluginManifest = (id, name, version, platform) => ({ schema_version: 1, id, name, version, description: name + ' 适配插件', author: 'Sub2API', requires: { sub2api_version: '>=1.0.0', plugin_protocol: 1, transport_api: 1, ui_bridge: 1 }, capabilities: [{ id: platform + '.chat', platform, account_type: 'oauth' }], ui: { entrypoint: 'index.js' } })
const MOCK_PLUGINS = [
  { id: 1, plugin_key: 'kimi-adapter', name: 'Kimi 适配器', version: '1.2.0', description: 'Moonshot Kimi OAuth 账号接入与配额同步', author: 'Sub2API', manifest: pluginManifest('kimi-adapter', 'Kimi 适配器', '1.2.0', 'kimi'), signature_status: 'trusted', state: 'enabled', last_error: '', installed_at: iso(NOW() - 20 * 864e5), enabled_at: iso(NOW() - 19 * 864e5), updated_at: iso(NOW() - 2 * 864e5), bindings: [{ id: 11, plugin_id: 1, capability: 'kimi.chat', platform: 'kimi', account_type: 'oauth', enabled: true, rollout_percent: 100 }], compatibility: { compatible: true, tested: true, status: 'compatible', message: '', current_sub2api_version: '1.8.2', required_sub2api_version: '>=1.0.0', recommended_sub2api_version: '1.8.0', plugin_protocol: 1, transport_api: 1, ui_bridge: 1 } },
  { id: 2, plugin_key: 'zhipu-adapter', name: '智谱 GLM 适配器', version: '0.9.1', description: '智谱 GLM API Key 账号接入（灰度）', author: 'community', manifest: pluginManifest('zhipu-adapter', '智谱 GLM 适配器', '0.9.1', 'zhipu'), signature_status: 'unsigned', state: 'disabled', last_error: '', installed_at: iso(NOW() - 5 * 864e5), updated_at: iso(NOW() - 5 * 864e5), bindings: [{ id: 21, plugin_id: 2, capability: 'zhipu.chat', platform: 'zhipu', account_type: 'apikey', enabled: false, rollout_percent: 25 }], compatibility: { compatible: true, tested: false, status: 'untested', message: '未在当前版本测试', current_sub2api_version: '1.8.2', required_sub2api_version: '>=1.6.0', recommended_sub2api_version: '1.7.0', plugin_protocol: 1, transport_api: 1, ui_bridge: 1 } },
]

const emptyPage = (query) => ({ items: [], total: 0, page: Number(query.get('page') || 1), page_size: Number(query.get('page_size') || 20), pages: 0 })

const rangeDays = (query) => {
  const s = query.get('start_date'), e = query.get('end_date')
  if (s && e) {
    const d = Math.round((new Date(e) - new Date(s)) / 86_400_000) + 1
    if (Number.isFinite(d) && d > 0 && d <= 366) return d
  }
  return 14
}

// UI preview helper: /setup/status normally reports "already installed" so app views load
// straight into the dashboard. Setting MOCK_NEEDS_SETUP=1 (or hitting GET /setup/dev-toggle,
// see below) flips it to the fresh-install branch so SetupWizardView.vue's wizard renders.
let SETUP_NEEDS_SETUP = process.env.MOCK_NEEDS_SETUP === '1'

// Exact-path routes: key = "METHOD path"
const routes = {
  // ---- public / auth ----
  'GET /api/v1/settings/public': () => PUBLIC_SETTINGS,
  'GET /setup/status': () => ({ needs_setup: SETUP_NEEDS_SETUP, step: SETUP_NEEDS_SETUP ? 'database' : 'done' }),
  'GET /api/v1/setup/status': () => ({ needs_setup: SETUP_NEEDS_SETUP, step: SETUP_NEEDS_SETUP ? 'database' : 'done' }),
  'POST /setup/test-db': () => ({ success: true, message: 'Database connection successful' }),
  'POST /setup/test-redis': () => ({ success: true, message: 'Redis connection successful' }),
  'POST /setup/install': () => {
    SETUP_NEEDS_SETUP = false
    return { message: 'Installation successful', restart: false }
  },
  'POST /api/v1/auth/login': (ctx) => {
    const user = ctx.body && /user|xiaoyu/i.test(String(ctx.body.email || '')) ? NORMAL_USER : ADMIN_USER
    return { ...tokens(user), user: { ...user, run_mode: 'standard' } }
  },
  'GET /api/v1/auth/me': (ctx) => ({ ...currentUser(ctx.req), run_mode: 'standard' }),
  'POST /api/v1/auth/refresh': (ctx) => {
    const isUser = String((ctx.body && ctx.body.refresh_token) || '').includes('user')
    return tokens(isUser ? NORMAL_USER : ADMIN_USER)
  },
  'POST /api/v1/auth/logout': () => ({}),
  'POST /api/v1/auth/revoke-all-sessions': () => ({ message: 'ok' }),

  // ---- user ----
  'GET /api/v1/user/profile': (ctx) => currentUser(ctx.req),
  'GET /api/v1/user/rpm-status': () => ({ user_rpm_used: 3, user_rpm_limit: 0, current_concurrency: 1 }),
  'GET /api/v1/user/platform-quotas': () => ({
    platform_quotas: [
      { platform: 'anthropic', daily_limit_usd: 50, weekly_limit_usd: null, monthly_limit_usd: 600, daily_usage_usd: 11.02, weekly_usage_usd: 61.8, monthly_usage_usd: 212.4, daily_window_resets_at: inHours(9) },
      { platform: 'openai', daily_limit_usd: null, weekly_limit_usd: null, monthly_limit_usd: null, daily_usage_usd: 1.84, weekly_usage_usd: 9.7, monthly_usage_usd: 38.2 }
    ]
  }),
  'GET /api/v1/user/totp/status': () => ({ enabled: false, encryption_key_configured: true }),
  'GET /api/v1/user/passkeys': () => [],
  'GET /api/v1/user/aff': () => ({
    aff_code: 'S2A-XY42', invite_url: 'https://console.sub2api.dev/register?aff=S2A-XY42', rebate_rate: 0.1, invitee_count: 6,
    total_rebate: 18.4, available_rebate: 6.2, frozen_rebate: 2.1, transferred_rebate: 10.1, invitees: []
  }),

  // ---- keys / groups / subscriptions ----
  'GET /api/v1/keys': (ctx) => paginate(API_KEYS, ctx.query, 10),
  'POST /api/v1/keys': (ctx) => makeKey({ id: 599, name: (ctx.body && ctx.body.name) || 'new-key', key: 'sk-s2a-new0000000000000000000000000000000' }),
  'GET /api/v1/groups/available': () => GROUPS.filter((g) => !g.is_exclusive || g.id === 2).map(publicGroup),
  'GET /api/v1/groups/rates': () => ({ 2: 1.1 }),
  'GET /api/v1/subscriptions': () => [],
  'GET /api/v1/subscriptions/active': () => [],
  'GET /api/v1/subscriptions/progress': () => [],
  'GET /api/v1/subscriptions/summary': () => ({ active_count: 0, subscriptions: [] }),

  // ---- usage (user) ----
  'GET /api/v1/usage': (ctx) => paginate(usageLogs(12), ctx.query),
  'GET /api/v1/usage/stats': () => ({
    period: 'all', total_requests: USER_DASHBOARD_STATS.total_requests, total_input_tokens: USER_DASHBOARD_STATS.total_input_tokens, total_output_tokens: USER_DASHBOARD_STATS.total_output_tokens,
    total_cache_tokens: 349_000_000, total_cache_read_tokens: 310_000_000, total_cache_creation_tokens: 39_000_000, total_tokens: USER_DASHBOARD_STATS.total_tokens,
    total_cost: USER_DASHBOARD_STATS.total_cost, total_actual_cost: USER_DASHBOARD_STATS.total_actual_cost, average_duration_ms: 1_620,
    models: { 'claude-sonnet-4-5': 36_900, 'gpt-5-codex': 9_100, 'gemini-2.5-pro': 2_210 }, endpoints: ENDPOINT_STATS
  }),
  'GET /api/v1/usage/dashboard/stats': () => USER_DASHBOARD_STATS,
  'GET /api/v1/usage/dashboard/trend': (ctx) => { const d = rangeDays(ctx.query); return { trend: trendFor(ctx.query, 0.011), start_date: dateStr(-(d - 1)), end_date: dateStr(0), granularity: ctx.query.get('granularity') || 'day' } },
  'GET /api/v1/usage/dashboard/models': (ctx) => { const d = rangeDays(ctx.query); return { models: MODEL_STATS.map((m) => ({ ...m, requests: Math.round(m.requests * 0.011), input_tokens: Math.round(m.input_tokens * 0.011), output_tokens: Math.round(m.output_tokens * 0.011), cache_creation_tokens: Math.round(m.cache_creation_tokens * 0.011), cache_read_tokens: Math.round(m.cache_read_tokens * 0.011), total_tokens: Math.round(m.total_tokens * 0.011), cost: round2(m.cost * 0.011), actual_cost: round2(m.actual_cost * 0.011), account_cost: undefined })), start_date: dateStr(-(d - 1)), end_date: dateStr(0) } },
  'GET /api/v1/usage/dashboard/snapshot-v2': (ctx) => { const d = rangeDays(ctx.query); return { generated_at: iso(NOW()), start_date: dateStr(-(d - 1)), end_date: dateStr(0), granularity: 'day', trend: trendSeries(d, 0.011), models: MODEL_STATS.slice(0, 4), groups: GROUP_STATS } },
  'POST /api/v1/usage/dashboard/api-keys-usage': (ctx) => {
    const ids = (ctx.body && ctx.body.api_key_ids) || API_KEYS.map((k) => k.id)
    const stats = {}
    ids.forEach((id, i) => { stats[String(id)] = { api_key_id: id, today_actual_cost: round2([8.42, 3.1, 1.69, 0, 0][i % 5]), total_actual_cost: round2([212.35, 37.8, 88.12, 49.96, 2.4][i % 5]) } })
    return { stats }
  },
  'GET /api/v1/usage/errors': (ctx) => emptyPage(ctx.query),

  // ---- announcements / tickets / payment / misc user-side ----
  'GET /api/v1/announcements': () => [],
  'GET /api/v1/tickets': (ctx) => emptyPage(ctx.query),
  'GET /api/v1/tickets/unread-count': () => ({ count: 3 }),
  'GET /api/v1/tickets/rate-groups': () => [],
  'GET /api/v1/payment/config': () => USER_PAYMENT_CONFIG,
  'GET /api/v1/payment/plans': () => [],
  'GET /api/v1/payment/orders/my': (ctx) => emptyPage(ctx.query),
  'GET /api/v1/payment/invoices': (ctx) => emptyPage(ctx.query),
  'GET /api/v1/redeem/history': () => [],
  'GET /api/v1/channels/available': () => [],
  'GET /api/v1/channel-monitors': () => [],
  'GET /api/v1/model-plaza': () => ({
    description: '',
    groups: [
      {
        id: 1, name: '默认分组', description: '', platform: 'anthropic', subscription_type: 'standard', rate_multiplier: 1,
        peak_rate_enabled: false, peak_start: '', peak_end: '', peak_rate_multiplier: 1, is_exclusive: false,
        image_rate_independent: false, image_rate_multiplier: 1, long_context_pricing_enabled: false,
        models: [
          { name: 'claude-sonnet-4-5', platform: 'anthropic', pricing: { input_price: 0.000003, output_price: 0.000015, cache_write_price: 0.00000375, cache_read_price: 0.0000003 }, official_pricing: null },
          { name: 'claude-opus-4-1', platform: 'anthropic', pricing: { input_price: 0.000015, output_price: 0.000075, cache_write_price: 0.00001875, cache_read_price: 0.0000015 }, official_pricing: null },
          { name: 'gpt-5', platform: 'openai', pricing: { input_price: 0.00000125, output_price: 0.00001, cache_write_price: null, cache_read_price: 0.000000125 }, official_pricing: null },
          { name: 'gpt-5-codex', platform: 'openai', pricing: { input_price: 0.00000125, output_price: 0.00001, cache_write_price: null, cache_read_price: 0.000000125 }, official_pricing: null },
          { name: 'gemini-2.5-pro', platform: 'gemini', pricing: { input_price: 0.00000125, output_price: 0.00001, cache_write_price: null, cache_read_price: 0.0000003 }, official_pricing: null },
          { name: 'grok-4', platform: 'grok', pricing: { input_price: 0.000003, output_price: 0.000015, cache_write_price: null, cache_read_price: 0.00000075 }, official_pricing: null, time_pricing: { timezone: 'Asia/Shanghai', periods: [{ start_time: '09:00', end_time: '18:00', multiplier: 1.5 }] } }
        ]
      }
    ]
  }),

  // ---- admin: dashboard ----
  'GET /api/v1/admin/dashboard/stats': () => ({ ...ADMIN_DASHBOARD_STATS, stats_updated_at: minutesAgo(1) }),
  'GET /api/v1/admin/dashboard/realtime': () => ({ active_requests: 37, requests_per_minute: 842, average_response_time: 1_800, error_rate: 0.6 }),
  'GET /api/v1/admin/dashboard/trend': (ctx) => { const d = rangeDays(ctx.query); return { trend: trendFor(ctx.query), start_date: dateStr(-(d - 1)), end_date: dateStr(0), granularity: ctx.query.get('granularity') || 'day' } },
  'GET /api/v1/admin/dashboard/models': (ctx) => { const d = rangeDays(ctx.query); return { models: MODEL_STATS, start_date: dateStr(-(d - 1)), end_date: dateStr(0) } },
  'GET /api/v1/admin/dashboard/groups': (ctx) => { const d = rangeDays(ctx.query); return { groups: GROUP_STATS, start_date: dateStr(-(d - 1)), end_date: dateStr(0) } },
  'GET /api/v1/admin/dashboard/user-breakdown': (ctx) => {
    const d = rangeDays(ctx.query)
    const users = RANKED_USERS.map((u, i) => {
      const requests = 118_000 - i * 13_500
      const input = requests * 4_300, output = requests * 600, cache = requests * 7_100
      const cost = round2((input + output + cache) / 1e6 * 0.66)
      return { user_id: u.user_id, email: u.email, requests, input_tokens: input, output_tokens: output, cache_tokens: cache, total_tokens: input + output + cache, cost, actual_cost: round2(cost * 1.03), account_cost: round2(cost * 0.86) }
    })
    return { users, start_date: dateStr(-(d - 1)), end_date: dateStr(0) }
  },
  'GET /api/v1/admin/dashboard/snapshot-v2': (ctx) => {
    const d = rangeDays(ctx.query)
    const inc = (k) => ctx.query.get(k) !== 'false'
    return {
      generated_at: iso(NOW()), start_date: dateStr(-(d - 1)), end_date: dateStr(0), granularity: ctx.query.get('granularity') || 'day',
      ...(inc('include_stats') ? { stats: { ...ADMIN_DASHBOARD_STATS, stats_updated_at: minutesAgo(1) } } : {}),
      ...(inc('include_trend') ? { trend: trendFor(ctx.query) } : {}),
      ...(inc('include_model_stats') ? { models: MODEL_STATS } : {}),
      ...(inc('include_group_stats') ? { groups: GROUP_STATS } : {}),
      ...(inc('include_users_trend') ? { users_trend: usersTrend(d, Number(ctx.query.get('users_trend_limit') || 5)) } : {})
    }
  },
  'GET /api/v1/admin/dashboard/api-keys-trend': (ctx) => { const d = rangeDays(ctx.query); return { trend: apiKeysTrend(d), start_date: dateStr(-(d - 1)), end_date: dateStr(0), granularity: 'day' } },
  'GET /api/v1/admin/dashboard/users-trend': (ctx) => { const d = rangeDays(ctx.query); return { trend: usersTrend(d, Number(ctx.query.get('limit') || 5)), start_date: dateStr(-(d - 1)), end_date: dateStr(0), granularity: 'day' } },
  'GET /api/v1/admin/dashboard/users-ranking': (ctx) => usersRanking(Number(ctx.query.get('limit') || 10)),
  'POST /api/v1/admin/dashboard/users-usage': (ctx) => {
    const stats = {}
    ;((ctx.body && ctx.body.user_ids) || []).forEach((id, i) => { stats[String(id)] = { user_id: id, today_actual_cost: round2(13.21 - i * 1.1 > 0 ? 13.21 - i * 1.1 : 0.3), total_actual_cost: round2(428.9 - i * 31), today_balance_actual_cost: 9.1, today_subscription_actual_cost: 4.1 } })
    return { stats }
  },
  'POST /api/v1/admin/dashboard/api-keys-usage': (ctx) => {
    const stats = {}
    ;((ctx.body && ctx.body.api_key_ids) || []).forEach((id, i) => { stats[String(id)] = { api_key_id: id, today_actual_cost: round2(8.42 - i * 1.5 > 0 ? 8.42 - i * 1.5 : 0), total_actual_cost: round2(212.35 - i * 40) } })
    return { stats }
  },

  // ---- admin: accounts ----
  'GET /api/v1/admin/accounts': (ctx) => {
    let items = ACCOUNTS
    const platform = ctx.query.get('platform'), status = ctx.query.get('status'), search = ctx.query.get('search')
    if (platform) items = items.filter((a) => a.platform === platform)
    if (status) items = items.filter((a) => a.status === status)
    if (search) items = items.filter((a) => a.name.includes(search))
    return paginate(items, ctx.query)
  },
  'GET /api/v1/admin/accounts/upstream-billing-rates': (ctx) => ({ items: ACCOUNTS.map((a) => ({ account_id: a.id, snapshot: null })), total: ACCOUNTS.length, page: 1, page_size: Number(ctx.query.get('page_size') || 20) }),
  'GET /api/v1/admin/accounts/upstream-billing-probe/settings': () => ({ enabled: false, interval_minutes: 60 }),
  'GET /api/v1/admin/accounts/ollama-cloud-usage/settings': () => ({ enabled: false, interval_minutes: 30, debounce_minutes: 5 }),
  'POST /api/v1/admin/accounts/usage/batch': (ctx) => {
    const usage = {}
    ;((ctx.body && ctx.body.account_ids) || ACCOUNTS.map((a) => a.id)).forEach((id) => { const a = ACCOUNTS.find((x) => x.id === id); if (a) usage[String(id)] = accountUsage(a) })
    return { usage, errors: {} }
  },
  'POST /api/v1/admin/accounts/today-stats/batch': (ctx) => {
    const stats = {}
    ;((ctx.body && ctx.body.account_ids) || ACCOUNTS.map((a) => a.id)).forEach((id) => { const a = ACCOUNTS.find((x) => x.id === id); if (a) stats[String(id)] = accountTodayStats(a) })
    return { stats }
  },
  'GET /api/v1/admin/accounts/data': () => ({ version: 1, exported_at: iso(NOW()), proxies: [], accounts: [] }),

  // ---- admin: groups / proxies / users / keys ----
  'GET /api/v1/admin/groups': (ctx) => paginate(GROUPS, ctx.query),
  'GET /api/v1/admin/groups/all': (ctx) => { const p = ctx.query.get('platform'); return p ? GROUPS.filter((g) => g.platform === p) : GROUPS },
  'GET /api/v1/admin/groups/live-capability': () => ({ supported: true }),
  'GET /api/v1/admin/proxies': (ctx) => emptyPage(ctx.query),
  'GET /api/v1/admin/proxies/all': () => [],
  'GET /api/v1/admin/plugins': () => MOCK_PLUGINS,
  'GET /api/v1/admin/user-attributes': () => [],
  'GET /api/v1/admin/users': (ctx) => paginate(ADMIN_USERS, ctx.query),
  'GET /api/v1/admin/usage': (ctx) => paginate(usageLogs(20).map((l) => ({ ...l, user: { id: 42, email: NORMAL_USER.email, username: NORMAL_USER.username }, account: { id: l.account_id, name: (ACCOUNTS.find((a) => a.id === l.account_id) || {}).name } })), ctx.query),
  'GET /api/v1/admin/usage/stats': () => ({
    period: 'all', total_requests: ADMIN_DASHBOARD_STATS.total_requests, total_input_tokens: ADMIN_DASHBOARD_STATS.total_input_tokens, total_output_tokens: ADMIN_DASHBOARD_STATS.total_output_tokens,
    total_cache_tokens: 135_600_000_000, total_cache_read_tokens: ADMIN_DASHBOARD_STATS.total_cache_read_tokens, total_cache_creation_tokens: ADMIN_DASHBOARD_STATS.total_cache_creation_tokens,
    total_tokens: ADMIN_DASHBOARD_STATS.total_tokens, total_cost: ADMIN_DASHBOARD_STATS.total_cost, total_actual_cost: ADMIN_DASHBOARD_STATS.total_actual_cost, average_duration_ms: 1_800,
    models: Object.fromEntries(MODEL_STATS.map((m) => [m.model, m.requests])), endpoints: ENDPOINT_STATS, upstream_endpoints: ENDPOINT_STATS, endpoint_paths: ENDPOINT_STATS
  }),

  // ---- admin: settings & feature status ----
  'GET /api/v1/admin/settings': () => ADMIN_SETTINGS,
  'PUT /api/v1/admin/settings': (ctx) => ({ ...ADMIN_SETTINGS, ...(ctx.body || {}) }),
  'GET /api/v1/admin/settings/admin-api-key': () => ({ exists: true, masked_key: 'sk-admin-****3f2a' }),
  'GET /api/v1/admin/settings/overload-cooldown': () => ({ enabled: true, cooldown_minutes: 10 }),
  'GET /api/v1/admin/settings/rate-limit-429-cooldown': () => ({ enabled: true, cooldown_seconds: 60, strategy: 'cooldown', retry_interval_ms: 500, retry_max_duration_seconds: 30, max_account_switches: 3 }),
  'GET /api/v1/admin/settings/panel-rate-limit': () => ({ enabled: true, user_rpm: 120, heavy_rpm: 20, exempt_admin: true, public_ip_rpm: 60 }),
  'GET /api/v1/admin/settings/stream-timeout': () => ({ enabled: true, action: 'temp_unsched', temp_unsched_minutes: 30, threshold_count: 3, threshold_window_minutes: 10 }),
  'GET /api/v1/admin/settings/rectifier': () => ({ enabled: true, thinking_signature_enabled: true, thinking_budget_enabled: true, apikey_signature_enabled: false, apikey_signature_patterns: [] }),
  'GET /api/v1/admin/settings/beta-policy': () => ({ rules: [] }),
  'GET /api/v1/admin/settings/web-search-emulation': () => ({ enabled: false, providers: [] }),
  'GET /api/v1/admin/settings/email-templates': () => ({ events: [], locales: ['zh', 'en'], templates: [] }),
  'GET /api/v1/admin/payment/config': () => ADMIN_PAYMENT_CONFIG,
  'GET /api/v1/admin/payment/providers': () => [],
  'GET /api/v1/admin/payment/channels': () => [],
  'GET /api/v1/admin/payment/plans': () => [],
  'GET /api/v1/admin/payment/invoices/unread-count': () => ({ count: 2 }),
  'GET /api/v1/admin/payment/dashboard': () => ({ total_orders: 4_120, paid_orders: 3_871, total_amount: 186_300, today_orders: 31, today_amount: 1_640, pending_orders: 4, refund_amount: 1_120 }),
  'GET /api/v1/admin/tickets/unread-count': () => ({ count: 3 }),
  'GET /api/v1/admin/tickets/reply-templates': () => [],
  'GET /api/v1/admin/announcements': (ctx) => emptyPage(ctx.query),
  'GET /api/v1/admin/compliance': () => ({
    required: false, version: 'v2026.06.10',
    document_path_zh: 'docs/legal/admin-compliance.zh.md', document_path_en: 'docs/legal/admin-compliance.en.md',
    document_url_zh: 'https://github.com/Wei-Shaw/sub2api/blob/main/docs/legal/admin-compliance.zh.md', document_url_en: 'https://github.com/Wei-Shaw/sub2api/blob/main/docs/legal/admin-compliance.en.md',
    ack_phrase_zh: '我已阅读、理解并同意 Sub2API 部署与运营合规承诺', ack_phrase_en: 'I have read, understood, and agree to the Sub2API Deployment and Operation Compliance Commitment',
    acknowledgement: { version: 'v2026.06.10', document_zh: '', document_en: '', admin_user_id: 1, accepted_at: daysAgo(40) }
  }),
  'GET /api/v1/admin/system/version': () => ({ version: '1.8.2' }),
  'GET /api/v1/admin/system/check-updates': () => ({ current_version: '1.8.2', latest_version: '1.8.2', has_update: false, cached: true, build_type: 'release' }),
  'GET /api/v1/admin/ops/dashboard/overview': () => ({}),
  'GET /api/v1/admin/ops/account-availability': () => ({ ...ACCOUNT_AVAILABILITY, timestamp: iso(NOW()) }),
  'GET /api/v1/admin/ops/errors': (ctx) => { const size = Number(ctx.query.get('page_size') || 20); return { items: OPS_ERROR_LOGS.slice(0, size), total: OPS_ERROR_LOGS.length, page: 1, page_size: size, pages: 1 } },
  'GET /api/v1/admin/affiliates/users': (ctx) => emptyPage(ctx.query)
}

// Pattern routes for parameterized paths
const patternRoutes = [
  [/^GET \/api\/v1\/admin\/accounts\/(\d+)$/, (ctx, m) => ACCOUNTS.find((a) => a.id === Number(m[1])) || ACCOUNTS[0]],
  [/^GET \/api\/v1\/admin\/accounts\/(\d+)\/usage$/, (ctx, m) => accountUsage(ACCOUNTS.find((a) => a.id === Number(m[1])) || ACCOUNTS[0])],
  [/^GET \/api\/v1\/admin\/accounts\/(\d+)\/today-stats$/, (ctx, m) => accountTodayStats(ACCOUNTS.find((a) => a.id === Number(m[1])) || ACCOUNTS[0])],
  [/^GET \/api\/v1\/admin\/accounts\/(\d+)\/stats$/, (ctx, m) => accountStats(ACCOUNTS.find((a) => a.id === Number(m[1])) || ACCOUNTS[0], Number(ctx.query.get('days') || 30))],
  [/^GET \/api\/v1\/admin\/accounts\/(\d+)\/models$/, () => [{ id: 'claude-sonnet-4-5', display_name: 'Claude Sonnet 4.5', created_at: '2025-09-29T00:00:00Z', type: 'model' }, { id: 'claude-fable-4-1', display_name: 'Claude Fable 4.1', created_at: '2025-08-05T00:00:00Z', type: 'model' }]],
  [/^GET \/api\/v1\/admin\/accounts\/(\d+)\/cyber-events$/, () => ({ events: [], total: 0, page: 1, page_size: 20 })],
  [/^POST \/api\/v1\/admin\/accounts\/(\d+)\/(schedulable|clear-error|refresh|recover-state|clear-rate-limit|set-privacy)$/, (ctx, m) => {
    const a = ACCOUNTS.find((x) => x.id === Number(m[1])) || ACCOUNTS[0]
    if (m[2] === 'schedulable' && ctx.body && typeof ctx.body.schedulable === 'boolean') a.schedulable = ctx.body.schedulable
    if (m[2] === 'clear-error' || m[2] === 'recover-state') { a.status = 'active'; a.error_message = null }
    return a
  }],
  [/^PUT \/api\/v1\/admin\/accounts\/(\d+)$/, (ctx, m) => ({ ...(ACCOUNTS.find((x) => x.id === Number(m[1])) || ACCOUNTS[0]), ...(ctx.body || {}) })],
  [/^GET \/api\/v1\/admin\/groups\/(\d+)$/, (ctx, m) => groupById(Number(m[1])) || GROUPS[0]],
  [/^GET \/api\/v1\/admin\/groups\/(\d+)\/api-keys$/, (ctx) => emptyPage(ctx.query)],
  [/^GET \/api\/v1\/admin\/groups\/(\d+)\/composite-routes$/, () => []],
  [/^GET \/api\/v1\/admin\/users\/(\d+)$/, (ctx, m) => ADMIN_USERS.find((u) => u.id === Number(m[1])) || ADMIN_USERS[1]],
  [/^GET \/api\/v1\/admin\/users\/(\d+)\/api-keys$/, (ctx) => paginate(API_KEYS, ctx.query)],
  [/^GET \/api\/v1\/keys\/(\d+)$/, (ctx, m) => API_KEYS.find((k) => k.id === Number(m[1])) || API_KEYS[0]],
  [/^PUT \/api\/v1\/keys\/(\d+)$/, (ctx, m) => {
    const k = API_KEYS.find((x) => x.id === Number(m[1])) || API_KEYS[0]
    if (ctx.body && ctx.body.status) k.status = ctx.body.status
    if (ctx.body && 'group_id' in ctx.body) { k.group_id = ctx.body.group_id || null; k.group = k.group_id ? publicGroup(groupById(k.group_id) || GROUPS[0]) : undefined }
    return k
  }],
  [/^GET \/api\/v1\/user\/api-keys\/(\d+)\/usage\/daily$/, (ctx) => {
    const days = Number(ctx.query.get('days') || 30)
    const items = trendSeries(days, 0.004).map((t) => ({ date: t.date, requests: t.requests, input_tokens: t.input_tokens, output_tokens: t.output_tokens, cache_read_tokens: t.cache_read_tokens, cache_write_tokens: t.cache_creation_tokens, total_tokens: t.total_tokens, cost: t.cost, actual_cost: t.actual_cost }))
    return { items, days, start_date: dateStr(-(days - 1)), end_date: dateStr(0) }
  }],
  [/^GET \/api\/v1\/usage\/(\d+)$/, () => usageLogs(1)[0]]
]

// ==================== HTTP plumbing ====================
function send(res, status, payload) {
  const body = JSON.stringify(payload)
  res.writeHead(status, {
    'Content-Type': 'application/json; charset=utf-8',
    'Content-Length': Buffer.byteLength(body),
    'Access-Control-Allow-Origin': '*',
    'Access-Control-Allow-Credentials': 'true',
    'Access-Control-Allow-Methods': 'GET,POST,PUT,PATCH,DELETE,OPTIONS',
    'Access-Control-Allow-Headers': '*',
    'Access-Control-Expose-Headers': 'ETag',
    'Cache-Control': 'no-store'
  })
  res.end(body)
}

function readBody(req) {
  return new Promise((resolve) => {
    const chunks = []
    req.on('data', (c) => chunks.push(c))
    req.on('end', () => {
      const raw = Buffer.concat(chunks).toString('utf8')
      if (!raw) return resolve(null)
      try { resolve(JSON.parse(raw)) } catch { resolve(raw) }
    })
    req.on('error', () => resolve(null))
  })
}

const server = http.createServer(async (req, res) => {
  const url = new URL(req.url, `http://localhost:${PORT}`)
  const path = url.pathname.replace(/\/+$/, '') || '/'
  const method = req.method.toUpperCase()
  const stamp = new Date().toISOString().slice(11, 19)

  // --- UI preview helper: flips the in-memory needs_setup flag so /setup renders its wizard
  // branch instead of redirecting away. GET /setup/dev-toggle?needs_setup=1|0 ---
  if (path === '/setup/dev-toggle') {
    SETUP_NEEDS_SETUP = url.searchParams.get('needs_setup') === '1'
    return send(res, 200, { code: 0, message: 'ok', data: { needs_setup: SETUP_NEEDS_SETUP } })
  }

  // --- UI preview helper: seeds a logged-in session then redirects (served through the vite proxy) ---
  if (path === '/setup/seed') {
    const role = url.searchParams.get('role') || 'admin'
    const to = url.searchParams.get('to') || (role === 'admin' ? '/admin/dashboard' : '/dashboard')
    const theme = (url.searchParams.get('theme') || 'light').replace(/^glass-/, '')
    const admin = role === 'admin'
    const user = admin
      ? { id: 1, username: 'Admin', email: 'admin@sub2api.dev', role: 'admin', balance: 142.6, frozen_balance: 0, concurrency: 20, rpm_limit: 0, status: 'active', allowed_groups: null, balance_notify_enabled: true, balance_notify_threshold: 10, balance_notify_extra_emails: [], subscriptions: [], avatar_url: null, email_bound: true, linuxdo_bound: false, oidc_bound: false, wechat_bound: false, created_at: '2025-03-12T08:30:00Z', updated_at: '2026-09-03T00:00:00Z' }
      : { id: 42, username: '林小雨', email: 'xiaoyu.lin@example.com', role: 'user', balance: 38.25, frozen_balance: 0, concurrency: 5, rpm_limit: 0, status: 'active', allowed_groups: null, balance_notify_enabled: true, balance_notify_threshold: 10, balance_notify_extra_emails: [], subscriptions: [], avatar_url: null, email_bound: true, linuxdo_bound: false, oidc_bound: false, wechat_bound: false, created_at: '2025-11-02T02:14:00Z', updated_at: '2026-09-03T00:00:00Z' }
    if (role === 'guest') {
      res.writeHead(200, { 'Content-Type': 'text/html; charset=utf-8' })
      res.end(`<!doctype html><meta charset="utf-8"><script>
['auth_token','refresh_token','token_expires_at','auth_user'].forEach(k => localStorage.removeItem(k));
localStorage.setItem('admin_guide', 'true');
localStorage.setItem('user_guide', 'true');
localStorage.setItem('theme', ${JSON.stringify(theme)});
localStorage.setItem('sub2api_locale', ${JSON.stringify(url.searchParams.get('locale') || 'zh')});
['onboarding_tour','admin_guide','user_guide'].forEach(function(b){['guest','1','42'].forEach(function(uid){['user','admin'].forEach(function(r){localStorage.setItem(b+'_'+uid+'_'+r+'_v4_interactive','true')})})});
location.replace(${JSON.stringify(to)});
</script>`)
      return
    }
    if (url.searchParams.get('dump') === '1') {
      res.writeHead(200, { 'Content-Type': 'text/html; charset=utf-8' })
      res.end(`<!doctype html><meta charset="utf-8"><body style="font:12px monospace;white-space:pre-wrap"><script>document.body.textContent=Object.keys(localStorage).sort().map(function(k){return k+' = '+localStorage.getItem(k).slice(0,60)}).join('\\n')<\/script>`)
      return
    }
    const html = `<!doctype html><meta charset="utf-8"><script>
localStorage.setItem('onboarding_tour_1_admin_v4_interactive', 'true');
localStorage.setItem('onboarding_tour_42_user_v4_interactive', 'true');
localStorage.setItem('auth_token', ${JSON.stringify(admin ? 'mock-token' : 'mock-user-token')});
localStorage.setItem('refresh_token', 'mock-refresh');
localStorage.setItem('token_expires_at', String(Date.now() + 86400 * 1000));
localStorage.setItem('auth_user', ${JSON.stringify(JSON.stringify(user))});
localStorage.setItem('admin_guide', 'true');
localStorage.setItem('user_guide', 'true');
localStorage.setItem('theme', ${JSON.stringify(theme)});
localStorage.setItem('sub2api_locale', ${JSON.stringify(url.searchParams.get('locale') || 'zh')});
['onboarding_tour','admin_guide','user_guide'].forEach(function(b){['guest','1','42'].forEach(function(uid){['user','admin'].forEach(function(r){localStorage.setItem(b+'_'+uid+'_'+r+'_v4_interactive','true')})})});
location.replace(${JSON.stringify(to)});
</script>`
    res.writeHead(200, { 'Content-Type': 'text/html; charset=utf-8' })
    res.end(html)
    return
  }

  if (method === 'OPTIONS') {
    res.writeHead(204, {
      'Access-Control-Allow-Origin': '*',
      'Access-Control-Allow-Credentials': 'true',
      'Access-Control-Allow-Methods': 'GET,POST,PUT,PATCH,DELETE,OPTIONS',
      'Access-Control-Allow-Headers': '*',
      'Access-Control-Max-Age': '86400'
    })
    return res.end()
  }

  const body = method === 'GET' || method === 'HEAD' ? null : await readBody(req)
  const ctx = { req, res, query: url.searchParams, body, path }
  const key = `${method} ${path}`

  let handler = routes[key]
  let match = null
  if (!handler) {
    for (const [re, fn] of patternRoutes) {
      const m = key.match(re)
      if (m) { handler = fn; match = m; break }
    }
  }

  if (!handler) {
    const fallback = method === 'GET' ? emptyPage(url.searchParams) : {}
    console.log(`${stamp} ${method} ${path}${url.search}  -> (fallback)`)
    return send(res, 200, { code: 0, message: 'ok', data: fallback })
  }

  try {
    const data = await handler(ctx, match)
    console.log(`${stamp} ${method} ${path}${url.search}`)
    send(res, 200, { code: 0, message: 'ok', data })
  } catch (err) {
    console.error(`${stamp} ${method} ${path} !! ${err && err.stack}`)
    send(res, 200, { code: 0, message: 'ok', data: method === 'GET' ? emptyPage(url.searchParams) : {} })
  }
})

server.listen(PORT, '0.0.0.0', () => {
  console.log(`Sub2API mock backend listening on http://localhost:${PORT}`)
  console.log(`Exact routes: ${Object.keys(routes).length}, pattern routes: ${patternRoutes.length}`)
})
