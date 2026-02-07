# PanSou 快速开始 v6.1

## 一键启动

```bash
# 1. 编译
go build -o pansou.exe

# 2. 启动
start.bat

# 3. 测试
test-pioz.bat
```

---

## 配置说明

### 当前配置（v6.1 优化版）

- ✅ **只使用 Pioz**（已禁用 Telegram）
- ✅ **网站地址**：https://www.pioz.cn
- ✅ **响应超时**：10秒（优化）
- ✅ **工作线程**：16个（优化）
- ✅ **缓存配置**：500MB / 120分钟（优化）

---

## 登录认证

```bash
# 获取Token
curl -X POST http://localhost:8889/api/auth/login \
  -H "Content-Type: application/json" \
  -d "{\"username\":\"admin\",\"password\":\"123456\"}"

# 返回示例
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "expires_at": "2025-02-14T12:00:00Z"
}
```

---

## 搜索测试

```bash
# 使用Token搜索（只使用Pioz，不使用Telegram）
curl -X POST http://localhost:8889/api/search \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"keyword\":\"电影\",\"channels\":[],\"concurrency\":5}"
```

---

## 测试工具

### 1. Pioz 专项测试（推荐）

```bash
test-pioz.bat
```

**功能**：
- 测试 Pioz 网站访问
- 测试 3 个不同关键词
- 保存结果到 JSON 文件

### 2. 系统诊断

```bash
diagnose.bat
```

**功能**：
- 检查服务状态
- 检查 Pioz 配置
- 快速搜索测试

---

## 配置修改

编辑 `start.bat`：

```batch
# 端口
set PORT=8889

# Telegram（已禁用）
set CHANNELS=

# 插件（只使用Pioz）
set ENABLED_PLUGINS=pioz

# 认证
set AUTH_ENABLED=true
set AUTH_USERS=admin:123456

# 性能（已优化）
set CONCURRENCY=15
set ASYNC_RESPONSE_TIMEOUT=10
set ASYNC_MAX_BACKGROUND_WORKERS=16
set CACHE_MAX_SIZE=500
set CACHE_TTL=120
```

---

## 常见问题

### Pioz 返回 0 个结果

**检查**：
```bash
# 1. 测试网站访问
curl -I https://www.pioz.cn/

# 2. 运行诊断
diagnose.bat

# 3. 查看日志
# 启动服务后查看控制台输出
```

**解决**：
```batch
# 增加超时时间
set ASYNC_RESPONSE_TIMEOUT=15

# 或配置代理
set PROXY=http://127.0.0.1:7890
```

### 搜索很慢

**首次搜索**：
- 5-10秒是正常的（无缓存）
- 后续搜索会很快（有缓存）

**优化**：
```batch
# 增加缓存
set CACHE_MAX_SIZE=1000
set CACHE_TTL=180
```

### 编译失败

```bash
go clean -modcache
go mod download
go build -o pansou.exe
```

---

## 优化亮点

### v6.1 优化

1. ✅ **禁用 Telegram** - 专注 Pioz 搜索
2. ✅ **增加超时** - 10秒响应，20秒插件
3. ✅ **更多线程** - 16个工作线程
4. ✅ **更大缓存** - 500MB，120分钟
5. ✅ **专项测试** - test-pioz.bat

### 性能提升

- 响应超时：5秒 → 10秒（+100%）
- 工作线程：12 → 16（+33%）
- 缓存大小：300MB → 500MB（+67%）
- 缓存时间：90分钟 → 120分钟（+33%）

---

## 文档

- [README.md](README.md) - 完整文档
- [OPTIMIZATION_APPLIED.md](OPTIMIZATION_APPLIED.md) - 优化详情
- [官方说明](docs/PanSou网盘搜索官方说明.md)
- [安装指南](docs/Windows安装部署指南.md)
- [FAQ](docs/PanSou安装配置问答集.md)

---

## 客户端示例

- Python: `api-client-examples/python_client.py`
- PowerShell: `api-client-examples/powershell_client.ps1`
- Web: `api-client-examples/web_client.html`

---

**版本**: v6.1 (优化版)  
**更新**: 2025-02-07  
**状态**: ✅ 已优化，专注 Pioz
