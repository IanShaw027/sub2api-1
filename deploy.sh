#!/bin/bash
set -e  # 遇到错误立即退出

REPO_ROOT="${REPO_ROOT:-/opt/sub2api}"
BACKEND_DIR="$REPO_ROOT/backend"
FRONTEND_DIR="$REPO_ROOT/frontend"
BINARY_PATH="$REPO_ROOT/sub2api"
NEW_BINARY_PATH="${NEW_BINARY_PATH:-$BINARY_PATH.new}"
SERVICE_NAME="${SERVICE_NAME:-sub2api}"
CACHE_DIR="${DEPLOY_CACHE_DIR:-$REPO_ROOT/deploy/.cache}"
CACHE_FILE="${DEPLOY_CACHE_FILE:-$CACHE_DIR/deploy-state.env}"
BINARY_BACKUP_DIR="${DEPLOY_BINARY_BACKUP_DIR:-$REPO_ROOT/deploy/.backup/binaries}"

require_command() {
    local cmd="$1"
    if ! command -v "$cmd" >/dev/null 2>&1; then
        echo "❌ 缺少依赖命令: $cmd" >&2
        exit 1
    fi
}

hash_file_manifest() {
    if [ "$#" -eq 0 ]; then
        printf 'empty\n' | sha256sum | awk '{print $1}'
        return 0
    fi

    local file rel
    (
        for file in "$@"; do
            rel="${file#$REPO_ROOT/}"
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
            -path "$FRONTEND_DIR/coverage/*" -o \
            -name "*.tsbuildinfo" -o \
            -name "vite.config.js.timestamp-*" \
        \) -prune -o -type f -print | LC_ALL=C sort
}

collect_frontend_input_files() {
    find "$FRONTEND_DIR" \
        \( \
            -path "$FRONTEND_DIR/node_modules" -o \
            -path "$FRONTEND_DIR/node_modules/*" -o \
            -path "$FRONTEND_DIR/dist" -o \
            -path "$FRONTEND_DIR/dist/*" -o \
            -path "$FRONTEND_DIR/coverage" -o \
            -path "$FRONTEND_DIR/coverage/*" -o \
            -name "*.tsbuildinfo" -o \
            -name "vite.config.js.timestamp-*" \
        \) -prune -o -type f -print | LC_ALL=C sort
}

compute_frontend_input_hash() {
    local files=()
    mapfile -t files < <(collect_frontend_input_files)
    if [ "${#files[@]}" -eq 0 ]; then
        echo "❌ 未收集到任何前端输入文件，无法计算 frontend input hash" >&2
        return 1
    fi
    hash_file_manifest "${files[@]}"
}

compute_source_hash() {
    local files=()
    mapfile -t files < <(collect_source_files)
    if [ "${#files[@]}" -eq 0 ]; then
        echo "❌ 未收集到任何后端/前端源码文件，无法计算 source hash" >&2
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
    mapfile -t files < <(collect_frontend_dist_files)
    if [ "${#files[@]}" -eq 0 ]; then
        echo "❌ 未找到可嵌入的前端构建产物: $BACKEND_DIR/internal/web/dist" >&2
        return 1
    fi
    hash_file_manifest "${files[@]}"
}

get_git_commit() {
    git -C "$REPO_ROOT" rev-parse HEAD
}

get_git_dirty_state() {
    local status
    status="$(
        git -C "$REPO_ROOT" status --porcelain --untracked-files=all -- backend frontend |
            grep -vE '^[ MADRCU?!]{2} frontend/.*\.tsbuildinfo$|^[ MADRCU?!]{2} frontend/.*vite\.config\.js\.timestamp-[^/]+$' || true
    )"
    if [ -n "$status" ]; then
        echo "dirty"
    else
        echo "clean"
    fi
}

verify_binary_metadata() {
    local binary_path="$1"
    local expected_line="$2"
    local actual_line

    actual_line="$("$binary_path" -version | tr -d '\r')"
    if [ "$actual_line" != "$expected_line" ]; then
        echo "❌ 二进制元数据校验失败" >&2
        echo "expected: $expected_line" >&2
        echo "actual:   $actual_line" >&2
        return 1
    fi
}

read_cache_value() {
    local key="$1"
    if [ ! -f "$CACHE_FILE" ]; then
        return 1
    fi
    awk -F= -v key="$key" '$1 == key { print substr($0, index($0, "=") + 1); exit }' "$CACHE_FILE"
}

write_deploy_cache() {
    local frontend_input_hash="$1"
    local frontend_dist_hash="$2"
    local source_hash="$3"
    local cache_tmp

    mkdir -p "$CACHE_DIR"
    cache_tmp="$CACHE_FILE.tmp.$$"
    {
        printf 'FRONTEND_INPUT_HASH=%s\n' "$frontend_input_hash"
        printf 'FRONTEND_DIST_HASH=%s\n' "$frontend_dist_hash"
        printf 'SOURCE_HASH=%s\n' "$source_hash"
    } > "$cache_tmp"
    mv -f "$cache_tmp" "$CACHE_FILE"
}

