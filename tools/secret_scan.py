#!/usr/bin/env python3
"""Lightweight high-confidence secret scan for local pre-commit checks."""

from __future__ import annotations

import os
import re
import subprocess
import sys
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]

SKIP_PARTS = {
    ".git",
    "node_modules",
    "dist",
    "coverage",
    "__pycache__",
    ".pnpm-store",
}

ALLOW_VALUE_FRAGMENTS = (
    "change",
    "development",
    "example",
    "placeholder",
    "dummy",
    "local",
    "mock",
    "fake",
    "replace",
    "sample",
    "test",
    "your",
)

YAML_CONFIGURED_SECRET_PATTERN = re.compile(
    r"(?i)^\s*(?:-\s*)?[\"']?(?:(?:aws_)?secret_access_key|"
    r"(?:database|db|jwt|redis)_?(?:password|secret)|"
    r"download_signing_secret|client_secret|access_token|refresh_token|"
    r"webhook_secret|merchant_private_key|payment_(?:secret|api_key|private_key)|"
    r"(?:stripe|airwallex|paypal|wechat|alipay)_"
    r"(?:api_key|secret(?:_key)?|client_secret|webhook_secret|private_key))"
    r"[\"']?"
    r"\s*:\s*(?:"
    r"(?P<quote>[\"'])(?P<secret_quoted>[A-Za-z0-9_./+!=-]{16,})(?P=quote)"
    r"|(?P<secret_unquoted>[A-Za-z0-9_./+!=-]{16,})"
    r")\s*[,}]?\s*(?:#.*)?$"
)

PATTERNS: tuple[tuple[str, re.Pattern[str]], ...] = (
    ("private key block", re.compile(r"-----BEGIN (?:RSA |EC |OPENSSH |DSA |)?PRIVATE KEY-----")),
    ("GitHub token", re.compile(r"\bgh[opsu]_[A-Za-z0-9_]{36,}\b")),
    ("GitHub fine-grained PAT", re.compile(r"\bgithub_pat_[A-Za-z0-9_]{82,}\b")),
    ("Stripe live secret", re.compile(r"\b(?:sk|rk)_live_[A-Za-z0-9]{24,}\b")),
    ("Stripe webhook secret", re.compile(r"\bwhsec_[A-Za-z0-9]{24,}\b")),
    ("OpenAI-style API key", re.compile(r"\bsk-(?:proj-|ant-api03-)?[A-Za-z0-9_-]{24,}\b")),
    ("Google API key", re.compile(r"\bAIza[0-9A-Za-z_-]{32,}\b")),
    ("AWS access key ID", re.compile(r"\b(?:AKIA|ASIA)[A-Z0-9]{16}\b")),
    ("GitLab token", re.compile(r"\bglpat-[A-Za-z0-9_-]{20,}\b")),
    ("Hugging Face token", re.compile(r"\bhf_[A-Za-z0-9]{30,}\b")),
    ("Slack token", re.compile(r"\bxox[baprs]-[A-Za-z0-9-]{20,}\b")),
    ("Google OAuth access token", re.compile(r"\bya29\.[A-Za-z0-9_-]{30,}\b")),
    ("Google OAuth refresh token", re.compile(r"\b1//[A-Za-z0-9_-]{30,}\b")),
    ("npm token", re.compile(r"\bnpm_[A-Za-z0-9]{30,}\b")),
    (
        "SendGrid API key",
        re.compile(r"\bSG\.[A-Za-z0-9_-]{20,}\.[A-Za-z0-9_-]{20,}\b"),
    ),
    (
        "JWT",
        re.compile(
            r"\b(?P<secret>eyJ[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{20,})\b"
        ),
    ),
    (
        "database URL password",
        re.compile(
            r"\b(?:postgres(?:ql)?|mysql|mariadb|mongodb(?:\+srv)?)://"
            r"[^\s:/@]+:(?P<secret>[^\s/@]{12,})@"
        ),
    ),
    (
        "Redis URL password",
        re.compile(
            r"\brediss?://[^/\s:@]*:(?P<secret>[^/\s@]{12,})@"
        ),
    ),
    (
        "configured secret",
        YAML_CONFIGURED_SECRET_PATTERN,
    ),
    (
        "configured secret",
        re.compile(
            r"^\s*(?:export\s+)?(?:(?:AWS_)?SECRET_ACCESS_KEY|"
            r"(?:DATABASE|DB|JWT|REDIS)_(?:PASSWORD|SECRET)|"
            r"DOWNLOAD_SIGNING_SECRET|CLIENT_SECRET|ACCESS_TOKEN|REFRESH_TOKEN|"
            r"WEBHOOK_SECRET|MERCHANT_PRIVATE_KEY|PAYMENT_(?:SECRET|API_KEY|PRIVATE_KEY)|"
            r"(?:STRIPE|AIRWALLEX|PAYPAL|WECHAT|ALIPAY)_"
            r"(?:API_KEY|SECRET(?:_KEY)?|CLIENT_SECRET|WEBHOOK_SECRET|PRIVATE_KEY))"
            r"\s*=\s*[\"']?(?P<secret>[A-Za-z0-9_./+!=-]{16,})[\"']?\s*(?:#.*)?$"
        ),
    ),
)


