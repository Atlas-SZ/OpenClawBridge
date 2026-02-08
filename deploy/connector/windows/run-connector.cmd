@echo off
setlocal

set "CONFIG=%CONFIG%"
if "%CONFIG%"=="" set "CONFIG=%~dp0config\connector.json"
if not exist "%CONFIG%" set "CONFIG=%~dp0..\config\connector.json"

set "BIN=%~dp0openclaw-connector.exe"
if not exist "%BIN%" set "BIN=%~dp0..\bin\openclaw-connector.exe"
if not exist "%BIN%" (
  echo openclaw-connector.exe not found: %BIN%
  exit /b 1
)

if not exist "%CONFIG%" (
  echo config not found: %CONFIG%
  echo copy config\connector.json.example to config\connector.json and edit it first.
  exit /b 1
)

"%BIN%" -config "%CONFIG%"
exit /b %ERRORLEVEL%
