# Changelog

## 0.1.152

### Security

- Sanitize home HTML content and auth redirects (including email-verify completion).
- Admin compliance hard-blocks SPA navigation; temporary status-load failures use a retryable unavailable state.
- Add `SECURITY.md` with private reporting contact and response expectations.
- Pin `govulncheck` in the security-scan workflow; run secret scan in CI.

### Fixed

- OpenAI API-key Chat Completions raw path now passes `promptCacheKey` correctly (multi-tenant cache isolation).
- Payment: late paid webhooks after cancel/expire fulfill; balance cache re-invalidates on COMPLETED; CNY subscription checkout requires a positive USD→CNY rate (see `docs/PAYMENT.md`).
- Grok OAuth default concurrency is 1 (backend + UI); `>1` requires `XAI_GROK_UNSAFE_ALLOW_CONCURRENCY_GT_ONE=1`.
- Token cache invalidation covers setup-token accounts; bulk model-routing rejects mixed platforms.
- Migrations 195a/199a create unique indexes before dropping legacy indexes, with INVALID-index retry on restart.
- Deploy defaults to checksum **backup only**; rewrites are opt-in and allowlisted.

### Changed

- Content-moderation admin copy clarifies audit **service failure** policy (not only HTTP 429).
- Multi-language README Go version badges aligned to 1.26.5.
