# PanSou - 网盘搜索聚合服务

一个高性能的网盘资源搜索聚合服务，支持多个搜索插件。

## 快速开始

### 1. 启动服务
```bash
start-simple.bat
```

### 2. 测试搜索
```bash
test-final.bat
```

### 3. 停止服务
```bash
stop.bat
```

## 核心功能

- ✅ **Pioz2 插件**: 主要搜索源，支持深度搜索 API
- ✅ **异步搜索**: 高性能异步插件系统
- ✅ **智能缓存**: 两级缓存（内存+磁盘）
- ✅ **JWT 认证**: 安全的 API 访问控制

## 配置说明

### 环境变量（start-simple.bat）
```batch
set PORT=8889                              # 服务端口
set ENABLED_PLUGINS=pioz2                  # 启用的插件
set ASYNC_RESPONSE_TIMEOUT=10              # 异步超时（秒）
set ASYNC_MAX_BACKGROUND_WORKERS=16        # 工作线程数
set CACHE_MAX_SIZE=500                     # 缓存大小（MB）
set CACHE_TTL=120                          # 缓存时间（分钟）
```

### Pioz2 插件配置
- **API**: https://www.pioz.cn/api/deep-search
- **搜索超时**: 15 秒
- **优先级**: 1（高质量数据源）
- **缓存 TTL**: 30 分钟

## API 使用

### 1. 登录获取 Token
```bash
curl -X POST http://localhost:8889/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"123456"}'
```

### 2. 搜索资源（支持 keyword 和 kw 参数）
```bash
# 使用 keyword 参数
curl "http://localhost:8889/api/search?keyword=太奶奶" \
  -H "Authorization: Bearer YOUR_TOKEN"

# 使用 kw 参数（兼容）
curl "http://localhost:8889/api/search?kw=太奶奶" \
  -H "Authorization: Bearer YOUR_TOKEN"

# 强制刷新缓存
curl "http://localhost:8889/api/search?keyword=太奶奶&refresh=true" \
  -H "Authorization: Bearer YOUR_TOKEN"
```

### 3. 健康检查
```bash
curl http://localhost:8889/api/health
```

## 最近修复（v7.0 - 2026-02-08）

### 创建 Pioz2 插件
- **基于**: xdpan 和 pansearch 的成功模式
- **特点**: 简化实现，直接调用 Pioz API
- **状态**: ✅ 成功返回44个搜索结果

### 修复 1: API 参数名不匹配
- **问题**: handler 使用 `kw` 参数，URL 使用 `keyword` 参数
- **修复**: 同时支持 `keyword` 和 `kw` 参数
- **文件**: `api/handler.go`

### 修复 2: 空链接过滤
- **问题**: 结果没有链接被过滤掉
- **修复**: 添加详情页链接到结果中
- **文件**: `plugin/pioz2/pioz2.go`

详细修复说明见 [PLUGIN_FIX_GUIDE.md](PLUGIN_FIX_GUIDE.md)

## 故障排除

### 问题：搜索返回 0 结果
**解决方案**:
1. 使用 `refresh=true` 强制刷新缓存
2. 检查日志中的关键词是否正确传递
3. 清除缓存：`rd /s /q cache && mkdir cache`
4. 重启服务：`stop.bat && start-simple.bat`

### 问题：API 返回 400 错误
**原因**: 关键词为空或格式错误
**解决**: 检查 URL 参数名（使用 `keyword` 或 `kw`）

### 问题：服务无法启动
**原因**: 端口被占用
**解决**: 运行 `stop.bat` 或更改端口

## 项目结构

```
pansou/
├── api/                    # API 路由和处理器
├── config/                 # 配置管理
├── model/                  # 数据模型
├── plugin/                 # 插件系统
│   ├── pioz/              # Pioz 插件（原版）
│   └── pioz2/             # Pioz2 插件（推荐）
├── service/               # 业务逻辑
├── util/                  # 工具函数
│   └── cache/            # 缓存系统
├── start-simple.bat       # 启动脚本（推荐）
├── stop.bat              # 停止脚本
├── test-final.bat        # 测试脚本
└── PLUGIN_FIX_GUIDE.md   # 插件开发指南
```

## 开发指南

### 编译
```bash
go build -o pansou.exe
```

### 添加新插件
参考 [PLUGIN_FIX_GUIDE.md](PLUGIN_FIX_GUIDE.md) 获取详细的插件开发指南，包括：
- 问题诊断与解决经验
- 创建新插件的最佳实践
- 调试技巧
- 常见问题排查
- 性能优化建议

## 性能优化

- **并发控制**: 限制同时请求数，避免过载
- **连接池**: 复用 HTTP 连接，减少开销
- **智能缓存**: 内存+磁盘两级缓存
- **异步处理**: 后台任务不阻塞响应
- **优先级队列**: 高质量数据源优先返回

## 相关文档

- [PLUGIN_FIX_GUIDE.md](PLUGIN_FIX_GUIDE.md) - 插件开发与修复指南（推荐）
- [HOW_TO_USE.md](HOW_TO_USE.md) - 使用指南
- [PROJECT_STATUS.md](PROJECT_STATUS.md) - 项目状态

## 许可证

见 [LICENSE](LICENSE) 文件

---

**版本**: v7.0  
**更新**: 2026-02-08  
**状态**: ✅ Pioz2 插件工作正常，返回44个搜索结果

