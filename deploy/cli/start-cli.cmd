@echo off
setlocal

if /I "%~1"=="-h" goto usage
if /I "%~1"=="--help" goto usage

set "RELAY_URL=%~1"
set "ACCESS_CODE=%~2"
set "RESPONSE_TIMEOUT=%~3"

if "%RELAY_URL%"=="" goto usage
if "%ACCESS_CODE%"=="" goto usage
if "%RESPONSE_TIMEOUT%"=="" set "RESPONSE_TIMEOUT=30s"

set "BIN_PATH=%~dp0..\..\openclaw-cli.exe"
if not exist "%BIN_PATH%" (
  echo openclaw-cli not found: %BIN_PATH%
  exit /b 1
)

"%BIN_PATH%" -relay-url "%RELAY_URL%" -access-code "%ACCESS_CODE%" -response-timeout "%RESPONSE_TIMEOUT%"
exit /b %ERRORLEVEL%

:usage
echo Usage:
echo   start-cli.cmd ^<relay-url^> ^<access-code^> [response-timeout]
echo Example:
echo   start-cli.cmd wss://bridge.example.com/client A-123456 30s
exit /b 1
