#!/bin/bash
set -e  # 遇到错误立即退出

REPO_ROOT="/opt/sub2api"
BACKEND_DIR="$REPO_ROOT/backend"
FRONTEND_DIR="$REPO_ROOT/frontend"
BINARY_PATH="$REPO_ROOT/sub2api"

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
            -path "$FRONTEND_DIR/coverage/*" \
        \) -prune -o -type f -print | LC_ALL=C sort
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
    if [ -n "$(git -C "$REPO_ROOT" status --porcelain --untracked-files=all -- backend frontend)" ]; then
        echo "dirty"
    else
        echo "clean"
    fi
}

verify_built_binary_metadata() {
    local expected_line="$1"
    local actual_line

    actual_line="$("$BINARY_PATH" -version | tr -d '\r')"
    if [ "$actual_line" != "$expected_line" ]; then
        echo "❌ 二进制元数据校验失败" >&2
        echo "expected: $expected_line" >&2
        echo "actual:   $actual_line" >&2
        return 1
    fi
}

find_config_file() {
    local candidates=(
        "/etc/sub2api/config.yaml"
        "/opt/sub2api/config.yaml"
        "/opt/sub2api/data/config.yaml"
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

    if [ -z "$password" ]; then
        echo "❌ 无法确定数据库密码，请检查 config.yaml 或导出 DATABASE_PASSWORD" >&2
        return 1
    fi

    printf "host='%s' port='%s' user='%s' password='%s' dbname='%s' sslmode='%s'" \
        "$(escape_pg_dsn_value "$host")" \
        "$(escape_pg_dsn_value "$port")" \
        "$(escape_pg_dsn_value "$user")" \
        "$(escape_pg_dsn_value "$password")" \
        "$(escape_pg_dsn_value "$dbname")" \
        "$(escape_pg_dsn_value "$sslmode")"
}

echo "🚀 开始更新..."
require_command git
require_command go
require_command pnpm
require_command sha256sum

# 0. 同步前端依赖，避免 package.json / pnpm-lock.yaml 已更新但 node_modules 仍是旧状态
echo "📚 同步前端依赖..."
pnpm --dir "$FRONTEND_DIR" install --frozen-lockfile

# 1. 构建前端
echo "📦 构建前端..."
pnpm --dir "$FRONTEND_DIR" run build

echo "🧾 计算源码身份..."
COMMIT="$(get_git_commit)"
DIRTY="$(get_git_dirty_state)"
SOURCE_HASH="$(compute_source_hash)"
FRONTEND_DIST_HASH="$(compute_frontend_dist_hash)"
BUILD_DATE="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"
BUILD_TYPE="source"

echo "   commit: $COMMIT"
echo "   dirty: $DIRTY"
echo "   source_hash: $SOURCE_HASH"
echo "   frontend_dist_hash: $FRONTEND_DIST_HASH"

# 2. 清理 Go 缓存（确保 embed 使用最新文件）
echo "🧹 清理 Go 缓存..."
cd "$BACKEND_DIR"
go clean -cache

# 3. 编译后端
echo "🔨 编译后端..."
VERSION=$(tr -d '\r\n' < ./cmd/server/VERSION)
LDFLAGS="-s -w -X main.Version=${VERSION} -X main.Commit=${COMMIT} -X main.Date=${BUILD_DATE} -X main.BuildType=${BUILD_TYPE} -X main.Dirty=${DIRTY} -X main.SourceHash=${SOURCE_HASH} -X main.FrontendDistHash=${FRONTEND_DIST_HASH}"
CGO_ENABLED=0 go build -tags embed -ldflags="$LDFLAGS" -trimpath -o "$BINARY_PATH" ./cmd/server

echo "🔍 校验二进制身份..."
EXPECTED_VERSION_LINE="Sub2API version=${VERSION} commit=${COMMIT} built=${BUILD_DATE} build_type=${BUILD_TYPE} dirty=${DIRTY} source_hash=${SOURCE_HASH} frontend_dist_hash=${FRONTEND_DIST_HASH}"
verify_built_binary_metadata "$EXPECTED_VERSION_LINE"
echo "✓ 二进制与当前源码身份一致"

# 4. 检查并等待备份完成
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

# 5. 停止服务
echo "⏸️  停止服务..."
systemctl stop sub2api

# 6. 基于当前配置显式同步数据库校验和
echo "🔄 同步数据库校验和..."
CONFIG_FILE="$(find_config_file || true)"
if [ -z "$CONFIG_FILE" ]; then
    echo "❌ 未找到 config.yaml，尝试过 /etc/sub2api/config.yaml、/opt/sub2api/config.yaml、/opt/sub2api/data/config.yaml" >&2
    exit 1
fi

echo "   使用配置文件: $CONFIG_FILE"
DATABASE_DSN="$(build_database_dsn "$CONFIG_FILE")"
go run ./cmd/sync_checksums "$DATABASE_DSN"
echo "✓ 数据库校验和同步完成"

# 7. 启动服务
echo "▶️  启动服务..."
systemctl start sub2api

# 8. 等待并检查服务状态
sleep 3
if systemctl is-active --quiet sub2api; then
    echo "✅ 部署成功！服务正常运行"
    systemctl status sub2api --no-pager -l | head -15
else
    echo "❌ 服务启动失败，查看日志："
    journalctl -u sub2api -n 20 --no-pager
    exit 1
fi
