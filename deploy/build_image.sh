#!/usr/bin/env bash
# 本地构建镜像的快速脚本，避免在命令行反复输入构建参数。

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

pnpm --dir "${REPO_ROOT}/frontend" install --frozen-lockfile
pnpm --dir "${REPO_ROOT}/frontend" run build
eval "$(bash "${REPO_ROOT}/deploy/build_metadata.sh")"

docker build -t sub2api:latest \
    --build-arg GOPROXY=https://goproxy.cn,direct \
    --build-arg GOSUMDB=sum.golang.google.cn \
    --build-arg DIRTY="${DIRTY}" \
    --build-arg SOURCE_HASH="${SOURCE_HASH}" \
    --build-arg FRONTEND_DIST_HASH="${FRONTEND_DIST_HASH}" \
    -f "${REPO_ROOT}/Dockerfile" \
    "${REPO_ROOT}"
