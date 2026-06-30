#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."

[[ -f configs/config.yaml ]] || { echo "缺少 configs/config.yaml，请先复制 sample 并配置。" >&2; exit 1; }
mkdir -p logs
docker compose -f docker-compose-local.yml up -d --build --remove-orphans
docker compose -f docker-compose-local.yml ps
echo "MergeWong: http://localhost:${MERGEWONG_PORT:-8080}"
