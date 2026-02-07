@echo off
chcp 65001 >nul
title Stop PanSou Service

echo ================================
echo   Stop PanSou Service
echo ================================
echo:

echo Stopping PanSou service...
echo:

REM Find and kill pansou.exe process
tasklist | findstr /I "pansou.exe" >nul
if %errorlevel% equ 0 (
    echo Found PanSou process, stopping...
    taskkill /F /IM pansou.exe
    if %errorlevel% equ 0 (
        echo [OK] PanSou service stopped
    ) else (
        echo [X] Failed to stop PanSou service
    )
) else (
    echo [!] PanSou service is not running
)

echo:
echo Checking port 8889...
netstat -an | findstr ":8889" >nul
if %errorlevel% equ 0 (
    echo [!] Port 8889 is still occupied
    echo:
    echo Finding process using port 8889...
    for /f "tokens=5" %%a in ('netstat -ano ^| findstr ":8889"') do (
        echo Process ID: %%a
        tasklist /fi "PID eq %%a" 2>nul | findstr /v "INFO:"
    )
) else (
    echo [OK] Port 8889 is free
)

echo:
echo ================================
echo Done!
echo ================================
pause
