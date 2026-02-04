# Pioz HTML 数据结构分析

## 基本信息
- **数据源类型**: HTML 网页
- **搜索URL格式**: `https://www.pioz.cn/search?q={关键词}`
- **详情URL格式**: `https://www.pioz.cn/detail/{资源ID}`
- **数据特点**: 夸克小站，专注于夸克网盘资源的搜索引擎
- **特殊说明**: 搜索结果页只显示标题，需要二次跳转到详情页获取真实网盘链接

## HTML 页面结构

### 搜索结果页面
搜索结果页面包含多个搜索项，每个搜索项的HTML结构如下：

```html
<div class="search-result-item">
    <div class="title">
        <a href="/detail/65281">09.别时明月满西楼&我寄愁心与明月（71集）谢佳成&宋暖</a>
    </div>
    <div class="description">
        资源描述信息
    </div>
</div>
```

### 详情页面
详情页面包含"打开链接"按钮，点击后显示真实的网盘链接：

```html
<div class="detail-page">
    <h1>09.别时明月满西楼&我寄愁心与明月（71集）谢佳成&宋暖</h1>
    <div class="info">
        <span>夸克网盘</span>
        <span>共 73 个文件 456.5M</span>
        <span>永久有效</span>
    </div>
    <div class="actions">
        <button class="open-link">打开链接</button>
    </div>
    <!-- 真实链接 -->
    <a href="https://pan.quark.cn/s/7cdff3011b66">https://pan.quark.cn/s/7cdff3011b66</a>
</div>
```

## 插件所需字段映射

| 源字段 | 目标字段 | 说明 |
|--------|----------|------|
| 详情页URL中的ID | `UniqueID` | 格式: `pioz-html-{index}` 或 `pioz-{id}` |
| `.title a` 文本 | `Title` | 资源标题 |
| `.description` 文本 | `Content` | 资源描述 |
| 详情页中的网盘链接 | `Links` | 解析为Link数组 |
| `""` | `Channel` | 插件搜索结果Channel为空 |
| `time.Time{}` | `Datetime` | 使用零值 |

## 二次跳转机制 ⭐

### 工作流程
```
搜索结果页 
  ↓
提取详情页链接 (/detail/65281)
  ↓
访问详情页 (https://www.pioz.cn/detail/65281)
  ↓
提取"打开链接"按钮后的真实链接
  ↓
返回夸克网盘链接 (https://pan.quark.cn/s/7cdff3011b66)
```

### 实现要点
1. **识别详情页链接**: 在搜索结果中查找 `/detail/数字` 格式的链接
2. **发起HTTP请求**: 访问详情页（5秒超时）
3. **解析详情页HTML**: 使用goquery提取真实网盘链接
4. **提取密码信息**: 从页面文本中提取密码
5. **降级处理**: 如果详情页访问失败，回退到从搜索结果直接提取

## 下载链接解析

### 链接提取方式
- **从详情页提取**: 优先从详情页的 `<a href>` 标签提取链接
- **从搜索结果提取**: 如果详情页失败，从搜索结果页直接提取
- **正则表达式匹配**: 使用正则表达式匹配夸克网盘链接

### 链接类型识别
主要支持夸克网盘，通过正则表达式匹配：

```go
// 夸克网盘链接正则
quarkLinkRegex = regexp.MustCompile(`https?://pan\.quark\.cn/s/[0-9a-zA-Z]+`)

// 示例链接
https://pan.quark.cn/s/7cdff3011b66
https://pan.quark.cn/s/abc123xyz
```

### 密码提取
从页面文本中提取密码，支持多种格式：

```go
// 密码提取正则
passwordRegex = regexp.MustCompile(`(?:提取码|密码)[：:]\s*([a-zA-Z0-9]{4})`)

// 示例
提取码：1234
密码: abcd
提取码: xyz9
```

## 支持的网盘类型

### 主要支持
- **quark (夸克网盘)**: `https://pan.quark.cn/s/{分享码}`

### 部分支持
- **uc (UC网盘)**: `https://drive.uc.cn/s/{分享码}`
- **baidu (百度网盘)**: `https://pan.baidu.com/s/{分享码}`
- **aliyun (阿里云盘)**: `https://aliyundrive.com/s/{分享码}`
- **others (其他类型)**: 其他网盘链接

## 插件开发指导

### 搜索请求示例
```go
searchURL := fmt.Sprintf("https://www.pioz.cn/search?q=%s", url.QueryEscape(keyword))
```

### 详情页请求示例
```go
detailURL := fmt.Sprintf("https://www.pioz.cn/detail/%s", itemID)
```

### HTML解析流程
1. **搜索页面解析**: 使用 goquery 解析搜索结果页面
2. **提取搜索项**: 遍历搜索结果元素
3. **识别详情页链接**: 查找 `/detail/数字` 格式的链接
4. **二次跳转**: 访问详情页获取真实网盘链接
5. **降级处理**: 如果详情页失败，从搜索结果直接提取
6. **缓存管理**: 使用 sync.Map 缓存搜索结果，TTL为1小时

### SearchResult构建示例
```go
result := model.SearchResult{
    UniqueID: fmt.Sprintf("pioz-html-%d", index),
    Title:    title,
    Content:  content,
    Links:    links,
    Channel:  "", // 插件搜索结果Channel为空
    Datetime: time.Time{}, // 使用零值
}
```

### 性能配置
- **搜索超时**: 10秒 (DefaultTimeout)
- **详情页超时**: 5秒 (fetchDetailPageLinks)
- **最大重试次数**: 3次 (MaxRetries)
- **缓存TTL**: 1小时 (cacheTTL)
- **HTTP连接池**: 200个空闲连接

## 与其他插件的差异

| 特性 | pioz | zhizhen | 说明 |
|------|------|---------|------|
| **域名** | `www.pioz.cn` | `xiaomi666.fun` | 不同域名 |
| **数据格式** | HTML | HTML | 都是HTML格式 |
| **二次跳转** | ✅ 需要 | ❌ 不需要 | pioz需要访问详情页 |
| **网盘类型** | 主要夸克 | 16种网盘 | pioz专注夸克 |
| **优先级** | 1 | 1 | 都是最高质量 |

## 注意事项
1. **二次跳转**: 搜索结果页不包含真实链接，必须访问详情页
2. **超时控制**: 详情页请求设置5秒超时，避免阻塞
3. **降级处理**: 详情页失败时回退到搜索结果提取
4. **缓存管理**: 使用 sync.Map 缓存搜索结果，避免重复请求
5. **链接验证**: 使用正则表达式验证夸克网盘链接格式
6. **密码提取**: 从页面文本中提取密码信息
7. **HTTP连接池**: 复用HTTP连接，提高性能

## 开发建议
- **参考zhizhen插件**: 学习HTML解析和异步处理的最佳实践
- **关键差异**: pioz需要实现二次跳转机制
- **测试覆盖**: 重点测试详情页访问和降级处理
- **性能优化**: 使用HTTP连接池和缓存机制
- **错误处理**: 确保所有错误都有降级处理，不影响整体搜索

## 二次跳转优势
- ✅ **用户体验**: 用户无需手动点击"打开链接"
- ✅ **自动化**: 插件自动完成所有步骤
- ✅ **健壮性**: 降级处理确保稳定性
- ✅ **性能**: 超时控制避免阻塞
- ✅ **可扩展**: 易于应用到其他插件
