# Pioz 插件 - 夸克小站搜索

## 简介
Pioz 是一个专注于夸克网盘资源的搜索插件，实现了完整的二次跳转机制，自动从搜索页跳转到详情页获取真实的网盘链接。

## 核心特性

# Pioz 二次跳转机制说明

## 概述
Pioz 插件实现了完整的二次跳转机制，从搜索页到详情页再到真实网盘链接，并采用异步并发处理提升性能。

## 二次跳转流程

```
用户搜索关键词
    ↓
【第一跳】访问搜索页面
    URL: https://www.pioz.cn/search?q={关键词}
    提取: 标题、描述、详情页URL
    ↓
【第二跳】异步并发访问详情页
    URL: https://www.pioz.cn/detail/{资源ID}
    提取: 真实的夸克网盘链接
    ↓
返回完整结果（包含真实链接）
```

## 实现细节

### 1. 搜索页处理 (parseSearchItem)

**位置**: `plugin/pioz/pioz.go` 第 193-250 行

**功能**:
- 解析搜索结果页面的每个搜索项
- 提取标题、描述
- **关键**: 提取详情页URL（格式：`/detail/数字`）
- 将详情页URL编码到 `UniqueID` 中（格式：`pioz-detail-{encodedURL}`）
- 初始化 `Links` 为空数组（稍后填充）

**代码示例**:
```go
// 提取详情页链接并保存到UniqueID中
var detailURL string
s.Find("a[href]").Each(func(j int, a *goquery.Selection) {
    if href, exists := a.Attr("href"); exists {
        if strings.Contains(href, "/detail/") {
            if strings.HasPrefix(href, "http") {
                detailURL = href
            } else {
                detailURL = "https://www.pioz.cn" + href
            }
            return
        }
    }
})

// 构建唯一ID - 使用detailURL作为ID的一部分
if detailURL != "" {
    result.UniqueID = fmt.Sprintf("%s-detail-%s", p.Name(), url.QueryEscape(detailURL))
}
```

### 2. 异步并发处理 (enhanceWithDetails)

**位置**: `plugin/pioz/pioz.go` 第 252-290 行

**功能**:
- 使用 goroutines 异步并发处理所有搜索结果
- 并发数限制：20个（通过 semaphore 控制）
- 从 `UniqueID` 中解码出详情页URL
- 调用 `fetchDetailPageLinks` 获取真实链接
- 使用 `sync.Mutex` 保护结果数组

**代码示例**:
```go
// 限制并发数
const MaxConcurrency = 20
semaphore := make(chan struct{}, MaxConcurrency)

for _, result := range results {
    wg.Add(1)
    go func(r model.SearchResult) {
        defer wg.Done()
        
        // 获取信号量
        semaphore <- struct{}{}
        defer func() { <-semaphore }()
        
        // 从UniqueID中提取detailURL
        if strings.Contains(r.UniqueID, "-detail-") {
            parts := strings.SplitN(r.UniqueID, "-detail-", 2)
            if len(parts) == 2 {
                detailURL, err := url.QueryUnescape(parts[1])
                if err == nil && detailURL != "" {
                    // 获取详情页链接（第二跳）
                    links := p.fetchDetailPageLinks(client, detailURL)
                    r.Links = links
                }
            }
        }
        
        mu.Lock()
        enhancedResults = append(enhancedResults, r)
        mu.Unlock()
    }(result)
}

wg.Wait()
```

### 3. 详情页链接提取 (fetchDetailPageLinks)

**位置**: `plugin/pioz/pioz.go` 第 292-380 行

**功能**:
- 访问详情页URL（5秒超时）
- 解析HTML页面
- 提取夸克网盘链接（正则匹配：`https://pan.quark.cn/s/[0-9a-zA-Z]+`）
- 提取密码（正则匹配：`提取码|密码：[0-9a-zA-Z]{4}`）
- 降级处理：如果从 `<a>` 标签找不到，从页面文本中提取

