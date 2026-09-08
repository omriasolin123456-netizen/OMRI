@echo off
setlocal EnableExtensions
pushd "%~dp0" >nul 2>nul
if not exist "go.mod" (
  echo ERROR: go.mod was not found. Extract the full ZIP first.
  pause
  popd
  exit /b 1
)
where go >nul 2>nul || (echo Go is not installed.& pause & popd & exit /b 1)
go mod tidy
if errorlevel 1 (pause & popd & exit /b 1)
go run .
pause
popd
