@echo off
setlocal EnableExtensions
pushd "%~dp0" >nul 2>nul
set "SRC=%CD%\dist\MarketplacePoster.exe"
set "INSTALLDIR=%LOCALAPPDATA%\MarketplacePosterApp"
set "TARGET=%INSTALLDIR%\MarketplacePoster.exe"
if not exist "%SRC%" (
  echo MarketplacePoster.exe was not found in dist.
  echo Building it first...
  call "%CD%\BUILD_EXE.bat"
  if not exist "%SRC%" (
    echo ERROR: Build did not produce the EXE.
    pause
    popd
    exit /b 1
  )
)
if not exist "%INSTALLDIR%" mkdir "%INSTALLDIR%"
copy /Y "%SRC%" "%TARGET%" >nul
powershell -NoProfile -ExecutionPolicy Bypass -Command "$w=New-Object -ComObject WScript.Shell; $s=$w.CreateShortcut([Environment]::GetFolderPath('Desktop')+'\Marketplace Poster.lnk'); $s.TargetPath='%TARGET%'; $s.WorkingDirectory='%INSTALLDIR%'; $s.Save(); $sm=[Environment]::GetFolderPath('StartMenu')+'\Programs\Marketplace Poster.lnk'; $s2=$w.CreateShortcut($sm); $s2.TargetPath='%TARGET%'; $s2.WorkingDirectory='%INSTALLDIR%'; $s2.Save()"
if errorlevel 1 (
  echo WARNING: EXE installed, but Windows could not create one of the shortcuts.
)
echo.
echo Installed to:
echo %TARGET%
echo.
start "" "%TARGET%"
pause
popd
