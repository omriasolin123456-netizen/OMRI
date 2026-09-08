@echo off
setlocal EnableExtensions
set "INSTALLDIR=%LOCALAPPDATA%\MarketplacePosterApp"
set "DESKTOPLINK=%USERPROFILE%\Desktop\Marketplace Poster.lnk"
set "STARTLINK=%APPDATA%\Microsoft\Windows\Start Menu\Programs\Marketplace Poster.lnk"
echo This removes the installed EXE and shortcuts.
echo Your ads/settings/history in %%APPDATA%%\MarketplacePoster are NOT deleted.
choice /M "Continue"
if errorlevel 2 exit /b 0
del /Q "%DESKTOPLINK%" 2>nul
del /Q "%STARTLINK%" 2>nul
rmdir /S /Q "%INSTALLDIR%" 2>nul
echo Uninstalled. Your data was preserved.
pause