version_line_matches_identity() {
    local line="$1"
    printf '%s\n' "$line" | grep -F "version=${VERSION} " >/dev/null &&
        printf '%s\n' "$line" | grep -F "commit=${COMMIT} " >/dev/null &&
        printf '%s\n' "$line" | grep -F "build_type=${BUILD_TYPE} " >/dev/null &&
        printf '%s\n' "$line" | grep -F "dirty=${DIRTY} " >/dev/null &&
        printf '%s\n' "$line" | grep -F "source_hash=${SOURCE_HASH} " >/dev/null &&
        printf '%s\n' "$line" | grep -F "frontend_dist_hash=${FRONTEND_DIST_HASH}" >/dev/null
}

binary_matches_identity() {
    local binary_path="$1"
    local line

    if [ ! -x "$binary_path" ]; then
        return 1
    fi
    line="$("$binary_path" -version 2>/dev/null | tr -d '\r' || true)"
    version_line_matches_identity "$line"
}

running_binary_matches_identity() {
    local active_state pid line

    active_state="$(systemctl show "$SERVICE_NAME" --property=ActiveState --value 2>/dev/null || true)"
    if [ "$active_state" != "active" ]; then
        return 1
    fi

    pid="$(systemctl show "$SERVICE_NAME" --property=ExecMainPID --value 2>/dev/null || true)"
    if [ -n "$pid" ] && [ "$pid" != "0" ] && [ -x "/proc/$pid/exe" ]; then
        line="$(/proc/"$pid"/exe -version 2>/dev/null | tr -d '\r' || true)"
        version_line_matches_identity "$line"
        return
    fi

    binary_matches_identity "$BINARY_PATH"
}

wait_for_service_active() {
    local attempts="${DEPLOY_START_WAIT_ATTEMPTS:-15}"
    local delay="${DEPLOY_START_WAIT_SECONDS:-1}"
    local i

    for ((i = 1; i <= attempts; i++)); do
        if systemctl is-active --quiet "$SERVICE_NAME"; then
            return 0
        fi
        sleep "$delay"
    done

    return 1
}

backup_current_binary() {
    if [ ! -e "$BINARY_PATH" ]; then
        return 0
    fi

    mkdir -p "$BINARY_BACKUP_DIR"
    local backup_path
    backup_path="$BINARY_BACKUP_DIR/sub2api.$(date -u +"%Y%m%dT%H%M%SZ").bak"
    cp -a "$BINARY_PATH" "$backup_path"
    printf '%s\n' "$backup_path"
}

find_config_file() {
    local candidates=(
        "/etc/sub2api/config.yaml"
        "$REPO_ROOT/config.yaml"
        "$REPO_ROOT/data/config.yaml"
    )

    local path
    for path in "${candidates[@]}"; do
        if [ -f "$path" ]; then
            echo "$path"
            return 0
        fi
    done

    return 1
}

strip_yaml_quotes() {
    local value="$1"
    value="${value%\"}"
    value="${value#\"}"
    value="${value%\'}"
    value="${value#\'}"
    printf '%s' "$value"
}

