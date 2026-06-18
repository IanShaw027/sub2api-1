from __future__ import annotations

import argparse
import hashlib
import json
import os
import pathlib
import subprocess
import sys
import time
from dataclasses import asdict, dataclass
from typing import Any

try:
    import requests
except ImportError:  # pragma: no cover - optional runtime dependency
    requests = None


DEFAULT_PROMPT = "生成一张雨夜霓虹街景海报，电影感，中文招牌，反射丰富"
DEFAULT_MAIN_MODEL = "gpt-5.4-mini"
DEFAULT_TOOL_MODEL = "gpt-image-2"
DEFAULT_SIZE = "1024x1024"
DEFAULT_TIMEOUT = 60
DEFAULT_STREAM_SECONDS = 15
DEFAULT_MAX_EVENTS = 12
DEFAULT_OUTPUT_ROOT = pathlib.Path("/opt/sub2api/tmp/oauth-responses-probe")


@dataclass
class ProbeSummary:
    account_id: int
    header_mode: str
    transport: str
    status_code: int | None
    http_version: str
    elapsed_ms: int
    timed_out: bool
    event_count: int
    x_oai_request_id: str
    output_dir: str


def require_requests() -> None:
    if requests is None:
        raise RuntimeError("requests is required for transport=requests")


def run_psql_json(sql: str) -> dict[str, Any]:
    cmd = [
        "sudo",
        "-u",
        "postgres",
        "psql",
        "-d",
        "sub2api",
        "-At",
        "-c",
        sql,
    ]
    proc = subprocess.run(cmd, check=True, capture_output=True, text=True)
    raw = proc.stdout.strip()
    if not raw:
        raise RuntimeError("psql returned empty result")
    return json.loads(raw)


def load_account(account_id: int) -> dict[str, Any]:
    sql = (
        "SELECT json_build_object("
        "'id', id, "
        "'name', name, "
        "'type', type, "
        "'platform', platform, "
        "'credentials', credentials, "
        "'extra', extra"
        ")::text "
        f"FROM public.accounts WHERE id = {int(account_id)};"
    )
    account = run_psql_json(sql)
    if not account:
        raise RuntimeError(f"account {account_id} not found")
    return account


def sticky_session_seed(endpoint: str, model: str, size: str, prompt: str) -> str:
    seed = "|".join(
        [
            "openai-images",
            endpoint.strip(),
            model.strip(),
            size.strip(),
            prompt.strip(),
        ]
    )
    digest = hashlib.sha256(seed.encode("utf-8")).hexdigest()[:16]
    return f"openai-images-{digest}"


def build_payload(args: argparse.Namespace) -> dict[str, Any]:
    payload: dict[str, Any] = {
        "instructions": "",
        "stream": True,
        "reasoning": {"effort": "medium", "summary": "auto"},
        "parallel_tool_calls": True,
        "include": ["reasoning.encrypted_content"],
        "model": args.main_model,
        "store": False,
        "input": [
            {
                "type": "message",
                "role": "user",
                "content": [{"type": "input_text", "text": args.prompt}],
            }
        ],
        "tools": [
            {
                "type": "image_generation",
                "action": "generate",
                "model": args.tool_model,
                "size": args.size,
                "quality": args.quality,
                "background": args.background,
                "output_format": args.output_format,
                "moderation": args.moderation,
                "output_compression": args.output_compression,
                "partial_images": args.partial_images,
            }
        ],
        "tool_choice": {"type": "image_generation"},
    }
    return payload