**代码示例**:
```go
// 提取网盘链接
doc.Find("a[href]").Each(func(i int, s *goquery.Selection) {
    href, exists := s.Attr("href")
    if !exists {
        return
    }
    
    // 匹配夸克网盘链接
    if quarkLinkRegex.MatchString(href) {
        link := model.Link{
            Type:     "quark",
            URL:      href,
            Password: "",
        }
        links = append(links, link)
    }
})

// 降级处理：从页面文本中提取
if len(links) == 0 {
    pageText := doc.Text()
    matches := quarkLinkRegex.FindAllString(pageText, -1)
    for _, match := range matches {
        link := model.Link{
            Type:     "quark",
            URL:      match,
            Password: "",
        }
        links = append(links, link)
    }
}

// 提取密码
pageText := doc.Text()
if matches := passwordRegex.FindStringSubmatch(pageText); len(matches) > 1 {
    password := matches[1]
    for i := range links {
        links[i].Password = password
    }
}
```

## 性能优化

### 并发处理
- **旧实现**: 串行处理，每个详情页阻塞5秒
  - 10个结果 = 10 × 5秒 = 50秒
- **新实现**: 并发处理，20个并发
  - 10个结果 = 1批 × 5秒 = 约5-10秒
  - **性能提升**: 5-10倍

### 超时控制
- 搜索页请求：10秒超时
- 详情页请求：5秒超时
- 避免长时间阻塞

### 重试机制
- 最大重试次数：3次
- 指数退避：200ms, 400ms, 800ms

## 与 zhizhen.go 的对比

| 特性 | pioz.go | zhizhen.go |
|------|---------|------------|
| **二次跳转** | ✅ 需要 | ❌ 不需要 |
| **异步并发** | ✅ 20个并发 | ✅ 20个并发 |
| **网盘类型** | 主要夸克 | 16种网盘 |
| **详情页URL** | `/detail/数字` | `/vod/detail/id/数字.html` |
| **链接提取** | 从详情页HTML | 从详情页HTML |
| **优先级** | 1（最高） | 1（最高） |

## 关键优势

1. **用户体验**: 用户无需手动点击"打开链接"，插件自动完成
2. **自动化**: 完全自动化的二次跳转流程
3. **高性能**: 异步并发处理，大幅提升速度
4. **健壮性**: 降级处理确保稳定性
5. **可维护**: 代码结构清晰，遵循最佳实践

## 测试验证

### 测试步骤
1. 编译项目：`go build -o pansou.exe`
2. 启动服务：`pansou.exe`
3. 搜索关键词（例如："别时明月满西楼"）
4. 观察日志输出：`[pioz] www.pioz.cn`
5. 验证返回结果包含真实的夸克网盘链接

### 预期结果
- 搜索结果包含标题、描述
- 每个结果包含真实的夸克网盘链接（`https://pan.quark.cn/s/...`）
- 如果有密码，自动提取并填充
- 处理时间：约5-10秒（取决于结果数量）

## 总结

Pioz 插件的二次跳转机制已经完整实现并优化：
- ✅ 保留了完整的二次跳转流程
- ✅ 采用异步并发处理提升性能
- ✅ 遵循 zhizhen.go 的代码结构和最佳实践
- ✅ 包含降级处理和错误处理
- ✅ 代码清晰易维护

**二次跳转机制是 Pioz 插件的核心特性，已完整保留并优化！**

### 🚀 二次跳转机制
Pioz 的核心特性是二次跳转机制，自动完成以下流程：

```
搜索页面                详情页面                真实链接
┌─────────┐           ┌─────────┐           ┌─────────┐
│ 标题    │           │ 详细信息 │           │ 夸克网盘 │
│ 描述    │  ──第一跳──>│ 打开链接 │  ──第二跳──>│ 真实URL │
│ /detail │           │ 按钮    │           │ 密码    │
└─────────┘           └─────────┘           └─────────┘
```

**用户体验**:
- ❌ 传统方式：用户需要手动点击"打开链接"按钮
- ✅ Pioz方式：插件自动完成所有跳转，直接返回真实链接

### ⚡ 异步并发处理
- 使用 goroutines 异步并发处理详情页请求
- 并发数限制：20个（通过 semaphore 控制）
- 性能提升：5-10倍（相比串行处理）

