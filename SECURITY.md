# Security Policy

## Supported Versions

Security fixes are handled on the default branch. Users should run the latest
released image or build from the latest tagged release unless a maintainer asks
for a branch-specific verification.

## Reporting a Vulnerability

Please do not open a public issue for exploitable vulnerabilities. Contact the
maintainers privately with:

- affected version or commit
- deployment mode and relevant configuration
- reproduction steps or proof of concept
- expected impact and any known mitigations

If private contact details are not available for your fork or deployment, open a
minimal public issue that asks maintainers to establish a private disclosure
channel and avoid including exploit details.

## Security Checks

CI currently runs:

- `govulncheck ./...` for Go dependency and reachable vulnerability checks
- `python tools/secret_scan.py` for high-confidence committed secret patterns
- `pnpm audit --prod --audit-level=high` with reviewed exceptions from
  `.github/audit-exceptions.yml`

`golangci-lint` runs in backend CI. `gosec` is not currently a CI gate; add it as
a separate workflow change if the finding baseline is reviewed.

## Dependency Audit Exceptions

High and critical frontend audit findings must either be fixed or listed in
`.github/audit-exceptions.yml` with an advisory id, mitigation, owner, and
expiration date. Expired exceptions fail CI and require a new review.
