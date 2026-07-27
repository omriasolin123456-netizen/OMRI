@echo off
chcp 65001 >nul
cd /d "%~dp0"

echo === Microinvest Parts Assistant: установка ===
where python >nul 2>&1
if errorlevel 1 (
  where py >nul 2>&1
  if errorlevel 1 (
    echo Python не найден в PATH. Установите Python 3.10+ с python.org
    echo Обязательно отметьте "Add python.exe to PATH"
    pause
    exit /b 1
  )
  set PY=py -3
) else (
  set PY=python
)

echo Используем: %PY%
%PY% -m pip install --upgrade pip
%PY% -m pip install -r requirements.txt
if errorlevel 1 (
  echo Ошибка установки зависимостей.
  pause
  exit /b 1
)

if not exist "price" mkdir price
if not exist "logs" mkdir logs

echo.
echo Проверка импорта...
%PY% -c "import pandas, openpyxl, requests; print('OK', __import__('sys').version)"
if errorlevel 1 (
  echo Импорт не удался.
  pause
  exit /b 1
)

echo.
echo Готово.
echo 1. Положите свои .xlsx прайсы в папку price\
echo 2. При необходимости отредактируйте config.ini
echo 3. Запустите MicroinvestAssistant.ahk
echo 4. В трее: "Проверить Python" — убедиться что всё работает
echo 5. В карточке товара Microinvest нажмите F8
pause
