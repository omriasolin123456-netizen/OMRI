@echo off
setlocal EnableExtensions
pushd "%~dp0" >nul 2>nul
call "%CD%\BUILD_EXE.bat"
if not exist "%CD%\dist\MarketplacePoster.exe" (
  echo Build did not produce MarketplacePoster.exe.
  pause
  popd
  exit /b 1
)
call "%CD%\INSTALL.bat"
popd
