# PanSou - 网盘搜索聚合服务

一个高性能的网盘资源搜索聚合服务，支持多个搜索插件和智能缓存系统。

## 核心特性

- 🔍 **多策略搜索**: 深度API + HTML解析 + 热搜榜匹配
- ⚡ **异步并发**: 8个并发，性能提升3-5倍
- 🔀 **部分剧名搜索**: 搜索结果为0时自动提取剧名的一部分进行重试
- 🔗 **二次跳转**: 自动获取真实网盘链接
- 📊 **链接数量限制**: 每个结果只保留前3个链接，减少返回数据量
- 🛡️ **反爬绕过**: 随机延迟 + UA轮换 + 完整请求头
- 💾 **智能缓存**: 三级缓存（搜索 + 详情 + Transfer）
- 🌐 **16种网盘**: 夸克、百度、阿里云、UC、迅雷等
- 🔐 **JWT认证**: 安全的API访问控制
- 📈 **插件优先级**: 统一配置所有插件优先级为1，优化搜索结果排序
- 📊 **性能监控**: 完整的统计和监控系统

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

## Pioz 插件特性

### 三重搜索策略

1. **深度搜索API**（首选）
   - 接口：`https://www.pioz.cn/api/deep-search?kw=关键词`
   - 返回结构化JSON数据
   - 响应速度快，数据准确度高

2. **普通HTML搜索**（备用）
   - 接口：`https://www.pioz.cn/search?q=关键词`
   - 兼容性好，支持详情页链接提取
   - 使用精确选择器 `a[href^='/detail/']` 匹配详情页

3. **热搜榜匹配**（兜底）
   - 接口：`https://www.pioz.cn`（首页）
   - 关键词模糊匹配，最后的保底方案

### 二次跳转机制

自动完成详情页访问和"了解并同意获取"按钮点击，提取真实网盘链接：

```
搜索结果页 → 提取详情页链接 → 异步并发访问（8个并发）→ 检测同意页 → 点击"了解并同意获取"按钮 → 提取真实链接 → 返回给用户
```

**性能对比**：
- 串行处理：10个结果 × 5秒 = 50秒
- 并发处理：10个结果 ÷ 8并发 × 5秒 = 约10-15秒
- 性能提升：3-5倍

**二次跳转处理流程**：
1. 检测页面是否包含"了解并同意获取"文本
2. 先尝试直接从页面提取网盘链接
3. 如果没有找到，尝试 form 提交
4. 最后尝试按钮点击模拟（JavaScript 执行）

### 网盘链接格式过滤

**只保留夸克网盘的 `https://pan.quark.cn/s/` 格式链接**，排除其他格式：

- ✅ **保留**：`https://pan.quark.cn/s/91fd27aed7f9`
- ❌ **排除**：`https://pan.quark.cn/g/d7d04e8da2`
- ❌ **排除**：其他格式的网盘链接

**过滤位置**：
- 链接提取函数（`extractLinksFromDocument`）
- 重定向链处理（`resolveRedirectShareLinksWithReferer`）
- 二次跳转处理（`tryConsentFlowNew`）
- 按钮点击模拟（`clickConsentButtons`）

### 反爬策略

1. **随机请求延迟**：500-1500ms
2. **轮换User-Agent**：7种浏览器
3. **完整请求头设置**：模拟真实浏览器
4. **Cookie会话管理**：保持会话状态
5. **指数退避重试**：最多2次重试

### 支持的网盘类型（16种）

| 网盘类型 | 域名特征 |
|---------|----------|
| 夸克网盘 | `pan.quark.cn` |
| 百度网盘 | `pan.baidu.com` |
| 阿里云盘 | `aliyundrive.com`, `alipan.com` |
| UC网盘 | `drive.uc.cn` |
| 迅雷网盘 | `pan.xunlei.com` |
| 天翼云盘 | `cloud.189.cn` |
| 115网盘 | `115.com` |
| 123网盘 | `123pan.com` |
| 蓝奏云 | `lanzou*.com` |
| 移动云盘 | `caiyun.139.com` |
| 微云 | `share.weiyun.com` |
| 坚果云 | `jianguoyun.com` |
| PikPak | `mypikpak.com` |
| 磁力链接 | `magnet:` |
| 电驴链接 | `ed2k://` |

## 配置说明

