# PanSou 插件修复指南

## 问题诊断与解决经验

### 问题1: 插件返回0结果

**症状**: 插件API测试正常返回数据，但通过服务搜索返回0结果

**诊断步骤**:
1. 检查API直接调用: `curl "https://www.pioz.cn/api/deep-search?kw=太奶奶"` ✅ 返回44个结果
2. 检查服务调用: `curl "http://localhost:8889/api/search?keyword=太奶奶"` ❌ 返回0个结果
3. 添加调试日志追踪关键词传递链路

**根本原因**:
1. **API参数名不匹配**: `api/handler.go` 中使用 `c.Query("kw")` 读取参数，但URL使用 `keyword` 参数
2. **空链接过滤**: `service/search_service.go` 中过滤掉所有没有链接的结果

**解决方案**:

#### 修复1: API参数兼容性 (api/handler.go)
```go
// 修改前
keyword := c.Query("kw")

// 修改后 - 同时支持 keyword 和 kw 参数
keyword := c.Query("keyword")
if keyword == "" {
    keyword = c.Query("kw")
}
```

#### 修复2: 添加详情页链接 (plugin/pioz2/pioz2.go)
```go
// 修改前
Links: []model.Link{}, // 空链接导致结果被过滤

// 修改后 - 添加详情页链接
detailLink := model.Link{
    URL:      viewURL,
    Type:     "detail",
    Password: "",
}
Links: []model.Link{detailLink},
```

---

## 创建新插件的最佳实践

### 参考实现模式

基于 **xdpan** 和 **pansearch** 插件的成功模式:

#### 1. 基本结构
```go
package pioz2

import (
    "pansou/model"
    "pansou/plugin"
)

type Pioz2Plugin struct {
    *plugin.BaseAsyncPlugin
    client *http.Client
}

func init() {
    plugin.RegisterGlobalPlugin(NewPioz2Plugin())
}
```

#### 2. 必需的方法实现
```go
// Search - 同步搜索接口
func (p *Pioz2Plugin) Search(keyword string, ext map[string]interface{}) ([]model.SearchResult, error) {
    result, err := p.SearchWithResult(keyword, ext)
    if err != nil {
        return nil, err
    }
    return result.Results, nil
}

// SearchWithResult - 带结果统计的搜索接口
func (p *Pioz2Plugin) SearchWithResult(keyword string, ext map[string]interface{}) (model.PluginSearchResult, error) {
    return p.AsyncSearchWithResult(keyword, p.searchImpl, p.MainCacheKey, ext)
}

// searchImpl - 实际搜索逻辑
func (p *Pioz2Plugin) searchImpl(client *http.Client, keyword string, ext map[string]interface{}) ([]model.SearchResult, error) {
    // 实现具体的API调用和数据解析
}
```

#### 3. 关键配置项
```go
const (
    SiteBaseURL = "https://www.example.com"
    APIBaseURL  = "https://www.example.com/api"
    DefaultTimeout = 15 * time.Second
    CacheTTL = 30 * time.Minute
)

// 创建插件时设置优先级
plugin.NewBaseAsyncPlugin("pluginname", 1) // 1=高质量, 2=中高质量, 3=普通质量
```

#### 4. 结果转换规范
```go
return model.SearchResult{
    MessageID: uniqueID,
    UniqueID:  uniqueID,
    Title:     item.Title,
    Content:   strings.Join(contentParts, " | "),
    Datetime:  parsedTime,
    Links:     []model.Link{detailLink}, // ⚠️ 必须至少有一个链接
    Channel:   "",                       // ⚠️ 插件结果Channel必须为空
}
```

#### 5. 时间解析
```go
func (p *Plugin) parseTime(timeStr string) time.Time {
    if timeStr == "" {
        return time.Now()
    }
    
    formats := []string{
        time.RFC3339,
        "2006-01-02T15:04:05Z",
        "2006-01-02 15:04:05",
        "2006-01-02",
    }
    
    for _, format := range formats {
        if t, err := time.Parse(format, timeStr); err == nil {
            return t
        }
    }
    
    return time.Now()
}
```

---

## 插件注册流程

### 1. 创建插件文件
```
plugin/
  └── pluginname/
      └── pluginname.go
```

### 2. 在 main.go 中导入
```go
import (
    _ "pansou/plugin/pluginname"
)
```

