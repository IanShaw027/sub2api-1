#!/usr/bin/env bash

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
COMPOSE_FILE="${DEPLOY_COMPOSE_FILE:-$REPO_ROOT/deploy/docker-compose.standalone.host.yml}"
ENV_FILE="${DEPLOY_ENV_FILE:-$REPO_ROOT/deploy/.env}"
CONTAINER_NAME="${DEPLOY_CONTAINER_NAME:-sub2api}"
IMAGE_REPOSITORY="${DEPLOY_IMAGE_REPOSITORY:-ianshaw027/sub2api}"
CACHE_DIR="${DEPLOY_CACHE_DIR:-$REPO_ROOT/deploy/.cache}"
LOCK_DIR="$CACHE_DIR/docker-deploy.lock"
HEALTH_ATTEMPTS="${DEPLOY_HEALTH_ATTEMPTS:-30}"
HEALTH_DELAY_SECONDS="${DEPLOY_HEALTH_DELAY_SECONDS:-2}"

override_file=""

cleanup() {
    if [ -n "$override_file" ] && [ -f "$override_file" ]; then
        rm -f "$override_file"
    fi
    rmdir "$LOCK_DIR" 2>/dev/null || true
}
trap cleanup EXIT

require_command() {
    if ! command -v "$1" >/dev/null 2>&1; then
        echo "missing required command: $1" >&2
        exit 1
    fi
}

write_override() {
    local image="$1"
    override_file="$(mktemp "$CACHE_DIR/docker-compose.override.XXXXXX.yml")"
    printf 'services:\n  sub2api:\n    image: %s\n' "$image" >"$override_file"
}

compose_up() {
    local image="$1"
    if [ -n "$override_file" ] && [ -f "$override_file" ]; then
        rm -f "$override_file"
        override_file=""
    fi
    write_override "$image"
    docker compose \
        --env-file "$ENV_FILE" \
        -f "$COMPOSE_FILE" \
        -f "$override_file" \
        up -d --force-recreate --no-deps sub2api
}

wait_for_health() {
    local attempt status
    for attempt in $(seq 1 "$HEALTH_ATTEMPTS"); do
        status="$(docker inspect "$CONTAINER_NAME" --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' 2>/dev/null || true)"
        if [ "$status" = "healthy" ] && curl -fsS http://127.0.0.1:8080/health >/dev/null; then
            return 0
        fi
        if [ "$status" = "exited" ] || [ "$status" = "dead" ]; then
            return 1
        fi
        sleep "$HEALTH_DELAY_SECONDS"
    done
    return 1
}

require_command docker
require_command curl
require_command git

if [ ! -f "$COMPOSE_FILE" ]; then
    echo "compose file not found: $COMPOSE_FILE" >&2
    exit 1
fi
if [ ! -f "$ENV_FILE" ]; then
    echo "environment file not found: $ENV_FILE" >&2
    exit 1
fi

mkdir -p "$CACHE_DIR"
if ! mkdir "$LOCK_DIR" 2>/dev/null; then
    echo "another deployment is running: $LOCK_DIR" >&2
    exit 1
fi

commit="$(git -C "$REPO_ROOT" rev-parse HEAD)"
short_commit="$(git -C "$REPO_ROOT" rev-parse --short=12 HEAD)"
build_date="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
version="$(tr -d '[:space:]' <"$REPO_ROOT/backend/cmd/server/VERSION")"
image="$IMAGE_REPOSITORY:$short_commit"
previous_image="$(docker inspect "$CONTAINER_NAME" --format '{{.Image}}' 2>/dev/null || true)"

echo "building $image from $commit"
docker build \
    --build-arg "VERSION=$version" \
    --build-arg "COMMIT=$commit" \
    --build-arg "DATE=$build_date" \
    --build-arg "GOPROXY=https://goproxy.cn,direct" \
    --build-arg "GOSUMDB=sum.golang.google.cn" \
    -t "$image" \
    -f "$REPO_ROOT/Dockerfile" \
    "$REPO_ROOT"

echo "verifying image identity"
identity="$(docker run --rm --entrypoint /app/sub2api "$image" -version 2>&1)"
printf '%s\n' "$identity"
if [[ "$identity" != *"$commit"* ]]; then
    echo "built image does not report expected commit $commit" >&2
    exit 1
fi

echo "recreating $CONTAINER_NAME"
compose_up "$image"
if wait_for_health; then
    docker inspect "$CONTAINER_NAME" --format 'container={{.Name}} image={{.Config.Image}} image_id={{.Image}} started={{.State.StartedAt}} health={{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}'
    exit 0
fi

echo "new container failed health checks" >&2
docker logs --tail 100 "$CONTAINER_NAME" >&2 || true
if [ -n "$previous_image" ]; then
    echo "rolling back to $previous_image" >&2
    compose_up "$previous_image"
    if wait_for_health; then
        echo "rollback succeeded" >&2
    else
        echo "rollback failed" >&2
    fi
fi
exit 1
