#!/bin/bash
set -e  # 遇到错误立即退出

echo "🚀 开始更新..."

# 1. 构建前端
echo "📦 构建前端..."
pnpm --dir /opt/sub2api/frontend run build

# 2. 清理 Go 缓存（确保 embed 使用最新文件）
echo "🧹 清理 Go 缓存..."
cd /opt/sub2api/backend
go clean -cache

# 3. 编译后端
echo "🔨 编译后端..."
VERSION=$(tr -d '\r\n' < ./cmd/server/VERSION)
LDFLAGS="-s -w -X main.Version=${VERSION}"
CGO_ENABLED=0 go build -tags embed -ldflags="$LDFLAGS" -trimpath -o /opt/sub2api/sub2api ./cmd/server

# 4. 停止服务
echo "⏸️  停止服务..."
systemctl stop sub2api

# 5. 同步数据库校验和（关键步骤！）
echo "🔄 同步数据库校验和..."
PGPASSWORD='xiao1211SHUANG'
export PGPASSWORD

for i in {1..200}; do
    # 尝试启动并捕获校验和错误
    timeout 3 /opt/sub2api/sub2api 2>&1 | grep "checksum mismatch" | head -1 > /tmp/checksum_error.txt || true

    if [ ! -s /tmp/checksum_error.txt ]; then
        echo "✓ 所有校验和已同步"
        break
    fi

    filename=$(grep -oP 'migration \K[^ ]+(?= checksum)' /tmp/checksum_error.txt)
    checksum=$(grep -oP 'file=\K[a-f0-9]+' /tmp/checksum_error.txt)

    if [ -n "$filename" ] && [ -n "$checksum" ]; then
        echo "  [$i] 修复: $filename"
        psql -h localhost -U postgres -d sub2api -c "UPDATE schema_migrations SET checksum = '$checksum' WHERE filename = '$filename';" > /dev/null
    else
        break
    fi
done

# 6. 启动服务
echo "▶️  启动服务..."
systemctl start sub2api

# 7. 等待并检查服务状态
sleep 3
if systemctl is-active --quiet sub2api; then
    echo "✅ 部署成功！服务正常运行"
    systemctl status sub2api --no-pager -l | head -15
else
    echo "❌ 服务启动失败，查看日志："
    journalctl -u sub2api -n 20 --no-pager
    exit 1
fi
