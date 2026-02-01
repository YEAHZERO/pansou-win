#!/bin/bash

# PanSou MCP 文件清理工具 (Linux/macOS 版本)

echo "================================"
echo "   PanSou MCP 文件清理工具"
echo "================================"
echo

echo "🔍 此工具将删除以下 MCP 相关文件和目录:"
echo
echo "📁 目录:"
echo "  - typescript/                    (整个 TypeScript MCP 服务目录)"
echo
echo "📄 配置文件:"
echo "  - mcp-config.json               (MCP 服务配置)"
echo "  - mcp-config-remote.json        (MCP 远程服务配置)"
echo "  - package.json                  (根目录 Node.js 配置)"
echo "  - package-lock.json             (根目录 Node.js 锁定文件)"
echo
echo "📜 脚本文件:"
echo "  - setup-remote.js               (MCP 远程服务配置脚本)"
echo "  - test-remote-connection.js     (MCP 远程连接测试脚本)"
echo
echo "📚 文档文件:"
echo "  - docs/MCP-SERVICE.md           (MCP 服务文档)"
echo "  - docs/官方服务连接指南.md       (MCP 连接指南)"
echo
echo "⚠️  警告: 此操作不可逆，请确保你不需要 MCP 功能！"
echo
echo "✅ 保留的核心文件:"
echo "  - Go 后端服务 (main.go, api/, service/, plugin/ 等)"
echo "  - 配置和工具 (config/, util/, model/)"
echo "  - 核心文档 (README.md, 系统设计文档等)"
echo

read -p "确认删除 MCP 相关文件? (y/N): " confirm
if [[ ! "$confirm" =~ ^[Yy]$ ]]; then
    echo "操作已取消"
    exit 0
fi

echo
echo "🗑️  开始清理 MCP 相关文件..."
echo

# 删除 TypeScript 目录
if [ -d "typescript" ]; then
    echo "删除目录: typescript/"
    rm -rf "typescript"
    if [ $? -eq 0 ]; then
        echo "✅ 已删除 typescript/ 目录"
    else
        echo "❌ 删除 typescript/ 目录失败"
    fi
else
    echo "⚠️  typescript/ 目录不存在"
fi

# 删除 MCP 配置文件
files_to_delete=(
    "mcp-config.json"
    "mcp-config-remote.json"
    "package.json"
    "package-lock.json"
    "setup-remote.js"
    "test-remote-connection.js"
)

for file in "${files_to_delete[@]}"; do
    if [ -f "$file" ]; then
        echo "删除文件: $file"
        rm -f "$file"
        if [ $? -eq 0 ]; then
            echo "✅ 已删除 $file"
        else
            echo "❌ 删除 $file 失败"
        fi
    else
        echo "⚠️  $file 不存在"
    fi
done

# 删除 MCP 相关文档
if [ -f "docs/MCP-SERVICE.md" ]; then
    echo "删除文件: docs/MCP-SERVICE.md"
    rm -f "docs/MCP-SERVICE.md"
    if [ $? -eq 0 ]; then
        echo "✅ 已删除 docs/MCP-SERVICE.md"
    else
        echo "❌ 删除 docs/MCP-SERVICE.md 失败"
    fi
else
    echo "⚠️  docs/MCP-SERVICE.md 不存在"
fi

if [ -f "docs/官方服务连接指南.md" ]; then
    echo "删除文件: docs/官方服务连接指南.md"
    rm -f "docs/官方服务连接指南.md"
    if [ $? -eq 0 ]; then
        echo "✅ 已删除 docs/官方服务连接指南.md"
    else
        echo "❌ 删除 docs/官方服务连接指南.md 失败"
    fi
else
    echo "⚠️  docs/官方服务连接指南.md 不存在"
fi

echo
echo "🎉 MCP 文件清理完成！"
echo
echo "📊 清理结果:"
echo "  - 删除了 TypeScript MCP 服务目录"
echo "  - 删除了 MCP 配置文件"
echo "  - 删除了 MCP 相关脚本"
echo "  - 删除了 MCP 相关文档"
echo
echo "✅ 保留的核心功能:"
echo "  - Go 后端服务 (完整保留)"
echo "  - 搜索插件系统 (完整保留)"
echo "  - 缓存系统 (完整保留)"
echo "  - 认证系统 (完整保留)"
echo "  - API 接口 (完整保留)"
echo
echo "🚀 下一步操作:"
echo "  1. 编译 Go 项目: go build -o pansou ."
echo "  2. 启动服务: ./pansou"
echo "  3. 或使用启动脚本: ./start-pansou.sh"
echo
echo "📖 相关文档:"
echo "  - docs/纯API使用指南.md"
echo "  - docs/Windows源码安装指南.md"
echo "  - api-client-examples/ (客户端示例)"
echo