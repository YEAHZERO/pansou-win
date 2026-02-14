@echo off
chcp 65001 >nul
title PanSou - Build (Optimized)

echo ================================
echo   PanSou - Optimized Build
echo ================================
echo:

cd /d "%~dp0"

echo [1/3] Cleaning old build...
if exist "pansou.exe" del /f /q "pansou.exe"

echo [2/3] Building with optimizations...
echo     - Removing debug info (-ldflags "-s -w")
echo     - Trimming paths (-trimpath)
echo:

go build -ldflags "-s -w" -trimpath -o pansou.exe .

if %errorlevel% neq 0 (
    echo:
    echo [X] Build failed!
    pause
    exit /b 1
)

echo:
echo [3/3] Build complete!
echo:

for %%A in ("pansou.exe") do echo Size: %%~zA bytes

echo:
echo Run 'start.bat' to launch the service
pause
