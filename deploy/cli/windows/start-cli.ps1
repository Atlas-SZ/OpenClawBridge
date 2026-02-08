param(
  [Parameter(Mandatory = $true)][string]$RelayUrl,
  [Parameter(Mandatory = $true)][string]$AccessCode,
  [string]$ResponseTimeout = "30s"
)

$binPath = Join-Path $PSScriptRoot "..\bin\openclaw-cli.exe"
if (-not (Test-Path $binPath)) {
  Write-Error "openclaw-cli.exe not found: $binPath"
  exit 1
}

& $binPath -relay-url $RelayUrl -access-code $AccessCode -response-timeout $ResponseTimeout
exit $LASTEXITCODE
