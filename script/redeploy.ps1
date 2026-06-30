#!/usr/bin/env pwsh
param([ValidateSet('deploy', 'down', 'logs', 'status')][string]$Command = 'deploy')
$ErrorActionPreference = 'Stop'
Set-Location (Split-Path -Parent $PSScriptRoot)

function Invoke-Compose([string[]]$Arguments) {
  & docker compose -f docker-compose.yml @Arguments
  if ($LASTEXITCODE) { throw "docker compose $($Arguments -join ' ') 失败" }
}

switch ($Command) {
  'deploy' {
    if (-not (Test-Path 'configs/config.yaml')) { throw '缺少 configs/config.yaml' }
    New-Item -ItemType Directory -Force 'logs' | Out-Null
    Invoke-Compose @('pull')
    Invoke-Compose @('up', '-d', '--remove-orphans')
    Invoke-Compose @('ps')
  }
  'down'   { Invoke-Compose @('down') }
  'logs'   { Invoke-Compose @('logs', '-f', 'app') }
  'status' { Invoke-Compose @('ps') }
}
