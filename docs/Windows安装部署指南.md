# PanSou Windows 安装部署指南

> **完整的 Windows 安装和部署指南，支持任意目录安装**

---

## 📋 系统要求

- **操作系统**: Windows 10/11 或 Windows Server 2019+
- **内存**: 最低 2GB，推荐 4GB+
- **存储**: 至少 1GB 可用空间
- **网络**: 稳定的互联网连接

---

## 🚀 快速开始

### 方法1：使用通用部署脚本（推荐）⭐

**支持任意目录安装**，包括：
- `C:\Program Files\pansou`
- `D:\Apps\pansou`
- `C:\Users\YourName\pansou`
- 任何包含空格的路径

```cmd
# 1. 解压到任意目录
# 例如：C:\Program Files\pansou

# 2. 打开命令提示符（以管理员身份，如果在Program Files下）
cd "C:\Program Files\pansou"

# 3. 运行通用部署脚本
deploy-universal.bat

# 4. 按提示选择是否立即启动
```

### 方法2：源码编译

```cmd
# 1. 下载项目
git clone https://github.com/fish2018/pansou.git
cd pansou

# 2. 编译项目
go build -o pansou.exe

# 3. 启动服务
start.bat
```

---

## 📖 详细安装步骤

### 步骤 1: 安装 Go 环境

