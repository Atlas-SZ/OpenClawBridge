@echo off
setlocal

set "TARGET=%~1"
if "%TARGET%"=="" set "TARGET=C:\Program Files\OpenClawBridge\relay\openclaw-relay.exe"

if exist "%TARGET%" (
  del /f /q "%TARGET%"
  echo Removed %TARGET%
) else (
  echo Not found: %TARGET%
)
