# PanSou MCP 文件清理说明

## 🎯 目的

如果你只想使用 PanSou 的核心搜索功能，而不需要 MCP (Model Context Protocol) 集成，可以使用清理工具删除相关文件，简化项目结构。

## 🛠️ 清理工具

### Windows 用户
```cmd
cleanup-mcp.bat
```

### Linux/macOS 用户
```bash
chmod +x cleanup-mcp.sh
./cleanup-mcp.sh
```

## 🗑️ 将被删除的文件

### 1. TypeScript MCP 服务 (整个目录)
```
typescript/
├── src/
│   ├── index.ts                     # MCP 服务器主入口
│   ├── tools/                       # MCP 工具集
│   └── utils/                       # MCP 工具库
├── package.json                     # Node.js 依赖
├── package-lock.json               # 依赖锁定
└── tsconfig.json                   # TypeScript 配置
```

### 2. MCP 配置文件
- `mcp-config.json` - MCP 服务配置
- `mcp-config-remote.json` - MCP 远程服务配置
- `package.json` (根目录) - Node.js 配置
- `package-lock.json` (根目录) - Node.js 锁定文件

### 3. MCP 相关脚本
- `setup-remote.js` - MCP 远程服务配置脚本
- `test-remote-connection.js` - MCP 远程连接测试脚本

### 4. MCP 相关文档
- `docs/MCP-SERVICE.md` - MCP 服务文档
- `docs/官方服务连接指南.md` - MCP 连接指南

## ✅ 保留的核心文件

### Go 后端服务 (完整保留)
```
main.go                             # 主程序入口
go.mod                              # Go 模块定义
go.sum                              # Go 依赖锁定

api/                                # API 服务层
├── auth_handler.go                 # 认证处理
├── handler.go                      # 请求处理
├── middleware.go                   # 中间件
└── router.go                       # 路由配置

service/                            # 业务服务层
├── cache_integration.go            # 缓存集成
└── search_service.go              # 搜索服务

plugin/                             # 插件系统
├── plugin.go                       # 插件接口
└── [70+ 插件目录]/                # 各种搜索插件

config/                             # 配置管理
└── config.go                       # 配置文件

model/                              # 数据模型
├── plugin_result.go                # 插件结果
├── request.go                      # 请求模型
└── response.go                     # 响应模型

util/                               # 工具库
├── cache/                          # 缓存系统
├── pool/                           # 工作池
├── json/                           # JSON 工具
└── [其他工具文件]                   # HTTP、JWT、解析等工具
```

### 文档和配置 (保留)
```
README.md                           # 项目说明
LICENSE                             # 许可证
.gitignore                          # Git 忽略文件
Dockerfile                          # Docker 配置
docker-compose.yml                  # Docker Compose 配置

docs/                               # 文档目录
├── 系统开发设计文档.md              # 技术文档
├── 插件开发指南.md                  # 插件开发
└── [其他核心文档]                   # 保留的文档
```

## 🚀 清理后的使用方式

### 1. 编译项目
```bash
# 下载依赖
go mod download

# 编译
go build -o pansou.exe .  # Windows
go build -o pansou .      # Linux/macOS
```

### 2. 启动服务
```bash
# Windows
.\pansou.exe

# Linux/macOS
./pansou
```

### 3. 使用 API
```bash
# 健康检查
curl http://localhost:8888/api/health

# 搜索资源
curl -X POST http://localhost:8888/api/search \
  -H "Content-Type: application/json" \
  -d '{"kw":"速度与激情","res":"merge"}'
```

## 📊 清理前后对比

### 清理前
- **文件数量**: ~100+ 个文件
- **技术栈**: Go + TypeScript + Node.js
- **依赖**: Go 模块 + npm 包
- **复杂度**: 高（需要了解 MCP 协议）

### 清理后
- **文件数量**: ~80 个文件
- **技术栈**: 纯 Go
- **依赖**: 仅 Go 模块
- **复杂度**: 低（标准 HTTP API）

## 🎯 适用场景

### 适合清理 MCP 的情况
- ✅ 只需要搜索 API 功能
- ✅ 不使用 Claude Desktop 或 Cherry Studio
- ✅ 希望简化项目结构
- ✅ 自己开发客户端应用
- ✅ 集成到现有系统中

### 不适合清理 MCP 的情况
- ❌ 需要在 Claude Desktop 中使用
- ❌ 需要在 Cherry Studio 中使用
- ❌ 依赖 MCP 协议的其他应用
- ❌ 需要 MCP 的特定功能

## ⚠️ 注意事项

1. **不可逆操作**: 清理后无法恢复，请确保备份
2. **功能完整**: 清理后不影响核心搜索功能
3. **文档更新**: 清理后请参考纯 API 使用文档
4. **客户端**: 可以使用提供的客户端示例

## 🆘 如果需要恢复

如果清理后需要恢复 MCP 功能：
1. 重新解压源代码
2. 或从 Git 仓库重新克隆
3. 参考 MCP 相关文档重新配置

## 📞 获取帮助

- 查看 `docs/纯API使用指南.md`
- 查看 `api-client-examples/` 目录
- 提交 GitHub Issues
- 参考项目文档