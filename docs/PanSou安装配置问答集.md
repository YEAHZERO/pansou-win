# PanSou 安装配置问答集

本文档整理了 PanSou 在 Windows 系统上安装和配置过程中的常见问题和解决方案。

## 📋 目录

- [项目概述](#项目概述)
- [安装方式选择](#安装方式选择)
- [Windows 源码安装](#windows-源码安装)
- [配置文件生成](#配置文件生成)
- [端口占用问题](#端口占用问题)
- [API 使用方式](#api-使用方式)
- [客户端示例](#客户端示例)
- [故障排除](#故障排除)

---

## 项目概述

### Q: PanSou 是什么项目？

**A**: PanSou 是一个高性能的网盘资源搜索API服务，采用 Go + TypeScript 双语言架构，支持 Telegram 频道搜索和自定义插件搜索。

**核心特性**:
- **异步插件系统**: 双级超时控制（4秒快速响应 + 30秒完整处理）
- **二级缓存系统**: 分片内存缓存 + 分片磁盘缓存
- **智能排序算法**: 插件等级权重52% + 关键词匹配权重22% + 时间新鲜度权重26%
- **支持70+个搜索插件**: 如jikepan、pan666、hunhepan等
- **支持12种网盘类型**: 百度、阿里云、夸克、天翼云、UC、移动云、115、PikPak、迅雷、123、磁力链接、电驴链接

### Q: 项目的整体架构是什么？

**A**: 
```
┌─────────────────────────────────────────────────────────────┐
│                    PanSou 搜索系统                            │
├─────────────────────────────────────────────────────────────┤
│  Go 后端服务 (8888端口)     │  TypeScript MCP服务            │
│  ├─ API层 (Gin框架)        │  ├─ MCP协议实现                │
│  ├─ 搜索服务层              │  ├─ 后端管理器                 │
│  ├─ 异步插件系统            │  └─ HTTP客户端                 │
│  ├─ 二级缓存系统            │                               │
│  └─ 工作池管理              │                               │
└─────────────────────────────────────────────────────────────┘
```

---

## 安装方式选择

### Q: 在 Windows 上有哪些安装方式？

**A**: PanSou 在 Windows 上支持以下安装方式：

1. **源码编译安装**（推荐，灵活性高）
   ```cmd
   git clone https://github.com/fish2018/pansou.git
   cd pansou
   go build -o pansou.exe .
   ```

2. **预编译二进制**（简单快速）
   - 从 GitHub Releases 下载 `pansou-windows-amd64.exe`
   - 重命名为 `pansou.exe`

### Q: 如何进行源码编译安装？

**A**: 推荐使用源码编译安装，步骤如下：

1. **安装 Go 环境**
   - 访问 https://golang.org/dl/
   - 下载 Windows .msi 文件
   - 双击安装，验证：`go version`

2. **获取源码**
   ```cmd
   git clone https://github.com/fish2018/pansou.git
   cd pansou
   ```

3. **一键安装**
   ```cmd
   install-windows.bat
   # 选择 "1. 源码编译安装"
   ```

4. **启动服务**
   ```cmd
   start-pansou.bat
   ```

---

## Windows 源码安装

### Q: 如何在 Windows 上进行源码安装？

**A**: 详细步骤：

#### 步骤 1: 环境准备
```cmd
# 检查 Go 版本（需要 >= 1.21）
go version

# 如果没有安装 Go，访问 https://golang.org/dl/ 下载安装
```

#### 步骤 2: 获取源码
```cmd
# 方式一：Git 克隆
git clone https://github.com/fish2018/pansou.git
cd pansou

# 方式二：直接下载 ZIP 并解压
```

#### 步骤 3: 编译
```cmd
# 下载依赖
go mod download

# 如果下载慢，设置代理
go env -w GOPROXY=https://goproxy.cn,direct

# 编译
go build -o pansou.exe .
```

#### 步骤 4: 配置和启动
使用提供的启动脚本或手动配置环境变量。

### Q: 编译过程中遇到问题怎么办？

**A**: 常见问题解决：

1. **依赖下载失败**
   ```cmd
   go env -w GOPROXY=https://goproxy.cn,direct
   go clean -modcache
   go mod download
   ```

2. **Go 版本过低**
   - 确保 Go 版本 >= 1.21
   - 重新下载安装最新版本

3. **网络问题**
   - 使用代理或 VPN
   - 尝试不同的 GOPROXY 设置

---

## 配置文件生成

### Q: 如何生成自定义的启动配置？

**A**: 根据你的需求，我可以生成定制的 `start.bat` 文件。

### Q: 需要生成一个 start.bat，设置认证用户名为 admin，密码为 123456，pansou.exe 路径为 C:\Users\Administrator\pansou\pansou.exe

**A**: 已生成 `start.bat` 文件，配置如下：

**路径配置**:
- 可执行文件: `C:\Users\Administrator\pansou\pansou.exe`
- 缓存目录: `C:\Users\Administrator\pansou\cache`
- 工作目录: `C:\Users\Administrator\pansou`

**认证配置**:
- 认证状态: 已启用
- 用户名: `admin`
- 密码: `123456`

**服务配置**:
- 端口: 8888
- 缓存: 已启用
- 插件系统: 已启用
- 并发数: 15

**启用的插件**:
```
labi,zhizhen,shandian,duoduo,muou,wanou,hunhepan,jikepan,pansearch,panta,qupansou,hdr4k,pan666,susu,thepiratebay,xuexizhinan,panyq,ouge,huban
```

### Q: 不想设置 Token 有效期怎么办？

**A**: 已更新配置，移除了 `AUTH_TOKEN_EXPIRY` 设置。现在 Token 将永不过期（或使用系统默认值），一次登录可长期使用。

---

## 端口占用问题

### Q: 启动时显示端口被占用错误怎么办？

**A**: 错误信息：
```
listen tcp :8888: bind: Only one usage of each socket address (protocol/network address/port) is normally permitted.
```

这表示端口 8888 已被占用。解决方案：

#### 方案一：使用改进的启动脚本
更新后的 `start.bat` 会自动：
- ✅ 检测端口 8888 是否被占用
- ✅ 显示占用进程的详细信息
- ✅ 自动切换到端口 8889

#### 方案二：使用端口检查工具
运行 `check-port.bat`：
- 🔍 检查端口占用详情
- 🛠️ 提供多种解决方案
- ⚡ 可以自动结束占用进程

#### 手动解决步骤：
1. **检查占用进程**
   ```cmd
   netstat -ano | findstr :8888
   ```

2. **结束占用进程**
   ```cmd
   taskkill /PID <进程ID> /F
   ```

3. **或更改端口**
   ```cmd
   set PORT=8889
   ```

### Q: 如何创建端口检查工具？

**A**: 已创建 `check-port.bat` 工具，功能包括：

- **端口状态检查**: 自动检测端口 8888 占用情况
- **进程信息显示**: 显示占用进程的详细信息
- **多种解决方案**: 
  - 手动关闭进程
  - 自动结束进程（谨慎使用）
  - 使用其他端口
- **安全提示**: 在自动结束进程前会确认

使用方法：
```cmd
check-port.bat
# 根据提示选择解决方案
```

---

## API 使用方式

### Q: 如何直接使用 HTTP API 而不依赖 MCP？

**A**: PanSou 提供标准的 HTTP API，可以通过任何支持 HTTP 的工具调用：

#### 基础 API 调用
```bash
# 健康检查
curl http://localhost:8888/api/health

# 登录获取 Token（如果启用认证）
curl -X POST http://localhost:8888/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"123456"}'

# 搜索资源
curl -X POST http://localhost:8888/api/search \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -d '{"kw":"速度与激情","res":"merge"}'
```

#### 使用官方服务
```bash
# 连接官方服务 https://so.252035.xyz
curl -X POST https://so.252035.xyz/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"your_password"}'
```

### Q: API 支持哪些参数？

**A**: 主要 API 参数：

**搜索接口 `/api/search`**:
- `kw`: 搜索关键词（必填）
- `res`: 返回格式（`merge`/`all`/`results`）
- `src`: 数据源（`all`/`tg`/`plugin`）
- `cloud_types`: 网盘类型过滤
- `plugins`: 指定插件列表
- `refresh`: 强制刷新缓存
- `ext`: 扩展参数

**认证接口**:
- `/api/auth/login`: 用户登录
- `/api/auth/verify`: 验证 Token
- `/api/auth/logout`: 退出登录

---

## 客户端示例

### Q: 有哪些现成的客户端可以使用？

**A**: 项目提供了多种语言的客户端示例：

#### Python 客户端
```bash
# 安装依赖
pip install requests

# 使用客户端
python api-client-examples/python_client.py "速度与激情" \
  --url http://localhost:8888 \
  --username admin \
  --password 123456
```

**特性**:
- ✅ 完整的命令行参数支持
- ✅ 彩色输出和格式化显示
- ✅ 详细的错误处理和提示
- ✅ 健康检查功能

#### PowerShell 客户端
```powershell
.\api-client-examples\powershell_client.ps1 \
  -Keyword "速度与激情" \
  -ApiUrl "http://localhost:8888" \
  -Username "admin" \
  -Password "123456"
```

**特性**:
- ✅ Windows 原生支持
- ✅ 完整的参数验证
- ✅ 彩色控制台输出
- ✅ 详细的帮助信息

#### Web 客户端
直接在浏览器中打开 `api-client-examples/web_client.html`

**特性**:
- ✅ 现代化的响应式界面
- ✅ 实时搜索状态显示
- ✅ 多种网盘类型选择
- ✅ 自动服务状态检测
- ✅ 移动端友好设计

### Q: 如何自定义开发客户端？

**A**: 基础 HTTP 请求示例：

```bash
# 健康检查
curl https://so.252035.xyz/api/health

# 登录
curl -X POST https://so.252035.xyz/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"your_password"}'

# 搜索
curl -X POST https://so.252035.xyz/api/search \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -d '{"kw":"速度与激情","res":"merge"}'
```

**错误处理**:
- **401 Unauthorized**: 认证失败或 token 过期
- **400 Bad Request**: 请求参数错误
- **500 Internal Server Error**: 服务器内部错误

**最佳实践**:
1. 缓存 Token，避免频繁登录
2. 实现错误重试机制
3. 控制请求频率
4. 提供用户友好的界面

---

## 故障排除

### Q: 常见的安装和运行问题有哪些？

**A**: 

#### 编译问题
1. **Go 版本过低**
   - 确保 Go >= 1.21
   - 重新安装最新版本

2. **依赖下载失败**
   ```cmd
   go env -w GOPROXY=https://goproxy.cn,direct
   go clean -modcache
   go mod download
   ```

3. **找不到 go.mod**
   - 确保在项目根目录执行命令
   - 检查项目完整性

#### 运行问题
1. **端口被占用**
   - 使用 `check-port.bat` 检查
   - 更改端口或结束占用进程

2. **防火墙阻止**
   - 在 Windows 防火墙中添加 `pansou.exe` 例外
   - 检查杀毒软件设置

3. **插件无法加载**
   ```cmd
   # 检查插件配置
   set ENABLED_PLUGINS=labi,zhizhen,shandian,duoduo
   set ASYNC_PLUGIN_ENABLED=true
   ```

4. **缓存问题**
   ```cmd
   # 清理缓存
   rmdir /s /q cache
   ```

#### 性能问题
1. **响应慢**
   ```cmd
   # 调整并发数
   set CONCURRENCY=20
   set ASYNC_MAX_BACKGROUND_WORKERS=12
   ```

2. **内存占用高**
   ```cmd
   # 限制缓存大小
   set CACHE_MAX_SIZE=100
   ```

### Q: 如何监控服务状态？

**A**: 

#### 查看日志
```cmd
# 输出到文件
pansou.exe > pansou.log 2>&1

# 实时查看
tail -f pansou.log
```

#### 性能监控
```powershell
# 查看进程资源使用
Get-Process pansou | Select-Object CPU,WorkingSet,VirtualMemorySize

# 查看网络连接
Get-NetTCPConnection -LocalPort 8888
```

#### 健康检查
```bash
# 定期检查服务状态
curl http://localhost:8888/api/health
```

---

## 总结

### Q: 推荐的最佳实践是什么？

**A**: 

#### 安装建议
1. **使用源码编译**: 灵活性高，便于定制
2. **配置认证**: 提高安全性
3. **使用启动脚本**: 简化操作，避免手动配置
4. **定期更新**: 获取最新功能和修复

#### 使用建议
1. **选择合适的客户端**: Python（功能全面）、PowerShell（Windows原生）、Web（界面友好）
2. **合理配置插件**: 根据需求启用相关插件
3. **监控服务状态**: 定期检查健康状态和性能
4. **备份配置**: 保存重要的配置文件

#### 安全建议
1. **启用认证**: 防止未授权访问
2. **定期更换密码**: 提高安全性
3. **限制网络访问**: 仅允许必要的连接
4. **监控日志**: 及时发现异常

---

## 相关文档

- [Windows源码安装指南.md](Windows源码安装指南.md) - 详细安装指南
- [纯API使用指南.md](纯API使用指南.md) - API 使用说明
- [api-client-examples/](../api-client-examples/) - 客户端示例
- [README.md](../README.md) - 项目主文档

---

*本文档基于实际问答整理，持续更新中。如有问题请参考相关文档或提交 GitHub Issues。*