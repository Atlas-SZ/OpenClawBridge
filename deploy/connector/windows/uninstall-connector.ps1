param(
  [string]$Target = "C:\Program Files\OpenClawBridge\connector\openclaw-connector.exe"
)

if (Test-Path $Target) {
  Remove-Item -Force $Target
  Write-Host "Removed $Target"
} else {
  Write-Host "Not found: $Target"
}