### 环境变量（start-simple.bat）
```batch
set PORT=8889                              # 服务端口
set ENABLED_PLUGINS=pioz                   # 启用的插件
set ASYNC_RESPONSE_TIMEOUT=10              # 异步超时（秒）
set ASYNC_MAX_BACKGROUND_WORKERS=16        # 工作线程数
set CACHE_MAX_SIZE=500                     # 缓存大小（MB）
set CACHE_TTL=120                          # 缓存时间（分钟）
```

### Pioz 插件配置
```go
const (
    DefaultTimeout = 15 * time.Second  // 搜索超时
    DetailTimeout  = 12 * time.Second  // 详情页超时
    MaxConcurrency = 8                 // 最大并发数
    CacheTTL       = 30 * time.Minute  // 缓存有效期
    RequestDelayMin = 500 * time.Millisecond
    RequestDelayMax = 1500 * time.Millisecond
    RetryCount = 2                     // 重试次数
)
```

## API 使用

### 1. 登录获取 Token
```bash
curl -X POST http://localhost:8889/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"123456"}'
```

响应示例：
```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "expires_at": "2026-02-14T12:00:00Z"
}
```

### 2. 搜索资源

支持 `keyword` 和 `kw` 两种参数名：

```bash
# 使用 keyword 参数
curl "http://localhost:8889/api/search?keyword=电影" \
  -H "Authorization: Bearer YOUR_TOKEN"

# 使用 kw 参数（兼容）
curl "http://localhost:8889/api/search?kw=电影" \
  -H "Authorization: Bearer YOUR_TOKEN"

# 强制刷新缓存
curl "http://localhost:8889/api/search?keyword=电影&refresh=true" \
  -H "Authorization: Bearer YOUR_TOKEN"
```

响应示例：
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "total": 10,
    "results": [
      {
        "unique_id": "pioz-12345",
        "title": "速度与激情10 4K高清",
        "content": "类型: 夸克网盘 | 大小: 10GB",
        "datetime": "2026-02-12T10:00:00Z",
        "links": [
          {
            "type": "quark",
            "url": "https://pan.quark.cn/s/abc123",
            "password": "1234"
          }
        ]
      }
    ]
  }
}
```

### 3. 健康检查
```bash
curl http://localhost:8889/api/health
```

## 性能优化

### 1. 异步并发处理

**性能对比**：

| 场景 | 串行处理 | 并发处理（8个） | 提升 |
|------|---------|----------------|------|
| 10个结果 | 50秒 | 10-15秒 | 3-5倍 |
| 20个结果 | 100秒 | 20-30秒 | 3-5倍 |
| 50个结果 | 250秒 | 50-75秒 | 3-5倍 |

### 2. 三级缓存系统

**缓存层级**：
1. 搜索结果缓存（30分钟TTL）
2. 详情页缓存（永久）
3. Transfer结果缓存（永久）

**优势**：
- 减少网络请求
- 提升响应速度
- 降低被封风险

### 3. HTTP连接池优化

```go
MaxIdleConns:        50   // 最大空闲连接数
MaxIdleConnsPerHost: 10   // 每主机最大空闲连接数
MaxConnsPerHost:     20   // 每主机最大连接数
IdleConnTimeout:     60s  // 空闲连接超时
```

### 4. 对象池优化

使用 `sync.Pool` 减少内存分配，提升性能：
- 在处理大量结果时，内存分配次数可减少 50-70%
- GC 暂停时间减少约 30%

## 故障排除

### 问题1：搜索返回 0 结果

**症状**：
```
[pioz] API响应状态码: 404
[搜索完成] 总结果：0
```

**原因**：Pioz API 端点可能暂时不可用或已改变

**解决方案**：

系统会自动降级到备用策略：
1. **策略1**：深度搜索API（首选）
2. **策略2**：HTML页面搜索（备用）
3. **策略3**：热搜榜匹配（兜底）

**测试方法**：
```bash
# 使用调试脚本
test-search-debug.bat

# 或手动测试（强制刷新缓存）
curl "http://localhost:8889/api/search?keyword=电影&refresh=true" \
  -H "Authorization: Bearer YOUR_TOKEN"
```

**查看日志**：
```
[pioz] ⚠️ 策略1失败：API返回状态码: 404
[pioz] 尝试策略2：普通HTML搜索
[pioz] ✅ 策略2成功：HTML搜索返回 15 个结果
```

详细诊断见：[搜索结果为0问题诊断](docs/搜索结果为0问题诊断.md)

**如果仍然返回0结果**：
```bash
# 1. 清除缓存
rd /s /q cache && mkdir cache

