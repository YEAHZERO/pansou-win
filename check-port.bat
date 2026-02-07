@echo off
chcp 65001 >nul
title Port Check and Cleanup Tool

echo ================================
echo    Port Check and Cleanup Tool
echo ================================
echo.

set TARGET_PORT=8889

echo Checking port %TARGET_PORT% usage...
echo.

REM Check if port is occupied
netstat -ano | findstr ":%TARGET_PORT% " >nul
if %errorlevel% neq 0 (
    echo [OK] Port %TARGET_PORT% is currently available
    echo.
    pause
    exit /b 0
)

echo [X] Port %TARGET_PORT% is occupied
echo.
echo Occupation details:
echo ================================

REM Display detailed information about port occupation
for /f "tokens=1,2,5" %%a in ('netstat -ano ^| findstr ":%TARGET_PORT% "') do (
    echo Protocol: %%a
    echo Address: %%b
    echo Process ID: %%c
    
    REM Get process name
    for /f "tokens=1" %%d in ('tasklist /fi "PID eq %%c" /fo csv /nh 2^>nul ^| findstr /v "INFO:"') do (
        set PROCESS_NAME=%%d
        set PROCESS_NAME=!PROCESS_NAME:"=!
        echo Process Name: !PROCESS_NAME!
    )
    echo --------------------------------
)

echo.
echo Solutions:
echo ================================
echo 1. Manually close the occupying process
echo 2. Automatically kill the occupying process (use with caution)
echo 3. Use a different port
echo 4. Exit
echo.

set /p choice="Please choose action (1-4): "

if "%choice%"=="1" goto manual_close
if "%choice%"=="2" goto auto_kill
if "%choice%"=="3" goto use_other_port
if "%choice%"=="4" goto end

:manual_close
echo.
echo Manual process closing steps:
echo 1. Open Task Manager (Ctrl+Shift+Esc)
echo 2. Switch to "Details" tab
echo 3. Find the process name and PID shown above
echo 4. Right-click the process, select "End Task"
echo 5. Re-run start.bat
echo.
pause
goto end

:auto_kill
echo.
echo Warning: About to automatically kill the process occupying the port
echo This may affect other running programs
echo.
set /p confirm="Confirm continue? (y/N): "
if /i not "%confirm%"=="y" goto end

echo.
echo Killing occupying process...

for /f "tokens=5" %%a in ('netstat -ano ^| findstr ":%TARGET_PORT% "') do (
    echo Killing process PID: %%a
    taskkill /PID %%a /F >nul 2>&1
    if !errorlevel! equ 0 (
        echo [OK] Process %%a killed
    ) else (
        echo [X] Cannot kill process %%a
    )
)

echo.
echo Rechecking port status...
netstat -ano | findstr ":%TARGET_PORT% " >nul
if %errorlevel% neq 0 (
    echo [OK] Port %TARGET_PORT% is now available
    echo You can run start.bat to start the service
) else (
    echo [X] Port still occupied, please handle manually
)
echo.
pause
goto end

:use_other_port
echo.
echo Recommended alternative ports:
echo   8889 - Recommended
echo   9999 - Alternative
echo   8080 - Common
echo   3000 - Common for development
echo.
echo How to modify:
echo 1. Edit start.bat file
echo 2. Change "set PORT=8889" to "set PORT=9999"
echo 3. Save and re-run
echo.
pause
goto end

:end
echo.
echo Tool finished
pause
