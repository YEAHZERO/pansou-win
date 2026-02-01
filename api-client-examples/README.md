# PanSou API 客户端示例

本目录包含了使用不同编程语言和方式调用 PanSou API 的示例代码。

## 📁 文件说明

| 文件 | 语言/技术 | 描述 |
|------|-----------|------|
| `python_client.py` | Python | 功能完整的 Python 命令行客户端 |
| `web_client.html` | HTML/JavaScript | 美观的 Web 界面客户端 |
| `powershell_client.ps1` | PowerShell | Windows PowerShell 客户端 |

## 🚀 快速开始

### Python 客户端

```bash
# 安装依赖
pip install requests

# 基础搜索
python python_client.py "速度与激情"

# 连接本地服务
python python_client.py "Python教程" --url http://localhost:8888

# 认证搜索
python python_client.py "电影" --username admin --password your_password

# 指定网盘类型
python python_client.py "资源" --cloud-types baidu,aliyun --limit 5

# 健康检查
python python_client.py --health "test"
```

### Web 客户端

```bash
# 直接在浏览器中打开
start web_client.html

# 或使用 HTTP 服务器
python -m http.server 8080
# 然后访问 http://localhost:8080/web_client.html
```

### PowerShell 客户端

```powershell
# 基础搜索
.\powershell_client.ps1 -Keyword "速度与激情"

# 连接本地服务
.\powershell_client.ps1 -Keyword "Python教程" -ApiUrl "http://localhost:8888"

# 认证搜索
.\powershell_client.ps1 -Keyword "电影" -Username "admin" -Password "your_password"

# 指定网盘类型
.\powershell_client.ps1 -Keyword "资源" -CloudTypes @("baidu","aliyun") -Limit 5

# 健康检查
.\powershell_client.ps1 -Health -Keyword "test"
```

## 🔧 配置说明

### API 服务地址

- **官方服务**: `https://so.252035.xyz` (需要认证)
- **本地服务**: `http://localhost:8888` (通常不需要认证)

### 认证参数

如果服务启用了认证，需要提供：
- `username`: 用户名 (通常是 `admin`)
- `password`: 密码

### 搜索参数

- `keyword`: 搜索关键词 (必填)
- `cloud_types`: 网盘类型过滤 (可选)
  - 支持: `baidu`, `aliyun`, `quark`, `tianyi`, `uc`, `mobile`, `115`, `pikpak`, `xunlei`, `123`, `magnet`, `ed2k`, `others`
- `plugins`: 插件过滤 (可选)
- `source`: 数据源 (`all`/`tg`/`plugin`)
- `refresh`: 强制刷新缓存

## 📊 响应格式

所有客户端都会返回统一的 JSON 格式：

```json
{
  "total": 15,
  "merged_by_type": {
    "baidu": [
      {
        "url": "https://pan.baidu.com/s/1abcdef",
        "password": "1234",
        "note": "资源标题",
        "datetime": "2023-06-10T14:23:45Z",
        "source": "tg:频道名称"
      }
    ],
    "aliyun": [...],
    "quark": [...]
  }
}
```

## 🛠️ 自定义开发

### 基础 HTTP 请求

```bash
# 健康检查
curl https://so.252035.xyz/api/health

# 登录 (如果需要)
curl -X POST https://so.252035.xyz/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"your_password"}'

# 搜索
curl -X POST https://so.252035.xyz/api/search \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -d '{"kw":"速度与激情","res":"merge"}'
```

### 错误处理

所有客户端都应该处理以下错误：

- **401 Unauthorized**: 认证失败或 token 过期
- **400 Bad Request**: 请求参数错误
- **500 Internal Server Error**: 服务器内部错误
- **网络错误**: 连接超时、DNS 解析失败等

### 最佳实践

1. **缓存 Token**: 登录后缓存 token，避免频繁登录
2. **错误重试**: 网络错误时实现重试机制
3. **限流控制**: 避免过于频繁的请求
4. **结果分页**: 大量结果时考虑分页显示
5. **用户体验**: 提供搜索进度提示和结果统计

## 🔍 功能特性

### Python 客户端特性
- ✅ 完整的命令行参数支持
- ✅ 彩色输出和格式化显示
- ✅ 详细的错误处理和提示
- ✅ 健康检查功能
- ✅ 支持所有 API 参数

### Web 客户端特性
- ✅ 现代化的响应式界面
- ✅ 实时搜索状态显示
- ✅ 多种网盘类型选择
- ✅ 自动服务状态检测
- ✅ 移动端友好设计

### PowerShell 客户端特性
- ✅ Windows 原生支持
- ✅ 完整的参数验证
- ✅ 彩色控制台输出
- ✅ 详细的帮助信息
- ✅ 错误处理和调试模式

## 🚨 注意事项

1. **认证信息安全**: 不要在代码中硬编码密码
2. **请求频率**: 合理控制请求频率，避免被限制
3. **网络超时**: 设置合理的网络超时时间
4. **结果处理**: 大量结果时注意内存使用
5. **版本兼容**: API 可能会更新，注意版本兼容性

## 📞 获取帮助

如果遇到问题：

1. 检查网络连接和服务状态
2. 验证认证信息是否正确
3. 查看详细的错误日志
4. 参考项目的 GitHub Issues
5. 联系服务提供方

## 🎉 贡献代码

欢迎提交更多语言的客户端示例：

- Java 客户端
- C# 客户端
- Go 客户端
- PHP 客户端
- Ruby 客户端
- 等等...

提交 Pull Request 时请确保：
- 代码风格一致
- 包含完整的错误处理
- 提供使用示例
- 更新相关文档