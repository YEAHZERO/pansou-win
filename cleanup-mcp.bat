@echo off
chcp 65001 >nul
title PanSou MCP 文件清理工具

echo ================================
echo    PanSou MCP 文件清理工具
echo ================================
echo.

echo 🔍 此工具将删除以下 MCP 相关文件和目录:
echo.
echo 📁 目录:
echo   - typescript\                    (整个 TypeScript MCP 服务目录)
echo.
echo 📄 配置文件:
echo   - mcp-config.json               (MCP 服务配置)
echo   - mcp-config-remote.json        (MCP 远程服务配置)
echo   - package.json                  (根目录 Node.js 配置)
echo   - package-lock.json             (根目录 Node.js 锁定文件)
echo.
echo 📜 脚本文件:
echo   - setup-remote.js               (MCP 远程服务配置脚本)
echo   - test-remote-connection.js     (MCP 远程连接测试脚本)
echo.
echo 📚 文档文件:
echo   - docs\MCP-SERVICE.md           (MCP 服务文档)
echo   - docs\官方服务连接指南.md       (MCP 连接指南)
echo.
echo ⚠️  警告: 此操作不可逆，请确保你不需要 MCP 功能！
echo.
echo ✅ 保留的核心文件:
echo   - Go 后端服务 (main.go, api\, service\, plugin\ 等)
echo   - 配置和工具 (config\, util\, model\)
echo   - 核心文档 (README.md, 系统设计文档等)
echo.

set /p confirm="确认删除 MCP 相关文件? (y/N): "
if /i not "%confirm%"=="y" (
    echo 操作已取消
    pause
    exit /b 0
)

echo.
echo 🗑️  开始清理 MCP 相关文件...
echo.

REM 删除 TypeScript 目录
if exist "typescript" (
    echo 删除目录: typescript\
    rmdir /s /q "typescript"
    if %errorlevel% equ 0 (
        echo ✅ 已删除 typescript\ 目录
    ) else (
        echo ❌ 删除 typescript\ 目录失败
    )
) else (
    echo ⚠️  typescript\ 目录不存在
)

REM 删除 MCP 配置文件
set "files_to_delete=mcp-config.json mcp-config-remote.json package.json package-lock.json setup-remote.js test-remote-connection.js"

for %%f in (%files_to_delete%) do (
    if exist "%%f" (
        echo 删除文件: %%f
        del /q "%%f"
        if %errorlevel% equ 0 (
            echo ✅ 已删除 %%f
        ) else (
            echo ❌ 删除 %%f 失败
        )
    ) else (
        echo ⚠️  %%f 不存在
    )
)

REM 删除 MCP 相关文档
if exist "docs\MCP-SERVICE.md" (
    echo 删除文件: docs\MCP-SERVICE.md
    del /q "docs\MCP-SERVICE.md"
    if %errorlevel% equ 0 (
        echo ✅ 已删除 docs\MCP-SERVICE.md
    ) else (
        echo ❌ 删除 docs\MCP-SERVICE.md 失败
    )
) else (
    echo ⚠️  docs\MCP-SERVICE.md 不存在
)

if exist "docs\官方服务连接指南.md" (
    echo 删除文件: docs\官方服务连接指南.md
    del /q "docs\官方服务连接指南.md"
    if %errorlevel% equ 0 (
        echo ✅ 已删除 docs\官方服务连接指南.md
    ) else (
        echo ❌ 删除 docs\官方服务连接指南.md 失败
    )
) else (
    echo ⚠️  docs\官方服务连接指南.md 不存在
)

echo.
echo 🎉 MCP 文件清理完成！
echo.
echo 📊 清理结果:
echo   - 删除了 TypeScript MCP 服务目录
echo   - 删除了 MCP 配置文件
echo   - 删除了 MCP 相关脚本
echo   - 删除了 MCP 相关文档
echo.
echo ✅ 保留的核心功能:
echo   - Go 后端服务 (完整保留)
echo   - 搜索插件系统 (完整保留)
echo   - 缓存系统 (完整保留)
echo   - 认证系统 (完整保留)
echo   - API 接口 (完整保留)
echo.
echo 🚀 下一步操作:
echo   1. 编译 Go 项目: go build -o pansou.exe .
echo   2. 启动服务: .\pansou.exe
echo   3. 或使用启动脚本: start-pansou.bat
echo.
echo 📖 相关文档:
echo   - docs\纯API使用指南.md
echo   - docs\Windows源码安装指南.md
echo   - api-client-examples\ (客户端示例)
echo.
pause