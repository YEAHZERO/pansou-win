@echo off
chcp 65001 >nul
title PanSou MCP File Cleanup Tool

echo ================================
echo    PanSou MCP File Cleanup Tool
echo ================================
echo.

echo This tool will delete the following MCP related files and directories:
echo.
echo Directories:
echo   - typescript\                    (Entire TypeScript MCP service directory)
echo.
echo Configuration files:
echo   - mcp-config.json               (MCP service config)
echo   - mcp-config-remote.json        (MCP remote service config)
echo   - package.json                  (Root Node.js config)
echo   - package-lock.json             (Root Node.js lock file)
echo.
echo Script files:
echo   - setup-remote.js               (MCP remote service setup script)
echo   - test-remote-connection.js     (MCP remote connection test script)
echo.
echo Documentation files:
echo   - docs\MCP-SERVICE.md           (MCP service documentation)
echo   - docs\官方服务连接指南.md       (MCP connection guide)
echo.
echo Warning: This operation is irreversible! Make sure you don't need MCP functionality!
echo.
echo Files to keep:
echo   - Go backend service (main.go, api\, service\, plugin\ etc)
echo   - Configuration and tools (config\, util\, model\)
echo   - Core documentation (README.md, system design docs etc)
echo.

set /p confirm="Confirm deletion of MCP related files? (y/N): "
if /i not "%confirm%"=="y" (
    echo Operation cancelled
    pause
    exit /b 0
)

echo.
echo Starting MCP file cleanup...
echo.

REM Delete TypeScript directory
if exist "typescript" (
    echo Deleting directory: typescript\
    rmdir /s /q "typescript"
    if %errorlevel% equ 0 (
        echo [OK] Deleted typescript\ directory
    ) else (
        echo [X] Failed to delete typescript\ directory
    )
) else (
    echo [!] typescript\ directory does not exist
)

REM Delete MCP configuration files
set "files_to_delete=mcp-config.json mcp-config-remote.json package.json package-lock.json setup-remote.js test-remote-connection.js"

for %%f in (%files_to_delete%) do (
    if exist "%%f" (
        echo Deleting file: %%f
        del /q "%%f"
        if %errorlevel% equ 0 (
            echo [OK] Deleted %%f
        ) else (
            echo [X] Failed to delete %%f
        )
    ) else (
        echo [!] %%f does not exist
    )
)

REM Delete MCP related documentation
if exist "docs\MCP-SERVICE.md" (
    echo Deleting file: docs\MCP-SERVICE.md
    del /q "docs\MCP-SERVICE.md"
    if %errorlevel% equ 0 (
        echo [OK] Deleted docs\MCP-SERVICE.md
    ) else (
        echo [X] Failed to delete docs\MCP-SERVICE.md
    )
) else (
    echo [!] docs\MCP-SERVICE.md does not exist
)

if exist "docs\官方服务连接指南.md" (
    echo Deleting file: docs\官方服务连接指南.md
    del /q "docs\官方服务连接指南.md"
    if %errorlevel% equ 0 (
        echo [OK] Deleted docs\官方服务连接指南.md
    ) else (
        echo [X] Failed to delete docs\官方服务连接指南.md
    )
) else (
    echo [!] docs\官方服务连接指南.md does not exist
)

echo.
echo MCP file cleanup completed!
echo.
echo Cleanup results:
echo   - Deleted TypeScript MCP service directory
echo   - Deleted MCP configuration files
echo   - Deleted MCP related scripts
echo   - Deleted MCP related documentation
echo.
echo Core functionality preserved:
echo   - Go backend service (fully preserved)
echo   - Search plugin system (fully preserved)
echo   - Cache system (fully preserved)
echo   - Authentication system (fully preserved)
echo   - API interfaces (fully preserved)
echo.
echo Next steps:
echo   1. Build Go project: go build -o pansou.exe .
echo   2. Start service: .\pansou.exe
echo   3. Or use start script: start-pansou.bat
echo.
echo Related documentation:
echo   - docs\纯API使用指南.md
echo   - docs\Windows源码安装指南.md
echo   - api-client-examples\ (Client examples)
echo.
pause
