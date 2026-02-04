@echo off
chcp 65001 >nul
title PanSou 通用部署脚本

echo ================================
echo   PanSou 通用部署脚本
echo ================================
echo.

REM 获取当前脚本所在目录
set CURRENT_DIR=%~dp0
REM 移除末尾的反斜杠
set CURRENT_DIR=%CURRENT_DIR:~0,-1%

echo 当前目录: %CURRENT_DIR%
echo.

REM 步骤 1: 检查是否存在 pansou.exe
echo [1/4] 检查可执行文件...
if not exist "%CURRENT_DIR%\pansou.exe" (
    echo ❌ 未找到 pansou.exe
    echo.
    echo 请先编译项目:
    echo   go build -o pansou.exe .
    echo.
    echo 或从 GitHub Releases 下载预编译版本
    pause
    exit /b 1
)
echo ✅ 找到 pansou.exe
echo.

REM 步骤 2: 停止现有服务
echo [2/4] 检查并停止现有服务...
tasklist | findstr /I "pansou.exe" >nul
if %errorlevel% equ 0 (
    echo ⚠️  检测到运行中的服务，正在停止...
    taskkill /F /IM pansou.exe 2>nul
    if %errorlevel% equ 0 (
        echo ✅ 已停止现有服务
        timeout /t 2 /nobreak >nul
    ) else (
        echo ⚠️  无法停止服务，请手动关闭后重试
        pause
        exit /b 1
    )
) else (
    echo ℹ️  没有运行中的服务
)
echo.

REM 步骤 3: 创建必要的目录
echo [3/4] 创建必要的目录...
if not exist "%CURRENT_DIR%\cache" (
    mkdir "%CURRENT_DIR%\cache"
    echo ✅ 已创建缓存目录
) else (
    echo ℹ️  缓存目录已存在
)
echo.

REM 步骤 4: 验证部署
echo [4/4] 验证部署...
if not exist "%CURRENT_DIR%\pansou.exe" (
    echo ❌ 验证失败: 未找到 pansou.exe
    pause
    exit /b 1
)

if not exist "%CURRENT_DIR%\start-pansou.bat" (
    echo ⚠️  未找到 start-pansou.bat，将使用 start.bat
)

echo ✅ 部署验证通过
echo.

REM 显示部署摘要
echo ================================
echo   部署成功！
echo ================================
echo.
echo 部署信息:
echo   部署目录: %CURRENT_DIR%
echo   可执行文件: pansou.exe
echo   缓存目录: cache\
echo.
echo 可用的启动脚本:
if exist "%CURRENT_DIR%\start-pansou.bat" (
    echo   - start-pansou.bat (推荐，简单配置)
)
if exist "%CURRENT_DIR%\start.bat" (
    echo   - start.bat (完整配置)
)
echo.

REM 询问是否启动服务
echo 是否立即启动服务？
echo   [Y] 是，启动服务
echo   [N] 否，稍后手动启动
echo.
choice /C YN /N /M "请选择 (Y/N): "

if errorlevel 2 (
    echo.
    echo ℹ️  稍后可以运行以下命令启动服务:
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
    echo 🚀 正在启动服务...
    echo.
    cd /d "%CURRENT_DIR%"
    
    REM 优先使用 start-pansou.bat
    if exist "%CURRENT_DIR%\start-pansou.bat" (
        start "" "%CURRENT_DIR%\start-pansou.bat"
    ) else if exist "%CURRENT_DIR%\start.bat" (
        start "" "%CURRENT_DIR%\start.bat"
    ) else (
        echo ❌ 未找到启动脚本
        pause
        exit /b 1
    )
    
    echo.
    echo ✅ 服务已启动
    echo.
    echo 访问地址: http://localhost:8888
    echo 健康检查: http://localhost:8888/api/health
    echo.
    timeout /t 3 /nobreak >nul
    exit /b 0
)
