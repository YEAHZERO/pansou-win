# PanSou 优化指南

## 测试结果分析

根据实际测试日志：
```
[主程序] 缓存更新完成: 4c47c4c605e53eb363ffa51a942191f3 | 结果数: 0
✅ [搜索完成] 总结果: 3
```

### 问题分析

1. **Pioz 插件返回 0 个结果**
   - 可能原因：网络问题、网站访问限制、关键词不匹配
   - Telegram 频道返回了 3 个结果

2. **响应时间 5.2 秒**
   - 首次搜索（无缓存）的正常时间
   - 后续搜索会更快（有缓存）

---

## 优化建议

### 1. 网络优化

#### 检查 Pioz 网站访问
```bash
# 测试 Pioz 网站是否可访问
curl -I https://www.pioz.cc

# 如果无法访问，可能需要配置代理
```

#### 配置代理（如果需要）
编辑 `start.bat`：
```batch
REM 添加代理配置
set PROXY=http://127.0.0.1:7890
set HTTP_PROXY=http://127.0.0.1:7890
set HTTPS_PROXY=http://127.0.0.1:7890
```

---

### 2. 性能优化

#### 增加异步超时时间
如果 Pioz 响应较慢，可以增加超时时间：

```batch
REM 从 5 秒增加到 8 秒
set ASYNC_RESPONSE_TIMEOUT=8
```

#### 增加工作线程
```batch
REM 增加后台工作者数量
set ASYNC_MAX_BACKGROUND_WORKERS=20
set ASYNC_MAX_BACKGROUND_TASKS=100
```

#### 优化并发数
```batch
REM 根据 CPU 核心数调整
REM 4核CPU: 20-25
REM 8核CPU: 30-40
set CONCURRENCY=30
```

---

### 3. 缓存优化

#### 增加缓存大小和时间
```batch
REM 增加缓存大小到 500MB
set CACHE_MAX_SIZE=500

REM 增加缓存有效期到 120 分钟
set CACHE_TTL=120
```

#### 清理旧缓存
```bash
# 如果缓存过大或损坏，可以清理
rmdir /s /q cache
```

---

### 4. 搜索优化

#### 使用更精确的关键词
```json
{
  "keyword": "电影 4K",
  "concurrency": 10
}
```

#### 只搜索 Pioz（跳过 Telegram）
```json
{
  "keyword": "电影",
  "channels": [],
  "concurrency": 5
}
```

#### 增加 Telegram 频道
编辑 `start.bat`：
```batch
REM 添加更多高质量频道
set CHANNELS=tgsearchers3,tgsearchers4,Aliyun_4K_Movies,bdbdndn11,yunpanx,bsbdbfjfjff,yp123pan,sbsbsnsqq,yunpanxunlei,tianyifc,BaiduCloudDisk,txtyzy,peccxinpd,gotopan,PanjClub,更多频道...
```

---

### 5. 调试 Pioz 插件

#### 检查 Pioz 插件日志
查看服务器日志中的 Pioz 相关信息：
```
📊 [pioz] 搜索完成: X个结果 | 耗时: Xms
```

#### 常见问题

**问题 1: Pioz 返回 0 个结果**
- 检查网站是否可访问
- 检查关键词是否合适
- 检查是否需要代理

**问题 2: Pioz 响应超时**
- 增加 `ASYNC_RESPONSE_TIMEOUT`
- 检查网络连接
- 考虑使用代理

**问题 3: Pioz 被反爬限制**
- Pioz 插件已内置反爬绕过机制
- 随机延迟、UA 轮换、完整请求头
- 如果仍被限制，可能需要等待一段时间

---

### 6. 监控和诊断

#### 使用诊断脚本
```bash
# 运行诊断脚本
diagnose.bat
```

#### 使用测试脚本
```bash
# 运行搜索测试
test-search.bat
```

#### 查看实时日志
服务器日志会显示：
- 插件加载状态
- 搜索请求和响应
- 缓存命中情况
- 错误信息

---

### 7. 推荐配置

#### 高性能配置（8核CPU，良好网络）
```batch
set PORT=8889
set CONCURRENCY=40
set ASYNC_RESPONSE_TIMEOUT=8
set ASYNC_MAX_BACKGROUND_WORKERS=20
set ASYNC_MAX_BACKGROUND_TASKS=100
set CACHE_MAX_SIZE=500
set CACHE_TTL=120
set HTTP_READ_TIMEOUT=40
set HTTP_WRITE_TIMEOUT=40
```

#### 稳定配置（4核CPU，一般网络）
```batch
set PORT=8889
set CONCURRENCY=25
set ASYNC_RESPONSE_TIMEOUT=10
set ASYNC_MAX_BACKGROUND_WORKERS=12
set ASYNC_MAX_BACKGROUND_TASKS=60
set CACHE_MAX_SIZE=300
set CACHE_TTL=90
set HTTP_READ_TIMEOUT=30
set HTTP_WRITE_TIMEOUT=30
```

#### 低资源配置（2核CPU，慢速网络）
```batch
set PORT=8889
set CONCURRENCY=15
set ASYNC_RESPONSE_TIMEOUT=15
set ASYNC_MAX_BACKGROUND_WORKERS=8
set ASYNC_MAX_BACKGROUND_TASKS=40
set CACHE_MAX_SIZE=200
set CACHE_TTL=60
set HTTP_READ_TIMEOUT=30
set HTTP_WRITE_TIMEOUT=30
```

---

### 8. 恢复备用插件

如果 Pioz 效果不理想，可以启用备用插件：

#### 步骤 1: 编辑 main.go
```go
// 取消注释需要的插件
_ "pansou/plugin/pioz"
_ "pansou/plugin/pansearch"
_ "pansou/plugin/wanou"
_ "pansou/plugin/xdpan"
_ "pansou/plugin/zhizhen"
```

#### 步骤 2: 编辑 start.bat
```batch
set ENABLED_PLUGINS=pioz,pansearch,wanou,xdpan,zhizhen
```

#### 步骤 3: 重新编译部署
```bash
go build -o pansou.exe
deploy.bat
```

---

### 9. 性能监控

#### 关键指标
- **响应时间**: 首次 5-10 秒，缓存后 < 1 秒
- **结果数量**: Pioz 通常返回 10-50 个结果
- **缓存命中率**: 应该 > 50%
- **内存使用**: 应该 < 500MB

#### 监控命令
```bash
# 查看进程内存使用
tasklist /fi "imagename eq pansou.exe" /fo table

# 查看缓存大小
dir cache /s
```

---

### 10. 故障排除

#### Pioz 完全无响应
1. 检查网站是否可访问
2. 检查防火墙设置
3. 尝试配置代理
4. 查看详细日志

#### 搜索结果质量差
1. 优化关键词
2. 增加 Telegram 频道
3. 启用更多备用插件
4. 调整搜索参数

#### 服务频繁崩溃
1. 检查内存使用
2. 减少并发数
3. 减少缓存大小
4. 检查磁盘空间

---

## 测试清单

- [ ] 服务正常启动
- [ ] 健康检查通过
- [ ] 登录认证成功
- [ ] Pioz 插件加载
- [ ] 搜索返回结果
- [ ] 缓存正常工作
- [ ] 响应时间合理
- [ ] 内存使用正常

---

## 联系支持

如果问题仍未解决：
1. 查看详细日志
2. 运行诊断脚本
3. 检查 GitHub Issues
4. 提交问题报告

---

**最后更新**: 2025-02-07  
**版本**: v6.0
