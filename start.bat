@echo off
chcp 65001 >nul
title PanSou 网盘搜索服务

echo ================================
echo    PanSou 网盘搜索服务
echo ================================
echo.

REM 获取当前脚本所在目录
set SCRIPT_DIR=%~dp0
REM 移除末尾的反斜杠
set SCRIPT_DIR=%SCRIPT_DIR:~0,-1%

REM 设置 PanSou 可执行文件路径（使用相对路径）
set PANSOU_PATH=%SCRIPT_DIR%\pansou.exe

REM 检查可执行文件是否存在
if not exist "%PANSOU_PATH%" (
    echo ? 未找到 PanSou 可执行文件！
    echo 路径: %PANSOU_PATH%
    echo.
    echo 请检查:
    echo 1. 文件路径是否正确
    echo 2. 是否已编译生成 pansou.exe
    echo 3. 是否有访问权限
    pause
    exit /b 1
)

REM 基础配置
set PORT=8889
set CACHE_ENABLED=true
set CACHE_PATH=%SCRIPT_DIR%\cache
set TZ=Asia/Shanghai

REM 检查端口是否被占用
echo ?? 检查端口 %PORT% 是否可用...
netstat -an | findstr ":%PORT% " >nul
if %errorlevel% equ 0 (
    echo ? 端口 %PORT% 已被占用！
    echo.
    echo 正在查找占用端口的进程...
    for /f "tokens=5" %%a in ('netstat -ano ^| findstr ":%PORT% "') do (
        echo 进程 ID: %%a
        tasklist /fi "PID eq %%a" 2>nul | findstr /v "INFO:"
    )
    echo.
    echo 解决方案:
    echo 1. 关闭占用端口的程序
    echo 2. 或者按任意键使用其他端口 (8889)
    echo 3. 或者按 Ctrl+C 退出
    echo.
    pause
    set PORT=8889
    echo ? 已切换到端口 %PORT%
    echo.
)

REM Telegram 频道配置（推荐配置，可根据需要调整）
set CHANNELS=tgsearchers3,tgsearchers4,Aliyun_4K_Movies,bdbdndn11,yunpanx,bsbdbfjfjff,yp123pan,sbsbsnsqq,yunpanxunlei,tianyifc,BaiduCloudDisk,txtyzy,peccxinpd,gotopan,PanjClub

REM 插件配置
set ASYNC_PLUGIN_ENABLED=true
set ENABLED_PLUGINS=pioz,labi,zhizhen,shandian,duoduo,muou,wanou,hunhepan,jikepan,pansearch,panta,qupansou,hdr4k,pan666,susu,thepiratebay,xuexizhinan,panyq,ouge,huban,gying,qqpd,weibo,panwiki,quark4k,quarksoo,qupanshe,miaoso,yunsou,sousou,xdpan,mikuclub

REM 性能配置（根据频道和插件数量自动调整）
set CONCURRENCY=25
set ASYNC_RESPONSE_TIMEOUT=5
set ASYNC_MAX_BACKGROUND_WORKERS=12
set ASYNC_MAX_BACKGROUND_TASKS=60
set CACHE_MAX_SIZE=300
set CACHE_TTL=90

REM 认证配置
set AUTH_ENABLED=true
set AUTH_USERS=admin:123456
set AUTH_JWT_SECRET=pansou-secret-key-2024

REM HTTP 服务器配置
set HTTP_READ_TIMEOUT=30
set HTTP_WRITE_TIMEOUT=30
set HTTP_IDLE_TIMEOUT=120
set HTTP_MAX_CONNS=200

echo 配置信息:
echo ================================
echo 可执行文件: %PANSOU_PATH%
echo 服务端口: %PORT%
echo 缓存目录: %CACHE_PATH%
echo 缓存启用: %CACHE_ENABLED%
echo 插件系统: %ASYNC_PLUGIN_ENABLED%
echo 认证功能: %AUTH_ENABLED%
echo 认证用户: admin
echo Telegram频道: %CHANNELS%
echo 启用插件: %ENABLED_PLUGINS%
echo ================================
echo.

REM 创建缓存目录
if not exist "%CACHE_PATH%" (
    mkdir "%CACHE_PATH%"
    echo ? 已创建缓存目录: %CACHE_PATH%
)

REM 设置工作目录为脚本所在目录
cd /d "%SCRIPT_DIR%"

echo ?? 启动 PanSou 服务器...
echo.
echo 服务信息:
echo   访问地址: http://localhost:%PORT%
echo   健康检查: http://localhost:%PORT%/api/health
echo   认证用户: admin
echo   认证密码: 123456
echo.
echo ?? 认证已启用，API 调用需要先登录获取 Token
echo   Token 永不过期，请妥善保管
echo.
echo 登录示例:
echo   curl -X POST http://localhost:%PORT%/api/auth/login \
echo     -H "Content-Type: application/json" \
echo     -d "{\"username\":\"admin\",\"password\":\"123456\"}"
echo.
echo 按 Ctrl+C 停止服务
echo.

REM 启动服务
"%PANSOU_PATH%"

echo.
echo 服务器已停止
pause