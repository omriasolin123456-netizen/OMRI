@echo off
setlocal EnableExtensions
cd /d "%~dp0"

echo === Microinvest Parts Assistant: setup ===

set "PY="
where python >nul 2>&1
if %ERRORLEVEL%==0 (
  set "PY=python"
) else (
  where py >nul 2>&1
  if %ERRORLEVEL%==0 (
    set "PY=py -3"
  )
)

if not defined PY (
  echo ERROR: Python not found in PATH.
  echo Install Python 3.10+ from python.org
  echo Enable checkbox: Add python.exe to PATH
  echo Then run this file again.
  pause
  exit /b 1
)

echo Using: %PY%
echo.

%PY% -m pip install --upgrade pip
if errorlevel 1 (
  echo ERROR: pip upgrade failed.
  pause
  exit /b 1
)

%PY% -m pip install -r "%~dp0requirements.txt"
if errorlevel 1 (
  echo ERROR: dependency install failed.
  echo Try manually:
  echo   %PY% -m pip install -r requirements.txt
  pause
  exit /b 1
)

if not exist "%~dp0price" mkdir "%~dp0price"
if not exist "%~dp0logs" mkdir "%~dp0logs"

echo.
echo Checking imports...
%PY% -c "import sys; import pandas; import openpyxl; import requests; print('OK', sys.version)"
if errorlevel 1 (
  echo ERROR: Python imports failed.
  pause
  exit /b 1
)

echo.
echo Setup complete.
echo 1. Put your .xlsx files into folder: price
echo 2. Edit config.ini if needed
echo 3. Start MicroinvestAssistant.ahk or run.bat
echo 4. Tray menu: Check Python
echo 5. In Microinvest product card press F8
echo.
pause
endlocal
