param(
  [string]$Target = "C:\Program Files\OpenClawBridge\relay\openclaw-relay.exe"
)

if (Test-Path $Target) {
  Remove-Item -Force $Target
  Write-Host "Removed $Target"
} else {
  Write-Host "Not found: $Target"
}
