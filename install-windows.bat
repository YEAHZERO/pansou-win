@echo off
chcp 65001 >nul
title PanSou Windows Installation Wizard

echo ================================
echo    PanSou Windows Installation
echo ================================
echo.

REM Check admin privileges
net session >nul 2>&1
if %errorlevel% neq 0 (
    echo Warning: Recommend running as administrator
    echo.
)

echo Checking system environment...
echo.

REM Check Go environment
echo Checking Go:
go version >nul 2>&1
if %errorlevel% equ 0 (
    echo [OK] Go installed
    go version
) else (
    echo [X] Go not installed
    echo Please visit https://golang.org/dl/ to download Go
    echo.
)

REM Check Git environment
echo.
echo Checking Git:
git --version >nul 2>&1
if %errorlevel% equ 0 (
    echo [OK] Git installed
    git --version
) else (
    echo [X] Git not installed
    echo Please visit https://git-scm.com/ to download Git
    echo.
)

echo.
echo ================================
echo    Choose Installation Method
echo ================================
echo.
echo 1. Build from source (Recommended, requires Go)
echo 2. Use pre-compiled binary
echo 3. Exit
echo.

set /p choice="Please choose (1-3): "

if "%choice%"=="1" goto source_install
if "%choice%"=="2" goto binary_install
if "%choice%"=="3" goto end
goto invalid_choice

:source_install
echo.
echo Building from source...
echo ================================
echo.

go version >nul 2>&1
if %errorlevel% neq 0 (
    echo [X] Go not installed, cannot use this method
    goto end
)

if not exist "go.mod" (
    echo [X] go.mod not found
    echo Please run this script in PanSou project root directory
    goto end
)

echo Downloading dependencies...
go mod download

if %errorlevel% neq 0 (
    echo [X] Failed to download dependencies, trying with proxy...
    go env -w GOPROXY=https://goproxy.cn,direct
    go mod download
)

echo Building project...
go build -ldflags="-s -w" -o pansou.exe .

if %errorlevel% equ 0 (
    echo [OK] Build successful
    echo.
    echo Generated files:
    echo   - pansou.exe (main program)
    echo   - start-pansou.bat (start script)
    echo   - test-pansou.bat (test script)
    echo.
    echo You can now start the service with:
    echo   start-pansou.bat
    echo.
    echo Or manually:
    echo   pansou.exe
    echo.
    echo More info: docs\Windows安装部署指南.md
) else (
    echo [X] Build failed
    echo.
    echo Possible solutions:
    echo 1. Check Go version ^>= 1.21
    echo 2. Check network connection
    echo 3. Try cleaning module cache: go clean -modcache
)
goto end

:binary_install
echo.
echo Pre-compiled binary installation
echo ================================
echo.
echo Please follow these steps manually:
echo.
echo 1. Visit GitHub Releases page:
echo    https://github.com/fish2018/pansou/releases
echo.
echo 2. Download latest Windows version:
echo    pansou-windows-amd64.exe
echo.
echo 3. Rename file to pansou.exe
echo.
echo 4. Place in current directory
echo.
echo 5. Run start script:
echo    start-pansou.bat
echo.
echo Details: docs\Windows安装部署指南.md
echo.
goto end

:invalid_choice
echo [X] Invalid choice, please run script again
goto end

:end
echo.
echo Related documentation:
echo   - docs\Windows安装部署指南.md (Installation guide)
echo   - docs\纯API使用指南.md (API usage)
echo   - api-client-examples\ (Client examples)
echo.
echo After installation you can:
echo   1. Visit http://localhost:8889/api/health to check service
echo   2. Use client tools for searching
echo   3. Read documentation for more features
echo.
echo Installation wizard finished
pause
