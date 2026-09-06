#!/usr/bin/env python3
"""Migrate dark: and gray-* Tailwind utilities to Glass semantic tokens."""

from __future__ import annotations

import re
import sys
from pathlib import Path

SCOPE_DIRS = [
    "src/components/common",
    "src/components/layout",
    "src/components/auth",
    "src/components/payment",
    "src/components/user",
    "src/components/charts",
    "src/components/channels",
    "src/components/keys",
    "src/components/tickets",
    "src/components/modelPlaza",
    "src/features",
]

# Payment brand button class names to preserve
PAYMENT_BRAND_PATTERNS = [
    r"bg-\[#",
    r"border-\[#",
    r"text-\[#",
    r"from-\[#",
    r"to-\[#",
    r"ring-\[#",
    r"hover:bg-\[#",
    r"hover:border-\[#",
]

DARK_VARIANT_RE = re.compile(
    r"\s+dark:(?:\[[^\]]+\]|[^\s'\"`,]+)"
)

# Ordered replacements: (pattern, replacement)
# Process longer/more-specific patterns first within each category.
GRAY_REPLACEMENTS: list[tuple[re.Pattern[str], str]] = []

def _add_gray(pattern: str, replacement: str) -> None:
    GRAY_REPLACEMENTS.append((re.compile(pattern), replacement))

PREFIXES = ["", "hover:", "focus:", "active:", "disabled:", "group-hover:"]

for prefix in PREFIXES:
    p = prefix.replace(":", r"\:") if prefix else ""
    # text
    _add_gray(rf"\b{p}text-gray-900\b", f"{prefix}text-foreground")
    _add_gray(rf"\b{p}text-gray-800\b", f"{prefix}text-foreground")
    _add_gray(rf"\b{p}text-gray-700\b", f"{prefix}text-foreground")
    for shade in ("600", "500", "400", "300", "200", "100"):
        _add_gray(rf"\b{p}text-gray-{shade}\b", f"{prefix}text-muted")
    # bg
    for shade in ("50", "100", "200"):
        _add_gray(rf"\b{p}bg-gray-{shade}\b", f"{prefix}bg-surface-2")
    for shade in ("300", "400", "500", "600", "700", "800", "900"):
        _add_gray(rf"\b{p}bg-gray-{shade}\b", f"{prefix}bg-surface-3")
    # border
    for shade in ("100", "200", "300", "400", "500", "600", "700", "800", "900"):
        _add_gray(rf"\b{p}border-gray-{shade}\b", f"{prefix}border-line")
    # divide
    for shade in ("100", "200", "300", "400", "500", "600", "700", "800", "900"):
        _add_gray(rf"\b{p}divide-gray-{shade}\b", f"{prefix}divide-line")
    # ring
    for shade in ("100", "200", "300", "400", "500", "600", "700", "800", "900"):
        _add_gray(rf"\b{p}ring-gray-{shade}\b", f"{prefix}ring-line")
    # placeholder
    for shade in ("400", "500"):
        _add_gray(rf"\b{p}placeholder-gray-{shade}\b", f"{prefix}placeholder-muted")

# dark-* standalone color scale (not dark: variant)
for prefix in PREFIXES:
    p = prefix.replace(":", r"\:") if prefix else ""
    for shade in ("100", "200"):
        _add_gray(rf"\b{p}text-dark-{shade}\b", f"{prefix}text-foreground")
    for shade in ("300", "400", "500", "600"):
        _add_gray(rf"\b{p}text-dark-{shade}\b", f"{prefix}text-muted")
    for shade in ("50", "100", "200"):
        _add_gray(rf"\b{p}bg-dark-{shade}\b", f"{prefix}bg-surface-2")
    for shade in ("300", "400", "500", "600", "700", "800", "900"):
        _add_gray(rf"\b{p}bg-dark-{shade}\b", f"{prefix}bg-surface-3")
    # with opacity
    _add_gray(rf"\b{p}bg-dark-800/50\b", f"{prefix}bg-surface-3/50")
    _add_gray(rf"\b{p}bg-dark-800/60\b", f"{prefix}bg-surface-3/60")
    _add_gray(rf"\b{p}bg-dark-700/60\b", f"{prefix}bg-surface-3/60")
    _add_gray(rf"\b{p}border-dark-700/60\b", f"{prefix}border-line/60")
    for shade in ("400", "500", "600", "700", "800"):
        _add_gray(rf"\b{p}border-dark-{shade}\b", f"{prefix}border-line")
    for shade in ("400", "500", "600", "700", "800"):
        _add_gray(rf"\b{p}ring-dark-{shade}\b", f"{prefix}ring-line")
    _add_gray(rf"\b{p}!bg-dark-800\b", f"{prefix}!bg-surface-3")

