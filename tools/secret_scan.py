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
    "example",
    "placeholder",
    "dummy",
    "mock",
    "fake",
    "your",
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
        "configured secret",
        re.compile(
            r"(?i)\b(?:(?:aws_)?secret_access_key|(?:database|db|jwt)_?(?:password|secret)|"
            r"download_signing_secret)"
            r"\s*[=:]\s*[\"']?(?P<secret>[A-Za-z0-9_./+!=-]{16,})[\"']?\s*[,}]?\s*(?:#.*)?$"
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
        for name, pattern in PATTERNS:
            for match in pattern.finditer(line):
                value = match.groupdict().get("secret") or match.group(0)
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
