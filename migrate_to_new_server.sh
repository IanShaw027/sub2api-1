#!/bin/bash
# =============================================================================
# Sub2API 迁移脚本 (简化版 - 从 GitHub 拉取代码)
# =============================================================================

set -e

TARGET_HOST="43.240.223.39"
TARGET_USER="root"
TARGET_PASSWORD="xiao1211SHUANG"
BACKUP_DIR="/tmp/sub2api_migration_$(date +%Y%m%d_%H%M%S)"

echo "=============================================="
echo "Sub2API 迁移脚本"
echo "目标服务器: $TARGET_HOST"
echo "=============================================="

mkdir -p "$BACKUP_DIR"

# -----------------------------------------------------------------------------
# Step 1: 导出 PostgreSQL 数据库
# -----------------------------------------------------------------------------
echo ""
echo "[1/4] 导出 PostgreSQL 数据库..."
PGPASSWORD=xiao1211SHUANG pg_dump -h localhost -p 5678 -U postgres -d sub2api \
    --no-owner --no-acl \
    -F c -f "$BACKUP_DIR/sub2api.dump"
echo "数据库导出完成: $(du -h $BACKUP_DIR/sub2api.dump | cut -f1)"

# -----------------------------------------------------------------------------
# Step 2: 导出 Redis 数据
# -----------------------------------------------------------------------------
echo ""
echo "[2/4] 导出 Redis 数据..."
redis-cli -a xiao1211SHUANG BGSAVE 2>/dev/null || true
sleep 2
cp /var/lib/redis/dump.rdb "$BACKUP_DIR/redis_dump.rdb" 2>/dev/null || echo "跳过"

# -----------------------------------------------------------------------------
# Step 3: 保存配置文件
# -----------------------------------------------------------------------------
echo ""
echo "[3/4] 保存配置文件..."
cp /opt/services/sub2api/deploy/.env "$BACKUP_DIR/deploy.env"

# -----------------------------------------------------------------------------
# Step 4: 传输到目标服务器
# -----------------------------------------------------------------------------
echo ""
echo "[4/4] 传输文件到目标服务器..."

sshpass -p "$TARGET_PASSWORD" ssh -o StrictHostKeyChecking=no $TARGET_USER@$TARGET_HOST 'mkdir -p /tmp/sub2api_restore'
sshpass -p "$TARGET_PASSWORD" scp -o StrictHostKeyChecking=no $BACKUP_DIR/* $TARGET_USER@$TARGET_HOST:/tmp/sub2api_restore/

echo "传输完成!"

# -----------------------------------------------------------------------------
# 生成恢复脚本
# -----------------------------------------------------------------------------
cat > "$BACKUP_DIR/restore.sh" << 'RESTORE_SCRIPT'
#!/bin/bash
set -e

RESTORE_DIR="/tmp/sub2api_restore"

echo "=============================================="
echo "Sub2API 恢复脚本"
echo "=============================================="

# 1. 安装依赖
echo "[1/7] 安装依赖..."
apt-get update
apt-get install -y curl gnupg2 lsb-release git

# Docker
if ! command -v docker &> /dev/null; then
    curl -fsSL https://get.docker.com | sh
    systemctl enable docker && systemctl start docker
fi

# 2. PostgreSQL 17
echo "[2/7] 安装 PostgreSQL..."
if ! command -v psql &> /dev/null; then
    echo "deb http://apt.postgresql.org/pub/repos/apt $(lsb_release -cs)-pgdg main" > /etc/apt/sources.list.d/pgdg.list
    curl -fsSL https://www.postgresql.org/media/keys/ACCC4CF8.asc | gpg --dearmor -o /etc/apt/trusted.gpg.d/postgresql.gpg
    apt-get update
    apt-get install -y postgresql-17
fi

systemctl enable postgresql && systemctl start postgresql

# 配置 PostgreSQL
PG_CONF="/etc/postgresql/17/main/postgresql.conf"
PG_HBA="/etc/postgresql/17/main/pg_hba.conf"
sed -i "s/^#*port = .*/port = 5678/" $PG_CONF
sed -i "s/^#*listen_addresses = .*/listen_addresses = '*'/" $PG_CONF
grep -q "172.17.0.0/16" $PG_HBA || echo "host all all 172.17.0.0/16 md5" >> $PG_HBA
grep -q "0.0.0.0/0" $PG_HBA || echo "host all all 0.0.0.0/0 md5" >> $PG_HBA
systemctl restart postgresql && sleep 3
sudo -u postgres psql -p 5678 -c "ALTER USER postgres PASSWORD 'xiao1211SHUANG';"

