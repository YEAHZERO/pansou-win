@echo off
chcp 65001 >nul
title 端口检查和清理工具

echo ================================
echo    端口检查和清理工具
echo ================================
echo.

set TARGET_PORT=8888

echo 🔍 检查端口 %TARGET_PORT% 的使用情况...
echo.

REM 检查端口是否被占用
netstat -ano | findstr ":%TARGET_PORT% " >nul
if %errorlevel% neq 0 (
    echo ✅ 端口 %TARGET_PORT% 当前可用
    echo.
    pause
    exit /b 0
)

echo ❌ 端口 %TARGET_PORT% 已被占用
echo.
echo 占用详情:
echo ================================

REM 显示占用端口的详细信息
for /f "tokens=1,2,5" %%a in ('netstat -ano ^| findstr ":%TARGET_PORT% "') do (
    echo 协议: %%a
    echo 地址: %%b
    echo 进程ID: %%c
    
    REM 获取进程名称
    for /f "tokens=1" %%d in ('tasklist /fi "PID eq %%c" /fo csv /nh 2^>nul ^| findstr /v "INFO:"') do (
        set PROCESS_NAME=%%d
        set PROCESS_NAME=!PROCESS_NAME:"=!
        echo 进程名: !PROCESS_NAME!
    )
    echo --------------------------------
)

echo.
echo 解决方案:
echo ================================
echo 1. 手动关闭占用进程
echo 2. 自动结束占用进程 (谨慎使用)
echo 3. 使用其他端口
echo 4. 退出
echo.

set /p choice="请选择操作 (1-4): "

if "%choice%"=="1" goto manual_close
if "%choice%"=="2" goto auto_kill
if "%choice%"=="3" goto use_other_port
if "%choice%"=="4" goto end

:manual_close
echo.
echo 📋 手动关闭进程步骤:
echo 1. 打开任务管理器 (Ctrl+Shift+Esc)
echo 2. 切换到 "详细信息" 选项卡
echo 3. 找到上面显示的进程名和PID
echo 4. 右键点击进程，选择 "结束任务"
echo 5. 重新运行 start.bat
echo.
pause
goto end

:auto_kill
echo.
echo ⚠️  警告: 即将自动结束占用端口的进程
echo 这可能会影响其他正在运行的程序
echo.
set /p confirm="确认继续? (y/N): "
if /i not "%confirm%"=="y" goto end

echo.
echo 🔄 正在结束占用进程...

for /f "tokens=5" %%a in ('netstat -ano ^| findstr ":%TARGET_PORT% "') do (
    echo 结束进程 PID: %%a
    taskkill /PID %%a /F >nul 2>&1
    if !errorlevel! equ 0 (
        echo ✅ 进程 %%a 已结束
    ) else (
        echo ❌ 无法结束进程 %%a
    )
)

echo.
echo 🔍 重新检查端口状态...
netstat -ano | findstr ":%TARGET_PORT% " >nul
if %errorlevel% neq 0 (
    echo ✅ 端口 %TARGET_PORT% 现在可用
    echo 可以运行 start.bat 启动服务
) else (
    echo ❌ 端口仍被占用，请手动处理
)
echo.
pause
goto end

:use_other_port
echo.
echo 🔄 建议使用的替代端口:
echo   8889 - 推荐
echo   9999 - 备选
echo   8080 - 常用
echo   3000 - 开发常用
echo.
echo 修改方法:
echo 1. 编辑 start.bat 文件
echo 2. 将 "set PORT=8888" 改为 "set PORT=8889"
echo 3. 保存并重新运行
echo.
pause
goto end

:end
echo.
echo 工具结束
pause