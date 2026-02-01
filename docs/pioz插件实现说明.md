# Pioz 插件实现说明

## 概述

本文档说明了 Pioz 插件（夸克小站搜索插件）的实现细节和使用方法。

## 实现背景

用户要求新增 https://www.pioz.cn/ （夸克小站）作为搜索源。该网站是一个前端渲染的网站，初始HTML内容很少，实际数据通过JavaScript API加载。

## 实现策略

由于无法直接访问网站的实际API结构（需要浏览器环境），插件采用了**双模式解析**策略：

### 1. JSON API模式（优先）

- 尝试将响应解析为JSON格式
- 适用于标准的RESTful API
- 解析速度快，准确度高

### 2. HTML解析模式（备用）

- 当JSON解析失败时自动切换
- 使用goquery库解析HTML
- 支持多种HTML结构选择器
- 兼容性更强

## 核心特性

### 1. 智能链接提取

```go
// 使用正则表达式提取夸克网盘链接
quarkLinkRegex = regexp.MustCompile(`https?://pan\.quark\.cn/s/[0-9a-zA-Z]+`)

// 从文本中提取密码
passwordRegex = regexp.MustCompile(`(?:提取码|密码)[：:]\s*([a-zA-Z0-9]{4})`)
```

### 2. 多选择器HTML解析

```go
// 搜索结果项选择器
".search-result-item", ".result-item", ".item", ".list-item"

// 标题选择器
".title", "h3", "h4", ".name", "[class*='title']"

// 描述选择器
".description", ".desc", ".content", "p", "[class*='desc']"
```

### 3. 性能优化

- **HTTP连接池**：复用连接，减少握手开销
- **智能缓存**：1小时TTL，减少重复请求
- **指数退避重试**：200ms → 400ms → 800ms
- **并发控制**：避免过多并发请求

### 4. 错误处理

- 请求失败自动重试（最多3次）
- 超时控制（10秒）
- 优雅降级（JSON失败→HTML解析）

## 文件结构

```
plugin/pioz/
├── pioz.go          # 插件主文件（约400行）
└── README.md        # 使用说明文档
```

## 关键代码段

### 双模式解析逻辑

```go
// 先尝试JSON解析
var apiResp PiozAPIResponse
if err := json.Unmarshal(respBody, &apiResp); err == nil && apiResp.Code == 200 {
    // JSON API响应
    results = p.convertAPIResults(apiResp.Data, keyword)
} else {
    // HTML响应，使用goquery解析
    results, err = p.parseHTMLResults(respBody, keyword)
    if err != nil {
        return nil, fmt.Errorf("[%s] 解析HTML失败: %w", p.Name(), err)
    }
}
```

### HTML解析示例

```go
func (p *PiozAsyncPlugin) parseHTMLResults(htmlBody []byte, keyword string) ([]model.SearchResult, error) {
    doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(htmlBody)))
    if err != nil {
        return nil, err
    }

    var results []model.SearchResult
    
    // 尝试多种可能的选择器
    doc.Find(".search-result-item, .result-item, .item, .list-item").Each(func(i int, s *goquery.Selection) {
        result := p.parseSearchItem(s, keyword, i)
        if result.UniqueID != "" && len(result.Links) > 0 {
            results = append(results, result)
        }
    })

    return results, nil
}
```

## 配置说明

### 插件参数

| 参数 | 值 | 说明 |
|------|-----|------|
| 名称 | pioz | 插件标识符 |
| 优先级 | 3 | 标准质量数据源 |
| 超时时间 | 10秒 | 请求超时 |
| 最大重试 | 3次 | 失败重试次数 |
| 缓存时间 | 1小时 | 结果缓存TTL |

### 启用方法

在 `start.bat` 中添加：

```batch
set ENABLED_PLUGINS=labi,zhizhen,shandian,pioz,pansearch,hunhepan
```

## 使用示例

### API调用

```bash
# 搜索关键词
curl "http://localhost:8888/api/search?kw=速度与激情&plugins=pioz"
```

### 响应格式

```json
{
  "results": [
    {
      "unique_id": "pioz-html-0",
      "title": "速度与激情全集",
      "content": "速度与激情系列电影1-10部",
      "links": [
        {
          "type": "quark",
          "url": "https://pan.quark.cn/s/xxxxx",
          "password": "1234"
        }
      ],
      "datetime": "2025-01-31T00:00:00Z",
      "channel": "",
      "tags": [],
      "images": []
    }
  ],
  "is_final": true
}
```

## 调试指南

### 1. 查看实际API结构

```bash
# 1. 打开浏览器开发者工具（F12）
# 2. 访问 https://www.pioz.cn/search?q=测试
# 3. 查看 Network 标签中的 XHR/Fetch 请求
# 4. 找到实际的API端点和响应格式
```

### 2. 修改插件代码

根据实际API结构，可能需要修改：

```go
// 1. 修改搜索URL
const SearchURL = "https://www.pioz.cn/api/v2/search" // 实际API端点

// 2. 修改响应结构
type PiozAPIResponse struct {
    Code    int        `json:"code"`
    Message string     `json:"msg"`      // 可能是 msg 而不是 message
    Data    []PiozItem `json:"results"`  // 可能是 results 而不是 data
}

// 3. 修改HTML选择器
doc.Find(".actual-selector").Each(...)
```

### 3. 测试插件

```bash
# 重启服务
start.bat

# 测试搜索
curl "http://localhost:8888/api/search?kw=测试&plugins=pioz"

# 查看日志
# 搜索 [pioz] 相关的输出
```

## 已知限制

1. **API结构未确认**：由于网站是前端渲染，实际API结构可能与代码中的假设不同
2. **需要实际测试**：建议在实际环境中测试并根据真实API调整代码
3. **仅支持夸克网盘**：当前版本主要针对夸克网盘链接
4. **HTML选择器可能失效**：如果网站更新HTML结构，选择器需要相应调整

## 后续优化建议

### 1. 确认实际API

使用浏览器开发者工具查看实际的API调用，然后更新：
- API端点URL
- 请求参数格式
- 响应数据结构

### 2. 增强HTML解析

如果网站主要使用HTML渲染：
- 添加更多备用选择器
- 实现更智能的内容提取
- 支持分页加载

### 3. 支持更多网盘

扩展 `detectLinkType` 函数，支持：
- UC网盘
- 百度网盘
- 阿里云盘
- 其他主流网盘

### 4. 性能优化

- 实现并发分页请求
- 优化缓存策略
- 减少不必要的HTTP请求

## 参考资料

- [插件开发指南](./插件开发指南.md)
- [PanSou 项目文档](../README.md)
- [goquery 文档](https://github.com/PuerkitoBio/goquery)

## 更新记录

| 日期 | 版本 | 说明 |
|------|------|------|
| 2025-01-31 | v1.0.0 | 初始实现，支持双模式解析 |

---

**作者**: Kiro AI Assistant  
**最后更新**: 2025-01-31  
**状态**: ⚠️ 需要根据实际网站API调整