def build_headers(account: dict[str, Any], args: argparse.Namespace) -> dict[str, str]:
    credentials = account.get("credentials") or {}
    extra = account.get("extra") or {}
    access_token = str(credentials.get("access_token") or "").strip()
    chatgpt_account_id = str(
        extra.get("chatgpt_account_id")
        or credentials.get("chatgpt_account_id")
        or ((extra.get("web_profile") or {}).get("account_id"))
        or ""
    ).strip()
    if not access_token:
        raise RuntimeError(f"account {account['id']} missing access_token")
    if not chatgpt_account_id:
        raise RuntimeError(f"account {account['id']} missing chatgpt_account_id")

    headers = {
        "authorization": f"Bearer {access_token}",
        "chatgpt-account-id": chatgpt_account_id,
        "accept": "text/event-stream",
        "content-type": "application/json",
    }

    if args.header_mode == "minimal":
        headers["originator"] = args.originator or "codex_cli_rs"
        headers["OpenAI-Beta"] = "responses=experimental"
        if not args.omit_user_agent:
            headers["user-agent"] = args.user_agent or "OpenAI/1.0 Codex/0.1 (oauth-probe)"
        return headers

    session_seed = sticky_session_seed(
        endpoint="/v1/images/generations",
        model=args.tool_model,
        size=args.size,
        prompt=args.prompt,
    )
    headers["originator"] = args.originator or "opencode"
    headers["OpenAI-Beta"] = "responses=experimental"
    headers["session_id"] = args.session_id or session_seed
    headers["conversation_id"] = args.conversation_id or session_seed
    if not args.omit_user_agent and args.user_agent:
        headers["user-agent"] = args.user_agent
    return headers


def sanitize_headers(headers: dict[str, str]) -> dict[str, str]:
    sanitized = dict(headers)
    auth = sanitized.get("authorization", "")
    if auth.lower().startswith("bearer ") and len(auth) > 20:
        sanitized["authorization"] = auth[:16] + "...redacted"
    account_id = sanitized.get("chatgpt-account-id", "")
    if account_id:
        sanitized["chatgpt-account-id"] = account_id[:6] + "...redacted"
    return sanitized


def ensure_output_dir(output_dir: str | None, account_id: int, header_mode: str, transport: str) -> pathlib.Path:
    if output_dir:
        path = pathlib.Path(output_dir)
    else:
        stamp = time.strftime("%Y%m%d-%H%M%S")
        path = DEFAULT_OUTPUT_ROOT / f"{stamp}_acct{account_id}_{header_mode}_{transport}"
    path.mkdir(parents=True, exist_ok=True)
    return path


def parse_http_version_from_requests(resp: requests.Response) -> str:
    version = getattr(resp.raw, "version", None)
    if version == 10:
        return "HTTP/1.0"
    if version == 11:
        return "HTTP/1.1"
    if version == 20:
        return "HTTP/2.0"
    return "unknown"


def parse_sse_events_from_text(text: str, max_events: int) -> list[dict[str, str]]:
    events: list[dict[str, str]] = []
    block: list[str] = []
    for raw_line in text.splitlines():
        line = raw_line.rstrip("\r")
        if line == "":
            if block:
                event = parse_sse_block(block)
                if event:
                    events.append(event)
                    if len(events) >= max_events:
                        break
                block = []
            continue
        block.append(line)
    if block and len(events) < max_events:
        event = parse_sse_block(block)
        if event:
            events.append(event)
    return events


def parse_sse_block(lines: list[str]) -> dict[str, str] | None:
    event_name = ""
    data_lines: list[str] = []
    for line in lines:
        if line.startswith("event:"):
            event_name = line.split(":", 1)[1].strip()
        elif line.startswith("data:"):
            data_lines.append(line.split(":", 1)[1].lstrip())
    if not event_name and not data_lines:
        return None
    return {"event": event_name, "data_preview": "\n".join(data_lines)[:800]}


