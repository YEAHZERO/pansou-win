@echo off
chcp 65001 >nul
title PanSou Universal Deploy Script

echo ================================
echo   PanSou Universal Deploy Script
echo ================================
echo.

REM Get current script directory
set CURRENT_DIR=%~dp0
REM Remove trailing backslash
set CURRENT_DIR=%CURRENT_DIR:~0,-1%

echo Current directory: %CURRENT_DIR%
echo.

REM Step 1: Check if pansou.exe exists
echo [1/4] Checking executable...
if not exist "%CURRENT_DIR%\pansou.exe" (
    echo [X] pansou.exe not found
    echo.
    echo Please build the project first:
    echo   go build -o pansou.exe .
    echo.
    echo Or download pre-compiled version from GitHub Releases
    pause
    exit /b 1
)
echo [OK] Found pansou.exe
echo.

REM Step 2: Stop existing service
echo [2/4] Checking and stopping existing service...
tasklist | findstr /I "pansou.exe" >nul
if %errorlevel% equ 0 (
    echo [!] Running service detected, stopping...
    taskkill /F /IM pansou.exe 2>nul
    if %errorlevel% equ 0 (
        echo [OK] Stopped existing service
        timeout /t 2 /nobreak >nul
    ) else (
        echo [!] Cannot stop service, please close manually and retry
        pause
        exit /b 1
    )
) else (
    echo [i] No running service found
)
echo.

REM Step 3: Create necessary directories
echo [3/4] Creating necessary directories...
if not exist "%CURRENT_DIR%\cache" (
    mkdir "%CURRENT_DIR%\cache"
    echo [OK] Created cache directory
) else (
    echo [i] Cache directory already exists
)
echo.

REM Step 4: Verify deployment
echo [4/4] Verifying deployment...
if not exist "%CURRENT_DIR%\pansou.exe" (
    echo [X] Verification failed: pansou.exe not found
    pause
    exit /b 1
)

if not exist "%CURRENT_DIR%\start-pansou.bat" (
    echo [!] start-pansou.bat not found, will use start.bat
)

echo [OK] Deployment verification passed
echo.

REM Display deployment summary
echo ================================
echo   Deployment Successful!
echo ================================
echo.
echo Deployment Info:
echo   Deploy Dir: %CURRENT_DIR%
echo   Executable: pansou.exe
echo   Cache Dir: cache\
echo.
echo Available start scripts:
if exist "%CURRENT_DIR%\start-pansou.bat" (
    echo   - start-pansou.bat (Recommended, simple config)
)
if exist "%CURRENT_DIR%\start.bat" (
    echo   - start.bat (Full config)
)
echo.

REM Ask if start service now
echo Start service now?
echo   [Y] Yes, start service
echo   [N] No, start manually later
echo.
choice /C YN /N /M "Please choose (Y/N): "

if errorlevel 2 (
    echo.
    echo [i] You can start service later with:
    if exist "%CURRENT_DIR%\start-pansou.bat" (
        echo   start-pansou.bat
    ) else (
        echo   start.bat
    )
    echo.
    pause
    exit /b 0
)

if errorlevel 1 (
    echo.
    echo Starting service...
    echo.
    cd /d "%CURRENT_DIR%"
    
    REM Prefer start-pansou.bat
    if exist "%CURRENT_DIR%\start-pansou.bat" (
        start "" "%CURRENT_DIR%\start-pansou.bat"
    ) else if exist "%CURRENT_DIR%\start.bat" (
        start "" "%CURRENT_DIR%\start.bat"
    ) else (
        echo [X] Start script not found
        pause
        exit /b 1
    )
    
    echo.
    echo [OK] Service started
    echo.
    echo Access URL: http://localhost:8889
    echo Health Check: http://localhost:8889/api/health
    echo.
    timeout /t 3 /nobreak >nul
    exit /b 0
)
