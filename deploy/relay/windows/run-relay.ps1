param(
  [string]$Addr = ":8080"
)

$binPath = Join-Path $PSScriptRoot "..\bin\openclaw-relay.exe"
if (-not (Test-Path $binPath)) {
  Write-Error "openclaw-relay.exe not found: $binPath"
  exit 1
}

& $binPath -addr $Addr
exit $LASTEXITCODE