# 3. 恢复数据库
echo "[3/7] 恢复数据库..."
sudo -u postgres psql -p 5678 -c "DROP DATABASE IF EXISTS sub2api;"
sudo -u postgres psql -p 5678 -c "CREATE DATABASE sub2api;"
sudo -u postgres pg_restore -p 5678 -d sub2api --no-owner --no-acl "$RESTORE_DIR/sub2api.dump"

# 4. Redis
echo "[4/7] 安装 Redis..."
apt-get install -y redis-server
REDIS_CONF="/etc/redis/redis.conf"
sed -i "s/^bind .*/bind 0.0.0.0/" $REDIS_CONF
grep -q "^requirepass" $REDIS_CONF && sed -i "s/^requirepass .*/requirepass xiao1211SHUANG/" $REDIS_CONF || echo "requirepass xiao1211SHUANG" >> $REDIS_CONF

if [ -f "$RESTORE_DIR/redis_dump.rdb" ]; then
    systemctl stop redis-server
    cp "$RESTORE_DIR/redis_dump.rdb" /var/lib/redis/dump.rdb
    chown redis:redis /var/lib/redis/dump.rdb
fi
systemctl enable redis-server && systemctl restart redis-server

# 5. 克隆项目
echo "[5/7] 克隆项目..."
mkdir -p /opt/services
cd /opt/services
rm -rf sub2api
git clone https://github.com/Wei-Shaw/sub2api.git

# 6. 恢复配置
echo "[6/7] 恢复配置..."
cp "$RESTORE_DIR/deploy.env" /opt/services/sub2api/deploy/.env
cd /opt/services/sub2api/deploy
sed -i 's/DATABASE_HOST=.*/DATABASE_HOST=host.docker.internal/' .env
sed -i 's/REDIS_HOST=.*/REDIS_HOST=host.docker.internal/' .env

# 7. 启动服务
echo "[7/7] 启动 Sub2API..."
docker pull weishaw/sub2api:latest
docker stop sub2api 2>/dev/null || true
docker rm sub2api 2>/dev/null || true

docker run -d \
    --name sub2api \
    --restart unless-stopped \
    --add-host host.docker.internal:host-gateway \
    -p 8080:8080 \
    -v sub2api_data:/app/data \
    --env-file .env \
    -e DATABASE_HOST=host.docker.internal \
    -e DATABASE_PORT=5678 \
    -e REDIS_HOST=host.docker.internal \
    -e REDIS_PORT=6379 \
    weishaw/sub2api:latest

sleep 10

echo ""
echo "=============================================="
echo "迁移完成!"
echo "=============================================="
echo "访问: http://$(hostname -I | awk '{print $1}'):8080"
echo ""
docker ps | grep sub2api
RESTORE_SCRIPT

sshpass -p "$TARGET_PASSWORD" scp -o StrictHostKeyChecking=no "$BACKUP_DIR/restore.sh" $TARGET_USER@$TARGET_HOST:/tmp/sub2api_restore/

echo ""
echo "=============================================="
echo "准备完成! 开始在目标服务器执行恢复..."
echo "=============================================="

sshpass -p "$TARGET_PASSWORD" ssh -o StrictHostKeyChecking=no $TARGET_USER@$TARGET_HOST \
    'chmod +x /tmp/sub2api_restore/restore.sh && /tmp/sub2api_restore/restore.sh'
