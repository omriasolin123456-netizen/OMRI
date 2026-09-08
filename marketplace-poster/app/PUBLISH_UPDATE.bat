@echo off
setlocal EnableExtensions
pushd "%~dp0"
echo Marketplace Poster - Publish Update
echo.
set /p VER=New version (example 2.0.2): 
if "%VER%"=="" exit /b 1
set /p NOTES=Short release notes: 
>VERSION echo %VER%
>RELEASE_NOTES.txt echo %NOTES%
powershell -NoProfile -Command "$p='models.go'; $s=Get-Content $p -Raw; $s=[regex]::Replace($s,'const AppVersion = \"[^\"]+\"','const AppVersion = \"%VER%\"'); [IO.File]::WriteAllText((Resolve-Path $p),$s,(New-Object Text.UTF8Encoding($false)))"
git add .
git commit -m "Marketplace Poster %VER%"
git push origin marketplace-poster-updates
echo.
echo GitHub Actions will build and publish the Windows update automatically.
pause
popd