read_database_yaml_value() {
    local config_file="$1"
    local key="$2"

    awk -v key="$key" '
        /^[[:space:]]*#/ { next }
        /^database:[[:space:]]*$/ { in_db=1; next }
        in_db && /^[^[:space:]#][^:]*:[[:space:]]*/ { exit }
        in_db {
            pattern = "^[[:space:]]*" key ":[[:space:]]*"
            if ($0 ~ pattern) {
                sub(pattern, "", $0)
                sub(/[[:space:]]+#.*$/, "", $0)
                gsub(/^[[:space:]]+|[[:space:]]+$/, "", $0)
                print $0
                exit
            }
        }
    ' "$config_file"
}

escape_pg_dsn_value() {
    local value="$1"
    value="${value//\\/\\\\}"
    value="${value//\'/\\\'}"
    printf "%s" "$value"
}

build_database_dsn() {
    local config_file="$1"
    local host port user password dbname sslmode

    host="${DATABASE_HOST:-$(strip_yaml_quotes "$(read_database_yaml_value "$config_file" "host")")}"
    port="${DATABASE_PORT:-$(strip_yaml_quotes "$(read_database_yaml_value "$config_file" "port")")}"
    user="${DATABASE_USER:-$(strip_yaml_quotes "$(read_database_yaml_value "$config_file" "user")")}"
    password="${DATABASE_PASSWORD:-$(strip_yaml_quotes "$(read_database_yaml_value "$config_file" "password")")}"
    dbname="${DATABASE_DBNAME:-$(strip_yaml_quotes "$(read_database_yaml_value "$config_file" "dbname")")}"
    sslmode="${DATABASE_SSLMODE:-$(strip_yaml_quotes "$(read_database_yaml_value "$config_file" "sslmode")")}"

    host="${host:-localhost}"
    port="${port:-5432}"
    user="${user:-postgres}"
    dbname="${dbname:-sub2api}"
    sslmode="${sslmode:-disable}"

    if [ -n "$password" ]; then
        printf "host='%s' port='%s' user='%s' password='%s' dbname='%s' sslmode='%s'" \
            "$(escape_pg_dsn_value "$host")" \
            "$(escape_pg_dsn_value "$port")" \
            "$(escape_pg_dsn_value "$user")" \
            "$(escape_pg_dsn_value "$password")" \
            "$(escape_pg_dsn_value "$dbname")" \
            "$(escape_pg_dsn_value "$sslmode")"
        return 0
    fi

    printf "host='%s' port='%s' user='%s' dbname='%s' sslmode='%s'" \
        "$(escape_pg_dsn_value "$host")" \
        "$(escape_pg_dsn_value "$port")" \
        "$(escape_pg_dsn_value "$user")" \
        "$(escape_pg_dsn_value "$dbname")" \
        "$(escape_pg_dsn_value "$sslmode")"
}

echo "🚀 开始更新..."
require_command git
require_command go
require_command pnpm
require_command sha256sum

echo "🧾 计算前端输入身份..."
FRONTEND_INPUT_HASH="$(compute_frontend_input_hash)"
CACHED_FRONTEND_INPUT_HASH="$(read_cache_value FRONTEND_INPUT_HASH || true)"
CACHED_FRONTEND_DIST_HASH="$(read_cache_value FRONTEND_DIST_HASH || true)"
CURRENT_FRONTEND_DIST_HASH="$(compute_frontend_dist_hash 2>/dev/null || true)"

if [ "${DEPLOY_FORCE_FRONTEND_BUILD:-0}" != "1" ] &&
    [ -n "$CURRENT_FRONTEND_DIST_HASH" ] &&
    [ "$FRONTEND_INPUT_HASH" = "$CACHED_FRONTEND_INPUT_HASH" ] &&
    [ "$CURRENT_FRONTEND_DIST_HASH" = "$CACHED_FRONTEND_DIST_HASH" ]; then
    echo "📦 前端输入未变化，跳过前端依赖同步和构建"
    FRONTEND_DIST_HASH="$CURRENT_FRONTEND_DIST_HASH"
else
    # 同步前端依赖，避免 package.json / pnpm-lock.yaml 已更新但 node_modules 仍是旧状态
    echo "📚 同步前端依赖..."
    pnpm --dir "$FRONTEND_DIR" install --frozen-lockfile

    echo "📦 构建前端..."
    pnpm --dir "$FRONTEND_DIR" run build
    FRONTEND_DIST_HASH="$(compute_frontend_dist_hash)"
fi

echo "🧾 计算源码身份..."
COMMIT="$(get_git_commit)"
DIRTY="$(get_git_dirty_state)"
SOURCE_HASH="$(compute_source_hash)"
BUILD_DATE="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"
BUILD_TYPE="source"
VERSION=$(tr -d '\r\n' < "$BACKEND_DIR/cmd/server/VERSION")

echo "   commit: $COMMIT"
echo "   dirty: $DIRTY"
echo "   frontend_input_hash: $FRONTEND_INPUT_HASH"
echo "   source_hash: $SOURCE_HASH"
echo "   frontend_dist_hash: $FRONTEND_DIST_HASH"

cd "$BACKEND_DIR"
EXPECTED_VERSION_LINE="Sub2API version=${VERSION} commit=${COMMIT} built=${BUILD_DATE} build_type=${BUILD_TYPE} dirty=${DIRTY} source_hash=${SOURCE_HASH} frontend_dist_hash=${FRONTEND_DIST_HASH}"

NEED_BACKEND_BUILD=1
if [ "${DEPLOY_FORCE_BACKEND_BUILD:-0}" != "1" ] && binary_matches_identity "$BINARY_PATH"; then
    NEED_BACKEND_BUILD=0
fi

if [ "$NEED_BACKEND_BUILD" -eq 0 ] &&
    [ "${DEPLOY_FORCE_RESTART:-0}" != "1" ] &&
    running_binary_matches_identity; then
    write_deploy_cache "$FRONTEND_INPUT_HASH" "$FRONTEND_DIST_HASH" "$SOURCE_HASH"
    echo "✅ 当前运行中的二进制已匹配当前源码和前端产物，无需部署/重启"
    exit 0
fi

if [ "${DEPLOY_CLEAN_GO_CACHE:-0}" = "1" ]; then
    echo "🧹 按 DEPLOY_CLEAN_GO_CACHE=1 清理 Go 缓存..."
    go clean -cache
else
    echo "🧹 跳过 Go 缓存清理（需要全量重编译时设置 DEPLOY_CLEAN_GO_CACHE=1）"
fi

if [ "$NEED_BACKEND_BUILD" -eq 1 ]; then
    echo "🔨 编译后端到临时二进制..."
    rm -f "$NEW_BINARY_PATH"
    LDFLAGS="-s -w -X main.Version=${VERSION} -X main.Commit=${COMMIT} -X main.Date=${BUILD_DATE} -X main.BuildType=${BUILD_TYPE} -X main.Dirty=${DIRTY} -X main.SourceHash=${SOURCE_HASH} -X main.FrontendDistHash=${FRONTEND_DIST_HASH}"
    CGO_ENABLED=0 go build -tags embed -ldflags="$LDFLAGS" -trimpath -o "$NEW_BINARY_PATH" ./cmd/server

    echo "🔍 校验临时二进制身份..."
    verify_binary_metadata "$NEW_BINARY_PATH" "$EXPECTED_VERSION_LINE"
    echo "✓ 临时二进制与当前源码身份一致"
else
    echo "✓ 当前磁盘二进制已匹配当前源码身份，跳过后端编译"
fi

# 检查并等待备份完成
echo "🔍 检查数据库备份状态..."
BACKUP_WAIT_COUNT=0
while pgrep -f "sub2api.*backup" > /dev/null || pgrep -f "pg_dump.*sub2api" > /dev/null; do
    if [ $BACKUP_WAIT_COUNT -eq 0 ]; then
        echo "⏳ 检测到备份正在运行，等待完成..."
    fi
    BACKUP_WAIT_COUNT=$((BACKUP_WAIT_COUNT + 1))
    if [ $BACKUP_WAIT_COUNT -gt 60 ]; then
        echo "⚠️  备份运行超过5分钟，是否继续等待？(y/n)"
        read -r -t 10 response || response="n"
        if [ "$response" != "y" ]; then
            echo "❌ 部署已取消" >&2
            exit 1
        fi
        BACKUP_WAIT_COUNT=0
    fi
    sleep 5
done
if [ $BACKUP_WAIT_COUNT -gt 0 ]; then
    echo "✓ 备份已完成"
fi

# 基于当前源码显式同步数据库校验和；放在停服前，失败时不影响当前服务。
echo "🔄 同步数据库校验和..."
CONFIG_FILE="$(find_config_file || true)"
if [ -z "$CONFIG_FILE" ]; then
    echo "❌ 未找到 config.yaml，尝试过 /etc/sub2api/config.yaml、$REPO_ROOT/config.yaml、$REPO_ROOT/data/config.yaml" >&2
    exit 1
fi

echo "   使用配置文件: $CONFIG_FILE"
DATABASE_DSN="$(build_database_dsn "$CONFIG_FILE")"
SUB2API_DATABASE_DSN="$DATABASE_DSN" go run ./cmd/sync_checksums
unset DATABASE_DSN
echo "✓ 数据库校验和同步完成"

BACKUP_BINARY_PATH=""
if [ "$NEED_BACKEND_BUILD" -eq 1 ]; then
    echo "🧷 备份当前二进制..."
    BACKUP_BINARY_PATH="$(backup_current_binary)"
    if [ -n "$BACKUP_BINARY_PATH" ]; then
        echo "   备份: $BACKUP_BINARY_PATH"
    fi

    echo "🔁 原子替换二进制..."
    mv -f "$NEW_BINARY_PATH" "$BINARY_PATH"
fi

echo "🔄 重启服务..."
systemctl restart "$SERVICE_NAME"

if wait_for_service_active; then
    write_deploy_cache "$FRONTEND_INPUT_HASH" "$FRONTEND_DIST_HASH" "$SOURCE_HASH"
    echo "✅ 部署成功！服务正常运行"
    systemctl status "$SERVICE_NAME" --no-pager -l | head -15
else
    echo "❌ 服务启动失败，查看日志："
    journalctl -u "$SERVICE_NAME" -n 20 --no-pager
    if [ -n "$BACKUP_BINARY_PATH" ] && [ -f "$BACKUP_BINARY_PATH" ]; then
        echo "↩️  尝试回滚到备份二进制..."
        cp -a "$BACKUP_BINARY_PATH" "$BINARY_PATH"
        systemctl restart "$SERVICE_NAME" || true
        if wait_for_service_active; then
            echo "✅ 已回滚到备份二进制并恢复服务"
        else
            echo "❌ 回滚后服务仍未恢复，请手动检查 systemd 日志" >&2
        fi
    fi
    exit 1
fi
