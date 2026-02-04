# Pioz 插件 - 夸克小站搜索

## 简介
Pioz 是一个专注于夸克网盘资源的搜索插件，实现了完整的二次跳转机制，自动从搜索页跳转到详情页获取真实的网盘链接。

## 核心特性

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
