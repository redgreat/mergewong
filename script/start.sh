#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."

[[ -f configs/config.yaml ]] || { echo "缺少 configs/config.yaml，请先复制 configs/config.yaml.sample。" >&2; exit 1; }
mkdir -p logs

if [[ ! -d web/node_modules ]]; then
  (cd web && npm ci)
fi
(cd web && npm run build)
exec go run ./cmd/server
