#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."

compose=(docker compose -f docker-compose.yml)
case "${1:-deploy}" in
  deploy)
    [[ -f configs/config.yaml ]] || { echo "缺少 configs/config.yaml" >&2; exit 1; }
    mkdir -p logs
    "${compose[@]}" pull
    "${compose[@]}" up -d --remove-orphans
    "${compose[@]}" ps
    ;;
  down) "${compose[@]}" down ;;
  logs) "${compose[@]}" logs -f app ;;
  status) "${compose[@]}" ps ;;
  *) echo "用法: $0 [deploy|down|logs|status]" >&2; exit 2 ;;
esac
