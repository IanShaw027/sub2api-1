# Sub2API mock backend

Standalone Node.js mock of the Sub2API Go backend for visual review of the Vue frontend. No npm dependencies.

## Start

```bash
cd frontend/scripts/mock
node server.js            # listens on http://localhost:8091  (override with PORT=...)
```

Point the Vite dev server at it (the dev proxy forwards `/api`, `/v1`, `/setup` to `VITE_DEV_PROXY_TARGET`, default `http://localhost:8080`):

```bash
cd /home/box/code/sub2api/frontend
VITE_DEV_PROXY_TARGET=http://localhost:8091 npm run dev
```

Every request path is logged to stdout (`server.log` when started with nohup). Unknown routes never 404:
GET returns `{code:0,data:{items:[],total:0,page:1,page_size:20,pages:0}}`, other methods return `{code:0,data:{}}`.
All responses use the `{ code: 0, message: "ok", data }` envelope and carry permissive CORS headers; OPTIONS is answered with 204.

## Endpoints implemented (all under /api/v1 unless noted)

Public / auth
- `GET /settings/public` – full `PublicSettings` (site_name "Sub2API", version 1.8.2, payment/ticket/affiliate/model plaza/channel monitor enabled, github/linuxdo/dingtalk OAuth on)
- `GET /setup/status` and `GET /api/v1/setup/status` – `{ needs_setup: <flag>, step: <flag ? "database" : "done"> }` (the frontend calls the un-prefixed one). `<flag>` defaults to `false` (start the server with `MOCK_NEEDS_SETUP=1` to default it to `true`), and can be flipped at runtime with `GET /setup/dev-toggle?needs_setup=1` (or `=0`) — handy for driving `/setup` into its wizard branch for screenshots without restarting the mock. `POST /setup/install` resets the flag back to `false`.
- `POST /setup/test-db`, `POST /setup/test-redis` – always `{ success: true, message: "…connection successful" }`
- `POST /setup/install` – `{ message: "Installation successful", restart: false }`, also clears the `needs_setup` flag above
- `POST /auth/login` – `AuthResponse` (admin by default; an email containing "user" or "xiaoyu" logs in as the normal user)
- `GET /auth/me` – `CurrentUserResponse` (the user object itself; admin unless the bearer token is `mock-user-token`)
- `POST /auth/refresh` – `RefreshTokenResponse`; `POST /auth/logout`, `POST /auth/revoke-all-sessions`

User side
- `GET /user/profile`, `GET /user/rpm-status`, `GET /user/platform-quotas`, `GET /user/totp/status`, `GET /user/passkeys`, `GET /user/aff`
- `GET /keys` (5 keys: production, claude-code-laptop, cursor-team, ci-runner [expired], temp-demo [disabled]), `GET /keys/:id`, `PUT /keys/:id`, `POST /keys`
- `GET /groups/available`, `GET /groups/rates`
- `GET /subscriptions`, `/subscriptions/active`, `/subscriptions/progress`, `/subscriptions/summary` – empty
- `GET /usage` (12 fake usage logs), `GET /usage/:id`, `GET /usage/stats`, `GET /usage/errors`
- `GET /usage/dashboard/stats`, `/usage/dashboard/trend`, `/usage/dashboard/models`, `/usage/dashboard/snapshot-v2`, `POST /usage/dashboard/api-keys-usage`, `GET /user/api-keys/:id/usage/daily`
- `GET /announcements` – `[]`; `GET /tickets` (empty), `GET /tickets/unread-count` → `{count:3}`, `GET /tickets/rate-groups`
- `GET /payment/config`, `/payment/plans`, `/payment/orders/my`, `/payment/invoices`, `GET /redeem/history`, `GET /channels/available`, `GET /channel-monitors`, `GET /model-plaza`

Admin dashboard
- `GET /admin/dashboard/stats` – every `DashboardStats` field (1284 users, 3902 keys, 86 accounts, 128 430 requests today, rpm 842, tpm 12.4M …)
- `GET /admin/dashboard/realtime`, `/trend`, `/models`, `/groups`, `/user-breakdown`, `/snapshot-v2` (stats+trend+models+groups+users_trend), `/api-keys-trend`, `/users-trend`, `/users-ranking`
- `POST /admin/dashboard/users-usage`, `POST /admin/dashboard/api-keys-usage`
- Series honour `start_date`/`end_date` (default 14 days) and `include_*` flags on snapshot-v2.

