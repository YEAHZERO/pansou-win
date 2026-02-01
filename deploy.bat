@echo off
chcp 65001 >nul
title PanSou 自动部署脚本

echo ================================
echo   PanSou 自动部署脚本
echo ================================
echo.

REM 配置路径
set SOURCE_DIR=C:\Projects\PT\pansou-main
set DEPLOY_DIR=C:\Users\Administrator\pansou

echo 配置信息:
echo   源目录: %SOURCE_DIR%
echo   部署目录: %DEPLOY_DIR%
echo.

REM 步骤 1: 检查源目录
echo [1/6] 检查源目录...
if not exist "%SOURCE_DIR%" (
    echo ❌ 源目录不存在: %SOURCE_DIR%
    pause
    exit /b 1
)

if not exist "%SOURCE_DIR%\pansou.exe" (
    echo ❌ 源目录中未找到 pansou.exe
    echo.
    echo 请先编译项目:
    echo   cd %SOURCE_DIR%
    echo   go build -o pansou.exe
    echo.
    pause
    exit /b 1
)

echo ✅ 源文件检查通过
echo.

REM 步骤 2: 检查部署目录
echo [2/6] 检查部署目录...
if not exist "%DEPLOY_DIR%" (
    echo ⚠️  部署目录不存在，正在创建...
    mkdir "%DEPLOY_DIR%"
    if %errorlevel% neq 0 (
        echo ❌ 创建部署目录失败，请检查权限
        pause
        exit /b 1
    )
    echo ✅ 已创建部署目录
)
echo ✅ 部署目录检查通过
echo.

REM 步骤 3: 停止现有服务
echo [3/6] 停止现有服务...
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

REM 步骤 4: 备份现有文件
echo [4/6] 备份现有文件...
if exist "%DEPLOY_DIR%\pansou.exe" (
    set BACKUP_NAME=pansou.exe.backup_%date:~0,4%%date:~5,2%%date:~8,2%_%time:~0,2%%time:~3,2%%time:~6,2%
    set BACKUP_NAME=%BACKUP_NAME: =0%
    copy /Y "%DEPLOY_DIR%\pansou.exe" "%DEPLOY_DIR%\%BACKUP_NAME%" >nul
    if %errorlevel% equ 0 (
        echo ✅ 已备份现有文件: %BACKUP_NAME%
    ) else (
        echo ⚠️  备份失败，但继续部署
    )
) else (
    echo ℹ️  没有需要备份的文件（首次部署）
)
echo.

REM 步骤 5: 复制新文件
echo [5/6] 复制新文件...

echo   - 复制可执行文件...
copy /Y "%SOURCE_DIR%\pansou.exe" "%DEPLOY_DIR%\pansou.exe" >nul
if %errorlevel% neq 0 (
    echo ❌ 复制 pansou.exe 失败
    pause
    exit /b 1
)

echo   - 复制启动脚本...
copy /Y "%SOURCE_DIR%\start.bat" "%DEPLOY_DIR%\start.bat" >nul
if %errorlevel% neq 0 (
    echo ❌ 复制 start.bat 失败
    pause
    exit /b 1
)

echo   - 复制插件目录...
if exist "%SOURCE_DIR%\plugin" (
    xcopy /E /I /Y /Q "%SOURCE_DIR%\plugin" "%DEPLOY_DIR%\plugin" >nul
    if %errorlevel% neq 0 (
        echo ⚠️  复制插件目录失败，但继续部署
    )
) else (
    echo ⚠️  源目录中没有 plugin 文件夹
)

echo   - 复制文档目录...
if exist "%SOURCE_DIR%\docs" (
    xcopy /E /I /Y /Q "%SOURCE_DIR%\docs" "%DEPLOY_DIR%\docs" >nul
    if %errorlevel% neq 0 (
        echo ⚠️  复制文档目录失败，但继续部署
    )
)

echo   - 复制其他必要文件...
if exist "%SOURCE_DIR%\README.md" (
    copy /Y "%SOURCE_DIR%\README.md" "%DEPLOY_DIR%\README.md" >nul 2>&1
)

echo ✅ 文件复制完成
echo.

REM 步骤 6: 验证部署
echo [6/6] 验证部署...
if not exist "%DEPLOY_DIR%\pansou.exe" (
    echo ❌ 部署失败: 未找到 pansou.exe
    pause
    exit /b 1
)

if not exist "%DEPLOY_DIR%\start.bat" (
    echo ❌ 部署失败: 未找到 start.bat
    pause
    exit /b 1
)

echo ✅ 部署验证通过
echo.

REM 显示部署摘要
echo ================================
echo   部署成功！
echo ================================
echo.
echo 部署信息:
echo   部署目录: %DEPLOY_DIR%
echo   可执行文件: pansou.exe
echo   启动脚本: start.bat
echo   服务端口: 8889
echo   pioz 插件: 已启用（优先级最高）
echo.
echo 配置详情:
echo   - 端口: 8889
echo   - 认证: 启用（用户名: admin, 密码: 123456）
echo   - 缓存: 启用
echo   - 插件顺序: pioz 优先
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
    echo   cd %DEPLOY_DIR%
    echo   start.bat
    echo.
    pause
    exit /b 0
)

if errorlevel 1 (
    echo.
    echo 🚀 正在启动服务...
    echo.
    cd /d "%DEPLOY_DIR%"
    start "" "%DEPLOY_DIR%\start.bat"
    echo.
    echo ✅ 服务已启动
    echo.
    echo 访问地址: http://localhost:8889
    echo 健康检查: http://localhost:8889/api/health
    echo.
    timeout /t 3 /nobreak >nul
    exit /b 0
)
