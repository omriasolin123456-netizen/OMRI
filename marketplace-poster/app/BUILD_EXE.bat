@echo off
setlocal EnableExtensions
pushd "%~dp0" >nul 2>nul
if errorlevel 1 (
  echo ERROR: Could not open the application folder.
  pause
  exit /b 1
)
cls
echo ==================================================
echo        Marketplace Poster V2 - Windows Build
echo ==================================================
echo.
if not exist "go.mod" (
  echo ERROR: go.mod was not found next to BUILD_EXE.bat.
  echo.
  echo Extract the ENTIRE ZIP first. Do not run this BAT from inside the ZIP.
  echo Current folder: %CD%
  echo.
  pause
  popd
  exit /b 1
)
where go >nul 2>nul
if errorlevel 1 (
  echo ERROR: Go is not installed or is not in PATH.
  echo Install Go 1.22 or newer and run this file again.
  pause
  popd
  exit /b 1
)
for /f "tokens=*" %%i in ('go version') do echo %%i
echo.
echo [1/3] Downloading/verifying Go dependencies...
go mod tidy
if errorlevel 1 (
  echo.
  echo ERROR: Go could not download dependencies.
  echo Read the Go error ABOVE this message. Usually this is Internet/DNS/firewall/proxy related.
  echo.
  pause
  popd
  exit /b 1
)
echo.
echo [2/3] Checking code...
go test ./...
if errorlevel 1 (
  echo.
  echo ERROR: Code check failed. Copy the compiler error above.
  pause
  popd
  exit /b 1
)
if not exist "dist" mkdir "dist"
echo.
echo [3/3] Building MarketplacePoster.exe...
go build -trimpath -ldflags "-H=windowsgui -s -w" -o "dist\MarketplacePoster.exe" .
if errorlevel 1 (
  echo.
  echo ERROR: Build failed. Copy the compiler error above.
  pause
  popd
  exit /b 1
)
echo.
echo ==================================================
echo SUCCESS
 echo EXE: %CD%\dist\MarketplacePoster.exe
echo ==================================================
start "" "%CD%\dist"
pause
popd