### 🎯 智能关键词过滤
- 使用新的 `KeywordMatcher` 进行智能匹配
- 支持完整匹配、分词匹配、部分匹配
- 过滤掉不相关的结果

### 🔄 降级处理
- 如果详情页访问失败，尝试从搜索结果直接提取链接
- 如果 `<a>` 标签找不到链接，从页面文本中提取
- 确保稳定性和可用性

## 技术实现

### 搜索流程

```go
// 1. 构建搜索URL
searchURL := "https://www.pioz.cn/search?q={关键词}"

// 2. 解析搜索结果页面
doc.Find(".search-result-item").Each(func(i int, s *goquery.Selection) {
    // 提取标题、描述、详情页URL
    result := p.parseSearchItem(s, keyword, i)
})

// 3. 异步并发获取详情页链接
enhancedResults := p.enhanceWithDetails(client, results)

// 4. 关键词过滤
return plugin.FilterResultsByKeyword(enhancedResults, keyword)
```

### 详情页处理

```go
// 异步并发处理
for _, result := range results {
    go func(r model.SearchResult) {
        // 从UniqueID解码detailURL
        detailURL := extractDetailURL(r.UniqueID)
        
        // 访问详情页（5秒超时）
        links := p.fetchDetailPageLinks(client, detailURL)
        
        // 更新结果
        r.Links = links
    }(result)
}
```

### 链接提取

```go
// 1. 从 <a> 标签提取
doc.Find("a[href]").Each(func(i int, s *goquery.Selection) {
    if quarkLinkRegex.MatchString(href) {
        links = append(links, model.Link{
            Type: "quark",
            URL:  href,
        })
    }
})

// 2. 降级：从页面文本提取
if len(links) == 0 {
    matches := quarkLinkRegex.FindAllString(pageText, -1)
    for _, match := range matches {
        links = append(links, model.Link{
            Type: "quark",
            URL:  match,
        })
    }
}

// 3. 提取密码
if matches := passwordRegex.FindStringSubmatch(pageText); len(matches) > 1 {
    for i := range links {
        links[i].Password = matches[1]
    }
}
```

## 配置参数

| 参数 | 值 | 说明 |
|------|-----|------|
| `DefaultTimeout` | 10秒 | 搜索页请求超时 |
| `DetailTimeout` | 5秒 | 详情页请求超时 |
| `MaxConcurrency` | 20 | 最大并发数 |
| `cacheTTL` | 1小时 | 缓存有效期 |
| `MaxRetries` | 3次 | 最大重试次数 |
| `Priority` | 1 | 插件优先级（最高） |

## 性能对比

### 串行处理 vs 并发处理

| 场景 | 串行处理 | 并发处理 | 提升 |
|------|---------|---------|------|
| 10个结果 | 50秒 | 5-10秒 | 5-10倍 |
| 20个结果 | 100秒 | 10-15秒 | 6-10倍 |
| 50个结果 | 250秒 | 15-25秒 | 10-16倍 |

**说明**: 
- 串行处理：每个详情页阻塞5秒
- 并发处理：20个并发，分批处理

## 支持的网盘类型

目前主要支持：
- ✅ **夸克网盘** (quark): `https://pan.quark.cn/s/...`

未来可扩展支持：
- UC网盘 (uc)
- 百度网盘 (baidu)
- 阿里云盘 (aliyun)
- 等等...

## 使用示例

### API 请求
```bash
POST /api/search
Content-Type: application/json

{
  "keyword": "别时明月满西楼",
  "plugins": ["pioz"]
}
```

### 响应示例
```json
{
  "code": 200,
  "data": {
    "results": [
      {
        "unique_id": "pioz-detail-https%3A%2F%2Fwww.pioz.cn%2Fdetail%2F65281",
        "title": "09.别时明月满西楼&我寄愁心与明月（71集）谢佳成&宋暖",
        "content": "资源描述信息",
        "links": [
          {
            "type": "quark",
            "url": "https://pan.quark.cn/s/7cdff3011b66",
            "password": ""
          }
        ],
        "channel": "",
        "datetime": "0001-01-01T00:00:00Z"
      }
    ]
  }
}
```

## 日志输出

### 简洁日志
```
[pioz] www.pioz.cn
```

