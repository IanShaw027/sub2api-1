# Node Echo

This directory is a minimal `node20` script skill bundle source.

- Runtime: `node20`
- Protocol: `json-file-v1`
- Entrypoint: `main.mjs`

## Package as zip

Run the zip command from this directory so `skill.yaml` stays at the archive
root:

```bash
cd /opt/sub2api/backend/internal/service/testdata/skills/script_node_echo
zip -r /tmp/script_node_echo.zip .
```

Do not add `Dockerfile`, `docker-compose.yml`, or other deployment files to the
bundle.

## Run with the sandbox example

If your host only provides `docker-compose`, replace `docker compose` below.

```bash
cd /opt/sub2api
bash deploy/skill-runner/prepare-scratch.sh node
docker compose -f deploy/skill-runner/docker-compose.example.yml run --rm skill-node20
cat deploy/skill-runner/runtime/node/output/response.json
```

Expected response:

```json
{"ok":true,"runtime":"node20","echo":"Hello From Node","lower":"hello from node"}
```
