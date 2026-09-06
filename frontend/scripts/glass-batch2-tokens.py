#!/usr/bin/env python3
"""Template-only Glass UI token replacements for Phase 3 Batch 2."""

from pathlib import Path

ROOT = Path("/Users/ianshaw/Documents/code/personal/sub2api/frontend")

FILES = [
    ROOT / "src/views/admin/UsersView.vue",
    ROOT / "src/views/admin/GroupsView.vue",
    ROOT / "src/views/admin/SubscriptionsView.vue",
    ROOT / "src/views/admin/ProxiesView.vue",
    ROOT / "src/views/admin/ChannelsView.vue",
    ROOT / "src/views/admin/ChannelMonitorView.vue",
    ROOT / "src/views/admin/PluginsView.vue",
    ROOT / "src/views/admin/AnnouncementsView.vue",
    ROOT / "src/views/admin/RedeemView.vue",
    ROOT / "src/views/admin/PromoCodesView.vue",
    ROOT / "src/views/admin/UsageView.vue",
    ROOT / "src/views/admin/AuditLogView.vue",
    ROOT / "src/views/admin/RiskControlView.vue",
    ROOT / "src/features/prompt-audit/PromptAuditView.vue",
    ROOT / "src/views/admin/affiliates/AdminAffiliateRecordsTable.vue",
    ROOT / "src/views/admin/orders/AdminPaymentDashboardView.vue",
    ROOT / "src/views/admin/orders/AdminOrdersView.vue",
    ROOT / "src/views/admin/orders/AdminInvoiceApplicationsView.vue",
    ROOT / "src/views/admin/orders/AdminPaymentPlansView.vue",
    ROOT / "src/views/admin/TicketsView.vue",
]

# Longest-first replacements applied only to the template section.
REPLACEMENTS = [
    (
        "text-gray-700 hover:bg-gray-100 dark:text-gray-300 dark:hover:bg-dark-700",
        "text-foreground hover:bg-surface-2",
    ),
    (
        "text-gray-700 hover:bg-gray-100 dark:text-gray-200 dark:hover:bg-dark-700",
        "text-foreground hover:bg-surface-2",
    ),
    (
        "text-sm text-gray-700 hover:bg-gray-100 dark:text-gray-200 dark:hover:bg-dark-700",
        "text-sm text-foreground hover:bg-surface-2",
    ),
    (
        "rounded-lg border border-gray-200 bg-white py-1 shadow-lg dark:border-dark-600 dark:bg-dark-800",
        "rounded-lg border border-line bg-surface py-1 shadow-lg",
    ),
    (
        "rounded-lg border border-gray-200 bg-white py-1 shadow-lg dark:border-dark-500 dark:bg-dark-700",
        "rounded-lg border border-line bg-surface py-1 shadow-lg",
    ),
    (
        "rounded-lg border border-gray-200 bg-white shadow-lg dark:border-dark-700 dark:bg-dark-800",
        "rounded-lg border border-line bg-surface shadow-lg",
    ),
    (
        "origin-top-right rounded-lg border border-gray-200 bg-white shadow-lg dark:border-dark-700 dark:bg-dark-800",
        "origin-top-right rounded-lg border border-line bg-surface shadow-lg",
    ),
    (
        "border-gray-200 bg-white py-1 shadow-lg dark:border-dark-600 dark:bg-dark-800",
        "border-line bg-surface py-1 shadow-lg",
    ),
    (
        "text-gray-900 underline decoration-dashed decoration-gray-300 underline-offset-4 transition-colors hover:text-primary-600 dark:text-white dark:decoration-dark-500 dark:hover:text-primary-400",
        "text-gray-900 underline decoration-dashed decoration-line underline-offset-4 transition-colors hover:text-accent dark:text-white",
    ),
    ("text-gray-500 dark:text-gray-400", "text-muted"),
    ("text-gray-500 dark:text-dark-400", "text-muted"),
    ("text-gray-500 dark:text-dark-300", "text-muted"),
    ("text-gray-600 dark:text-gray-400", "text-muted"),
    ("text-gray-600 dark:text-gray-300", "text-muted"),
    ("text-gray-700 dark:text-gray-300", "text-foreground"),
    ("text-gray-700 dark:text-gray-200", "text-foreground"),
    ("text-gray-800 dark:text-gray-200", "text-foreground"),
    ("text-gray-900 dark:text-white", "text-foreground"),
    ("text-gray-900 dark:text-gray-100", "text-foreground"),
    ("text-gray-950 dark:text-white", "text-foreground"),
    ("text-gray-400 dark:text-gray-500", "text-muted"),
    ("text-gray-400 hover:text-gray-600 dark:hover:text-gray-300", "text-muted hover:text-foreground"),
    ("hover:bg-gray-100 dark:hover:bg-dark-700", "hover:bg-surface-2"),
    ("hover:bg-gray-100 dark:hover:bg-dark-600", "hover:bg-surface-2"),
    ("hover:bg-gray-50 dark:hover:bg-dark-700", "hover:bg-surface-2"),
    ("border-gray-200 dark:border-dark-700", "border-line"),
    ("border-gray-200 dark:border-dark-600", "border-line"),
    ("border-gray-100 dark:border-dark-700", "border-line"),
    ("border-gray-100 dark:border-dark-700/50", "border-line"),
    ("border-gray-100 dark:border-dark-700/80", "border-line"),
    ("border-t border-gray-200 dark:border-dark-700", "border-t border-line"),
    ("border-t border-gray-200 dark:border-dark-600", "border-t border-line"),
    ("border-b border-gray-200 dark:border-dark-700", "border-b border-line"),
    ("border-b border-gray-200 pb-5 dark:border-dark-700", "border-b border-line pb-5"),
    ("border-b border-gray-100 pt-4 dark:border-dark-700", "border-b border-line pt-4"),
    ("bg-white dark:bg-dark-800", "bg-surface"),
    ("bg-white/95", "bg-surface/95"),
    ("dark:bg-dark-900/95", ""),
    ("text-primary-600 dark:text-primary-400", "text-accent"),
    ("text-primary-500", "text-accent"),
    ("hover:text-primary-600 dark:hover:text-primary-400", "hover:text-accent"),
    ("hover:text-primary-600 dark:text-white", "hover:text-accent"),
    ("hover:text-primary-600", "hover:text-accent"),
    ("border-primary-500", "border-accent"),
    ("text-primary-600", "text-accent"),
    ("bg-primary-100 dark:bg-primary-900/30", "bg-accent/15"),
    ("text-primary-700 dark:text-primary-300", "text-accent"),
    ("bg-primary-50 hover:text-primary-600 dark:text-dark-200 dark:hover:bg-primary-900/30 dark:hover:text-primary-400", "bg-accent/10 hover:text-accent"),
    ("hover:bg-primary-50 hover:text-primary-600 dark:text-dark-200 dark:hover:bg-primary-900/30 dark:hover:text-primary-400", "hover:bg-accent/10 hover:text-accent"),
    ("focus:ring-primary-500", "focus:ring-accent"),
    ("text-primary-600 focus:ring-primary-500", "text-accent focus:ring-accent"),
    ("border-gray-300 text-primary-600 focus:ring-primary-500", "border-line text-accent focus:ring-accent"),
    ("bg-gray-100 dark:bg-dark-700", "bg-surface-2"),
    ("bg-gray-100 px-2 py-1 dark:bg-dark-700", "bg-surface-2 px-2 py-1"),
    ("bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-300", "bg-surface-2 text-muted"),
    ("bg-gray-100 text-gray-800 dark:bg-dark-600 dark:text-gray-300", "bg-surface-2 text-foreground"),
    ("bg-gray-100 px-2 py-0.5 text-gray-600 dark:bg-dark-700 dark:text-gray-300", "bg-surface-2 px-2 py-0.5 text-muted"),
    ("text-gray-400", "text-muted"),
    ("text-gray-500", "text-muted"),
    ("text-gray-600", "text-muted"),
    ("text-gray-700", "text-foreground"),
    ("text-gray-800", "text-foreground"),
    ("text-gray-900", "text-foreground"),
    ("text-gray-300 dark:text-dark-700", "text-muted"),
    ("border-gray-200", "border-line"),
    ("border-gray-100", "border-line"),
    ("border-gray-300", "border-line"),
    ("dark:text-white", ""),
    ("dark:text-gray-300", ""),
    ("dark:text-gray-400", ""),
    ("dark:text-gray-200", ""),
    ("dark:text-gray-100", ""),
    ("dark:hover:bg-dark-700", ""),
    ("dark:hover:bg-dark-600", ""),
    ("dark:hover:text-white", ""),
    ("dark:border-dark-700", ""),
    ("dark:border-dark-600", ""),
    ("dark:bg-dark-800", ""),
    ("dark:bg-dark-700", ""),
]

