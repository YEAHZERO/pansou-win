@echo off
chcp 65001 >nul
title PanSou Windows 安装向导

echo ================================
echo    PanSou Windows 安装向导
echo ================================
echo.

REM 检查管理员权限
net session >nul 2>&1
if %errorlevel% neq 0 (
    echo ⚠️  建议以管理员身份运行此脚本
    echo.
)

echo 🔍 检查系统环境...
echo.

REM 检查 Go 环境
echo 检查 Go 环境:
go version >nul 2>&1
if %errorlevel% equ 0 (
    echo ✅ Go 已安装
    go version
) else (
    echo ❌ Go 未安装
    echo 请访问 https://golang.org/dl/ 下载安装 Go
    echo.
)

REM 检查 Git 环境
echo.
echo 检查 Git 环境:
git --version >nul 2>&1
if %errorlevel% equ 0 (
    echo ✅ Git 已安装
    git --version
) else (
    echo ❌ Git 未安装
    echo 请访问 https://git-scm.com/ 下载安装 Git
    echo.
)

echo.
echo ================================
echo    选择安装方式
echo ================================
echo.
echo 1. 源码编译安装 (推荐，需要 Go)
echo 2. 预编译二进制安装
echo 3. 退出
echo.

set /p choice="请选择安装方式 (1-3): "

if "%choice%"=="1" goto source_install
if "%choice%"=="2" goto binary_install
if "%choice%"=="3" goto end
goto invalid_choice

:source_install
echo.
echo 🔨 源码编译安装
echo ================================
echo.

go version >nul 2>&1
if %errorlevel% neq 0 (
    echo ❌ Go 未安装，无法使用此方式
    goto end
)

if not exist "go.mod" (
    echo ❌ 未找到 go.mod 文件
    echo 请确保在 PanSou 项目根目录中运行此脚本
    goto end
)

echo 📥 下载依赖...
go mod download

if %errorlevel% neq 0 (
    echo ❌ 依赖下载失败，尝试设置代理...
    go env -w GOPROXY=https://goproxy.cn,direct
    go mod download
)

echo 🔨 编译项目...
go build -ldflags="-s -w" -o pansou.exe .

if %errorlevel% equ 0 (
    echo ✅ 编译成功
    echo.
    echo 📁 生成的文件:
    echo   - pansou.exe (主程序)
    echo   - start-pansou.bat (启动脚本)
    echo   - test-pansou.bat (测试脚本)
    echo.
    echo 🚀 现在可以运行以下命令启动服务:
    echo   start-pansou.bat
    echo.
    echo 或者手动启动:
    echo   pansou.exe
    echo.
    echo 📖 更多信息请查看: docs\Windows源码安装指南.md
) else (
    echo ❌ 编译失败
    echo.
    echo 可能的解决方案:
    echo 1. 检查 Go 版本是否 >= 1.21
    echo 2. 检查网络连接
    echo 3. 尝试清理模块缓存: go clean -modcache
)
goto end

:binary_install
echo.
echo 📦 预编译二进制安装
echo ================================
echo.
echo 请手动执行以下步骤:
echo.
echo 1. 访问 GitHub Releases 页面:
echo    https://github.com/fish2018/pansou/releases
echo.
echo 2. 下载最新的 Windows 版本:
echo    pansou-windows-amd64.exe
echo.
echo 3. 将文件重命名为 pansou.exe
echo.
echo 4. 放置在当前目录
echo.
echo 5. 运行启动脚本:
echo    start-pansou.bat
echo.
echo 📖 详细说明请查看: docs\Windows源码安装指南.md
echo.
goto end

:invalid_choice
echo ❌ 无效选择，请重新运行脚本
goto end

:end
echo.
echo 📚 相关文档:
echo   - docs\Windows源码安装指南.md (详细安装指南)
echo   - docs\纯API使用指南.md (API 使用说明)
echo   - api-client-examples\ (客户端示例)
echo.
echo 🎯 安装完成后可以:
echo   1. 访问 http://localhost:8888/api/health 检查服务
echo   2. 使用客户端工具进行搜索
echo   3. 查看相关文档了解更多功能
echo.
echo 安装向导结束
pause