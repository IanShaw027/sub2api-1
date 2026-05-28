#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
BACKEND_DIR="${REPO_ROOT}/backend"
FRONTEND_DIR="${REPO_ROOT}/frontend"

hash_file_manifest() {
    if [ "$#" -eq 0 ]; then
        printf 'empty\n' | sha256sum | awk '{print $1}'
        return 0
    fi

    local file rel
    (
        for file in "$@"; do
            rel="${file#${REPO_ROOT}/}"
            printf 'path=%s\n' "$rel"
            sha256sum "$file"
        done
    ) | sha256sum | awk '{print $1}'
}

collect_source_files() {
    find "$BACKEND_DIR" "$FRONTEND_DIR" \
        \( \
            -path "$BACKEND_DIR/internal/web/dist" -o \
            -path "$BACKEND_DIR/internal/web/dist/*" -o \
            -path "$BACKEND_DIR/data" -o \
            -path "$BACKEND_DIR/data/*" -o \
            -path "$BACKEND_DIR/.gocache" -o \
            -path "$BACKEND_DIR/.gocache/*" -o \
            -path "$FRONTEND_DIR/node_modules" -o \
            -path "$FRONTEND_DIR/node_modules/*" -o \
            -path "$FRONTEND_DIR/dist" -o \
            -path "$FRONTEND_DIR/dist/*" -o \
            -path "$FRONTEND_DIR/coverage" -o \
            -path "$FRONTEND_DIR/coverage/*" \
        \) -prune -o -type f -print | LC_ALL=C sort
}

compute_source_hash() {
    local files=()
    local file
    while IFS= read -r file; do
        files+=("$file")
    done < <(collect_source_files)
    if [ "${#files[@]}" -eq 0 ]; then
        echo "No backend/frontend source files found; cannot compute source hash" >&2
        return 1
    fi
    hash_file_manifest "${files[@]}"
}

collect_frontend_dist_files() {
    if [ ! -d "$BACKEND_DIR/internal/web/dist" ]; then
        return 0
    fi

    find "$BACKEND_DIR/internal/web/dist" -type f ! -name ".keep" -print | LC_ALL=C sort
}

compute_frontend_dist_hash() {
    local files=()
    local file
    while IFS= read -r file; do
        files+=("$file")
    done < <(collect_frontend_dist_files)
    if [ "${#files[@]}" -eq 0 ]; then
        echo "No frontend dist files found: $BACKEND_DIR/internal/web/dist" >&2
        return 1
    fi
    hash_file_manifest "${files[@]}"
}

get_git_dirty_state() {
    if [ -n "$(git -C "$REPO_ROOT" status --porcelain --untracked-files=all -- backend frontend)" ]; then
        echo "dirty"
    else
        echo "clean"
    fi
}

DIRTY="$(get_git_dirty_state)"
SOURCE_HASH="$(compute_source_hash)"
FRONTEND_DIST_HASH="$(compute_frontend_dist_hash)"

printf 'export DIRTY=%s\n' "$DIRTY"
printf 'export SOURCE_HASH=%s\n' "$SOURCE_HASH"
printf 'export FRONTEND_DIST_HASH=%s\n' "$FRONTEND_DIST_HASH"
