# PanSou 纯 API 使用指南

本指南介绍如何直接使用 PanSou 的 HTTP API 进行网盘资源搜索，无需 MCP 服务。

## 🎯 使用方式

### 方式一：直接使用官方 API

官方服务地址：`https://so.252035.xyz`

### 方式二：本地部署 API 服务

在本地运行 PanSou 服务，然后通过 HTTP API 调用。

## 🚀 本地部署

### Docker 部署（推荐）

```bash
# 启动服务
docker run -d --name pansou -p 8888:8888 ghcr.io/fish2018/pansou:latest

# 验证服务
curl http://localhost:8888/api/health
```

### Windows 源码部署

```cmd
# 克隆项目
git clone https://github.com/fish2018/pansou.git
cd pansou

# 编译
go build -o pansou.exe .

# 启动
.\pansou.exe
```

## 📡 API 接口详解

### 1. 健康检查

**接口**: `GET /api/health`

```bash
curl https://so.252035.xyz/api/health
```

**响应示例**:
```json
{
  "status": "ok",
  "auth_enabled": true,
  "plugins_enabled": true,
  "plugin_count": 38,
  "plugins": ["labi", "zhizhen", "shandian", ...],
  "channels_count": 50,
  "channels": ["tgsearchers4", "Aliyun_4K_Movies", ...]
}
```

### 2. 用户认证（如果启用）

#### 登录获取 Token

**接口**: `POST /api/auth/login`

```bash
curl -X POST https://so.252035.xyz/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "username": "admin",
    "password": "your_password"
  }'
```

**响应示例**:
```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "expires_at": 1234567890,
  "username": "admin"
}
```

### 3. 搜索网盘资源

**接口**: `POST /api/search`

#### 基础搜索

```bash
# 未启用认证的服务
curl -X POST http://localhost:8888/api/search \
  -H "Content-Type: application/json" \
  -d '{
    "kw": "速度与激情"
  }'

# 启用认证的服务（需要先登录获取 token）
curl -X POST https://so.252035.xyz/api/search \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_TOKEN_HERE" \
  -d '{
    "kw": "速度与激情"
  }'
```

#### 高级搜索参数

```bash
curl -X POST https://so.252035.xyz/api/search \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_TOKEN_HERE" \
  -d '{
    "kw": "速度与激情",
    "res": "merge",
    "src": "all",
    "cloud_types": ["baidu", "aliyun", "quark"],
    "plugins": ["labi", "zhizhen"],
    "refresh": false,
    "ext": {
      "title_en": "Fast and Furious"
    }
  }'
```

**参数说明**:
- `kw`: 搜索关键词（必填）
- `res`: 返回格式 (`merge`/`all`/`results`)
- `src`: 数据源 (`all`/`tg`/`plugin`)
- `cloud_types`: 网盘类型过滤
- `plugins`: 指定插件
- `refresh`: 强制刷新缓存
- `ext`: 扩展参数

#### GET 方式搜索

```bash
# 简单搜索
curl "https://so.252035.xyz/api/search?kw=速度与激情" \
  -H "Authorization: Bearer YOUR_TOKEN_HERE"

# 带参数搜索
curl "https://so.252035.xyz/api/search?kw=速度与激情&res=merge&src=all&cloud_types=baidu,aliyun" \
  -H "Authorization: Bearer YOUR_TOKEN_HERE"
```

## 🔧 编程语言示例

### Python 示例

