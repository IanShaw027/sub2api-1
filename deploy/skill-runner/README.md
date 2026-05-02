# Script Skill Runner Sandbox Example

This directory is intentionally isolated from the main `deploy/docker-compose*.yml`
stack. It mirrors the phase-1 script sandbox contract implemented by
`backend/internal/integration/skillrunner`.

## What this example proves

- the skill process runs as fixed non-root user `65532:65532`
- outbound network is disabled with `network_mode: none`
- the container root filesystem is read-only
- `/tmp` is writable tmpfs only
- the skill bundle is mounted read-only at `/workspace/skill`
- per-run scratch space is mounted read-write at `/sandbox`
- request path: `SUB2API_SKILL_INPUT=/sandbox/input/request.json`
- response path: `SUB2API_SKILL_OUTPUT=/sandbox/output/response.json`

## Prerequisites

- Docker Engine
- `docker compose` plugin or legacy `docker-compose`
- repository layout kept unchanged, because the compose file bind-mounts
  `../../backend/internal/service/testdata/skills/*`

If your host only provides the legacy CLI, replace `docker compose` below with
`docker-compose`.

## Prepare scratch directories

The sample containers run as uid/gid `65532`, so the host output directories
must be writable before each run:

```bash
bash deploy/skill-runner/prepare-scratch.sh
```

This removes stale `response.json` files and makes
`deploy/skill-runner/runtime/*/output` writable for the non-root container user.

## Run the Python sample

```bash
docker compose -f deploy/skill-runner/docker-compose.example.yml run --rm skill-python311
cat deploy/skill-runner/runtime/python/output/response.json
```

Expected response:

```json
{"ok": true, "runtime": "python3.11", "echo": "Hello From Python", "upper": "HELLO FROM PYTHON"}
```

## Run the Node sample

```bash
docker compose -f deploy/skill-runner/docker-compose.example.yml run --rm skill-node20
cat deploy/skill-runner/runtime/node/output/response.json
```

Expected response:

```json
{"ok":true,"runtime":"node20","echo":"Hello From Node","lower":"hello from node"}
```

## Notes

- request payloads live under `deploy/skill-runner/runtime/*/input/request.json`
- stdout/stderr are debug logs only; business output must be written to
  `response.json`
- runtime dependencies must already exist inside the skill bundle; phase-1 does
  not install anything online during execution
