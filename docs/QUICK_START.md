# PanSou 快速开始

## 一键启动

```bash
# 1. 编译
go build -o pansou.exe

# 2. 启动
start.bat

# 3. 测试
curl http://localhost:8889/api/health
```

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
# 使用Token搜索
curl "http://localhost:8889/api/search?keyword=电影" \
  -H "Authorization: Bearer YOUR_TOKEN"
```

---

## 配置修改

编辑 `start.bat`：

```batch
# 端口
set PORT=8889

# 插件（当前只启用pioz）
set ENABLED_PLUGINS=pioz

# 认证
set AUTH_ENABLED=true
set AUTH_USERS=admin:123456

# 性能
set CONCURRENCY=25
set CACHE_MAX_SIZE=300
```

---

## 恢复备用插件

### 1. 编辑 main.go
```go
// 取消注释需要的插件
_ "pansou/plugin/pansearch"
_ "pansou/plugin/wanou"
```

### 2. 编辑 start.bat
```batch
set ENABLED_PLUGINS=pioz,pansearch,wanou
```

### 3. 重新编译部署
```bash
go build -o pansou.exe
deploy.bat
```

---

## 常见问题

### 端口被占用
```bash
# 查看占用
netstat -ano | findstr :8889

# 结束进程
taskkill /F /PID 进程ID
```

### 编译失败
```bash
go clean -modcache
go mod download
go build -o pansou.exe
```

### 搜索无结果
检查配置：
```batch
set ENABLED_PLUGINS=pioz
set ASYNC_PLUGIN_ENABLED=true
```

---

## 文档

- [官方说明](docs/PanSou网盘搜索官方说明.md)
- [安装指南](docs/Windows安装部署指南.md)
- [FAQ](docs/PanSou安装配置问答集.md)

---

## 客户端示例

- Python: `api-client-examples/python_client.py`
- PowerShell: `api-client-examples/powershell_client.ps1`
- Web: `api-client-examples/web_client.html`

---

**版本**: v6.0 (精简版)  
**更新**: 2025-02-07