def write_json(path: pathlib.Path, data: Any) -> None:
    path.write_text(json.dumps(data, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")


def probe_with_requests(
    url: str,
    payload: dict[str, Any],
    headers: dict[str, str],
    args: argparse.Namespace,
    output_dir: pathlib.Path,
) -> ProbeSummary:
    require_requests()
    started = time.time()
    response = requests.post(
        url,
        headers=headers,
        json=payload,
        stream=True,
        timeout=(10, args.timeout),
    )
    elapsed_ms = int((time.time() - started) * 1000)
    body_lines: list[str] = []
    events: list[dict[str, str]] = []
    deadline = time.time() + args.stream_seconds
    timed_out = False
    try:
        for raw_line in response.iter_lines():
            if raw_line is None:
                continue
            line = raw_line.decode("utf-8", errors="replace") if isinstance(raw_line, bytes) else raw_line
            body_lines.append(line)
            if line == "":
                events = parse_sse_events_from_text("\n".join(body_lines), args.max_events)
                if len(events) >= args.max_events:
                    break
            if time.time() >= deadline:
                timed_out = True
                break
    except requests.RequestException:
        timed_out = True
        events = parse_sse_events_from_text("\n".join(body_lines), args.max_events)
    response.close()

    response_headers = dict(response.headers)
    write_json(output_dir / "request_payload.json", payload)
    write_json(output_dir / "request_headers.json", sanitize_headers(headers))
    write_json(output_dir / "response_headers.json", response_headers)
    write_json(output_dir / "events.json", events)
    (output_dir / "response_body.txt").write_text("\n".join(body_lines), encoding="utf-8")

    return ProbeSummary(
        account_id=args.account_id,
        header_mode=args.header_mode,
        transport=args.transport,
        status_code=response.status_code,
        http_version=parse_http_version_from_requests(response),
        elapsed_ms=elapsed_ms,
        timed_out=timed_out,
        event_count=len(events),
        x_oai_request_id=response.headers.get("x-oai-request-id", ""),
        output_dir=str(output_dir),
    )


def parse_curl_write_out(stdout_text: str) -> tuple[int | None, str]:
    status_code: int | None = None
    http_version = "unknown"
    for line in stdout_text.splitlines():
        if line.startswith("__STATUS__:"):
            raw = line.split(":", 1)[1].strip()
            if raw.isdigit():
                status_code = int(raw)
        elif line.startswith("__HTTP_VERSION__:"):
            raw = line.split(":", 1)[1].strip()
            if raw == "2":
                http_version = "HTTP/2.0"
            elif raw == "1.1":
                http_version = "HTTP/1.1"
            elif raw == "1.0":
                http_version = "HTTP/1.0"
            elif raw:
                http_version = raw
    return status_code, http_version


def parse_status_from_header_text(header_text: str) -> tuple[int | None, str]:
    status_code: int | None = None
    http_version = "unknown"
    for line in header_text.splitlines():
        stripped = line.strip()
        if not stripped.startswith("HTTP/"):
            continue
        parts = stripped.split()
        if len(parts) >= 2 and parts[1].isdigit():
            status_code = int(parts[1])
        if parts:
            http_version = parts[0]
    return status_code, http_version


def build_curl_http2_command(
    url: str,
    payload_path: pathlib.Path,
    headers: dict[str, str],
    headers_path: pathlib.Path,
    body_path: pathlib.Path,
    config_path: pathlib.Path,
) -> list[str]:
    auth = str(headers.get("authorization", "")).strip()
    chatgpt_account_id = str(headers.get("chatgpt-account-id", "")).strip()
    config_lines = [
        "http2",
        "no-buffer",
        "silent",
        "show-error",
        'request = "POST"',
    ]
    for key, value in headers.items():
        if key.lower() in {"authorization", "chatgpt-account-id"}:
            continue
        config_lines.append(f'header = "{key}: {value}"')
    if not auth:
        raise RuntimeError("missing authorization header")
    if not chatgpt_account_id:
        raise RuntimeError("missing chatgpt-account-id header")
    config_lines.extend([
        f'url = "{url}"',
        f'data-binary = "@{payload_path}"',
        f'output = "{body_path}"',
        f'dump-header = "{headers_path}"',
        'write-out = "__STATUS__:%{http_code}\\n__HTTP_VERSION__:%{http_version}\\n"',
    ])
    config_path.write_text("\n".join(config_lines) + "\n", encoding="utf-8")
    config_path.chmod(0o600)
    return ["curl", "--config", str(config_path), "--config", "-"]


def build_curl_sensitive_config(headers: dict[str, str]) -> str:
    config_lines: list[str] = []
    for key, value in headers.items():
        if key.lower() in {"authorization", "chatgpt-account-id"}:
            config_lines.append(f'header = "{key}: {value}"')
    if not config_lines:
        raise RuntimeError("missing sensitive curl headers")
    return "\n".join(config_lines) + "\n"


def probe_with_curl_http2(
    url: str,
    payload: dict[str, Any],
    headers: dict[str, str],
    args: argparse.Namespace,
    output_dir: pathlib.Path,
) -> ProbeSummary:
    payload_path = output_dir / "request_payload.json"
    headers_path = output_dir / "response_headers.txt"
    body_path = output_dir / "response_body.txt"
    stderr_path = output_dir / "curl_stderr.txt"
    curl_config_path = output_dir / "curl_request.conf"
    payload_path.write_text(json.dumps(payload, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    write_json(output_dir / "request_headers.json", sanitize_headers(headers))

    cmd = build_curl_http2_command(url, payload_path, headers, headers_path, body_path, curl_config_path)

    started = time.time()
    proc = subprocess.Popen(cmd, stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
    timed_out = False
    try:
        stdout_text, stderr_text = proc.communicate(input=build_curl_sensitive_config(headers), timeout=args.stream_seconds)
    except subprocess.TimeoutExpired:
        timed_out = True
        proc.kill()
        stdout_text, stderr_text = proc.communicate()
    elapsed_ms = int((time.time() - started) * 1000)

    stderr_path.write_text(stderr_text, encoding="utf-8")
    status_code, http_version = parse_curl_write_out(stdout_text)
    body_text = body_path.read_text(encoding="utf-8", errors="replace") if body_path.exists() else ""
    events = parse_sse_events_from_text(body_text, args.max_events)
    write_json(output_dir / "events.json", events)

    x_oai_request_id = ""
    if headers_path.exists():
        header_text = headers_path.read_text(encoding="utf-8", errors="replace")
        if status_code is None or http_version == "unknown":
            parsed_status, parsed_version = parse_status_from_header_text(header_text)
            if status_code is None:
                status_code = parsed_status
            if http_version == "unknown" and parsed_version != "unknown":
                http_version = parsed_version
        for line in header_text.splitlines():
            if line.lower().startswith("x-oai-request-id:"):
                x_oai_request_id = line.split(":", 1)[1].strip()
                break

    return ProbeSummary(
        account_id=args.account_id,
        header_mode=args.header_mode,
        transport=args.transport,
        status_code=status_code,
        http_version=http_version,
        elapsed_ms=elapsed_ms,
        timed_out=timed_out,
        event_count=len(events),
        x_oai_request_id=x_oai_request_id,
        output_dir=str(output_dir),
    )


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Probe OpenAI OAuth image bridge upstream via /backend-api/codex/responses",
    )
    parser.add_argument("--account-id", type=int, required=True)
    parser.add_argument("--header-mode", choices=["minimal", "gateway"], default="minimal")
    parser.add_argument("--transport", choices=["requests", "curl-http2"], default="requests")
    parser.add_argument("--output-dir")
    parser.add_argument("--timeout", type=int, default=DEFAULT_TIMEOUT)
    parser.add_argument("--stream-seconds", type=int, default=DEFAULT_STREAM_SECONDS)
    parser.add_argument("--max-events", type=int, default=DEFAULT_MAX_EVENTS)
    parser.add_argument("--prompt", default=DEFAULT_PROMPT)
    parser.add_argument("--main-model", default=DEFAULT_MAIN_MODEL)
    parser.add_argument("--tool-model", default=DEFAULT_TOOL_MODEL)
    parser.add_argument("--size", default=DEFAULT_SIZE)
    parser.add_argument("--quality", default="high")
    parser.add_argument("--background", default="opaque")
    parser.add_argument("--output-format", default="webp")
    parser.add_argument("--moderation", default="auto")
    parser.add_argument("--output-compression", type=int, default=90)
    parser.add_argument("--partial-images", type=int, default=0)
    parser.add_argument("--originator")
    parser.add_argument("--user-agent")
    parser.add_argument("--omit-user-agent", action="store_true")
    parser.add_argument("--session-id")
    parser.add_argument("--conversation-id")
    parser.add_argument("--url", default="https://chatgpt.com/backend-api/codex/responses")
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    account = load_account(args.account_id)
    payload = build_payload(args)
    headers = build_headers(account, args)
    output_dir = ensure_output_dir(args.output_dir, args.account_id, args.header_mode, args.transport)

    write_json(
        output_dir / "account_summary.json",
        {
            "id": account.get("id"),
            "name": account.get("name"),
            "type": account.get("type"),
            "platform": account.get("platform"),
        },
    )

    if args.transport == "requests":
        summary = probe_with_requests(args.url, payload, headers, args, output_dir)
    else:
        summary = probe_with_curl_http2(args.url, payload, headers, args, output_dir)

    write_json(output_dir / "summary.json", asdict(summary))
    print(json.dumps(asdict(summary), ensure_ascii=False, indent=2))
    return 0


if __name__ == "__main__":
    sys.exit(main())