1. 访问 [Go 官网](https://golang.org/dl/)
2. 下载 Windows 版本（推荐 go1.21+）
3. 运行 `.msi` 安装包
4. 验证安装：
   ```cmd
   go version
   ```

### 步骤 2: 获取源码

```cmd
# 使用 Git 克隆
git clone https://github.com/fish2018/pansou.git
cd pansou

# 或直接下载 ZIP 并解压
```

### 步骤 3: 编译项目

```cmd
# 下载依赖（如果慢，设置代理）
go env -w GOPROXY=https://goproxy.cn,direct
go mod download

# 编译生成可执行文件
go build -o pansou.exe
```

### 步骤 4: 配置启动脚本

创建或编辑 `start.bat`：

```batch
@echo off
chcp 65001 >nul
title PanSou 网盘搜索服务

REM 基础配置
set PORT=8889
set CACHE_ENABLED=true
set CACHE_PATH=.\cache
set TZ=Asia/Shanghai

REM Telegram 频道配置
set CHANNELS=tgsearchers3,tgsearchers4,Aliyun_4K_Movies,bdbdndn11,yunpanx

REM 插件配置
set ASYNC_PLUGIN_ENABLED=true
set ENABLED_PLUGINS=pioz,xdpan,wanou,pansearch,zhizhen

REM 性能配置
set CONCURRENCY=25
set ASYNC_RESPONSE_TIMEOUT=5
set ASYNC_MAX_BACKGROUND_WORKERS=12
set CACHE_MAX_SIZE=300
set CACHE_TTL=90

REM 认证配置（可选）
set AUTH_ENABLED=true
set AUTH_USERS=admin:123456

REM 创建缓存目录
if not exist "%CACHE_PATH%" mkdir "%CACHE_PATH%"

echo 🚀 启动 PanSou 服务器...
echo 访问地址: http://localhost:%PORT%
echo 健康检查: http://localhost:%PORT%/api/health
echo.

pansou.exe
pause
```

### 步骤 5: 启动服务

```cmd
# 双击运行 start.bat 或命令行执行
start.bat
```

### 步骤 6: 验证安装

访问：http://localhost:8889/api/health

看到 JSON 响应即表示安装成功！

---

## 🔧 配置说明

### 端口配置

```batch
set PORT=8889  # 修改为你想要的端口
```

### 插件配置

```batch
# 启用的插件列表（逗号分隔）
set ENABLED_PLUGINS=pioz,xdpan,wanou,pansearch,zhizhen

# pioz 插件放在最前面表示优先搜索
```

### 认证配置

```batch
set AUTH_ENABLED=true           # 启用认证
set AUTH_USERS=admin:123456     # 用户名:密码
```

### 性能配置

```batch
set CONCURRENCY=25                    # 并发数
set CACHE_MAX_SIZE=300               # 缓存大小(MB)
set ASYNC_MAX_BACKGROUND_WORKERS=12  # 后台工作者数量
```

---

## 🛠️ 使用 API

### 健康检查

```bash
curl http://localhost:8889/api/health
```

### 登录获取 Token

```bash
curl -X POST http://localhost:8889/api/auth/login \
  -H "Content-Type: application/json" \
  -d "{\"username\":\"admin\",\"password\":\"123456\"}"
```

### 搜索资源

```bash
curl -X POST http://localhost:8889/api/search \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <token>" \
  -d "{\"kw\":\"速度与激情\",\"res\":\"merge\"}"
```

### 使用客户端

#### Python 客户端
```bash
pip install requests
python api-client-examples/python_client.py "速度与激情"
```

#### PowerShell 客户端
```powershell
.\api-client-examples\powershell_client.ps1 -Keyword "速度与激情"
```

#### Web 客户端
直接在浏览器中打开 `api-client-examples/web_client.html`

---

## 🚨 常见问题

### Q1: 编译失败

**错误**: `go: cannot find main module`  
**解决**: 确保在项目根目录（包含 `go.mod` 文件）中执行

**错误**: `timeout: dial tcp: i/o timeout`  
**解决**: 设置代理
```cmd
go env -w GOPROXY=https://goproxy.cn,direct
```

### Q2: 端口被占用

**错误**: `listen tcp :8889: bind: address already in use`  
**解决**: 更改端口或结束占用进程
```cmd
# 检查占用
netstat -ano | findstr :8889

# 结束进程
taskkill /PID <进程ID> /F

# 或更改端口
set PORT=9999
```

### Q3: 防火墙阻止

**现象**: 外部无法访问服务  
**解决**: 
1. 打开 Windows 安全中心
2. 防火墙和网络保护
3. 允许应用通过防火墙
4. 添加 `pansou.exe`

### Q4: 插件无法加载

**现象**: 搜索结果只有 TG 频道  
**解决**: 检查插件配置
```cmd
set ENABLED_PLUGINS=pioz,xdpan,wanou,pansearch,zhizhen
set ASYNC_PLUGIN_ENABLED=true
```

### Q5: 缓存问题

**现象**: 搜索结果不更新  
**解决**: 清理缓存
```cmd
# 停止服务后删除缓存目录
rmdir /s /q cache
```

---

## 🎯 性能优化

### 系统优化

1. 关闭不必要的后台程序
2. 使用 SSD 存储
3. 增加虚拟内存
4. 定期清理临时文件

### 配置优化

```batch
REM 高性能配置（根据硬件调整）
set CONCURRENCY=25
set CACHE_MAX_SIZE=300
set HTTP_MAX_CONNS=200
set ASYNC_MAX_BACKGROUND_WORKERS=12
set ASYNC_MAX_BACKGROUND_TASKS=60
```

---

## 📊 监控和维护

### 查看日志

```cmd
# 将日志输出到文件
pansou.exe > pansou.log 2>&1

# 实时查看日志（需要 tail 工具）
tail -f pansou.log
```

### 性能监控

```powershell
# 查看进程资源使用
Get-Process pansou | Select-Object CPU,WorkingSet,VirtualMemorySize

# 查看网络连接
Get-NetTCPConnection -LocalPort 8889
```

---

## 🔄 重新部署

### 自动部署（推荐）

```cmd
# 运行自动部署脚本
deploy.bat
```

脚本会自动：
1. 停止现有服务
2. 备份旧文件
3. 复制新文件
4. 验证部署
5. 询问是否启动

### 手动部署

```cmd
# 1. 停止服务
taskkill /F /IM pansou.exe

# 2. 备份
copy pansou.exe pansou.exe.backup

# 3. 编译新版本
go build -o pansou.exe

# 4. 启动服务
start.bat
```

---

## 🔨 重新编译（获取最新功能）

如果你看到旧版本的日志格式或缺少新功能，需要重新编译：

### 快速编译

```cmd
# 方法1：使用提供的脚本
rebuild.bat

# 方法2：手动编译
go build -o pansou.exe

# 方法3：清理后重新编译
del pansou.exe
go build -o pansou.exe
```

### 验证新功能

**1. 简洁的URL日志**：
```
✅ 新版本：[labi] xiaocge.fun
❌ 旧版本：[labi] 搜索 URL: http://xiaocge.fun/index.php/vod/search/wd/%E5%BC%80...
```

**2. 搜索完成摘要**：
```
📊 [labi] 搜索完成: 5个结果 | 耗时: 234ms
📊 [zhizhen] 搜索完成: 3个结果 | 耗时: 189ms
✅ [搜索完成] 总结果: 10 | 插件结果: labi(5), zhizhen(3)
```

---

## 📁 文件说明

| 文件 | 用途 |
|------|------|
| `pansou.exe` | 主程序（编译后生成） |
| `start.bat` | 启动脚本 |
| `deploy.bat` | 自动部署脚本 |
| `go.mod` | Go 模块配置 |
| `plugin/` | 插件目录 |
| `cache/` | 缓存目录（自动创建） |

---

## 🎉 完成！

现在你已经成功安装了 PanSou！

### 验证安装

1. ✅ 访问 http://localhost:8889/api/health
2. ✅ 看到服务状态信息
3. ✅ 尝试搜索测试

### 下一步

- 📖 阅读 [插件开发指南](插件开发指南.md) - 开发新插件
- 📖 阅读 [新增插件和重新部署流程](新增插件和重新部署流程.md) - 完整开发流程
- 🔧 查看 [搜索源配置说明](搜索源配置说明.md) - 配置搜索源
- 🔧 查看 [插件配置说明](插件配置说明.md) - 配置插件参数

### 获取帮助

- 📚 查看 [PanSou安装配置问答集](PanSou安装配置问答集.md)
- 🐛 提交 GitHub Issues
- 💬 参与社区讨论

---

**最后更新**: 2025-01-31  
**适用版本**: PanSou v1.0+  
**维护者**: abcxyzNone  
**致谢**: PanSou Team


---

**维护者**: abcxyzNone  
**AI工具**: Kiro (Claude Sonnet 4.5)  
**致谢**: PanSou Team & fish2018
