#!/usr/bin/env bash
# Start the mock backend (:8091) and Vite (:3777) for visual review. Ctrl-C stops both.
set -u
DIR="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$DIR"
node scripts/mock/server.js > .shots/mock.log 2>&1 &
MOCK=$!
VITE_DEV_PROXY_TARGET=http://127.0.0.1:8091 node_modules/.bin/vite --port 3777 --strictPort &
VITE=$!
trap 'kill $MOCK $VITE 2>/dev/null' EXIT
wait $VITE