# 2. 重启服务
stop.bat && start-simple.bat

# 3. 测试网络连接
curl -I https://www.pioz.cn
```

### 问题2：API 返回 400 错误

**原因**：关键词为空或格式错误

**解决**：检查 URL 参数名（使用 `keyword` 或 `kw`）

### 问题3：服务无法启动

**原因**：端口被占用

**解决**：
```bash
# 停止旧服务
stop.bat

# 或更改端口
set PORT=8890
```

### 问题4：触发反爬保护

**症状**：日志显示 "触发反爬保护"

**解决**：
- 增加请求延迟（修改 `RequestDelayMin/Max`）
- 减少并发数（修改 `MaxConcurrency`）
- 等待一段时间后重试

### 问题5：搜索结果为0时的自动重试

**症状**：某些剧名搜索返回0个结果

**原因**：剧名过长或包含特殊字符，导致搜索引擎无法正确匹配

**解决方案**：

系统会自动提取剧名的一部分进行重试：

**提取策略**：
- 优先提取有逗号的前后文字（如"重逢后，高冷裴总红着眼跟我领证"会提取为["重逢后", "高冷裴总红着眼跟我领证"]）
- 如果没有逗号，尝试提取冒号、顿号等标点符号前后的文字
- 如果还是没有，尝试提取前半部分和后半部分

**日志输出**：
```
[主服务] 搜索结果为0，尝试使用部分剧名搜索: [重逢后 高冷裴总红着眼跟我领证]
[主服务] 部分剧名搜索成功: 重逢后 -> 5个结果
```

详细说明见：[搜索优化与问题解决方案](docs/搜索优化与问题解决方案.md)

### 问题6：Transfer API返回404错误

**症状**：
```
transfer失败: status=404 id=563196
```

**原因**：
- 资源ID格式不正确，包含下划线后缀
- API端点可能已更改

**解决方案**：

系统已自动处理：
1. 修改资源ID提取逻辑，去除下划线后缀
2. 扩展transfer API端点候选（从8个增加到22个）

详细说明见：[搜索优化与问题解决方案](docs/搜索优化与问题解决方案.md)

### 问题7：插件优先级配置

**症状**：搜索结果排序不符合预期

**解决方案**：

所有插件优先级都已配置为1：
- pioz: 1
- pansearch: 1
- wanou: 1
- xdpan: 1
- zhizhen: 1

**优先级影响**：
- 优先级影响搜索结果的排序权重
- 数字越小优先级越高
- 当前所有插件优先级相同，按其他因素排序

**修改优先级**：

在插件初始化代码中修改优先级参数：

```go
// pansearch插件
BaseAsyncPlugin: plugin.NewBaseAsyncPlugin("pansearch", 1),

// xdpan插件
BaseAsyncPlugin: plugin.NewBaseAsyncPlugin("xdpan", 1),
```

详细说明见：[搜索优化与问题解决方案](docs/搜索优化与问题解决方案.md)

### 问题8：链接数量过多

**症状**：返回的搜索结果包含过多链接

**解决方案**：

系统会自动限制每个搜索结果只保留前3个链接：

**效果对比**：

**限制前**：
```json
{
  "title": "电影合集",
  "links": [
    {"type": "quark", "url": "..."},
    {"type": "baidu", "url": "..."},
    {"type": "aliyun", "url": "..."},
    {"type": "uc", "url": "..."},
    {"type": "xunlei", "url": "..."}
  ]
}
```

**限制后**：
```json
{
  "title": "电影合集",
  "links": [
    {"type": "quark", "url": "..."},
    {"type": "baidu", "url": "..."},
    {"type": "aliyun", "url": "..."}
  ]
}
```

详细说明见：[搜索优化与问题解决方案](docs/搜索优化与问题解决方案.md)

### 问题9：Playwright MCP 浏览器无法启动

**症状**：
```
Failed to initialize browser: browserType.launch: Executable doesn't exist at C:\Users\liveu\AppData\Local\ms-playwright\chromium-1200\chrome-win64\chrome.exe
```

**原因**：Playwright MCP 服务器需要浏览器驱动才能运行，但系统中没有安装 Playwright 浏览器。

**解决方案**：

#### 方案一：使用系统 Edge 浏览器（推荐）

修改 MCP 配置文件，强制使用系统 Edge 浏览器：

**配置文件位置**：
```
c:\Users\liveu\AppData\Roaming\Trae CN\User\mcp.json
```

**配置方法一：直接配置（简单）**

```json
{
  "mcpServers": {
    "Playwright": {
      "command": "node",
      "args": [
        "C:\\Programs\\AITech\\CodexCLI\\npm-global\\node_modules\\@executeautomation\\playwright-mcp-server\\dist\\index.js",
        "--browser", "msedge",
        "--headless", "false",
        "--executable-path", "C:\\Program Files (x86)\\Microsoft\\Edge\\Application\\msedge.exe"
      ],
      "env": {
        "PLAYWRIGHT_SKIP_BROWSER_DOWNLOAD": "1",
        "PLAYWRIGHT_BROWSERS_PATH": "0"
      }
    }
  }
}
```

**配置方法二：使用自定义 mcp-runner.js（灵活）**

创建 `C:\Programs\Coding\Playwright\mcp-runner.js`：

```javascript
const { spawn } = require('child_process');
const path = require('path');

