#!/usr/bin/env python3
"""Finish Phase 3 cleanup: primary-* tokens, dark: in TS, legacy card class in views."""

from __future__ import annotations

import re
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
SRC = ROOT / "src"

PRIMARY_REPLACEMENTS: list[tuple[str, str]] = [
    ("hover:bg-primary-50/50", "hover:bg-[color-mix(in_oklch,var(--accent)_12%,transparent)]"),
    ("bg-primary-50/80", "bg-[color-mix(in_oklch,var(--accent)_12%,transparent)]"),
    ("bg-primary-50/70", "bg-[color-mix(in_oklch,var(--accent)_12%,transparent)]"),
    ("bg-primary-50/50", "bg-[color-mix(in_oklch,var(--accent)_12%,transparent)]"),
    ("bg-primary-500/20", "bg-[color-mix(in_oklch,var(--accent)_20%,transparent)]"),
    ("bg-primary-500/10", "bg-[color-mix(in_oklch,var(--accent)_10%,transparent)]"),
    ("hover:bg-primary-700", "hover:opacity-90"),
    ("hover:bg-primary-600", "hover:opacity-90"),
    ("active:bg-primary-800", "active:opacity-80"),
    ("active:bg-primary-700", "active:opacity-80"),
    ("focus:ring-offset-primary-50", "focus:ring-offset-background"),
    ("focus:ring-primary-500", "focus:ring-accent"),
    ("decoration-primary-400", "decoration-accent"),
    ("hover:decoration-primary-400", "hover:decoration-accent"),
    ("hover:text-primary-800", "hover:text-accent"),
    ("hover:border-primary-500", "hover:border-accent"),
    ("hover:bg-primary-50", "hover:bg-[color-mix(in_oklch,var(--accent)_12%,transparent)]"),
    ("border-b-primary-500", "border-b-accent"),
    ("border-primary-600", "border-accent"),
    ("border-primary-500", "border-accent"),
    ("ring-primary-500", "ring-accent"),
    ("accent-primary-600", "accent-accent"),
    ("text-primary-900", "text-accent"),
    ("text-primary-800", "text-accent"),
    ("text-primary-700", "text-accent"),
    ("text-primary-600", "text-accent"),
    ("text-primary-500", "text-accent"),
    ("text-primary-400", "text-accent"),
    ("text-primary-300", "text-accent"),
    ("text-primary-200", "text-white/70"),
    ("text-primary-100", "text-white/90"),
    ("bg-primary-900", "bg-accent"),
    ("bg-primary-800", "bg-accent"),
    ("bg-primary-700", "bg-accent"),
    ("bg-primary-600", "bg-accent"),
    ("bg-primary-500", "bg-accent"),
    ("bg-primary-100", "bg-[color-mix(in_oklch,var(--accent)_16%,transparent)]"),
    ("bg-primary-50", "bg-[color-mix(in_oklch,var(--accent)_12%,transparent)]"),
    (
        "from-primary-400 to-primary-500",
        "from-[var(--accent)] to-[color-mix(in_oklch,var(--accent)_80%,black)]",
    ),
    (
        "from-primary-500 to-primary-600",
        "from-[var(--accent)] to-[color-mix(in_oklch,var(--accent)_80%,black)]",
    ),
]

COLOR_PATTERN = re.compile(
    r"bg-([\w]+)-100 text-\1-700 hover:bg-\1-200(?:\s+dark:[^\s'\"]+)*"
)
SLATE_PATTERN = re.compile(
    r"bg-slate-100 text-slate-700 hover:bg-slate-200(?:\s+dark:[^\s'\"]+)*"
)
DARK_SEGMENT = re.compile(r"\s+dark:[^\s'\"]+")


def migrate_preset_colors(text: str) -> str:
    text = SLATE_PATTERN.sub("bg-zinc-500/15 text-zinc-600 hover:bg-zinc-500/25", text)
    text = COLOR_PATTERN.sub(r"bg-\1-500/15 text-\1-700 hover:bg-\1-500/25", text)
    return DARK_SEGMENT.sub("", text)


def migrate_primary(text: str) -> str:
    for old, new in PRIMARY_REPLACEMENTS:
        text = text.replace(old, new)
    return text


def migrate_card_in_views(text: str) -> str:
    # Remove redundant legacy `card` when glass-card is already present.
    text = re.sub(r'\bclass="card glass-card', 'class="glass-card', text)
    text = re.sub(r"\bclass='card glass-card", "class='glass-card", text)
    # Standalone legacy card → glass-card
    text = re.sub(r'\bclass="card\b', 'class="glass-card', text)
    text = re.sub(r"\bclass='card\b", "class='glass-card", text)
    return text


def migrate_types_ts(text: str) -> str:
    replacements = {
        "bg-orange-100 text-orange-700 dark:bg-orange-900/30 dark:text-orange-400": "bg-orange-500/15 text-orange-600",
        "bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400": "bg-emerald-500/15 text-emerald-600",
        "bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400": "bg-blue-500/15 text-blue-600",
        "bg-purple-100 text-purple-700 dark:bg-purple-900/30 dark:text-purple-400": "bg-purple-500/15 text-purple-600",
        "bg-slate-100 text-slate-700 dark:bg-slate-800 dark:text-slate-300": "bg-zinc-500/15 text-zinc-600",
        "bg-pink-100 text-pink-700 dark:bg-pink-900/30 dark:text-pink-400": "bg-pink-500/15 text-pink-600",
        "bg-indigo-100 text-indigo-700 dark:bg-indigo-900/30 dark:text-indigo-400": "bg-indigo-500/15 text-indigo-600",
        "bg-teal-100 text-teal-700 dark:bg-teal-900/30 dark:text-teal-400": "bg-teal-500/15 text-teal-600",
        "bg-gray-100 text-gray-700 dark:bg-gray-900/30 dark:text-gray-400": "badge-tone-muted",
        "text-orange-700 dark:text-orange-400": "text-orange-600",
        "text-emerald-700 dark:text-emerald-400": "text-emerald-600",
        "text-blue-700 dark:text-blue-400": "text-blue-600",
        "text-purple-700 dark:text-purple-400": "text-purple-600",
        "text-slate-700 dark:text-slate-300": "text-zinc-600",
        "text-pink-700 dark:text-pink-400": "text-pink-600",
        "text-indigo-700 dark:text-indigo-400": "text-indigo-600",
        "text-teal-700 dark:text-teal-400": "text-teal-600",
    }
    for old, new in replacements.items():
        text = text.replace(old, new)
    return text


def process_file(path: Path) -> bool:
    original = path.read_text(encoding="utf-8")
    updated = original

    if path.suffix == ".vue":
        updated = migrate_primary(updated)
        if "views" in path.parts:
            updated = migrate_card_in_views(updated)
    elif path.name == "useModelWhitelist.ts":
        updated = migrate_preset_colors(updated)
    elif path.name == "types.ts" and "channel" in path.parts:
        updated = migrate_types_ts(updated)

    if updated != original:
        path.write_text(updated, encoding="utf-8")
        return True
    return False


def main() -> None:
    changed: list[Path] = []
    for path in sorted(SRC.rglob("*")):
        if path.suffix not in {".vue", ".ts"}:
            continue
        if path.name.endswith(".spec.ts"):
            continue
        if process_file(path):
            changed.append(path)
    print(f"Updated {len(changed)} files")
    for path in changed[:20]:
        print(f"  {path.relative_to(ROOT)}")
    if len(changed) > 20:
        print(f"  ... and {len(changed) - 20} more")


if __name__ == "__main__":
    main()
