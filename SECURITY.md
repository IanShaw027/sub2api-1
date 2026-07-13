# Security Policy

## Supported Versions

Security fixes are handled on the default branch. Users should run the latest
released image or build from the latest tagged release unless a maintainer asks
for a branch-specific verification.

| Version | Supported |
|---------|-----------|
| Latest release tag / default branch | Yes |
| Older tags | Best-effort only |

## Reporting a Vulnerability

Please **do not** open a public GitHub issue for exploitable vulnerabilities.

### Private contact

Prefer one of:

1. **GitHub Private Vulnerability Reporting** on this repository (Security → Report a vulnerability), when enabled for the repo.
2. Email maintainers at **security@pincc.ai** (or the contact listed in the project README sponsor/support section for your fork).

Include:

- affected version, image tag, or commit
- deployment mode and relevant configuration (redact secrets)
- reproduction steps or proof of concept
- expected impact and any known mitigations

### Response expectations

We aim to:

- **Acknowledge** private reports within **7 days**
- **Triage** severity and scope within **14 days** of acknowledgement
- **Ship or publish a fix / advisory** for confirmed high/critical issues as soon as practical after a patch is ready

Complex issues (supply-chain, multi-component, or third-party dependency) may take longer; we will keep reporters updated when possible.

### Scope (examples)

In scope for private reports:

- authentication / authorization bypass
- remote code execution, SSRF, path traversal against the gateway or admin UI
- stored XSS or open redirects that affect end users
- payment or balance integrity issues (double credit, unauthorized refunds, under/over charge via logic bugs)
- secret leakage from the repository or default configs

Out of scope / lower priority (may still be tracked publicly):

- denial of service via legitimate high-volume API use without a clear amplification bug
- issues only present with intentionally unsafe operator configuration (e.g. disabled URL allowlists, known-bad proxies)
- vulnerabilities solely in third-party upstream AI providers

## Security Checks

CI currently runs:

- **pinned** `govulncheck` for Go dependency and reachable vulnerability checks
- `python tools/secret_scan.py` for high-confidence committed secret patterns
- `pnpm audit --prod --audit-level=high` with reviewed exceptions from
  `.github/audit-exceptions.yml`

`golangci-lint` runs in backend CI. `gosec` is not currently a standalone CI
gate; findings may still surface via golangci configuration.

## Dependency Audit Exceptions

High and critical frontend audit findings must either be fixed or listed in
`.github/audit-exceptions.yml` with:

- advisory id
- severity
- mitigation
- **owner** (maintainer identity, not a placeholder domain)
- **expiration date**

Expired exceptions fail CI and require a new review.

## Operator hardening notes (recent)

When upgrading, operators should be aware of:

- **Subscription CNY checkout** requires a positive `subscription_usd_to_cny_rate` (see `docs/PAYMENT.md`).
- **Grok OAuth concurrency** follows the standard account concurrency setting without a platform-specific gate.
- **Admin compliance** hard-blocks the SPA until acknowledgement succeeds or status can be loaded; the API remains authoritative with HTTP 423.