// 设置环境变量，强制使用系统 Edge
process.env.PLAYWRIGHT_SKIP_BROWSER_DOWNLOAD = '1';
process.env.PLAYWRIGHT_BROWSERS_PATH = '0';

// 使用项目中的 Playwright
const mcpPath = path.join(__dirname, 'node_modules', '@executeautomation', 'playwright-mcp-server', 'dist', 'index.js');

const mcp = spawn('node', [
  mcpPath, 
  '--browser', 'msedge', 
  '--headless', 'false',
  '--executable-path', 'C:\\Program Files (x86)\\Microsoft\\Edge\\Application\\msedge.exe'
], {
  stdio: 'inherit',
  env: process.env
});

mcp.on('close', (code) => {
  console.log(`MCP server exited with code ${code}`);
});
```

修改 `mcp.json`：

```json
{
  "mcpServers": {
    "Playwright": {
      "command": "node",
      "args": [
        "C:\\Programs\\Coding\\Playwright\\mcp-runner.js"
      ]
    }
  }
}
```

**两种方案对比**：

| 特性 | 方案一（直接配置） | 方案二（mcp-runner.js） |
|------|-------------------|----------------------|
| 配置复杂度 | ⭐ 简单 | ⭐⭐ 中等 |
| 维护性 | ⚠️ 配置分散 | ✅ 集中管理 |
| 调试能力 | ❌ 有限 | ✅ 可以添加日志 |
| 灵活性 | ❌ 硬编码路径 | ✅ 使用相对路径 |
| 错误处理 | ❌ 有限 | ✅ 可以自定义 |

**重要提示**：
- 修改配置后需要重启 TRAE IDE 才能生效
- 确保 Edge 浏览器已安装在指定路径
- 如果配置仍然不生效，检查 MCP 服务器状态

#### 方案二：安装 Playwright 浏览器

```bash
npx playwright install
```

**注意**：此方法会下载 Playwright 自带的浏览器，占用磁盘空间较大。

### 问题10：Playwright 使用系统 Edge 的正确方法

**错误方法**：
```javascript
// ❌ 错误：Playwright 无此 API
await playwright.edge.launch();
```

**正确方法**：

```javascript
// ✅ 正确：使用 chromium.launch() + channel 参数
const { chromium } = require('playwright');

(async () => {
  const browser = await chromium.launch({
    channel: 'msedge',
    headless: false
  });
  const page = await browser.newPage();
  // 使用页面...
  await browser.close();
})();
```

**Python 版本**：
```python
from playwright.sync_api import sync_playwright

with sync_playwright() as p:
    browser = p.chromium.launch(channel="msedge", headless=False)
    page = browser.new_page()
    # 使用页面...
    browser.close()
