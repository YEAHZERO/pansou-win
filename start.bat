@echo off
chcp 65001 >nul
title PanSou Service - Simple Start

echo ================================
echo   PanSou Service - Simple Start
echo ================================
echo:

REM Get script directory
set SCRIPT_DIR=%~dp0
set SCRIPT_DIR=%SCRIPT_DIR:~0,-1%

REM Set PanSou executable path
set PANSOU_PATH=%SCRIPT_DIR%\pansou.exe

REM Check if executable exists
if not exist "%PANSOU_PATH%" (
    echo [X] pansou.exe not found
    echo Path: %PANSOU_PATH%
    echo:
    echo Please compile first: go build -o pansou.exe
    pause
    exit /b 1
)

REM Stop old service if running
tasklist | findstr /I "pansou.exe" >nul
if %errorlevel% equ 0 (
    echo Stopping old service...
    taskkill /F /IM pansou.exe >nul 2>&1
    timeout /t 2 /nobreak >nul
)

REM Basic configuration
set PORT=8889
set CACHE_ENABLED=true
set CACHE_PATH=%SCRIPT_DIR%\cache
set TZ=Asia/Shanghai

REM Telegram channels DISABLED
set CHANNELS=

REM Plugin configuration - MUST SET THIS
set ASYNC_PLUGIN_ENABLED=true
set ENABLED_PLUGINS=pioz,pansearch,wanou,xdpan,zhizhen

REM Performance configuration optimized
set CONCURRENCY=15
set ASYNC_RESPONSE_TIMEOUT=10
set ASYNC_MAX_BACKGROUND_WORKERS=16
set ASYNC_MAX_BACKGROUND_TASKS=80
set CACHE_MAX_SIZE=500
set CACHE_TTL=120
set PLUGIN_TIMEOUT=20

REM Authentication
set AUTH_ENABLED=true
set AUTH_USERS=admin:123456
set AUTH_JWT_SECRET=pansou-secret-key-2024

REM HTTP server configuration
set HTTP_READ_TIMEOUT=40
set HTTP_WRITE_TIMEOUT=40
set HTTP_IDLE_TIMEOUT=120
set HTTP_MAX_CONNS=300

echo Configuration:
echo ================================
echo Port: %PORT%
echo Telegram: DISABLED
echo Plugins: %ENABLED_PLUGINS%
echo Timeout: %ASYNC_RESPONSE_TIMEOUT% seconds
echo Workers: %ASYNC_MAX_BACKGROUND_WORKERS%
echo Cache: %CACHE_MAX_SIZE% MB / %CACHE_TTL% minutes
echo ================================
echo:

REM Create cache directory
if not exist "%CACHE_PATH%" (
    mkdir "%CACHE_PATH%"
    echo [OK] Created cache directory
)

REM Set working directory
cd /d "%SCRIPT_DIR%"

echo Starting PanSou service...
echo:
echo Service Info:
echo   URL: http://localhost:%PORT%
echo   Health: http://localhost:%PORT%/api/health
echo   User: admin / Password: 123456
echo:
echo Press Ctrl+C to stop
echo:

REM Start service
"%PANSOU_PATH%"

echo:
echo Service stopped
pause
