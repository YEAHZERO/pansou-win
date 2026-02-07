# PanSou - 网盘搜索聚合系统

> **版本**: v6.0 (精简版) | **最后更新**: 2025-02-07

---

## 项目简介

PanSou 是一个强大的网盘搜索聚合系统，采用纯 Go 语言实现，提供高性能的搜索服务。

### 核心特性

- ✅ Pioz 插件搜索（三重搜索策略 + 强大反爬绕过）
- ✅ 高性能缓存机制（两级缓存 + 异步更新）
- ✅ 支持16种网盘类型（夸克、百度、阿里云、UC、迅雷等）
- ✅ RESTful API 接口
- ✅ JWT 认证和权限控制
- ✅ 已禁用 Telegram 频道（专注 Pioz 搜索）
- ✅ 优化配置（10秒超时，16个工作线程）

---

## 快速开始

### 1. 编译项目

```bash
go build -o pansou.exe
```

### 2. 启动服务

```bash
start.bat
```

### 3. 验证安装

访问健康检查接口：
```bash
curl http://localhost:8889/api/health
```

### 4. 登录获取Token

```bash
curl -X POST http://localhost:8889/api/auth/login \
  -H "Content-Type: application/json" \
  -d "{\"username\":\"admin\",\"password\":\"123456\"}"
```

### 5. 搜索测试

```bash
curl "http://localhost:8889/api/search?keyword=测试" \
  -H "Authorization: Bearer YOUR_TOKEN"
```

---

## 配置说明

### 基础配置（start.bat）

```batch
REM 端口配置
set PORT=8889

REM Telegram频道配置（已禁用，只使用Pioz）
set CHANNELS=

REM 插件配置（只启用pioz）
set ASYNC_PLUGIN_ENABLED=true
set ENABLED_PLUGINS=pioz

REM 认证配置
set AUTH_ENABLED=true
set AUTH_USERS=admin:123456

REM 性能配置（已优化）
set CONCURRENCY=15
set ASYNC_RESPONSE_TIMEOUT=10
set ASYNC_MAX_BACKGROUND_WORKERS=16
set ASYNC_MAX_BACKGROUND_TASKS=80
set CACHE_MAX_SIZE=500
set CACHE_TTL=120
set PLUGIN_TIMEOUT=20
```

### 备用插件

项目保留了以下备用插件（需要时可在main.go中取消注释）：
- pansearch - 通用网盘搜索
- wanou - Wanou网盘搜索
- xdpan - 迅雷网盘搜索
- zhizhen - 指针网盘搜索

---

## API 接口

### 认证接口

**登录**
```
POST /api/auth/login
Content-Type: application/json

{
  "username": "admin",
  "password": "123456"
}
```

**刷新Token**
```
POST /api/auth/refresh
Authorization: Bearer YOUR_TOKEN
```

### 搜索接口

**GET 方式**
```
GET /api/search?keyword=关键词
Authorization: Bearer YOUR_TOKEN
```

**POST 方式**
```
POST /api/search
Authorization: Bearer YOUR_TOKEN
Content-Type: application/json

{
  "keyword": "关键词",
  "channels": ["tgsearchers3"],
  "concurrency": 10
}
```

### 健康检查

```
GET /api/health
```

---

## 目录结构

```
pansou/
├── main.go              # 程序入口
├── start.bat            # 启动脚本
├── deploy.bat           # 部署脚本
├── go.mod               # Go模块配置
├── LICENSE              # 许可证
├── README.md            # 本文档
├── api/                 # API层
│   ├── router.go        # 路由配置
│   ├── handler.go       # 请求处理
│   ├── middleware.go    # 中间件
│   └── auth_handler.go  # 认证处理
├── config/              # 配置管理
│   └── config.go
├── service/             # 业务逻辑层
│   └── search_service.go
├── model/               # 数据模型
│   ├── request.go
│   ├── response.go
│   └── plugin_result.go
├── plugin/              # 插件系统
│   ├── plugin.go        # 插件基础框架
│   ├── pioz/            # Pioz插件（主力）
│   ├── pansearch/       # 备用插件
│   ├── wanou/           # 备用插件
│   ├── xdpan/           # 备用插件
│   └── zhizhen/         # 备用插件
├── util/                # 工具库
│   ├── cache/           # 缓存系统
│   ├── pool/            # 工作池
│   └── http_util.go     # HTTP工具
├── api-client-examples/ # API客户端示例
│   ├── python_client.py
│   ├── powershell_client.ps1
│   └── web_client.html
└── docs/                # 文档目录
    ├── PanSou网盘搜索官方说明.md
    ├── Windows安装部署指南.md
    └── PanSou安装配置问答集.md
```

