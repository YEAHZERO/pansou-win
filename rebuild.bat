@echo off
chcp 65001 >nul
title Rebuild PanSou

echo ========================================
echo Rebuild PanSou
echo ========================================
echo.

echo Building...
go build -o pansou.exe main.go

if %ERRORLEVEL% EQU 0 (
    echo.
    echo ========================================
    echo Build Successful!
    echo ========================================
    echo.
    echo Executable: pansou.exe
    echo.
    echo Run command: pansou.exe
    echo Or use: start-pansou.bat
    echo.
) else (
    echo.
    echo ========================================
    echo Build Failed! Please check error messages
    echo ========================================
    echo.
)

pause