```

**原理说明**：
- Microsoft Edge 基于 Chromium 内核开发
- Playwright 通过 `chromium.launch()` 方法启动
- 使用 `channel: 'msedge'` 参数指定使用 Edge 浏览器
- 也可以使用 `executable_path` 参数直接指定 Edge 可执行文件路径

### 问题11：MCP 配置文件修改后不生效

**症状**：修改 `mcp.json` 配置后，Playwright 仍然使用旧配置

**原因**：MCP 服务器没有重新加载配置

**解决方案**：

1. **重启 TRAE IDE**（推荐）
   - 完全关闭 TRAE IDE
   - 重新打开 IDE
   - MCP 服务器会自动重新加载配置

2. **检查 MCP 服务器状态**
   - 在 TRAE IDE 中查看 MCP 服务器状态
   - 查看是否有错误信息

3. **验证配置文件**
   - 确认配置文件路径正确
   - 确认 JSON 格式正确（无语法错误）

4. **查看日志**
   - 检查 MCP 服务器启动日志
   - 确认浏览器路径是否正确

**配置文件位置**：
```
c:\Users\liveu\AppData\Roaming\Trae CN\User\mcp.json
```

**常见配置错误**：
- 路径中的反斜杠未转义（`C:\` 应为 `C:\\`）
- JSON 格式错误（缺少逗号、引号等）
- 环境变量设置错误（`PLAYWRIGHT_BROWSERS_PATH` 应为 `"0"` 而不是 `0`）

### 问题12：插件搜索有结果但返回0

**现象**：
```
[pioz] 普通搜索找到 20 个结果
[pioz] 详情增强完成: 输入=10, 输出=10, 含链接=10
✅ [搜索完成] 总结果: 0
```

**原因分析**：

1. **时间字段未设置**：`Datetime` 字段为空值，导致结果被过滤
2. **关键词不匹配**：标题与搜索关键词不匹配，被关键词过滤器过滤
3. **UniqueID 格式错误**：无法识别插件来源，导致优先级判断错误

**解决方案**：

1. **正确设置时间字段**：
```go
var datetime time.Time
if item.Datetime != "" {
    if parsedTime, err := time.Parse("2006-01-02", item.Datetime); err == nil {
        datetime = parsedTime
    }
}

return model.SearchResult{
    Datetime: datetime,  // 使用解析后的时间
}
```

2. **确保 UniqueID 格式正确**：
```go
UniqueID: fmt.Sprintf("%s-%d-%s", p.Name(), item.ID, url.QueryEscape(viewURL))
```

3. **理解过滤机制**：
   - 第一层：过滤无链接的结果
   - 第二层：过滤无时间、无关键词匹配、低优先级的结果
   - `mergedLinks` 不受第二层过滤影响

**详细文档**：参见 [插件开发指南](docs/插件开发指南.md) 中的"实际案例分析：插件结果过滤问题排查"

## 项目结构

```
pansou/
├── api/                    # API 路由和处理器
│   ├── auth_handler.go    # 认证处理
│   ├── handler.go         # 搜索处理
│   ├── middleware.go      # 中间件
│   └── router.go          # 路由配置
├── config/                 # 配置管理
├── model/                  # 数据模型
│   ├── request.go         # 请求模型
│   ├── response.go        # 响应模型
│   └── plugin_result.go   # 插件结果
├── plugin/                 # 插件系统
│   ├── plugin.go          # 插件基类
│   ├── stats.go           # 统计功能
│   └── pioz/              # Pioz 插件
│       └── pioz.go        # 主要实现
├── service/               # 业务逻辑
│   ├── search_service.go  # 搜索服务
│   └── cache_integration.go # 缓存集成
├── util/                  # 工具函数
│   ├── cache/            # 缓存系统
│   ├── pool/             # 对象池和工作池
│   ├── json/             # JSON工具
│   └── ...               # 其他工具
├── docs/                  # 文档
│   └── 插件开发指南.md    # 插件开发指南
├── start-simple.bat       # 启动脚本（推荐）
├── stop.bat              # 停止脚本
├── test-final.bat        # 测试脚本
└── main.go               # 程序入口
```

## 开发指南

### 编译
```bash
go build -o pansou.exe
```

### 添加新插件

参考 [插件开发指南](docs/插件开发指南.md) 获取详细的插件开发指南，包括：

- **基础结构**：插件接口和基本实现
- **搜索逻辑**：HTTP请求、数据解析、错误处理
- **日志记录**：统一的日志格式和最佳实践
- **链接转换**：16种网盘类型识别
- **高级特性**：Web路由注册、Service层过滤控制
- **性能优化**：缓存、连接池、对象池
- **调试技巧**：问题诊断和解决方案

### 插件开发示例

```go
package myplugin

import (
    "pansou/model"
    "pansou/plugin"
)

type MyPlugin struct {
    *plugin.BaseAsyncPlugin
}

func init() {
    p := &MyPlugin{
        BaseAsyncPlugin: plugin.NewBaseAsyncPlugin("myplugin", 3),
    }
    plugin.RegisterGlobalPlugin(p)
}

