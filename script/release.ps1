#!/usr/bin/env pwsh
param([Parameter(Position=0)][string]$Version = '')
$ErrorActionPreference = 'Stop'
Set-Location (Split-Path -Parent $PSScriptRoot)

if (git status --porcelain) { throw '工作区不干净，请先提交或暂存改动。' }
git fetch --tags origin
if ($LASTEXITCODE) { throw '拉取标签失败' }

if ([string]::IsNullOrWhiteSpace($Version)) {
  $latest = git tag --list 'v[0-9]*.[0-9]*.[0-9]*' --sort=-version:refname | Select-Object -First 1
  if (-not $latest) { $Version = 'v0.0.1' }
  elseif ($latest -match '^v(\d+)\.(\d+)\.(\d+)$') {
    $Version = "v$($Matches[1]).$($Matches[2]).$([int]$Matches[3] + 1)"
  }
}
if ($Version -notmatch '^v\d+\.\d+\.\d+$') { throw '版本必须为 vMAJOR.MINOR.PATCH' }
if (git tag --list $Version) { throw "标签 $Version 已存在" }

go test ./...
if ($LASTEXITCODE) { throw 'Go 测试失败' }
Push-Location web
try {
  npm ci; if ($LASTEXITCODE) { throw 'npm ci 失败' }
  npm run build; if ($LASTEXITCODE) { throw '前端构建失败' }
} finally { Pop-Location }

git tag -a $Version -m "Release $Version"
if ($LASTEXITCODE) { throw '创建标签失败' }
git push origin $Version
if ($LASTEXITCODE) { throw '推送标签失败' }
Write-Host "已推送 $Version，GitHub Actions 将发布 GHCR 和阿里云 ACR 镜像。" -ForegroundColor Green
