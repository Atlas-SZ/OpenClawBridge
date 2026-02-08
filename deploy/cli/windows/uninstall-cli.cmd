@echo off
setlocal

set "TARGET=%~1"
if "%TARGET%"=="" set "TARGET=C:\Program Files\OpenClawBridge\cli\openclaw-cli.exe"

if exist "%TARGET%" (
  del /f /q "%TARGET%"
  echo Removed %TARGET%
) else (
  echo Not found: %TARGET%
)