```python
import requests
import json

class PanSouAPI:
    def __init__(self, base_url, username=None, password=None):
        self.base_url = base_url.rstrip('/')
        self.session = requests.Session()
        self.token = None
        
        if username and password:
            self.login(username, password)
    
    def login(self, username, password):
        """登录获取 token"""
        url = f"{self.base_url}/api/auth/login"
        data = {"username": username, "password": password}
        
        response = self.session.post(url, json=data)
        if response.status_code == 200:
            result = response.json()
            self.token = result['token']
            self.session.headers.update({
                'Authorization': f'Bearer {self.token}'
            })
            print(f"登录成功，token 将在 {result['expires_at']} 过期")
        else:
            raise Exception(f"登录失败: {response.text}")
    
    def health_check(self):
        """健康检查"""
        url = f"{self.base_url}/api/health"
        response = self.session.get(url)
        return response.json()
    
    def search(self, keyword, **kwargs):
        """搜索网盘资源"""
        url = f"{self.base_url}/api/search"
        data = {"kw": keyword, **kwargs}
        
        response = self.session.post(url, json=data)
        if response.status_code == 200:
            return response.json()
        else:
            raise Exception(f"搜索失败: {response.text}")

# 使用示例
if __name__ == "__main__":
    # 连接官方服务（需要认证）
    api = PanSouAPI("https://so.252035.xyz", "admin", "your_password")
    
    # 或连接本地服务（无需认证）
    # api = PanSouAPI("http://localhost:8888")
    
    # 健康检查
    health = api.health_check()
    print(f"服务状态: {health['status']}")
    
    # 搜索
    results = api.search(
        keyword="速度与激情",
        res="merge",
        cloud_types=["baidu", "aliyun", "quark"]
    )
    
    # 处理结果
    if 'merged_by_type' in results:
        for cloud_type, links in results['merged_by_type'].items():
            print(f"\n{cloud_type} 网盘 ({len(links)} 个链接):")
            for link in links[:3]:  # 只显示前3个
                print(f"  - {link['note']}")
                print(f"    链接: {link['url']}")
                if link['password']:
                    print(f"    密码: {link['password']}")
```

### JavaScript/Node.js 示例

```javascript
const axios = require('axios');

class PanSouAPI {
    constructor(baseUrl, username = null, password = null) {
        this.baseUrl = baseUrl.replace(/\/$/, '');
        this.client = axios.create({
            baseURL: this.baseUrl,
            headers: {
                'Content-Type': 'application/json'
            }
        });
        this.token = null;
        
        if (username && password) {
            this.login(username, password);
        }
    }
    
    async login(username, password) {
        try {
            const response = await this.client.post('/api/auth/login', {
                username,
                password
            });
            
            this.token = response.data.token;
            this.client.defaults.headers.Authorization = `Bearer ${this.token}`;
            console.log(`登录成功，token 将在 ${response.data.expires_at} 过期`);
        } catch (error) {
            throw new Error(`登录失败: ${error.response?.data?.message || error.message}`);
        }
    }
    
    async healthCheck() {
        const response = await this.client.get('/api/health');
        return response.data;
    }
    
    async search(keyword, options = {}) {
        try {
            const response = await this.client.post('/api/search', {
                kw: keyword,
                ...options
            });
            return response.data;
        } catch (error) {
            throw new Error(`搜索失败: ${error.response?.data?.message || error.message}`);
        }
    }
}

// 使用示例
async function main() {
    try {
        // 连接官方服务（需要认证）
        const api = new PanSouAPI('https://so.252035.xyz', 'admin', 'your_password');
        
        // 或连接本地服务（无需认证）
        // const api = new PanSouAPI('http://localhost:8888');
        
        // 健康检查
        const health = await api.healthCheck();
        console.log(`服务状态: ${health.status}`);
        
        // 搜索
        const results = await api.search('速度与激情', {
            res: 'merge',
            cloud_types: ['baidu', 'aliyun', 'quark']
        });
        
        // 处理结果
        if (results.merged_by_type) {
            Object.entries(results.merged_by_type).forEach(([cloudType, links]) => {
                console.log(`\n${cloudType} 网盘 (${links.length} 个链接):`);
                links.slice(0, 3).forEach(link => {
                    console.log(`  - ${link.note}`);
                    console.log(`    链接: ${link.url}`);
                    if (link.password) {
                        console.log(`    密码: ${link.password}`);
                    }
                });
            });
        }
    } catch (error) {
        console.error('错误:', error.message);
    }
}

main();
```

### PowerShell 示例

