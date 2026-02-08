@echo off
setlocal

set "ADDR=%ADDR%"
if "%ADDR%"=="" set "ADDR=:8080"

set "BIN=%~dp0..\bin\openclaw-relay.exe"
if not exist "%BIN%" (
  echo openclaw-relay.exe not found: %BIN%
  exit /b 1
)

"%BIN%" -addr %ADDR%
exit /b %ERRORLEVEL%
