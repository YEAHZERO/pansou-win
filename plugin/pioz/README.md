# Pioz 插件（夸克小站）

## 简介

Pioz 插件用于搜索 [夸克小站](https://www.pioz.cn/) 网站的网盘资源。

夸克小站是一个专注于夸克网盘资源的搜索引擎，提供丰富的影视、文档、软件等资源。

## 特性

- ✅ 无需登录，开箱即用
- ✅ 双模式解析（JSON API + HTML）
- ✅ 自动提取夸克网盘链接和提取码
- ✅ 智能缓存机制（1小时有效期）
- ✅ 自动重试机制（指数退避）
- ✅ HTTP连接池优化

## 配置

### 在 start.bat 中启用

```batch
set ENABLED_PLUGINS=...,pioz
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

## 使用方法

### 1. 启用插件

在 `start.bat` 中添加 `pioz` 到 `ENABLED_PLUGINS`：

```batch
set ENABLED_PLUGINS=labi,zhizhen,shandian,pioz
```

### 2. 重启服务

```batch
start.bat
```

### 3. 测试搜索

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

## 技术实现

### 双模式解析

插件采用智能双模式解析策略：

1. **JSON API模式**（优先）
   - 尝试解析JSON格式的API响应
   - 适用于标准API接口
   - 解析速度快，准确度高

2. **HTML解析模式**（备用）
   - 当JSON解析失败时自动切换
   - 使用goquery解析HTML页面
   - 支持多种HTML结构选择器
   - 兼容性更强

### 搜索URL

```
GET https://www.pioz.cn/search?q=关键词
```

### JSON API响应格式（假设）

```json
{
  "code": 200,
  "message": "success",
  "data": [
    {
      "id": "123456",
      "title": "速度与激情全集",
      "description": "速度与激情系列电影1-10部",
      "url": "https://pan.quark.cn/s/xxxxx",
      "password": "1234",
      "create_time": "2025-01-31 10:00:00",
      "pan_type": "quark"
    }
  ],
  "total": 100
}
```

### HTML解析选择器

插件使用多种选择器来适配不同的HTML结构：

```go
// 搜索结果项
.search-result-item, .result-item, .item, .list-item

// 标题
.title, h3, h4, .name, [class*='title']

// 描述
.description, .desc, .content, p, [class*='desc']

// 链接
a[href] (包含夸克网盘链接)
```

## 支持的网盘类型

当前版本主要支持：

- ✅ **夸克网盘** (quark) - 主要支持
- ⚠️ UC网盘 (uc) - 部分支持
- ⚠️ 百度网盘 (baidu) - 部分支持
- ⚠️ 阿里云盘 (aliyun) - 部分支持
- ⚠️ 其他网盘 (others) - 有限支持

## 工作原理

### 1. 搜索流程

```
用户搜索 
  ↓
构建搜索URL (https://www.pioz.cn/search?q=关键词)
  ↓
发送HTTP请求（带重试）
  ↓
尝试JSON解析
  ↓
JSON成功？ → 是 → 提取API数据
  ↓ 否
HTML解析 → 使用goquery提取
  ↓
提取链接和密码
  ↓
关键词过滤
  ↓
返回结果
```

### 2. 链接提取

插件会自动：
- 使用正则表达式提取夸克网盘链接：`https://pan.quark.cn/s/[0-9a-zA-Z]+`
- 从文本中提取提取码（格式：`提取码：xxxx` 或 `密码：xxxx`）
- 根据URL自动识别网盘类型

### 3. 缓存机制

- 搜索结果缓存1小时
- 每小时自动清理过期缓存
- 减少重复请求，提升响应速度

### 4. 重试机制

- 请求失败自动重试（最多3次）
- 使用指数退避策略（200ms, 400ms, 800ms）
- 提高请求成功率

### 5. 性能优化

- HTTP连接池复用（最大200个空闲连接）
- 请求克隆避免并发问题
- 智能超时控制

## 故障排除

### Q1: 搜索无结果？

**A**: 可能原因：
1. 网站是前端渲染，实际API结构与预期不同
2. HTML结构变化，选择器失效
3. 关键词不匹配

**解决方法**：
1. 使用浏览器开发者工具（F12）查看实际的网络请求
2. 找到真实的API端点和响应格式
3. 修改 `plugin/pioz/pioz.go` 中的相关代码：
   - `SearchURL` 常量
   - `PiozAPIResponse` 和 `PiozItem` 结构体
   - `parseHTMLResults` 函数中的选择器

### Q2: 如何调试插件？

**A**: 调试步骤：

1. **查看实际API**
   ```bash
   # 打开浏览器开发者工具
   # 访问 https://www.pioz.cn/search?q=测试
   # 查看 Network 标签中的 XHR/Fetch 请求
   ```

2. **查看服务器日志**
   ```bash
   # 查看PanSou服务器输出
   # 搜索 [pioz] 相关的日志信息
   ```

3. **测试单个插件**
   ```bash
   curl "http://localhost:8888/api/search?kw=测试&plugins=pioz"
   ```

### Q3: 请求超时？

**A**: 可能原因：
1. 网络连接问题
2. 网站响应慢
3. 超时时间设置过短

**解决方法**：
1. 检查网络连接
2. 增加超时时间（修改 `DefaultTimeout` 为 15秒或更长）
3. 配置代理

### Q4: HTML解析失败？

**A**: 可能原因：
1. 网站HTML结构变化
2. 选择器不匹配

**解决方法**：
1. 查看网站实际的HTML结构
2. 修改 `parseHTMLResults` 和 `parseSearchItem` 函数中的选择器
3. 添加更多备用选择器

## 开发说明

### 文件结构

```
plugin/pioz/
├── pioz.go          # 插件主文件
└── README.md        # 说明文档
```

### 核心函数

- `NewPiozPlugin()`: 创建插件实例
- `doSearch()`: 执行搜索逻辑（双模式）
- `parseHTMLResults()`: 解析HTML响应
- `parseSearchItem()`: 解析单个搜索结果项
- `convertAPIResults()`: 转换API结果格式
- `extractLinkInfo()`: 提取链接信息
- `detectLinkType()`: 检测网盘类型
- `doRequestWithRetry()`: 带重试的HTTP请求

### 扩展开发

如果需要修改插件行为，可以：

1. **调整超时时间**
   ```go
   DefaultTimeout = 15 * time.Second
   ```

2. **增加结果数量**
   ```go
   PageSize = 50
   MaxResults = 500
   ```

3. **修改缓存时间**
   ```go
   cacheTTL = 2 * time.Hour
   ```

4. **添加新的HTML选择器**
   ```go
   // 在 parseHTMLResults 函数中添加
   doc.Find(".new-selector").Each(...)
   ```

5. **支持更多网盘类型**
   ```go
   // 在 detectLinkType 函数中添加
   case strings.Contains(url, "newpan.com"):
       return "newpan"
   ```

## 注意事项

1. ⚠️ **网站结构不确定**：由于 pioz.cn 是前端渲染网站，实际的API结构可能与代码中的假设不同
2. ⚠️ **需要实际测试**：建议在实际环境中测试并根据网站的真实API结构调整代码
3. ⚠️ **仅用于学习**：本插件仅用于学习和研究目的
4. ⚠️ **遵守规则**：请遵守网站的使用条款和robots.txt
5. ⚠️ **避免滥用**：不要过于频繁地请求，避免给网站造成压力
6. ⚠️ **版权声明**：资源版权归原作者所有

## 更新日志

### v1.0.0 (2025-01-31)

- ✨ 初始版本
- ✅ 实现双模式解析（JSON + HTML）
- ✅ 支持夸克网盘链接提取
- ✅ 实现缓存机制
- ✅ 实现重试机制
- ✅ HTTP连接池优化
- ⚠️ 注意：需要根据实际网站结构调整

## 相关链接

- 夸克小站: https://www.pioz.cn/
- PanSou 项目: https://github.com/fish2018/pansou
- 插件开发指南: ../../docs/插件开发指南.md

## 许可证

MIT License

---

**最后更新**: 2025-01-31
**状态**: ⚠️ 需要根据实际网站API调整