def git_files() -> list[Path]:
    cmd = ["git", "ls-files", "--cached", "--others", "--exclude-standard", "-z"]
    raw = subprocess.check_output(cmd, cwd=ROOT)
    files: list[Path] = []
    for item in raw.split(b"\0"):
        if not item:
            continue
        rel = Path(item.decode("utf-8", errors="replace"))
        if any(part in SKIP_PARTS for part in rel.parts):
            continue
        if is_test_fixture_path(rel):
            continue
        path = ROOT / rel
        if path.is_file():
            files.append(rel)
    return files


def is_test_fixture_path(rel: Path) -> bool:
    name = rel.name
    return (
        "testdata" in rel.parts
        or "fixtures" in rel.parts
        or "__fixtures__" in rel.parts
        or name.endswith("_test.go")
        or name.endswith("_test.py")
        or name.startswith("test_")
        or name.endswith(".spec.ts")
        or name.endswith(".spec.tsx")
        or name.endswith(".test.ts")
        or name.endswith(".test.tsx")
    )


def is_probably_placeholder(value: str) -> bool:
    lowered = value.lower()
    prefixes = (
        "sk-proj-",
        "sk-ant-api03-",
        "sk-",
        "ghp_",
        "gho_",
        "ghu_",
        "ghs_",
        "ghr_",
        "sk_live_",
        "rk_live_",
        "whsec_",
        "aiza",
    )
    core = lowered
    for prefix in prefixes:
        if core.startswith(prefix):
            core = core[len(prefix):]
            break
    compact = re.sub(r"[^a-z0-9]", "", core)
    if not compact:
        return True
    if compact in ALLOW_VALUE_FRAGMENTS:
        return True
    if any(compact.startswith(fragment) for fragment in ALLOW_VALUE_FRAGMENTS):
        return True
    if any(compact == fragment * (len(compact) // len(fragment)) for fragment in ALLOW_VALUE_FRAGMENTS if len(compact) % len(fragment) == 0):
        return True
    if set(compact) <= {"x"} or set(compact) <= {"0"}:
        return True
    if len(set(compact)) <= 2 and len(compact) >= 16:
        return True
    return False


def scan_file(rel: Path) -> list[str]:
    path = ROOT / rel
    try:
        data = path.read_bytes()
    except OSError as exc:
        return [f"{rel}:0: read failed: {exc}"]
    if b"\0" in data:
        return []
    try:
        text = data.decode("utf-8")
    except UnicodeDecodeError:
        text = data.decode("utf-8", errors="ignore")

    findings: list[str] = []
    for lineno, line in enumerate(text.splitlines(), 1):
        seen_values: set[str] = set()
        for name, pattern in PATTERNS:
            if (
                pattern is YAML_CONFIGURED_SECRET_PATTERN
                and rel.suffix.lower() not in {".yaml", ".yml", ".json", ".conf", ".config"}
            ):
                continue
            for match in pattern.finditer(line):
                groups = match.groupdict()
                value = (
                    groups.get("secret")
                    or groups.get("secret_quoted")
                    or groups.get("secret_unquoted")
                    or match.group(0)
                )
                if value in seen_values:
                    continue
                seen_values.add(value)
                if is_probably_placeholder(value):
                    continue
                findings.append(f"{rel}:{lineno}: possible {name}")
    return findings


def main() -> int:
    os.chdir(ROOT)
    findings: list[str] = []
    for rel in git_files():
        findings.extend(scan_file(rel))

    if findings:
        print("Secret scan failed:", file=sys.stderr)
        for finding in findings:
            print(f"  {finding}", file=sys.stderr)
        return 1

    print("Secret scan passed.")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
