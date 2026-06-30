#!/usr/bin/env pwsh
$answer = Read-Host '将清理未使用的 Docker 构建缓存、镜像和网络，继续？[y/N]'
if ($answer -notmatch '^[Yy]$') { exit 0 }
docker builder prune -af
if ($LASTEXITCODE) { exit $LASTEXITCODE }
docker system prune -af
exit $LASTEXITCODE