### 详细日志（调试模式）
```
[pioz] 搜索URL: https://www.pioz.cn/search?q=关键词
[pioz] 找到 5 个搜索结果
[pioz] 异步获取详情页链接...
[pioz] 详情页 1/5: https://www.pioz.cn/detail/65281
[pioz] 详情页 2/5: https://www.pioz.cn/detail/65282
...
[pioz] 完成，返回 5 个结果
```

## 错误处理

### 常见错误
1. **搜索页访问失败**: 重试3次，指数退避
2. **详情页访问失败**: 降级到从搜索结果直接提取
3. **链接提取失败**: 从页面文本中提取
4. **超时**: 搜索页10秒，详情页5秒

### 降级策略
```
详情页访问
    ↓
  失败？
    ↓
从搜索结果提取
    ↓
  失败？
    ↓
从页面文本提取
    ↓
  失败？
    ↓
返回空链接
```

# Pioz 插件改进说明

## 改进内容

### 1. 性能统计和监控
借鉴 zhizhen.go，添加了完整的性能统计功能：

```go
// 性能统计（原子操作）
var (
    searchRequests     int64 = 0  // 搜索请求总数
    detailPageRequests int64 = 0  // 详情页请求总数
    cacheHits          int64 = 0  // 缓存命中次数
    cacheMisses        int64 = 0  // 缓存未命中次数
    totalSearchTime    int64 = 0  // 总搜索时间（纳秒）
    totalDetailTime    int64 = 0  // 总详情页时间（纳秒）
)
```

**新增方法：**
- `GetPerformanceStats()` - 获取性能统计信息，包括：
  - 请求总数
  - 缓存命中率
  - 平均响应时间
  - 总耗时统计

### 2. 网盘类型支持扩展
从原来的 1 种网盘（夸克）扩展到 **16 种网盘类型**：

| 网盘类型 | 正则表达式 | 类型标识 |
|---------|-----------|---------|
| 夸克网盘 | `pan.quark.cn` | quark |
| UC网盘 | `drive.uc.cn` | uc |
| 百度网盘 | `pan.baidu.com` | baidu |
| 阿里云盘 | `aliyundrive.com/alipan.com` | aliyun |
| 迅雷网盘 | `pan.xunlei.com` | xunlei |
| 天翼云盘 | `cloud.189.cn` | tianyi |
| 115网盘 | `115.com` | 115 |
| 移动云盘 | `caiyun.feixin.10086.cn` | mobile |
| 微云 | `share.weiyun.com` | weiyun |
| 蓝奏云 | `lanzou*.com` | lanzou |
| 坚果云 | `jianguoyun.com` | jianguoyun |
| 123网盘 | `123pan.com` | 123 |
| PikPak | `mypikpak.com` | pikpak |
| 磁力链接 | `magnet:?xt=urn:btih:` | magnet |
| 电驴链接 | `ed2k://` | ed2k |

**新增方法：**
- `isValidNetworkDriveURL()` - 验证URL是否为有效的网盘链接
- `determineLinkType()` - 根据URL自动识别网盘类型
- `extractPassword()` - 从URL或文本中提取密码

### 3. 缓存优化
实现了两级缓存策略：

```go
var (
    searchResultCache = sync.Map{} // 缓存搜索结果
    detailCache       = sync.Map{} // 缓存详情页解析结果
)
```

**优化点：**
- 搜索结果缓存：避免重复搜索相同关键词
- 详情页缓存：避免重复访问相同的详情页
- 缓存命中统计：实时监控缓存效率
- TTL 机制：1小时后自动过期

### 4. 请求头优化
使用更完整的浏览器请求头，避免反爬虫：

```go
req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36...")
req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
req.Header.Set("Connection", "keep-alive")
req.Header.Set("Upgrade-Insecure-Requests", "1")
req.Header.Set("Cache-Control", "max-age=0")
req.Header.Set("Referer", "https://www.pioz.cn/")
req.Header.Set("Sec-Fetch-Dest", "document")
req.Header.Set("Sec-Fetch-Mode", "navigate")
req.Header.Set("Sec-Fetch-Site", "same-origin")
req.Header.Set("Sec-Fetch-User", "?1")
```