### 3. 在启动脚本中启用
```bat
REM start-simple.bat
set ENABLED_PLUGINS=pluginname
```

---

## 调试技巧

### 1. 添加调试日志
```go
fmt.Printf("[%s] 开始搜索，keyword='%s'\n", p.Name(), keyword)
fmt.Printf("[%s] 调用API: %s\n", p.Name(), apiURL)
fmt.Printf("[%s] API响应状态码: %d\n", p.Name(), resp.StatusCode)
fmt.Printf("[%s] API返回: code=%d, total=%d, results=%d\n", 
    p.Name(), apiResp.Code, apiResp.Total, len(apiResp.Results))
```

### 2. 测试API直接调用
```bash
# 测试API是否正常
curl "https://www.pioz.cn/api/deep-search?kw=太奶奶"

# 测试服务是否正常
curl -H "Authorization: Bearer TOKEN" "http://localhost:8889/api/search?keyword=太奶奶&refresh=true"
```

### 3. 检查关键词传递链路
```
API Handler (api/handler.go)
  ↓ keyword parameter
SearchService.Search (service/search_service.go)
  ↓ keyword parameter
searchPlugins (service/search_service.go)
  ↓ plugin.Search(keyword, ext)
Plugin.Search (plugin/xxx/xxx.go)
  ↓ SearchWithResult(keyword, ext)
Plugin.SearchWithResult
  ↓ AsyncSearchWithResult(keyword, searchImpl, ...)
Plugin.searchImpl(client, keyword, ext)
```

---

## 常见问题排查

### 问题: 关键词为空
**检查点**:
1. API handler 参数名是否正确 (`keyword` vs `kw`)
2. goroutine 闭包是否正确捕获变量
3. 函数签名是否匹配

### 问题: 返回0结果但API正常
**检查点**:
1. 结果是否有 Links 字段 (至少需要一个链接)
2. Channel 字段是否为空 (插件结果必须为空)
3. 是否被缓存了错误结果 (使用 `refresh=true` 强制刷新)

### 问题: JSON解析失败
**检查点**:
1. 结构体字段标签是否正确 (`json:"field_name"`)
2. 字段类型是否匹配 (string vs int)
3. API返回的实际JSON结构

---

## 性能优化建议

### 1. 使用缓存
```go
// 检查缓存
cacheKey := fmt.Sprintf("%s:%s", p.Name(), keyword)
if cached, ok := searchCache.Load(cacheKey); ok {
    if cachedResp, ok := cached.(cachedResponse); ok {
        if time.Since(cachedResp.timestamp) < CacheTTL {
            return cachedResp.results, nil
        }
    }
}

// 缓存结果
searchCache.Store(cacheKey, cachedResponse{
    results:   results,
    timestamp: time.Now(),
})
```

### 2. 优化HTTP客户端
```go
client := &http.Client{
    Timeout: DefaultTimeout,
    Transport: &http.Transport{
        MaxIdleConns:        50,
        MaxIdleConnsPerHost: 10,
        IdleConnTimeout:     60 * time.Second,
    },
}
```

### 3. 设置合理的超时
```go
ctx, cancel := context.WithTimeout(context.Background(), DefaultTimeout)
defer cancel()

req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
```

---

## 测试清单

- [ ] API直接调用返回正确数据
- [ ] 服务调用返回正确数据
- [ ] 关键词正确传递到插件
- [ ] 结果包含必需的Links字段
- [ ] 结果的Channel字段为空
- [ ] 时间解析正确
- [ ] 缓存工作正常
- [ ] 强制刷新 (refresh=true) 工作正常
- [ ] 并发搜索正常
- [ ] 错误处理正确

---

## 版本历史

### v1.0 - 2026-02-08
- 创建 pioz2 插件
- 修复 API 参数名不匹配问题
- 修复空链接过滤问题
- 成功返回44个搜索结果

---

## 参考文件

- `plugin/xdpan/xdpan.go` - HTML解析示例
- `plugin/pansearch/pansearch.go` - JSON API调用示例
- `plugin/pioz2/pioz2.go` - 简化实现示例
- `plugin/plugin.go` - BaseAsyncPlugin基类
- `service/search_service.go` - 搜索服务逻辑
- `api/handler.go` - API请求处理
