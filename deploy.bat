@echo off
chcp 65001 >nul
title PanSou Auto Deploy Script

echo ================================
echo   PanSou Auto Deploy Script
echo ================================
echo.

REM Configuration paths
set SOURCE_DIR=C:\Projects\PT\pansou-main
set DEPLOY_DIR=C:\Users\Administrator\pansou

echo Configuration:
echo   Source Dir: %SOURCE_DIR%
echo   Deploy Dir: %DEPLOY_DIR%
echo.

REM Step 1: Check source directory
echo [1/6] Checking source directory...
if not exist "%SOURCE_DIR%" (
    echo [X] Source directory not found: %SOURCE_DIR%
    pause
    exit /b 1
)

if not exist "%SOURCE_DIR%\pansou.exe" (
    echo [X] pansou.exe not found in source directory
    echo.
    echo Please build the project first:
    echo   cd %SOURCE_DIR%
    echo   go build -o pansou.exe
    echo.
    pause
    exit /b 1
)

echo [OK] Source files check passed
echo.

REM Step 2: Check deploy directory
echo [2/6] Checking deploy directory...
if not exist "%DEPLOY_DIR%" (
    echo [!] Deploy directory not found, creating...
    mkdir "%DEPLOY_DIR%"
    if %errorlevel% neq 0 (
        echo [X] Failed to create deploy directory, check permissions
        pause
        exit /b 1
    )
    echo [OK] Created deploy directory
)
echo [OK] Deploy directory check passed
echo.

REM Step 3: Stop existing service
echo [3/6] Stopping existing service...
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

REM Step 4: Backup existing files
echo [4/6] Backing up existing files...
if exist "%DEPLOY_DIR%\pansou.exe" (
    set BACKUP_NAME=pansou.exe.backup_%date:~0,4%%date:~5,2%%date:~8,2%_%time:~0,2%%time:~3,2%%time:~6,2%
    set BACKUP_NAME=%BACKUP_NAME: =0%
    copy /Y "%DEPLOY_DIR%\pansou.exe" "%DEPLOY_DIR%\%BACKUP_NAME%" >nul
    if %errorlevel% equ 0 (
        echo [OK] Backed up existing file: %BACKUP_NAME%
    ) else (
        echo [!] Backup failed, but continuing deployment
    )
) else (
    echo [i] No files to backup (first deployment)
)
echo.

REM Step 5: Copy new files
echo [5/6] Copying new files...

echo   - Copying executable...
copy /Y "%SOURCE_DIR%\pansou.exe" "%DEPLOY_DIR%\pansou.exe" >nul
if %errorlevel% neq 0 (
    echo [X] Failed to copy pansou.exe
    pause
    exit /b 1
)

echo   - Copying start script...
copy /Y "%SOURCE_DIR%\start.bat" "%DEPLOY_DIR%\start.bat" >nul
if %errorlevel% neq 0 (
    echo [X] Failed to copy start.bat
    pause
    exit /b 1
)

echo   - Copying plugin directory...
if exist "%SOURCE_DIR%\plugin" (
    xcopy /E /I /Y /Q "%SOURCE_DIR%\plugin" "%DEPLOY_DIR%\plugin" >nul
    if %errorlevel% neq 0 (
        echo [!] Failed to copy plugin directory, but continuing
    )
) else (
    echo [!] No plugin folder in source directory
)

echo   - Copying docs directory...
if exist "%SOURCE_DIR%\docs" (
    xcopy /E /I /Y /Q "%SOURCE_DIR%\docs" "%DEPLOY_DIR%\docs" >nul
    if %errorlevel% neq 0 (
        echo [!] Failed to copy docs directory, but continuing
    )
)

echo   - Copying other necessary files...
if exist "%SOURCE_DIR%\README.md" (
    copy /Y "%SOURCE_DIR%\README.md" "%DEPLOY_DIR%\README.md" >nul 2>&1
)

echo [OK] File copy completed
echo.

REM Step 6: Verify deployment
echo [6/6] Verifying deployment...
if not exist "%DEPLOY_DIR%\pansou.exe" (
    echo [X] Deployment failed: pansou.exe not found
    pause
    exit /b 1
)

if not exist "%DEPLOY_DIR%\start.bat" (
    echo [X] Deployment failed: start.bat not found
    pause
    exit /b 1
)

echo [OK] Deployment verification passed
echo.

REM Display deployment summary
echo ================================
echo   Deployment Successful!
echo ================================
echo.
echo Deployment Info:
echo   Deploy Dir: %DEPLOY_DIR%
echo   Executable: pansou.exe
echo   Start Script: start.bat
echo   Service Port: 8889
echo   pioz Plugin: Enabled (Highest Priority)
echo.
echo Configuration Details:
echo   - Port: 8889
echo   - Auth: Enabled (Username: admin, Password: 123456)
echo   - Cache: Enabled
echo   - Plugin Order: pioz first
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
    echo   cd %DEPLOY_DIR%
    echo   start.bat
    echo.
    pause
    exit /b 0
)

if errorlevel 1 (
    echo.
    echo Starting service...
    echo.
    cd /d "%DEPLOY_DIR%"
    start "" "%DEPLOY_DIR%\start.bat"
    echo.
    echo [OK] Service started
    echo.
    echo Access URL: http://localhost:8889
    echo Health Check: http://localhost:8889/api/health
    echo.
    timeout /t 3 /nobreak >nul
    exit /b 0
)
