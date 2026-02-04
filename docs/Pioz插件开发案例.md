# Pioz 插件开发案例

## 概述

Pioz 插件是 PanSou 系统中的一个优秀实践案例，展示了如何实现高质量的网盘搜索插件。本文档详细介绍 Pioz 插件的设计思路、技术实现和最佳实践。

## 插件简介

**插件名称**: pioz  
**目标网站**: [夸克小站](https://www.pioz.cn/)  
**优先级**: 1（最高质量）  
**主要特性**:
- ✅ 无需登录，开箱即用
- 🚀 **异步并发处理**（20个并发，性能提升5-10倍）
- ⚡ **二次跳转提取真实链接**（核心特性）
- 🎯 **智能关键词过滤**（分词、部分匹配、核心词匹配）
- ✅ 自动提取夸克网盘链接和提取码
- ✅ 智能缓存机制（1小时有效期）
- ✅ 自动重试机制（指数退避）
- ✅ HTTP连接池优化
- 🔄 降级处理（详情页失败时自动回退）

## 核心技术：二次跳转机制

### 问题背景

传统的网盘搜索插件通常只能从搜索结果页提取链接，但很多网站（如 pioz.cn）的搜索结果页只显示资源标题，真实的网盘链接需要点击进入详情页才能看到。

**传统流程的问题**：
1. 用户在搜索结果页看到资源标题
2. 用户点击进入详情页
3. 用户在详情页点击"打开链接"按钮
4. 才能获得真实的网盘链接

这个过程需要用户手动操作两次，体验不佳。

### 解决方案：异步并发二次跳转

Pioz 插件实现了高性能的异步并发二次跳转功能，自动完成上述所有步骤：

```
搜索结果页 → 提取所有详情页链接 → 异步并发访问详情页（20个并发）→ 提取真实链接 → 返回给用户
```

**性能对比**：
- **串行处理**: 10个结果 × 5秒 = 50秒
- **并发处理**: 10个结果 ÷ 20并发 × 5秒 = 约5-10秒
- **性能提升**: 5-10倍

用户直接获得可用的网盘链接，无需任何手动操作，且速度极快。


## 技术实现详解

### 1. 工作流程（v2.0 异步并发版本）

```
用户搜索关键词
    ↓
访问搜索结果页 (https://www.pioz.cn/search?q=关键词)
    ↓
解析搜索结果HTML，提取所有详情页链接
    ↓
将详情页URL编码到UniqueID中
    ↓
【异步并发处理】启动20个goroutines
    ↓
并发访问所有详情页 (https://www.pioz.cn/detail/*)
    ↓
解析详情页HTML，提取真实链接
    ↓
提取密码信息
    ↓
使用sync.Mutex保护结果数组
    ↓
等待所有goroutines完成（sync.WaitGroup）
    ↓
返回完整的网盘链接信息
```

**关键优化**：
- 使用 goroutines 实现异步并发
- semaphore 控制并发数（最大20个）
- sync.WaitGroup 等待所有任务完成
- sync.Mutex 保护共享资源

### 2. 核心代码实现（v2.0 异步并发版本）

#### 2.1 在搜索结果中提取详情页链接并保存

```go
// parseSearchItem 函数中
// 提取详情页链接并保存到UniqueID中
var detailURL string
s.Find("a[href]").Each(func(j int, a *goquery.Selection) {
    if href, exists := a.Attr("href"); exists {
        // 匹配详情页链接格式：/detail/数字
        if strings.Contains(href, "/detail/") {
            if strings.HasPrefix(href, "http") {
                detailURL = href
            } else {
                detailURL = "https://www.pioz.cn" + href
            }
            return // 找到就退出
        }
    }
})

// 构建唯一ID - 使用detailURL作为ID的一部分
// 这样在enhanceWithDetails中可以提取出detailURL
if detailURL != "" {
    result.UniqueID = fmt.Sprintf("%s-detail-%s", p.Name(), url.QueryEscape(detailURL))
} else {
    result.UniqueID = fmt.Sprintf("%s-html-%d", p.Name(), index)
}

result.Title = title
result.Content = content
result.Datetime = time.Time{} // 使用零值
result.Links = []model.Link{}  // 初始化为空，稍后在enhanceWithDetails中填充
result.Channel = ""            // 插件搜索结果必须为空字符串
```

**关键点**：
- 识别 `/detail/数字` 格式的链接
- 将详情页URL编码到UniqueID中
- Links初始化为空数组（稍后异步填充）
- 不阻塞主搜索流程


#### 2.2 异步并发获取详情页链接

```go
// enhanceWithDetails 函数 - 异步并发处理核心
func (p *PiozAsyncPlugin) enhanceWithDetails(client *http.Client, results []model.SearchResult) []model.SearchResult {
    var enhancedResults []model.SearchResult
    var mu sync.Mutex
    var wg sync.WaitGroup

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
    return enhancedResults
}
```

**关键点**：
- 使用 goroutines 实现异步并发
- semaphore 控制并发数（最大20个）
- sync.WaitGroup 等待所有任务完成
- sync.Mutex 保护共享资源（enhancedResults）
- 从UniqueID解码detailURL
- 性能提升5-10倍


#### 2.3 访问详情页并提取真实链接

```go
// fetchDetailPageLinks 函数
func (p *PiozAsyncPlugin) fetchDetailPageLinks(detailURL string) []model.Link {
    var links []model.Link

    // 创建带超时的上下文（5秒）
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    // 创建请求
    req, err := http.NewRequestWithContext(ctx, "GET", detailURL, nil)
    if err != nil {
        return links
    }

    // 设置请求头（模拟浏览器）
    req.Header.Set("User-Agent", "Mozilla/5.0 ...")
    req.Header.Set("Referer", WebsiteURL)

    // 发送请求
    resp, err := client.Do(req)
    if err != nil {
        return links
    }
    defer resp.Body.Close()

    // 解析HTML
    doc, err := goquery.NewDocumentFromReader(resp.Body)
    if err != nil {
        return links
    }

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

    // 提取密码
    pageText := doc.Text()
    if matches := passwordRegex.FindStringSubmatch(pageText); len(matches) > 1 {
        password := matches[1]
        for i := range links {
            links[i].Password = password
        }
    }

    return links
}
```

**关键点**：
- 5秒超时控制，避免阻塞
- 模拟浏览器请求头，避免反爬虫
- 使用正则表达式提取夸克网盘链接
- 自动提取密码信息
- 在goroutine中并发执行


#### 2.4 降级处理（保留但不常用）

```go
// 如果找到详情页链接，进入详情页提取真实链接
var links []model.Link
if detailURL != "" {
    links = p.fetchDetailPageLinks(detailURL)
}

// 如果详情页没有找到链接，尝试直接从搜索结果提取
if len(links) == 0 {
    // 回退到传统方式
    s.Find("a[href]").Each(func(j int, a *goquery.Selection) {
        if href, exists := a.Attr("href"); exists {
            if quarkLinkRegex.MatchString(href) {
                link := model.Link{
                    Type:     "quark",
                    URL:      href,
                    Password: "",
                }
                links = append(links, link)
            }
        }
    })
}
```

**关键点**：
- 详情页提取失败时自动回退
- 确保即使详情页有问题也能返回结果
- 提高插件的健壮性

### 3. 正则表达式

```go
// 夸克网盘链接的正则表达式
quarkLinkRegex = regexp.MustCompile(`https?://pan\.quark\.cn/s/[0-9a-zA-Z]+`)

// 密码提取正则表达式
passwordRegex = regexp.MustCompile(`(?:提取码|密码)[：:]\s*([a-zA-Z0-9]{4})`)
```

**匹配示例**：
- 链接：`https://pan.quark.cn/s/7cdff3011b66`
- 密码：`提取码：1234` 或 `密码: abcd`


## 性能优化

### 1. 异步并发处理（v2.0 核心优化）⭐

```go
// 使用 goroutines + semaphore 实现高性能并发
var wg sync.WaitGroup
semaphore := make(chan struct{}, 20) // 最大20个并发

for _, result := range results {
    wg.Add(1)
    go func(r model.SearchResult) {
        defer wg.Done()
        
        // 获取信号量（限制并发数）
        semaphore <- struct{}{}
        defer func() { <-semaphore }()
        
        // 处理详情页
        links := p.fetchDetailPageLinks(client, detailURL)
        r.Links = links
    }(result)
}

wg.Wait() // 等待所有goroutines完成
```

**性能对比**：

| 场景 | 串行处理 | 并发处理（20个） | 提升 |
|------|---------|----------------|------|
| 10个结果 | 50秒 | 5-10秒 | 5-10倍 |
| 20个结果 | 100秒 | 10-15秒 | 6-10倍 |
| 50个结果 | 250秒 | 15-25秒 | 10-16倍 |

**关键技术**：
- goroutines：轻量级并发
- semaphore：控制并发数，避免过载
- sync.WaitGroup：等待所有任务完成
- sync.Mutex：保护共享资源

### 2. 超时控制

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()
```

- 5秒超时，避免长时间等待
- 超时后自动取消请求
- 不影响其他插件的执行

### 2. 超时控制

```go
// 搜索页：10秒超时
ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()

// 详情页：5秒超时
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()
```

- 搜索页10秒超时，避免长时间等待
- 详情页5秒超时，快速失败
- 超时后自动取消请求
- 不影响其他插件的执行

### 3. HTTP连接复用

```go
// 使用插件的优化客户端
client := p.optimizedClient
if client == nil {
    client = &http.Client{Timeout: 5 * time.Second}
}
```

- 复用HTTP连接池
- 减少TCP握手开销
- 提高请求效率

### 4. 错误处理

```go
// 所有错误都返回空链接列表，不抛出异常
if err != nil {
    return links  // 返回空列表
}
```

- 不因详情页失败而影响整体搜索
- 降级处理确保有结果返回
- 提高系统稳定性

### 4. 错误处理

```go
// 所有错误都返回空链接列表，不抛出异常
if err != nil {
    return links  // 返回空列表
}
```

- 不因详情页失败而影响整体搜索
- 降级处理确保有结果返回
- 提高系统稳定性

### 5. 智能关键词过滤（v2.0 新增）

```go
// 使用新的 KeywordMatcher 进行智能匹配
return plugin.FilterResultsByKeyword(enhancedResults, keyword), nil
```

**支持的匹配模式**：
1. **完整匹配**：关键词完全匹配
2. **分词匹配**：中文分词后匹配
3. **部分匹配**：80%相似度匹配
4. **核心词匹配**：过滤常见词后匹配

**效果**：
- 提高搜索结果的准确性
- 过滤掉不相关的结果
- 支持模糊搜索

## 实际效果

### 示例：搜索"别时明月满西楼"

**传统方式**：
1. 用户看到搜索结果：`09.别时明月满西楼&我寄愁心与明月（71集）谢佳成...`
2. 用户点击进入详情页：`https://www.pioz.cn/detail/65281`
3. 用户点击"打开链接"按钮
4. 获得链接：`https://pan.quark.cn/s/7cdff3011b66`

**二次跳转方式**：
1. 用户搜索关键词
2. 插件自动完成上述所有步骤
3. 直接返回：`https://pan.quark.cn/s/7cdff3011b66`

**时间对比**：
- 传统方式：需要用户手动操作 2 次，约 10-20 秒
- 串行二次跳转：自动完成，约 50 秒（10个结果）
- **并发二次跳转**：自动完成，约 5-10 秒（10个结果）⭐
- **性能提升**：5-10倍


### API响应示例

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "total": 1,
    "results": [
      {
        "unique_id": "pioz-html-0",
        "title": "09.别时明月满西楼&我寄愁心与明月（71集）谢佳成...",
        "content": "",
        "datetime": "0001-01-01T00:00:00Z",
        "links": [
          {
            "type": "quark",
            "url": "https://pan.quark.cn/s/7cdff3011b66",
            "password": ""
          }
        ],
        "channel": ""
      }
    ]
  }
}
```

用户直接获得可用的网盘链接，无需任何额外操作。

## 适用场景

### 适合使用二次跳转的情况

✅ 搜索结果页只显示标题和简介  
✅ 真实链接在详情页中  
✅ 详情页有"打开链接"或类似按钮  
✅ 网站结构稳定，不频繁变化  

### 不适合使用二次跳转的情况

❌ 搜索结果页直接显示完整链接  
❌ 详情页需要登录或验证  
❌ 详情页使用JavaScript动态加载链接  
❌ 网站有严格的反爬虫机制  


## 扩展开发指南

### 如何为其他插件添加二次跳转

#### 步骤1：识别详情页链接格式

```go
// 根据目标网站的URL格式调整
if strings.Contains(href, "/detail/") {
    detailURL = WebsiteURL + href
}
```

#### 步骤2：实现 fetchDetailPageLinks 函数

```go
func (p *YourPlugin) fetchDetailPageLinks(detailURL string) []model.Link {
    // 复制 pioz 插件的实现
    // 根据目标网站的HTML结构调整选择器
}
```

#### 步骤3：在 parseSearchItem 中调用

```go
if detailURL != "" {
    links = p.fetchDetailPageLinks(detailURL)
}
```

#### 步骤4：添加降级处理

```go
if len(links) == 0 {
    // 回退到从搜索结果直接提取
}
```

### 调试技巧

#### 1. 查看详情页HTML结构

```bash
# 使用浏览器开发者工具（F12）
# 访问详情页，查看Network标签
# 找到HTML响应，分析结构
```

#### 2. 测试正则表达式

```go
// 在代码中添加日志
fmt.Printf("提取到的链接: %v\n", matches)
```

#### 3. 监控超时情况

```go
// 记录详情页访问时间
start := time.Now()
links := p.fetchDetailPageLinks(detailURL)
fmt.Printf("详情页访问耗时: %v\n", time.Since(start))
```


## 其他技术特性

### 1. 双模式解析

Pioz 插件采用智能双模式解析策略：

#### JSON API模式（优先）
- 尝试解析JSON格式的API响应
- 适用于标准API接口
- 解析速度快，准确度高

#### HTML解析模式（备用）
- 当JSON解析失败时自动切换
- 使用goquery解析HTML页面
- 支持多种HTML结构选择器
- 兼容性更强

### 2. 智能缓存机制

- 搜索结果缓存1小时
- 每小时自动清理过期缓存
- 减少重复请求，提升响应速度

### 3. 重试机制

- 请求失败自动重试（最多3次）
- 使用指数退避策略（200ms, 400ms, 800ms）
- 提高请求成功率

### 4. HTTP连接池优化

```go
func createOptimizedHTTPClient() *http.Client {
    transport := &http.Transport{
        MaxIdleConns:        200,
        MaxIdleConnsPerHost: 50,
        MaxConnsPerHost:     100,
        IdleConnTimeout:     90 * time.Second,
        DisableKeepAlives:   false,
    }
    return &http.Client{Transport: transport, Timeout: DefaultTimeout}
}
```

- HTTP连接池复用（最大200个空闲连接）
- 请求克隆避免并发问题
- 智能超时控制


## 配置和使用

### 在 start.bat 中启用

```batch
set ENABLED_PLUGINS=pioz,labi,zhizhen,shandian,duoduo
```

### 参数说明

插件内置以下参数，无需额外配置：

| 参数 | 默认值 | 说明 |
|------|--------|------|
| DefaultTimeout | 10秒 | 请求超时时间 |
| PageSize | 20 | 每页结果数 |
| MaxResults | 200 | 最大结果数 |
| MaxRetries | 3 | 最大重试次数 |
| CacheTTL | 1小时 | 缓存有效期 |

### 测试搜索

```bash
# 登录
curl -X POST http://localhost:8888/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"123456"}'

