@echo off
setlocal
cd /d "%~dp0"
if not exist "%~dp0MicroinvestAssistant.ahk" (
  echo ERROR: MicroinvestAssistant.ahk not found
  pause
  exit /b 1
)
start "" "%~dp0MicroinvestAssistant.ahk"
endlocal
