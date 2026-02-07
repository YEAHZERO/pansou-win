@echo off
chcp 65001 >nul
title PanSou Production Service

echo ================================
echo    PanSou Production Service
echo ================================
echo.

REM Get script directory
set SCRIPT_DIR=%~dp0
REM Remove trailing backslash
set SCRIPT_DIR=%SCRIPT_DIR:~0,-1%

REM Set PanSou executable path (use absolute path)
set PANSOU_PATH=%SCRIPT_DIR%\pansou.exe

REM Check if executable exists
if not exist "%PANSOU_PATH%" (
    echo [X] PanSou executable not found
    echo Path: %PANSOU_PATH%
    echo.
    echo Please check:
    echo 1. File path is correct
    echo 2. pansou.exe has been compiled
    echo 3. You have access permissions
    pause
    exit /b 1
)

REM Basic configuration
set PORT=8889
set CACHE_ENABLED=true
set CACHE_PATH=%SCRIPT_DIR%\cache
set TZ=Asia/Shanghai

REM Check if port is occupied
echo Checking if port %PORT% is available...
netstat -an | findstr ":%PORT% " >nul
if %errorlevel% equ 0 (
    echo [!] Port %PORT% is occupied
    echo.
    echo Finding process using this port...
    for /f "tokens=5" %%a in ('netstat -ano ^| findstr ":%PORT% "') do (
        echo Process ID: %%a
        tasklist /fi "PID eq %%a" 2>nul | findstr /v "INFO:"
    )
    echo.
    echo Solutions:
    echo 1. Close the process using this port
    echo 2. Or use a different port (8889)
    echo 3. Or press Ctrl+C to exit
    echo.
    pause
    set PORT=8889
    echo [OK] Switched to port %PORT%
    echo.
)

REM Telegram channels (DISABLED - not using Telegram)
set CHANNELS=

REM Plugin configuration (only pioz enabled, others are backup)
set ASYNC_PLUGIN_ENABLED=true
set ENABLED_PLUGINS=pioz

REM Performance configuration (optimized for Pioz)
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

REM HTTP server configuration (optimized for Pioz)
set HTTP_READ_TIMEOUT=40
set HTTP_WRITE_TIMEOUT=40
set HTTP_IDLE_TIMEOUT=120
set HTTP_MAX_CONNS=300

echo Configuration:
echo ================================
echo Executable: %PANSOU_PATH%
echo Port: %PORT%
echo Cache Dir: %CACHE_PATH%
echo Cache Enabled: %CACHE_ENABLED%
echo Async System: %ASYNC_PLUGIN_ENABLED%
echo Auth Enabled: %AUTH_ENABLED%
echo Auth User: admin
echo Telegram Channels: %CHANNELS%
echo Enabled Plugins: %ENABLED_PLUGINS%
echo ================================
echo.

REM Create cache directory
if not exist "%CACHE_PATH%" (
    mkdir "%CACHE_PATH%"
    echo [OK] Created cache directory: %CACHE_PATH%
)

REM Set working directory to script directory
cd /d "%SCRIPT_DIR%"

echo Starting PanSou service...
echo.
echo Service Info:
echo   Access URL: http://localhost:%PORT%
echo   Health Check: http://localhost:%PORT%/api/health
echo   Auth User: admin
echo   Auth Password: 123456
echo.
echo [!] Authentication enabled, API requires login to get Token
echo   Token is valid for 7 days
echo.
echo Login example:
echo   curl -X POST http://localhost:%PORT%/api/auth/login \
echo     -H "Content-Type: application/json" \
echo     -d "{\"username\":\"admin\",\"password\":\"123456\"}"
echo.
echo Press Ctrl+C to stop service
echo.

REM Start service
"%PANSOU_PATH%"

echo.
echo Service stopped
pause
