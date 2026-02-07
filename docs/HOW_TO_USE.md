# 如何使用 PanSou v6.1

## 问题修复

之前的测试遇到了以下问题：
1. ✅ 端口 8889 被占用
2. ✅ 插件未加载（环境变量未设置）
3. ✅ 批处理文件编码问题

**已修复！**

---

## 快速开始（3步）

### 1. 停止旧服务

```bash
stop.bat
```

### 2. 启动新服务

```bash
start-simple.bat
```

**重要**：使用 `start-simple.bat` 而不是 `start.bat`
- 已修复环境变量问题
- 已优化配置
- 已禁用 Telegram

### 3. 运行测试

```bash
quick-test.bat
```

---

## 详细说明

### 启动脚本对比

| 脚本 | 说明 | 推荐 |
|------|------|------|
| `start-simple.bat` | 简化版，环境变量正确设置 | ✅ 推荐 |
| `start.bat` | 原版，可能有编码问题 | ⚠️ 备用 |

### 测试脚本对比

| 脚本 | 说明 | 推荐 |
|------|------|------|
| `quick-test.bat` | 快速测试（1个关键词） | ✅ 推荐 |
| `test-pioz.bat` | 完整测试（3个关键词） | 📝 详细测试 |
| `diagnose.bat` | 系统诊断 | 🔧 故障排除 |

---

## 完整流程

### 步骤 1: 停止旧服务

```bash
stop.bat
```

**输出示例**：
```
Found PanSou process, stopping...
[OK] PanSou service stopped
[OK] Port 8889 is free
```

### 步骤 2: 启动服务

```bash
start-simple.bat
```

**输出示例**：
```
Configuration:
================================
Port: 8889
Telegram: DISABLED
Plugins: pioz
Timeout: 10 seconds
Workers: 16
Cache: 500 MB / 120 minutes
================================

Starting PanSou service...
服务器启动在 http://localhost:8889
已启用指定插件 (1个):
  - pioz (优先级: 1)
```

**关键检查**：
- ✅ 确认显示 "已启用指定插件 (1个): pioz"
- ✅ 确认端口 8889 正常监听
- ✅ 确认没有错误信息

### 步骤 3: 快速测试

**打开新的命令行窗口**，运行：

```bash
quick-test.bat
```

**输出示例**：
```
[1] Health Check
{"status":"ok"}

[2] Login
[OK] Login successful

[3] Search Test
[4] Results
{"total":10,"results":[...]}

Test completed!
```

---

## 常见问题

### 1. 端口被占用

**症状**：
```
listen tcp :8889: bind: Only one usage of each socket address
```

**解决**：
```bash
stop.bat
```

### 2. 插件未加载

**症状**：
```
未设置插件列表 (ENABLED_PLUGINS)，未加载任何插件
```

**解决**：
使用 `start-simple.bat` 而不是 `start.bat`

### 3. 批处理文件错误

**症状**：
```
'o' is not recognized as an internal or external command
'.' is not recognized as an internal or external command
```

**原因**：
`echo.` 语法在某些系统上有问题

**解决**：
使用新的脚本（已修复）

---

## 验证清单

启动服务后，检查以下内容：

- [ ] 端口 8889 正常监听
- [ ] 显示 "已启用指定插件 (1个): pioz"
- [ ] 没有错误信息
- [ ] 健康检查返回 `{"status":"ok"}`
- [ ] 登录成功获取 Token
- [ ] 搜索返回结果

---

## 配置说明

### start-simple.bat 配置

```batch
# 端口
set PORT=8889

# Telegram（已禁用）
set CHANNELS=

# 插件（必须设置！）
set ASYNC_PLUGIN_ENABLED=true
set ENABLED_PLUGINS=pioz

# 性能（已优化）
set CONCURRENCY=15
set ASYNC_RESPONSE_TIMEOUT=10
set ASYNC_MAX_BACKGROUND_WORKERS=16
set ASYNC_MAX_BACKGROUND_TASKS=80
set CACHE_MAX_SIZE=500
set CACHE_TTL=120
set PLUGIN_TIMEOUT=20

# 认证
set AUTH_ENABLED=true
set AUTH_USERS=admin:123456

# HTTP
set HTTP_READ_TIMEOUT=40
set HTTP_WRITE_TIMEOUT=40
set HTTP_MAX_CONNS=300
```

---

## 手动测试

如果自动测试失败，可以手动测试：

### 1. 健康检查

```bash
curl http://localhost:8889/api/health
```

**预期输出**：
```json
{"status":"ok"}
```

### 2. 登录

```bash
curl -X POST http://localhost:8889/api/auth/login ^
  -H "Content-Type: application/json" ^
  -d "{\"username\":\"admin\",\"password\":\"123456\"}"
```

**预期输出**：
```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "expires_at": "2025-02-14T12:00:00Z"
}
```

### 3. 搜索

```bash
curl -X POST http://localhost:8889/api/search ^
  -H "Authorization: Bearer YOUR_TOKEN" ^
  -H "Content-Type: application/json" ^
  -d "{\"keyword\":\"movie\",\"channels\":[],\"concurrency\":5}"
```

**预期输出**：
```json
{
  "total": 10,
  "results": [...]
}
```

---

## 脚本清单

### 核心脚本

| 脚本 | 功能 | 使用场景 |
|------|------|----------|
| `start-simple.bat` | 启动服务（推荐） | 日常使用 |
| `stop.bat` | 停止服务 | 重启前 |
| `quick-test.bat` | 快速测试 | 验证功能 |

### 高级脚本

| 脚本 | 功能 | 使用场景 |
|------|------|----------|
| `test-pioz.bat` | 完整测试 | 详细测试 |
| `diagnose.bat` | 系统诊断 | 故障排除 |
| `deploy.bat` | 部署脚本 | 更新部署 |

---

## 下一步

1. ✅ 运行 `stop.bat` 停止旧服务
2. ✅ 运行 `start-simple.bat` 启动新服务
3. ✅ 运行 `quick-test.bat` 验证功能
4. 📝 如果成功，可以运行 `test-pioz.bat` 进行详细测试
5. 📝 查看结果文件了解搜索效果

---

**版本**: v6.1  
**更新**: 2025-02-07  
**状态**: ✅ 问题已修复，可以使用
