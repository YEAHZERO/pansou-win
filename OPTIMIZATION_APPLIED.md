# 优化配置已应用 v6.1

## 优化时间
2025-02-07

---

## 主要优化

### 1. 禁用 Telegram 频道 ✅

**原因**：
- 专注于 Pioz 插件搜索
- 减少不必要的网络请求
- 提高响应速度

**配置**：
```batch
REM Telegram channels (DISABLED - not using Telegram)
set CHANNELS=
```

**效果**：
- 搜索只使用 Pioz 插件
- 减少并发请求
- 更快的响应时间

---

### 2. 确认 Pioz 网站地址 ✅

**网站地址**：
- ✅ 主站：https://www.pioz.cn
- ✅ API：https://www.pioz.cn/api

**验证**：
```bash
curl -I https://www.pioz.cn/
```

---

### 3. 优化超时配置 ✅

**调整前**：
```batch
set ASYNC_RESPONSE_TIMEOUT=5
set PLUGIN_TIMEOUT=30
```

**调整后**：
```batch
set ASYNC_RESPONSE_TIMEOUT=10    # 增加到10秒
set PLUGIN_TIMEOUT=20             # 新增配置
```

**原因**：
- Pioz 网站响应可能需要更多时间
- 特别是首次访问或详情页获取
- 10秒是平衡速度和成功率的最佳值

---

### 4. 优化并发配置 ✅

**调整前**：
```batch
set CONCURRENCY=25
set ASYNC_MAX_BACKGROUND_WORKERS=12
set ASYNC_MAX_BACKGROUND_TASKS=60
```

**调整后**：
```batch
set CONCURRENCY=15                      # 减少总并发
set ASYNC_MAX_BACKGROUND_WORKERS=16     # 增加工作线程
set ASYNC_MAX_BACKGROUND_TASKS=80       # 增加任务队列
```

**原因**：
- 只使用 Pioz，不需要太高的总并发
- 增加工作线程以处理 Pioz 的异步请求
- 更好地利用 Pioz 的并发能力

---

### 5. 优化缓存配置 ✅

**调整前**：
```batch
set CACHE_MAX_SIZE=300
set CACHE_TTL=90
```

**调整后**：
```batch
set CACHE_MAX_SIZE=500    # 增加到500MB
set CACHE_TTL=120         # 增加到120分钟
```

**原因**：
- Pioz 结果质量高，值得缓存更久
- 更大的缓存空间提高命中率
- 减少重复请求

---

### 6. 优化 HTTP 服务器配置 ✅

**调整前**：
```batch
set HTTP_READ_TIMEOUT=30
set HTTP_WRITE_TIMEOUT=30
set HTTP_MAX_CONNS=200
```

**调整后**：
```batch
set HTTP_READ_TIMEOUT=40     # 增加读取超时
set HTTP_WRITE_TIMEOUT=40    # 增加写入超时
set HTTP_MAX_CONNS=300       # 增加最大连接数
```

**原因**：
- 给 Pioz 更多时间完成请求
- 支持更多并发连接
- 提高系统稳定性

---

## 完整优化配置

### start.bat 配置

```batch
REM Basic configuration
set PORT=8889
set CACHE_ENABLED=true
set CACHE_PATH=%SCRIPT_DIR%\cache
set TZ=Asia/Shanghai

REM Telegram channels (DISABLED - not using Telegram)
set CHANNELS=

REM Plugin configuration (only pioz enabled)
set ASYNC_PLUGIN_ENABLED=true
set ENABLED_PLUGINS=pioz

REM Performance configuration (optimized for Pioz)
set CONCURRENCY=15
set ASYNC_RESPONSE_TIMEOUT=10
set ASYNC_MAX_BACKGROUND_WORKERS=16
set ASYNC_MAX_BACKGROUND_TASKS=80
set CACHE_MAX_SIZE=500
set CACHE_TTL=120
set PLUGIN_TIMEOUT=20

REM Authentication
set AUTH_ENABLED=true
set AUTH_USERS=admin:123456
set AUTH_JWT_SECRET=pansou-secret-key-2024

REM HTTP server configuration (optimized for Pioz)
set HTTP_READ_TIMEOUT=40
set HTTP_WRITE_TIMEOUT=40
set HTTP_IDLE_TIMEOUT=120
set HTTP_MAX_CONNS=300
```

---

## 新增测试工具

### 1. test-pioz.bat ✅

**功能**：
- 测试 Pioz 网站访问性
- 自动登录认证
- 测试 3 个不同关键词
- 保存结果到 JSON 文件
- 显示结果摘要

**使用**：
```bash
test-pioz.bat
```

