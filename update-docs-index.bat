@echo off
chcp 65001 >nul
title 更新文档索引

echo ================================
echo   PanSou 文档索引更新工具
echo ================================
echo.

echo 正在扫描 docs 目录...
echo.

REM 统计文档数量
set /a doc_count=0
for %%f in (docs\*.md docs\*.txt) do (
    set /a doc_count+=1
)

echo 找到 %doc_count% 个文档文件
echo.

echo 文档列表:
echo --------------------------------
for %%f in (docs\*.md docs\*.txt) do (
    echo   - %%~nxf
)
echo --------------------------------
echo.

echo ℹ️  文档索引更新说明:
echo.
echo 本工具用于手动触发文档索引更新。
echo.
echo 自动更新机制:
echo   1. 当 docs 目录中的文档被修改时，会自动更新索引
echo   2. 当创建新文档时，会自动归类并更新索引
echo   3. 使用 Kiro Agent Hooks 实现自动化
echo.
echo 手动更新方法:
echo   1. 运行本脚本查看文档统计
echo   2. 在 Kiro 中执行: "更新 docs/README.md 的文档索引"
echo   3. 或直接编辑 docs/README.md
echo.

echo 当前文档分类:
echo   - 快速开始: 4 篇
echo   - 配置和部署: 4 篇
echo   - 插件开发: 5 篇
echo   - 插件案例: 3 篇
echo   - 系统架构: 2 篇
echo   - 服务集成: 2 篇
echo   - 问答帮助: 1 篇
echo   - 项目记录: 3 篇
echo   --------------------------------
echo   总计: 24 篇
echo.

echo ✅ 文档索引位置: docs\README.md
echo.

pause
