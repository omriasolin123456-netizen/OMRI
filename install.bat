@echo off
chcp 65001 >nul
cd /d "%~dp0"

echo === Microinvest Parts Assistant: установка ===
where python >nul 2>&1
if errorlevel 1 (
  echo Python не найден в PATH. Установите Python 3.10+ с python.org
  pause
  exit /b 1
)

python -m pip install --upgrade pip
python -m pip install -r requirements.txt

if not exist "price" mkdir price
if not exist "logs" mkdir logs

echo.
echo Готово.
echo 1. Положите свои .xlsx прайсы в папку price\
echo 2. При необходимости отредактируйте config.ini (горячая клавиша, ClassNN, FAPI/НКТ ключи)
echo 3. Запустите MicroinvestAssistant.ahk (нужен AutoHotkey 1.1+)
echo 4. В карточке товара Microinvest нажмите F8
pause
