@echo off
chcp 65001 >nul
title PanSou Diagnostics

echo ================================
echo    PanSou Diagnostics
echo ================================
echo.

echo [1] Checking service status...
echo.

REM Check if service is running
netstat -an | findstr ":8889" >nul
if %errorlevel% equ 0 (
    echo [OK] Service is running on port 8889
) else (
    echo [X] Service is NOT running
    echo Please start the service first: start.bat
    pause
    exit /b 1
)
echo.

echo [2] Checking Pioz website accessibility...
echo.

curl -I -s https://www.pioz.cn/ | findstr "200"
if %errorlevel% equ 0 (
    echo [OK] Pioz website (pioz.cn) is accessible
) else (
    echo [!] Pioz website may not be accessible
    echo This could affect search results
)
echo.

echo [3] Checking health endpoint...
echo.

curl -s http://localhost:8889/api/health
echo.
echo.

echo [4] Checking plugin configuration...
echo.

REM Check main.go for pioz import
findstr /C:"plugin/pioz" main.go >nul
if %errorlevel% equ 0 (
    echo [OK] Pioz plugin is imported in main.go
) else (
    echo [X] Pioz plugin is NOT imported in main.go
)
echo.

REM Check if pioz directory exists
if exist "plugin\pioz\pioz.go" (
    echo [OK] Pioz plugin directory exists
    echo [OK] Using website: https://www.pioz.cn
) else (
    echo [X] Pioz plugin directory NOT found
)
echo.

echo [5] Checking configuration...
echo.

echo Current settings in start.bat:
findstr /C:"ENABLED_PLUGINS" start.bat
findstr /C:"ASYNC_RESPONSE_TIMEOUT" start.bat
findstr /C:"CHANNELS=" start.bat
echo.

echo [6] Checking cache directory...
echo.

if exist "cache" (
    echo [OK] Cache directory exists
    dir cache /b 2>nul | find /c /v "" > temp.txt
    set /p FILE_COUNT=<temp.txt
    del temp.txt
    echo Cache files: %FILE_COUNT%
) else (
    echo [X] Cache directory NOT found
)
echo.

echo [7] Testing quick search...
echo.

REM Login first
curl -s -X POST http://localhost:8889/api/auth/login ^
  -H "Content-Type: application/json" ^
  -d "{\"username\":\"admin\",\"password\":\"123456\"}" ^
  -o token.json 2>nul

if %errorlevel% neq 0 (
    echo [X] Login failed
    goto :end
)

REM Extract token
for /f "tokens=2 delims=:," %%a in ('type token.json ^| findstr "token"') do (
    set TOKEN=%%a
)
set TOKEN=%TOKEN:"=%
set TOKEN=%TOKEN: =%

REM Test search (no Telegram channels)
echo Testing search with keyword: 电影
echo (Telegram channels disabled, only Pioz)
echo.

curl -s -X POST http://localhost:8889/api/search ^
  -H "Authorization: Bearer %TOKEN%" ^
  -H "Content-Type: application/json" ^
  -d "{\"keyword\":\"电影\",\"channels\":[],\"concurrency\":5}" ^
  -o test_result.json

echo.
echo Search completed
echo.

REM Show summary
echo Result Summary:
type test_result.json | findstr "total"
type test_result.json | findstr "pioz"
echo.

del token.json 2>nul

:end
echo.
echo ================================
echo Diagnostics completed!
echo ================================
echo.

echo Key Points:
echo 1. Telegram channels: DISABLED (only using Pioz)
echo 2. Pioz website: https://www.pioz.cn
echo 3. Timeout: 10 seconds (optimized)
echo.

echo If Pioz returns 0 results:
echo 1. Check if pioz.cn is accessible
echo 2. Try different keywords (流浪地球, 美剧, etc.)
echo 3. Check service logs for errors
echo 4. Wait a moment and try again (anti-crawler)
echo.

pause
