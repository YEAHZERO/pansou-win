@echo off
chcp 65001 >nul
title PanSou 网盘搜索服务

echo ================================
echo    PanSou 网盘搜索服务
echo ================================
echo.

REM 检查是否存在 pansou.exe
if not exist "pansou.exe" (
    echo ? 未找到 pansou.exe 文件！
    echo.
    echo 请先构建项目：
    echo   go build -o pansou.exe .
    echo.
    echo 或从 GitHub Releases 下载预编译版本
    pause
    exit /b 1
)

REM 设置环境变量
set PORT=8888
set CACHE_ENABLED=true
set CACHE_PATH=.\cache
set ASYNC_PLUGIN_ENABLED=true
set ENABLED_PLUGINS=labi,zhizhen,shandian,duoduo,muou,wanou,hunhepan,jikepan,pansearch,panta,qupansou
set TZ=Asia/Shanghai
set ASYNC_RESPONSE_TIMEOUT=4
set ASYNC_MAX_BACKGROUND_WORKERS=10
set ASYNC_MAX_BACKGROUND_TASKS=50

REM 可选：启用认证（取消注释下面两行）
REM set AUTH_ENABLED=true
REM set AUTH_USERS=admin:your_password

echo 配置信息:
echo 端口: %PORT%
echo 缓存: %CACHE_ENABLED%
echo 缓存路径: %CACHE_PATH%
echo 插件系统: %ASYNC_PLUGIN_ENABLED%
echo 启用插件: %ENABLED_PLUGINS%
echo 时区: %TZ%
echo.

REM 创建缓存目录
if not exist "%CACHE_PATH%" (
    mkdir "%CACHE_PATH%"
    echo ? 已创建缓存目录: %CACHE_PATH%
)

echo ?? 启动服务器...
echo 访问地址: http://localhost:%PORT%
echo 健康检查: http://localhost:%PORT%/api/health
echo.
echo 按 Ctrl+C 停止服务
echo.

REM 启动服务
pansou.exe

echo.
echo 服务器已停止
pause