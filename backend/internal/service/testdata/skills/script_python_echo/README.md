# Python Echo

This directory is a minimal `python3.11` script skill bundle source.

- Runtime: `python3.11`
- Protocol: `json-file-v1`
- Entrypoint: `main.py`

## Package as zip

Run the zip command from this directory so `skill.yaml` stays at the archive
root:

```bash
cd /opt/sub2api/backend/internal/service/testdata/skills/script_python_echo
zip -r /tmp/script_python_echo.zip .
```

Do not add `Dockerfile`, `docker-compose.yml`, or other deployment files to the
bundle.

## Run with the sandbox example

If your host only provides `docker-compose`, replace `docker compose` below.

```bash
cd /opt/sub2api
bash deploy/skill-runner/prepare-scratch.sh python
docker compose -f deploy/skill-runner/docker-compose.example.yml run --rm skill-python311
cat deploy/skill-runner/runtime/python/output/response.json
```

Expected response:

```json
{"ok": true, "runtime": "python3.11", "echo": "Hello From Python", "upper": "HELLO FROM PYTHON"}
```
