@echo off
setlocal EnableExtensions
cd /d "%~dp0\.."

if not exist "logs" mkdir "logs"

set "QUERY_FILE=logs\query.txt"
set "RESULT_FILE=logs\last_result.json"
set "OUT_LOG=logs\python_stdout.log"
set "ERR_LOG=logs\python_stderr.log"
set "RUN_LOG=logs\python_run.log"

> "%RUN_LOG%" echo Mode=F7_NTIN
>> "%RUN_LOG%" echo WorkDir=%CD%

set "PY="
if exist "venv\Scripts\python.exe" set "PY=venv\Scripts\python.exe"
if not defined PY (
  where python >nul 2>&1
  if not errorlevel 1 set "PY=python"
)
if not defined PY (
  where py >nul 2>&1
  if not errorlevel 1 set "PY=py -3"
)
if not defined PY (
  > "%ERR_LOG%" echo ERROR: Python not found. Run install.bat
  > "%RESULT_FILE%" echo {"status":"error","message":"Python not found","name":"","barcode":"","catalog_number":"","ntin":"","ntin_missing":true}
  exit /b 3
)

if not exist "%QUERY_FILE%" (
  > "%ERR_LOG%" echo ERROR: logs\query.txt missing
  exit /b 3
)

del "%OUT_LOG%" >nul 2>&1
del "%ERR_LOG%" >nul 2>&1
del "%RESULT_FILE%" >nul 2>&1
del "logs\last_result.txt" >nul 2>&1

set PYTHONUTF8=1
set PYTHONIOENCODING=utf-8

%PY% -u "python\ntin_only.py" --query-file "%QUERY_FILE%" --config "config.ini" --result "%RESULT_FILE%" >"%OUT_LOG%" 2>"%ERR_LOG%"
set "EC=%ERRORLEVEL%"
>> "%RUN_LOG%" echo ExitCode=%EC%

if not exist "%RESULT_FILE%" (
  > "%RESULT_FILE%" echo {"status":"error","message":"Python failed, see logs/python_stderr.log","name":"","barcode":"","catalog_number":"","ntin":"","ntin_missing":true}
  exit /b 3
)

exit /b %EC%