```powershell
# PanSou API PowerShell 客户端

class PanSouAPI {
    [string]$BaseUrl
    [string]$Token
    [hashtable]$Headers
    
    PanSouAPI([string]$baseUrl) {
        $this.BaseUrl = $baseUrl.TrimEnd('/')
        $this.Headers = @{
            'Content-Type' = 'application/json'
        }
    }
    
    [void]Login([string]$username, [string]$password) {
        $loginData = @{
            username = $username
            password = $password
        } | ConvertTo-Json
        
        try {
            $response = Invoke-RestMethod -Uri "$($this.BaseUrl)/api/auth/login" -Method Post -Body $loginData -Headers $this.Headers
            $this.Token = $response.token
            $this.Headers['Authorization'] = "Bearer $($this.Token)"
            Write-Host "登录成功" -ForegroundColor Green
        }
        catch {
            throw "登录失败: $($_.Exception.Message)"
        }
    }
    
    [object]HealthCheck() {
        return Invoke-RestMethod -Uri "$($this.BaseUrl)/api/health" -Method Get -Headers $this.Headers
    }
    
    [object]Search([string]$keyword, [hashtable]$options = @{}) {
        $searchData = @{ kw = $keyword }
        $options.GetEnumerator() | ForEach-Object { $searchData[$_.Key] = $_.Value }
        $jsonData = $searchData | ConvertTo-Json -Depth 10
        
        try {
            return Invoke-RestMethod -Uri "$($this.BaseUrl)/api/search" -Method Post -Body $jsonData -Headers $this.Headers
        }
        catch {
            throw "搜索失败: $($_.Exception.Message)"
        }
    }
}

# 使用示例
try {
    # 创建 API 客户端
    $api = [PanSouAPI]::new("https://so.252035.xyz")
    
    # 登录（如果需要）
    $api.Login("admin", "your_password")
    
    # 健康检查
    $health = $api.HealthCheck()
    Write-Host "服务状态: $($health.status)" -ForegroundColor Green
    
    # 搜索
    $results = $api.Search("速度与激情", @{
        res = "merge"
        cloud_types = @("baidu", "aliyun", "quark")
    })
    
    # 显示结果
    if ($results.merged_by_type) {
        $results.merged_by_type.PSObject.Properties | ForEach-Object {
            $cloudType = $_.Name
            $links = $_.Value
            Write-Host "`n$cloudType 网盘 ($($links.Count) 个链接):" -ForegroundColor Yellow
            
            $links | Select-Object -First 3 | ForEach-Object {
                Write-Host "  - $($_.note)" -ForegroundColor White
                Write-Host "    链接: $($_.url)" -ForegroundColor Cyan
                if ($_.password) {
                    Write-Host "    密码: $($_.password)" -ForegroundColor Magenta
                }
            }
        }
    }
}
catch {
    Write-Error "错误: $($_.Exception.Message)"
}
```

## 🌐 Web 前端示例

### HTML + JavaScript

```html
<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>PanSou 网盘搜索</title>
    <style>
        body { font-family: Arial, sans-serif; max-width: 800px; margin: 0 auto; padding: 20px; }
        .search-box { margin-bottom: 20px; }
        .search-box input { width: 300px; padding: 10px; margin-right: 10px; }
        .search-box button { padding: 10px 20px; }
        .results { margin-top: 20px; }
        .cloud-type { margin-bottom: 20px; border: 1px solid #ddd; padding: 15px; }
        .cloud-type h3 { margin-top: 0; color: #333; }
        .link-item { margin-bottom: 10px; padding: 10px; background: #f9f9f9; }
        .link-url { color: #0066cc; text-decoration: none; }
        .link-password { color: #ff6600; font-weight: bold; }
        .loading { color: #666; }
        .error { color: #cc0000; }
    </style>
</head>
<body>
    <h1>PanSou 网盘搜索</h1>
    
    <div class="search-box">
        <input type="text" id="keyword" placeholder="输入搜索关键词" />
        <button onclick="search()">搜索</button>
    </div>
    
    <div id="results" class="results"></div>

    <script>
        const API_BASE = 'https://so.252035.xyz'; // 或 'http://localhost:8888'
        let authToken = null;

        // 登录函数（如果需要认证）
        async function login(username, password) {
            try {
                const response = await fetch(`${API_BASE}/api/auth/login`, {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ username, password })
                });
                
                if (response.ok) {
                    const data = await response.json();
                    authToken = data.token;
                    console.log('登录成功');
                } else {
                    throw new Error('登录失败');
                }
            } catch (error) {
                console.error('登录错误:', error);
            }
        }

        // 搜索函数
        async function search() {
            const keyword = document.getElementById('keyword').value.trim();
            if (!keyword) {
                alert('请输入搜索关键词');
                return;
            }

            const resultsDiv = document.getElementById('results');
            resultsDiv.innerHTML = '<div class="loading">搜索中...</div>';

            try {
                const headers = { 'Content-Type': 'application/json' };
                if (authToken) {
                    headers['Authorization'] = `Bearer ${authToken}`;
                }

                const response = await fetch(`${API_BASE}/api/search`, {
                    method: 'POST',
                    headers,
                    body: JSON.stringify({
                        kw: keyword,
                        res: 'merge',
                        cloud_types: ['baidu', 'aliyun', 'quark', 'tianyi']
                    })
                });

                if (response.ok) {
                    const data = await response.json();
                    displayResults(data);
                } else if (response.status === 401) {
                    resultsDiv.innerHTML = '<div class="error">需要登录认证</div>';
                    // 这里可以弹出登录框
                } else {
                    throw new Error(`搜索失败: ${response.statusText}`);
                }
            } catch (error) {
                resultsDiv.innerHTML = `<div class="error">错误: ${error.message}</div>`;
            }
        }

        // 显示结果
        function displayResults(data) {
            const resultsDiv = document.getElementById('results');
            
            if (!data.merged_by_type || Object.keys(data.merged_by_type).length === 0) {
                resultsDiv.innerHTML = '<div>未找到相关资源</div>';
                return;
            }

            let html = '';
            Object.entries(data.merged_by_type).forEach(([cloudType, links]) => {
                html += `
                    <div class="cloud-type">
                        <h3>${getCloudTypeName(cloudType)} (${links.length} 个链接)</h3>
                `;
                
                links.slice(0, 10).forEach(link => {
                    html += `
                        <div class="link-item">
                            <div><strong>${link.note}</strong></div>
                            <div>
                                <a href="${link.url}" target="_blank" class="link-url">${link.url}</a>
                            </div>
                            ${link.password ? `<div class="link-password">提取码: ${link.password}</div>` : ''}
                            <div style="font-size: 12px; color: #666;">
                                来源: ${link.source || '未知'} | 时间: ${link.datetime || '未知'}
                            </div>
                        </div>
                    `;
                });
                
                html += '</div>';
            });

            resultsDiv.innerHTML = html;
        }

        // 网盘类型名称映射
        function getCloudTypeName(type) {
            const names = {
                'baidu': '百度网盘',
                'aliyun': '阿里云盘',
                'quark': '夸克网盘',
                'tianyi': '天翼云盘',
                'uc': 'UC网盘',
                'mobile': '移动云盘',
                '115': '115网盘',
                'pikpak': 'PikPak',
                'xunlei': '迅雷网盘',
                '123': '123网盘',
                'magnet': '磁力链接',
                'ed2k': '电驴链接'
            };
            return names[type] || type;
        }

        // 页面加载时检查是否需要登录
        window.onload = async function() {
            try {
                const response = await fetch(`${API_BASE}/api/health`);
                const health = await response.json();
                
                if (health.auth_enabled) {
                    // 如果启用了认证，这里可以显示登录界面
                    console.log('服务启用了认证，需要登录');
                    // 示例：自动登录（实际使用时应该有登录界面）
                    // await login('admin', 'your_password');
                }
            } catch (error) {
                console.error('无法连接到服务:', error);
            }
        };

        // 回车搜索
        document.getElementById('keyword').addEventListener('keypress', function(e) {
            if (e.key === 'Enter') {
                search();
            }
        });
    </script>
</body>
</html>
```

## 🔧 工具和脚本

### 命令行搜索工具

创建 `search.py`:

```python
#!/usr/bin/env python3
import sys
import argparse
from pansou_api import PanSouAPI  # 使用上面的 Python 类

def main():
    parser = argparse.ArgumentParser(description='PanSou 命令行搜索工具')
    parser.add_argument('keyword', help='搜索关键词')
    parser.add_argument('--url', default='https://so.252035.xyz', help='API 地址')
    parser.add_argument('--username', help='用户名')
    parser.add_argument('--password', help='密码')
    parser.add_argument('--cloud-types', help='网盘类型，逗号分隔')
    parser.add_argument('--limit', type=int, default=10, help='每种网盘显示的链接数量')
    
    args = parser.parse_args()
    
    try:
        api = PanSouAPI(args.url, args.username, args.password)
        
        search_options = {}
        if args.cloud_types:
            search_options['cloud_types'] = args.cloud_types.split(',')
        
        results = api.search(args.keyword, res='merge', **search_options)
        
        if 'merged_by_type' in results:
            for cloud_type, links in results['merged_by_type'].items():
                print(f"\n{cloud_type.upper()} ({len(links)} 个链接):")
                print("-" * 50)
                
                for i, link in enumerate(links[:args.limit], 1):
                    print(f"{i}. {link['note']}")
                    print(f"   链接: {link['url']}")
                    if link['password']:
                        print(f"   密码: {link['password']}")
                    print()
        else:
            print("未找到相关资源")
            
    except Exception as e:
        print(f"错误: {e}", file=sys.stderr)
        sys.exit(1)

if __name__ == '__main__':
    main()
```

使用方法:
```bash
python search.py "速度与激情" --username admin --password your_password
python search.py "Python教程" --cloud-types baidu,aliyun --limit 5
```

## 📊 响应格式说明

### 搜索结果结构

```json
{
  "total": 15,
  "results": [
    {
      "message_id": "12345",
      "unique_id": "channel-12345",
      "channel": "tgsearchers3",
      "datetime": "2023-06-10T14:23:45Z",
      "title": "速度与激情全集1-10",
      "content": "速度与激情系列全集，1080P高清...",
      "links": [
        {
          "type": "baidu",
          "url": "https://pan.baidu.com/s/1abcdef",
          "password": "1234",
          "datetime": "2023-06-10T14:23:45Z",
          "work_title": "速度与激情全集1-10"
        }
      ],
      "tags": ["电影", "合集"],
      "images": ["https://cdn1.cdn-telegram.org/file/xxx.jpg"]
    }
  ],
  "merged_by_type": {
    "baidu": [
      {
        "url": "https://pan.baidu.com/s/1abcdef",
        "password": "1234",
        "note": "速度与激情全集1-10",
        "datetime": "2023-06-10T14:23:45Z",
        "source": "tg:频道名称",
        "images": ["https://cdn1.cdn-telegram.org/file/xxx.jpg"]
      }
    ]
  }
}
```

## 🚨 注意事项

1. **认证**: 官方服务可能需要认证，本地服务默认不需要
2. **限流**: 请合理控制请求频率，避免被限制
3. **错误处理**: 务必处理网络错误和 API 错误响应
4. **Token 管理**: 认证 token 有过期时间，需要定期刷新

## 🎉 总结

现在你可以通过纯 HTTP API 方式使用 PanSou：

1. **直接调用官方 API**: `https://so.252035.xyz`
2. **本地部署服务**: Docker 或源码编译
3. **多种编程语言**: Python、JavaScript、PowerShell 等
4. **Web 前端集成**: HTML + JavaScript
5. **命令行工具**: 自定义脚本

选择最适合你的方式开始使用吧！🚀