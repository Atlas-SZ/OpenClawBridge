param(
  [string]$Config = ""
)

if ($Config -eq "") {
  $Config = Join-Path $PSScriptRoot "..\config\connector.json"
}

$binPath = Join-Path $PSScriptRoot "..\bin\openclaw-connector.exe"
if (-not (Test-Path $binPath)) {
  Write-Error "openclaw-connector.exe not found: $binPath"
  exit 1
}

if (-not (Test-Path $Config)) {
  Write-Error "config not found: $Config"
  Write-Host "Copy connector.json.example to connector.json and edit it first."
  exit 1
}

& $binPath -config $Config
exit $LASTEXITCODE
