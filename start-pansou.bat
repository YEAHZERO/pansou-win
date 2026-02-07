@echo off
chcp 65001 >nul
title PanSou Service Startup

echo ================================
echo    PanSou Service Startup
echo ================================
echo.

REM Check if pansou.exe exists
if not exist "pansou.exe" (
    echo [X] pansou.exe not found
    echo.
    echo Please build the project first:
    echo   go build -o pansou.exe .
    echo.
    echo Or download from GitHub Releases
    pause
    exit /b 1
)

REM Environment configuration
set PORT=8889
set CACHE_ENABLED=true
set CACHE_PATH=.\cache
set ASYNC_PLUGIN_ENABLED=true
set ENABLED_PLUGINS=pioz,labi,zhizhen,shandian,duoduo,muou,wanou,hunhepan,jikepan,pansearch,panta,qupansou
set TZ=Asia/Shanghai
set ASYNC_RESPONSE_TIMEOUT=4
set ASYNC_MAX_BACKGROUND_WORKERS=10
set ASYNC_MAX_BACKGROUND_TASKS=50

REM Optional: Authentication (uncomment to enable)
REM set AUTH_ENABLED=true
REM set AUTH_USERS=admin:your_password

echo Configuration:
echo Port: %PORT%
echo Cache: %CACHE_ENABLED%
echo Cache Path: %CACHE_PATH%
echo Async System: %ASYNC_PLUGIN_ENABLED%
echo Enabled Plugins: %ENABLED_PLUGINS%
echo Timezone: %TZ%
echo.

REM Create cache directory
if not exist "%CACHE_PATH%" (
    mkdir "%CACHE_PATH%"
    echo [OK] Created cache directory: %CACHE_PATH%
)

echo Starting service...
echo Access URL: http://localhost:%PORT%
echo Health check: http://localhost:%PORT%/api/health
echo.
echo Press Ctrl+C to stop service
echo.

REM Start service
pansou.exe

echo.
echo Service stopped
pause