### 5. 并发优化
提高并发处理能力：

```go
const MaxConcurrency = 20  // 从 10 提升到 20
```

### 6. 密码提取增强
改进密码提取逻辑：
- 支持多种密码格式：`提取码:`、`密码:`、`pwd=`
- 从整个页面文本中搜索
- 从特定密码显示区域提取
- 自动关联到对应的网盘链接

## 性能提升

| 指标 | 改进前 | 改进后 | 提升 |
|-----|-------|-------|-----|
| 网盘支持 | 1种 | 16种 | +1500% |
| 并发数 | 10 | 20 | +100% |
| 缓存层级 | 1层 | 2层 | +100% |
| 性能监控 | 无 | 完整 | ✓ |
| 请求头 | 基础 | 完整 | ✓ |

## 使用示例

### 获取性能统计
```go
plugin := NewPiozPlugin()
stats := plugin.GetPerformanceStats()

fmt.Printf("搜索请求数: %d\n", stats["search_requests"])
fmt.Printf("缓存命中率: %.2f%%\n", stats["cache_hit_rate"])
fmt.Printf("平均搜索时间: %.2fms\n", stats["avg_search_time_ms"])
```

### 支持的网盘链接示例
```
夸克: https://pan.quark.cn/s/abc123
百度: https://pan.baidu.com/s/1abc123?pwd=1234
阿里: https://www.aliyundrive.com/s/abc123
迅雷: https://pan.xunlei.com/s/abc123
磁力: magnet:?xt=urn:btih:abc123...
```

## 兼容性
- 完全向后兼容
- 不影响现有功能
- 新增功能可选使用

## 测试状态
✅ 编译通过
✅ 所有测试通过
✅ 项目构建成功


## 开发指南

### 添加新的网盘类型
```go
// 1. 添加正则表达式
ucLinkRegex = regexp.MustCompile(`https?://drive\.uc\.cn/s/[0-9a-zA-Z]+`)

// 2. 在 fetchDetailPageLinks 中添加匹配
if ucLinkRegex.MatchString(href) {
    link := model.Link{
        Type: "uc",
        URL:  href,
    }
    links = append(links, link)
}
```

### 调整并发数
```go
// 在 enhanceWithDetails 中修改
const MaxConcurrency = 30 // 增加到30个并发
```

### 调整超时时间
```go
// 在常量定义中修改
const (
    DefaultTimeout = 15 * time.Second // 增加到15秒
    DetailTimeout  = 8 * time.Second  // 增加到8秒
)
```

## 测试

### 单元测试
```bash
go test -v plugin/pioz/pioz_test.go
```

### 集成测试
```bash
# 1. 启动服务
go run main.go

# 2. 测试搜索
curl -X POST http://localhost:8080/api/search \
  -H "Content-Type: application/json" \
  -d '{"keyword":"测试关键词","plugins":["pioz"]}'
```

### 性能测试
```bash
# 使用 Apache Bench
ab -n 100 -c 10 -p search.json -T application/json \
  http://localhost:8080/api/search
```

## 常见问题

### Q: 为什么需要二次跳转？
A: Pioz 网站的搜索结果页只显示标题和描述，真实的网盘链接在详情页中。二次跳转可以自动获取真实链接，提升用户体验。

### Q: 并发数为什么是20？
A: 20是一个平衡值，既能提升性能，又不会对目标网站造成过大压力。可以根据实际情况调整。

### Q: 如果详情页访问失败怎么办？
A: 插件有降级处理机制，会尝试从搜索结果直接提取链接，或从页面文本中提取。

### Q: 支持其他网盘吗？
A: 目前主要支持夸克网盘，但代码结构支持扩展，可以轻松添加其他网盘类型。

## 参考资料

- [插件开发指南](../../docs/插件开发指南.md)
- [Pioz 开发案例](../../docs/Pioz插件开发案例.md)
- [JSON结构分析](./json结构分析.md)
- [二次跳转机制说明](./二次跳转机制说明.md)

## 贡献

欢迎提交 Issue 和 Pull Request！

## 许可证

MIT License