Admin accounts
- `GET /admin/accounts` – 8 accounts (anthropic oauth/apikey/setup-token, openai oauth, gemini, antigravity, grok; one `error`, one rate-limited, one temp-unschedulable, two unschedulable). Supports `platform`, `status`, `search` filters + pagination.
- `GET /admin/accounts/:id`, `PUT /admin/accounts/:id`, `GET /admin/accounts/:id/usage`, `/today-stats`, `/stats?days=`, `/models`, `/cyber-events`
- `POST /admin/accounts/usage/batch` (AccountUsageInfo per platform: 5h/7d windows, gemini/antigravity/grok quotas), `POST /admin/accounts/today-stats/batch`
- `POST /admin/accounts/:id/(schedulable|clear-error|refresh|recover-state|clear-rate-limit|set-privacy)`
- `GET /admin/accounts/upstream-billing-rates`, `/upstream-billing-probe/settings`, `/ollama-cloud-usage/settings`, `/data`

Admin groups / users / proxies / usage
- `GET /admin/groups` (paginated, 4 AdminGroups: 默认分组, Claude Max, OpenAI Codex, Gemini), `GET /admin/groups/all`, `/admin/groups/live-capability`, `/admin/groups/:id`, `/admin/groups/:id/api-keys`, `/admin/groups/:id/composite-routes`
- `GET /admin/users` (9 AdminUsers), `GET /admin/users/:id`, `GET /admin/users/:id/api-keys`
- `GET /admin/proxies`, `/admin/proxies/all` – empty
- `GET /admin/usage` (20 logs), `GET /admin/usage/stats`

Admin settings & status
- `GET /admin/settings` (full `SystemSettings`), `PUT /admin/settings`
- `GET /admin/settings/admin-api-key`, `/overload-cooldown`, `/rate-limit-429-cooldown`, `/panel-rate-limit`, `/stream-timeout`, `/rectifier`, `/beta-policy`, `/web-search-emulation`, `/email-templates`
- `GET /admin/payment/config`, `/providers`, `/channels`, `/plans`, `/dashboard`, `GET /admin/payment/invoices/unread-count` → `{count:2}`
- `GET /admin/tickets/unread-count` → `{count:3}`, `/admin/tickets/reply-templates`, `GET /admin/announcements`
- `GET /admin/compliance` → `{required:false,…}`, `GET /admin/system/version`, `/admin/system/check-updates`, `GET /admin/affiliates/users`

## Pre-seeding a logged-in session

The auth store (`frontend/src/stores/auth.ts`) restores a session from these localStorage keys:
`auth_token`, `auth_user` (JSON `User`), `refresh_token`, `token_expires_at` (ms epoch string).
On boot it calls `GET /auth/me` with the bearer token and overwrites `auth_user`, so the token value decides the role:
`mock-token` → admin, `mock-user-token` → normal user.

Run either snippet in the browser devtools console on the dev server origin (e.g. http://localhost:3000), then reload.

### Admin session

```js
localStorage.setItem('auth_token', 'mock-token');
localStorage.setItem('refresh_token', 'mock-refresh-token');
localStorage.setItem('token_expires_at', String(Date.now() + 86400 * 1000));
localStorage.setItem('auth_user', JSON.stringify({
  id: 1, username: 'Admin', email: 'admin@sub2api.dev', role: 'admin',
  balance: 142.6, frozen_balance: 0, concurrency: 20, rpm_limit: 0, status: 'active',
  allowed_groups: null, balance_notify_enabled: true, balance_notify_threshold: 10,
  balance_notify_extra_emails: [], subscriptions: [], avatar_url: null,
  email_bound: true, linuxdo_bound: false, oidc_bound: false, wechat_bound: false,
  created_at: '2025-03-12T08:30:00Z', updated_at: '2026-09-03T00:00:00Z'
}));
location.href = '/admin/dashboard';
```

### Normal user session

```js
localStorage.setItem('auth_token', 'mock-user-token');
localStorage.setItem('refresh_token', 'mock-user-refresh-token');
localStorage.setItem('token_expires_at', String(Date.now() + 86400 * 1000));
localStorage.setItem('auth_user', JSON.stringify({
  id: 42, username: '林小雨', email: 'xiaoyu.lin@example.com', role: 'user',
  balance: 38.25, frozen_balance: 0, concurrency: 5, rpm_limit: 0, status: 'active',
  allowed_groups: null, balance_notify_enabled: true, balance_notify_threshold: 10,
  balance_notify_extra_emails: [], subscriptions: [], avatar_url: null,
  email_bound: true, linuxdo_bound: false, oidc_bound: false, wechat_bound: false,
  created_at: '2025-11-02T02:14:00Z', updated_at: '2026-09-03T00:00:00Z'
}));
location.href = '/dashboard';
```

To clear: `['auth_token','auth_user','refresh_token','token_expires_at'].forEach(k => localStorage.removeItem(k))`.

## Quick smoke test

```bash
curl -s localhost:8091/api/v1/settings/public | head -c 300
curl -s localhost:8091/api/v1/admin/dashboard/stats | head -c 300
curl -s 'localhost:8091/api/v1/admin/accounts?page=1&page_size=20' | head -c 300
curl -s localhost:8091/api/v1/keys | head -c 300
```
