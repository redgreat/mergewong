#!/usr/bin/env bash
set -euo pipefail
read -r -p "将清理未使用的 Docker 构建缓存、镜像和网络，继续？[y/N] " answer
[[ "$answer" =~ ^[Yy]$ ]] || exit 0
docker builder prune -af
docker system prune -af