---

## 文档

### 核心文档（3个）

| 文档 | 说明 |
|------|------|
| [PanSou网盘搜索官方说明.md](docs/PanSou网盘搜索官方说明.md) | 项目概述和功能介绍 |
| [Windows安装部署指南.md](docs/Windows安装部署指南.md) | 完整安装和部署指南 |
| [PanSou安装配置问答集.md](docs/PanSou安装配置问答集.md) | 常见问题解答 |

---

## 测试和诊断

### 快速测试

```bash
# 运行 Pioz 专项测试（推荐）
test-pioz.bat

# 运行系统诊断
diagnose.bat
```

### Pioz 专项测试

`test-pioz.bat` 会自动测试：
1. Pioz 网站访问性（pioz.cn）
2. 登录认证
3. 三个不同关键词的搜索
4. 结果分析和保存

### 手动测试

```bash
# 1. 健康检查
curl http://localhost:8889/api/health

# 2. 登录获取Token
curl -X POST http://localhost:8889/api/auth/login \
  -H "Content-Type: application/json" \
  -d "{\"username\":\"admin\",\"password\":\"123456\"}"

# 3. 搜索测试（只使用Pioz，不使用Telegram）
curl -X POST http://localhost:8889/api/search \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"keyword\":\"电影\",\"channels\":[],\"concurrency\":5}"
```

---

## 部署流程

### 首次部署

1. 编译项目
   ```bash
   go build -o pansou.exe
   ```

2. 配置 start.bat（可选）
   - 修改端口、频道、认证等配置

3. 启动服务
   ```bash
   start.bat
   ```

### 重新部署

使用自动部署脚本：
```bash
# 1. 编译新版本
go build -o pansou.exe

# 2. 运行部署脚本
deploy.bat
```

部署脚本会自动：
- 停止旧服务
- 备份旧文件
- 复制新文件
- 启动新服务

---

## Pioz 插件特性

### 三重搜索策略
1. **深度搜索API**（首选）- 最全面的结果
2. **普通HTML搜索**（备用）- 稳定可靠
3. **热搜榜匹配**（兜底）- 确保有结果

### 二次跳转机制
搜索页 → 详情页 → 真实链接，全自动获取网盘链接

### 强大反爬绕过
- 随机请求延迟（500-1500ms）
- 轮换User-Agent（7种浏览器）
- 完整请求头设置
- Cookie会话管理
- 指数退避重试机制

### 异步并发处理
8个并发 + goroutines，性能提升3-5倍

### 智能缓存系统
三级缓存（搜索结果 + 详情页 + Transfer结果）

### 支持16种网盘
夸克、百度、阿里云、UC、迅雷、天翼、蓝奏云、115、移动云盘、微云、坚果云、123云盘、PikPak、磁力链接、电驴链接

---

## 性能优化

### 缓存配置
```batch
set CACHE_ENABLED=true
set CACHE_MAX_SIZE=300    # MB
set CACHE_TTL=90          # 分钟
```

### 并发配置
```batch
set CONCURRENCY=25
set ASYNC_RESPONSE_TIMEOUT=5
set ASYNC_MAX_BACKGROUND_WORKERS=12
set ASYNC_MAX_BACKGROUND_TASKS=60
```

### HTTP服务器配置
```batch
set HTTP_READ_TIMEOUT=30
set HTTP_WRITE_TIMEOUT=30
set HTTP_IDLE_TIMEOUT=120
set HTTP_MAX_CONNS=200
```

---

## 常见问题

### 1. Pioz 返回 0 个结果

**可能原因**：
- 网站访问受限
- 关键词不匹配
- 网络连接问题