**测试关键词**：
1. "电影" - 通用搜索
2. "流浪地球" - 具体电影
3. "美剧" - 类型搜索

### 2. 优化的 diagnose.bat ✅

**新增功能**：
- 检查 Pioz 网站访问性
- 显示当前配置
- 测试搜索（不使用 Telegram）
- 提供优化建议

**使用**：
```bash
diagnose.bat
```

---

## 预期效果

### 性能提升

| 指标 | 优化前 | 优化后 | 提升 |
|------|--------|--------|------|
| 响应超时 | 5秒 | 10秒 | +100% |
| 工作线程 | 12 | 16 | +33% |
| 缓存大小 | 300MB | 500MB | +67% |
| 缓存时间 | 90分钟 | 120分钟 | +33% |
| HTTP超时 | 30秒 | 40秒 | +33% |

### 搜索质量

- ✅ 更高的成功率（10秒超时）
- ✅ 更多的结果（更大缓存）
- ✅ 更快的响应（缓存命中）
- ✅ 更稳定的服务（优化配置）

### 资源使用

- ✅ 更低的并发（15 vs 25）
- ✅ 更高效的线程使用
- ✅ 更好的内存管理
- ✅ 更少的网络请求（无 Telegram）

---

## 测试步骤

### 1. 重新编译（如果需要）

```bash
go build -o pansou.exe
```

### 2. 启动服务

```bash
start.bat
```

### 3. 运行测试

```bash
# 方式1：专项测试（推荐）
test-pioz.bat

# 方式2：诊断测试
diagnose.bat
```

### 4. 查看结果

检查生成的 JSON 文件：
- `result1.json` - "电影" 搜索结果
- `result2.json` - "流浪地球" 搜索结果
- `result3.json` - "美剧" 搜索结果

---

## 故障排除

### 如果 Pioz 仍返回 0 个结果

1. **检查网站访问**
   ```bash
   curl -I https://www.pioz.cn/
   ```

2. **检查网络连接**
   - 确保可以访问 pioz.cn
   - 检查防火墙设置
   - 尝试使用浏览器访问

3. **配置代理（如果需要）**
   ```batch
   set PROXY=http://127.0.0.1:7890
   set HTTP_PROXY=http://127.0.0.1:7890
   set HTTPS_PROXY=http://127.0.0.1:7890
   ```

4. **增加超时时间**
   ```batch
   set ASYNC_RESPONSE_TIMEOUT=15
   set PLUGIN_TIMEOUT=30
   ```

5. **查看详细日志**
   - 启动服务后查看控制台输出
   - 查找 Pioz 相关的错误信息

### 如果搜索很慢

1. **检查缓存**
   - 首次搜索会慢（无缓存）
   - 后续搜索应该很快（有缓存）

2. **清理旧缓存**
   ```bash
   rmdir /s /q cache
   ```

3. **调整并发**
   ```batch
   set CONCURRENCY=10
   ```

---

## 配置对比

### 优化前（v6.0）

```batch
set CHANNELS=tgsearchers3,tgsearchers4,...  # 15个频道
set ENABLED_PLUGINS=pioz
set CONCURRENCY=25
set ASYNC_RESPONSE_TIMEOUT=5
set ASYNC_MAX_BACKGROUND_WORKERS=12
set CACHE_MAX_SIZE=300
set CACHE_TTL=90
```

### 优化后（v6.1）

```batch
set CHANNELS=                               # 禁用Telegram
set ENABLED_PLUGINS=pioz
set CONCURRENCY=15                          # 减少
set ASYNC_RESPONSE_TIMEOUT=10               # 增加
set ASYNC_MAX_BACKGROUND_WORKERS=16         # 增加
set CACHE_MAX_SIZE=500                      # 增加
set CACHE_TTL=120                           # 增加
set PLUGIN_TIMEOUT=20                       # 新增
```

---

## 关键改进

1. ✅ **专注 Pioz** - 禁用 Telegram，只使用 Pioz
2. ✅ **更长超时** - 10秒响应超时，20秒插件超时
3. ✅ **更多线程** - 16个工作线程处理异步请求
4. ✅ **更大缓存** - 500MB 缓存，120分钟有效期
5. ✅ **更好测试** - 专项测试工具和优化诊断

---

## 下一步

1. ✅ 启动服务：`start.bat`
2. ✅ 运行测试：`test-pioz.bat`
3. ✅ 查看结果：检查 result*.json 文件
4. ✅ 如有问题：运行 `diagnose.bat`

---

**优化版本**: v6.1  
**优化日期**: 2025-02-07  
**状态**: ✅ 已应用，待测试