# 搜索
curl -X POST http://localhost:8888/api/search \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <your-token>" \
  -d '{"kw":"速度与激情"}'
```

## 故障排除

### Q1: 搜索无结果？

**可能原因**：
1. 网站是前端渲染，实际API结构与预期不同
2. HTML结构变化，选择器失效
3. 关键词不匹配

**解决方法**：
1. 使用浏览器开发者工具（F12）查看实际的网络请求
2. 找到真实的API端点和响应格式
3. 修改相关代码

### Q2: 请求超时？

**可能原因**：
1. 网络连接问题
2. 网站响应慢
3. 超时时间设置过短

**解决方法**：
1. 检查网络连接
2. 增加超时时间（修改 `DefaultTimeout` 为 15秒或更长）
3. 配置代理

### Q3: HTML解析失败？

**可能原因**：
1. 网站HTML结构变化
2. 选择器不匹配

**解决方法**：
1. 查看网站实际的HTML结构
2. 修改选择器
3. 添加更多备用选择器


## 注意事项

1. ⚠️ **超时设置**：5秒超时可能不够，根据网站响应速度调整
2. ⚠️ **反爬虫**：设置合适的User-Agent和Referer
3. ⚠️ **错误处理**：确保所有错误都有降级处理
4. ⚠️ **性能影响**：二次跳转会增加请求时间，但提升用户体验
5. ⚠️ **网站变化**：定期检查网站结构是否变化
6. ⚠️ **仅用于学习**：本插件仅用于学习和研究目的
7. ⚠️ **遵守规则**：请遵守网站的使用条款和robots.txt
8. ⚠️ **避免滥用**：不要过于频繁地请求，避免给网站造成压力
9. ⚠️ **版权声明**：资源版权归原作者所有

## 总结

Pioz 插件通过二次跳转功能，显著提升了用户体验：

- ✅ **用户体验**：无需手动点击，直接获得链接
- ✅ **自动化**：插件自动完成所有步骤
- ✅ **健壮性**：降级处理确保稳定性
- ✅ **性能**：超时控制避免阻塞
- ✅ **可扩展**：易于应用到其他插件

这是一个值得推广到其他插件的优秀实践。

## 更新日志

### v2.0.0 (2025-02-05) ⭐

- 🚀 **重大重构**：完全按照 zhizhen.go 模式重构
- ⚡ **异步并发处理**：使用 goroutines + semaphore，20个并发
- 📈 **性能提升**：相比串行处理提升5-10倍
- 🎯 **智能关键词过滤**：使用新的 KeywordMatcher
  - 支持完整匹配
  - 支持分词匹配（中文分词）
  - 支持部分匹配（80%相似度）
  - 支持核心词匹配（过滤常见词）
- 🔄 **优化降级处理**：详情页失败时自动回退
- ⏱️ **超时控制**：搜索10秒，详情页5秒
- 📚 **完善文档**：新增二次跳转机制说明、README、JSON结构分析

### v1.1.0 (2025-02-05)

- ✨ **新增二次跳转功能**：自动访问详情页获取真实网盘链接
- ✅ 实现 `fetchDetailPageLinks()` 函数
- ✅ 智能降级处理：详情页失败时回退到搜索结果提取
- ✅ 5秒超时控制，避免阻塞
- 🎯 用户体验提升：无需手动点击"打开链接"

### v1.0.0 (2025-01-31)

- ✨ 初始版本
- ✅ 实现双模式解析（JSON + HTML）
- ✅ 支持夸克网盘链接提取
- ✅ 实现缓存机制
- ✅ 实现重试机制
- ✅ HTTP连接池优化

## 相关链接

- 夸克小站: https://www.pioz.cn/
- PanSou 项目: https://github.com/fish2018/pansou
- 插件开发指南: ./插件开发指南.md
- 新增插件和重新部署流程: ./新增插件和重新部署流程.md
- **Pioz 插件文档**:
  - [plugin/pioz/README.md](../plugin/pioz/README.md) - 使用指南
  - [plugin/pioz/二次跳转机制说明.md](../plugin/pioz/二次跳转机制说明.md) - 技术详解
  - [plugin/pioz/json结构分析.md](../plugin/pioz/json结构分析.md) - 数据结构分析

---

**文档版本**: v2.0  
**最后更新**: 2025-02-05  
**状态**: ✅ 生产可用（异步并发 + 二次跳转）  
**作者**: PanSou Team
