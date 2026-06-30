#!/usr/bin/env pwsh
$ErrorActionPreference = 'Stop'
Set-Location (Split-Path -Parent $PSScriptRoot)

if (-not (Test-Path 'configs/config.yaml')) {
  throw '缺少 configs/config.yaml，请先复制 configs/config.yaml.sample。'
}
New-Item -ItemType Directory -Force 'logs' | Out-Null
if (-not (Test-Path 'web/node_modules')) {
  Push-Location web; try { npm ci; if ($LASTEXITCODE) { throw 'npm ci 失败' } } finally { Pop-Location }
}
Push-Location web; try { npm run build; if ($LASTEXITCODE) { throw '前端构建失败' } } finally { Pop-Location }
go run ./cmd/server
if ($LASTEXITCODE) { exit $LASTEXITCODE }