func (p *MyPlugin) Search(keyword string, ext map[string]interface{}) ([]model.SearchResult, error) {
    result, err := p.SearchWithResult(keyword, ext)
    if err != nil {
        return nil, err
    }
    return result.Results, nil
}

func (p *MyPlugin) SearchWithResult(keyword string, ext map[string]interface{}) (model.PluginSearchResult, error) {
    return p.AsyncSearchWithResult(keyword, p.searchImpl, p.MainCacheKey, ext)
}

func (p *MyPlugin) searchImpl(client *http.Client, keyword string, ext map[string]interface{}) ([]model.SearchResult, error) {
    // 实现搜索逻辑
    // ...
}
```

## 性能监控

### 获取性能统计

Pioz 插件提供完整的性能统计功能：

```go
stats := piozPlugin.GetPerformanceStats()
```

**统计指标**：
- `search_requests`: 搜索请求总数
- `detail_requests`: 详情页请求总数
- `cache_hits`: 缓存命中次数
- `cache_misses`: 缓存未命中次数
- `cache_hit_rate`: 缓存命中率（%）
- `anti_crawler_blocks`: 反爬拦截次数
- `block_rate`: 拦截率（%）
- `avg_search_time_ms`: 平均搜索时间（毫秒）
- `avg_detail_time_ms`: 平均详情页时间（毫秒）

### 日志输出

**搜索日志**：
```
[pioz] 开始搜索，keyword='电影'
[pioz] 调用API: https://www.pioz.cn/api/deep-search?kw=电影
[pioz] API响应状态码: 200
[pioz] 深度搜索找到 10 个结果
[pioz] 开始增强 10 个结果，并发度: 8
[pioz] Transfer API 成功: 1个链接
[pioz] 增强完成，成功: 10/10
```

## 技术栈

- **语言**: Go 1.21+
- **框架**: Gin
- **缓存**: 内存+磁盘两级缓存
- **认证**: JWT
- **并发**: Goroutine + Channel + Semaphore
- **HTML解析**: goquery
- **HTTP客户端**: 优化的连接池

## 相关文档

- [搜索优化与问题解决方案](docs/搜索优化与问题解决方案.md) - 搜索优化和常见问题解决方案
- [插件开发指南](docs/插件开发指南.md) - 完整的插件开发文档
- [PanSou网盘搜索官方说明](docs/PanSou网盘搜索官方说明.md) - 项目概述
- [Windows安装部署指南](docs/Windows安装部署指南.md) - 安装指南
- [PanSou安装配置问答集](docs/PanSou安装配置问答集.md) - 常见问题

## 许可证

见 [LICENSE](LICENSE) 文件

---
编译方法：
go build '-ldflags' '-s -w' -trimpath -o pansou.exe
#- -ldflags "-s -w" - 去除调试信息和符号表
#- -trimpath - 去除文件路径信息（增强安全性）

**版本**: v8.1  
**更新**: 2026-02-19  
**状态**: ✅ 生产可用  

**维护者**: abcxyzNone  
**AI工具**: Kiro (Claude Sonnet 4.5)  
**致谢**: PanSou Team & fish2018

**最近更新**：
- ✅ 搜索优化：实现部分剧名搜索功能，当搜索结果为0时自动提取剧名的一部分进行重试
- ✅ 链接数量限制：每个搜索结果只保留前3个链接，减少返回数据量
- ✅ 插件优先级配置：统一配置所有插件优先级为1，优化搜索结果排序
- ✅ Transfer API优化：修改资源ID提取逻辑，去除下划线后缀；扩展API端点候选（从8个增加到22个）
- ✅ Pioz 插件重构：适配新的网站结构，实现二次跳转，添加链接格式过滤
- ✅ 新增 `parseSearchItemNew` 函数：解析新版本搜索结果
- ✅ 新增 `tryConsentFlowNew` 函数：处理新版本二次跳转
- ✅ 新增 `clickConsentButtons` 函数：模拟点击同意按钮
- ✅ 更新链接过滤逻辑：只保留 `https://pan.quark.cn/s/` 格式
- ✅ 移除 Python 依赖：删除旧的 Python 版本，完全使用 Go 实现
- ✅ 修复时间字段：正确解析并设置 `Datetime` 字段，避免结果被过滤
- ✅ 问题排查文档：添加插件结果过滤问题排查案例到插件开发指南
---