# primary → accent (non-payment contexts handled per-file; apply safe text/focus replacements)
PRIMARY_REPLACEMENTS: list[tuple[re.Pattern[str], str]] = []
for prefix in ["", "hover:", "focus:", "group-hover:"]:
    p = prefix.replace(":", r"\:") if prefix else ""
    PRIMARY_REPLACEMENTS.append((re.compile(rf"\b{p}text-primary-600\b"), f"{prefix}text-accent"))
    PRIMARY_REPLACEMENTS.append((re.compile(rf"\b{p}text-primary-500\b"), f"{prefix}text-accent"))
    PRIMARY_REPLACEMENTS.append((re.compile(rf"\b{p}text-primary-400\b"), f"{prefix}text-accent"))
    PRIMARY_REPLACEMENTS.append((re.compile(rf"\b{p}text-primary-300\b"), f"{prefix}text-accent"))
    PRIMARY_REPLACEMENTS.append((re.compile(rf"\b{p}focus:ring-primary-500\b"), f"{prefix}focus:ring-accent"))
    PRIMARY_REPLACEMENTS.append((re.compile(rf"\b{p}focus:ring-primary-400\b"), f"{prefix}focus:ring-accent"))
    PRIMARY_REPLACEMENTS.append((re.compile(rf"\b{p}ring-primary-500\b"), f"{prefix}ring-accent"))
    PRIMARY_REPLACEMENTS.append((re.compile(rf"\b{p}border-primary-500\b"), f"{prefix}border-accent"))
    PRIMARY_REPLACEMENTS.append((re.compile(rf"\b{p}border-primary-400\b"), f"{prefix}border-accent"))
    PRIMARY_REPLACEMENTS.append((re.compile(rf"\b{p}decoration-primary-400\b"), f"{prefix}decoration-accent"))

# Common combined patterns after dark removal
COMBINED_REPLACEMENTS: list[tuple[re.Pattern[str], str]] = [
    (re.compile(r"\bbg-white\b"), "bg-surface"),
    (re.compile(r"\bhover:bg-white\b"), "hover:bg-surface"),
    (re.compile(r"\bfocus:ring-offset-white\b"), "focus:ring-offset-surface"),
    (re.compile(r"\bfocus:ring-offset-gray-50\b"), "focus:ring-offset-surface"),
]


def is_payment_brand_line(line: str) -> bool:
    return any(re.search(p, line) for p in PAYMENT_BRAND_PATTERNS)


def migrate_content(content: str, filepath: Path) -> str:
    is_payment = "components/payment/" in str(filepath).replace("\\", "/")

    lines = content.split("\n")
    result_lines = []
    for line in lines:
        if is_payment and is_payment_brand_line(line):
            # Still remove dark: variants but skip gray/primary on brand lines
            line = DARK_VARIANT_RE.sub("", line)
            result_lines.append(line)
            continue

        # Remove all dark: variants
        line = DARK_VARIANT_RE.sub("", line)

        # Apply gray replacements
        for pattern, replacement in GRAY_REPLACEMENTS:
            line = pattern.sub(replacement, line)

        # Apply combined replacements
        for pattern, replacement in COMBINED_REPLACEMENTS:
            line = pattern.sub(replacement, line)

        # Apply primary → accent (skip payment dir entirely for primary button colors)
        if not is_payment:
            for pattern, replacement in PRIMARY_REPLACEMENTS:
                line = pattern.sub(replacement, line)

        # Collapse double spaces in class strings
        line = re.sub(r"  +", " ", line)
        line = re.sub(r" \}", " }", line)

        result_lines.append(line)

    return "\n".join(result_lines)


def collect_vue_files(root: Path) -> list[Path]:
    files: list[Path] = []
    for scope in SCOPE_DIRS:
        scope_path = root / scope
        if scope_path.exists():
            files.extend(sorted(scope_path.rglob("*.vue")))
    return files


def main() -> int:
    root = Path(__file__).resolve().parent.parent
    files = collect_vue_files(root)
    changed = 0
    for f in files:
        original = f.read_text(encoding="utf-8")
        migrated = migrate_content(original, f)
        if migrated != original:
            f.write_text(migrated, encoding="utf-8")
            changed += 1
            print(f"  updated: {f.relative_to(root)}")
    print(f"\nDone: {changed}/{len(files)} files updated")
    return 0


if __name__ == "__main__":
    sys.exit(main())