**解决方案**：
```bash
# 1. 测试网站访问
curl -I https://www.pioz.cc

# 2. 配置代理（如果需要）
set PROXY=http://127.0.0.1:7890

# 3. 增加超时时间
set ASYNC_RESPONSE_TIMEOUT=10

# 4. 运行诊断
diagnose.bat
```

### 2. 端口被占用

**解决方案**：
```bash
# 查看占用端口的进程
netstat -ano | findstr :8889

# 结束进程
taskkill /F /PID 进程ID

# 或修改端口
set PORT=8890
```

### 2. 端口被占用

**解决方案**：
```bash
# 查看占用端口的进程
netstat -ano | findstr :8889

# 结束进程
taskkill /F /PID 进程ID

# 或修改端口
set PORT=8890
```

### 3. 搜索响应慢

**优化方案**：
```batch
# 增加超时时间
set ASYNC_RESPONSE_TIMEOUT=10

# 增加工作线程
set ASYNC_MAX_BACKGROUND_WORKERS=20

# 增加缓存
set CACHE_MAX_SIZE=500
set CACHE_TTL=120
```

详细优化指南请查看：[OPTIMIZATION_GUIDE.md](OPTIMIZATION_GUIDE.md)

### 4. 搜索无结果

**检查项**：
1. 确认插件已启用：`set ENABLED_PLUGINS=pioz`
2. 确认异步插件已启用：`set ASYNC_PLUGIN_ENABLED=true`
3. 查看服务器日志，检查是否有错误

### 4. 搜索无结果

**检查项**：
1. 确认插件已启用：`set ENABLED_PLUGINS=pioz`
2. 确认异步插件已启用：`set ASYNC_PLUGIN_ENABLED=true`
3. 运行诊断脚本：`diagnose.bat`
4. 查看服务器日志，检查是否有错误

### 5. 编译失败

**解决方案**：
```bash
# 清理缓存
go clean -modcache

# 重新下载依赖
go mod download

# 重新编译
go build -o pansou.exe
```

### 5. 编译失败

**解决方案**：
```bash
# 清理缓存
go clean -modcache

# 重新下载依赖
go mod download

# 重新编译
go build -o pansou.exe
```

### 6. 认证失败

**检查项**：
1. 确认认证已启用：`set AUTH_ENABLED=true`
2. 确认用户名密码正确：`set AUTH_USERS=admin:123456`
3. 确认Token未过期（默认7天）

---

## 技术栈

- **语言**: Go 1.24+
- **Web框架**: Gin
- **HTML解析**: goquery
- **JSON处理**: sonic (高性能)
- **认证**: JWT
- **缓存**: 自研两级缓存系统

---

## 更新记录

### v6.0 - 2025-02-07 (精简版)
- 🎯 **精简项目**: 只保留pioz核心插件
- 📚 **精简文档**: 从10篇精简到3篇核心文档
- 🧹 **清理脚本**: 删除冗余脚本，只保留核心功能
- 💾 **保留备用**: 其他4个插件保留目录，可随时恢复
- ⚡ **性能优化**: 减少编译时间和项目体积

### v5.3 - 2025-02-07
- 🔧 修复 Pioz 插件 Extra 字段编译错误
- 🛠️ 修复 BAT 文件编码问题
- 📚 完善插件开发指南

### v5.2 - 2025-02-07
- ✨ Pioz插件全面升级：三重搜索策略 + 强大反爬绕过
- 🚀 二次跳转机制
- 🛡️ 强大反爬绕过
- ⚡ 异步并发处理
- 💾 智能缓存系统

---

## 相关链接

- **项目主页**: [GitHub](https://github.com/fish2018/pansou)
- **客户端示例**: [api-client-examples/](api-client-examples/)
- **部署脚本**: [deploy.bat](deploy.bat)
- **启动脚本**: [start.bat](start.bat)

---

## 贡献者

**维护者**: abcxyzNone  
**AI工具**: Kiro (Claude Sonnet 4.5)  
**致谢**: PanSou Team & fish2018

---

## 许可证

本项目采用 MIT 许可证

---

**最后更新**: 2025-02-07  
**版本**: v6.0 (精简版)