BTN_REPLACEMENTS = [
    ("btn btn-primary btn-sm", "btn-glass-primary text-sm"),
    ("btn btn-secondary btn-sm", "btn-glass-secondary text-sm"),
    ("btn btn-danger btn-sm", "btn-glass-secondary text-sm text-red-600"),
    ("btn btn-primary", "btn-glass-primary"),
    ("btn btn-danger", "btn-glass-secondary text-red-600"),
]

PROTECTED_BTN_SECONDARY = "btn btn-secondary shrink-0 whitespace-nowrap"


def split_template(src: str) -> tuple[str, str]:
    idx = src.find("<script")
    if idx == -1:
        return src, ""
    return src[:idx], src[idx:]


def collapse_spaces(s: str) -> str:
    return " ".join(s.split())


def apply_replacements(template: str, path: Path) -> str:
    protect = path.name == "GroupsView.vue"
    if protect:
        template = template.replace(PROTECTED_BTN_SECONDARY, "<<<KEEP_GROUP_BTN>>>")

    for old, new in REPLACEMENTS:
        template = template.replace(old, new)

    for old, new in BTN_REPLACEMENTS:
        template = template.replace(old, new)

    # Remaining secondary buttons, except protected GroupsView toolbar.
    template = template.replace("btn btn-secondary", "btn-glass-secondary")

    if protect:
        template = template.replace("<<<KEEP_GROUP_BTN>>>", PROTECTED_BTN_SECONDARY)

    return template


def main() -> None:
    for path in FILES:
        src = path.read_text()
        template, rest = split_template(src)
        updated = apply_replacements(template, path)
        if updated != template:
            path.write_text(updated + rest)
            print(f"updated {path.relative_to(ROOT)}")
        else:
            print(f"unchanged {path.relative_to(ROOT)}")


if __name__ == "__main__":
    main()
