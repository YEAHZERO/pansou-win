# PanSou API PowerShell 客户端
# 支持官方服务和本地服务

param(
    [Parameter(Mandatory=$true)]
    [string]$Keyword,
    
    [string]$ApiUrl = "https://so.252035.xyz",
    [string]$Username,
    [string]$Password,
    [string[]]$CloudTypes,
    [string[]]$Plugins,
    [string]$Source = "all",
    [int]$Limit = 10,
    [switch]$Refresh,
    [switch]$Health,
    [switch]$Verbose
)

# 设置 TLS 版本
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12

class PanSouAPI {
    [string]$BaseUrl
    [string]$Token
    [hashtable]$Headers
    
    PanSouAPI([string]$baseUrl) {
        $this.BaseUrl = $baseUrl.TrimEnd('/')
        $this.Headers = @{
            'Content-Type' = 'application/json'
            'User-Agent' = 'PanSou-PowerShell-Client/1.0.0'
        }
    }
    
    [void]Login([string]$username, [string]$password) {
        $loginData = @{
            username = $username
            password = $password
        } | ConvertTo-Json -Depth 10
        
        try {
            Write-Host "🔐 正在登录..." -ForegroundColor Yellow
            $response = Invoke-RestMethod -Uri "$($this.BaseUrl)/api/auth/login" -Method Post -Body $loginData -Headers $this.Headers
            $this.Token = $response.token
            $this.Headers['Authorization'] = "Bearer $($this.Token)"
            Write-Host "✅ 登录成功，用户: $($response.username)" -ForegroundColor Green
        }
        catch {
            $errorMsg = "登录失败"
            if ($_.Exception.Response) {
                $statusCode = $_.Exception.Response.StatusCode.value__
                $errorMsg += " (状态码: $statusCode)"
            }
            throw $errorMsg
        }
    }
    
    [object]HealthCheck() {
        try {
            return Invoke-RestMethod -Uri "$($this.BaseUrl)/api/health" -Method Get -Headers $this.Headers
        }
        catch {
            $errorMsg = "健康检查失败"
            if ($_.Exception.Response) {
                $statusCode = $_.Exception.Response.StatusCode.value__
                $errorMsg += " (状态码: $statusCode)"
            }
            throw $errorMsg
        }
    }
    
    [object]Search([string]$keyword, [hashtable]$options = @{}) {
        $searchData = @{ kw = $keyword; res = "merge" }
        $options.GetEnumerator() | ForEach-Object { $searchData[$_.Key] = $_.Value }
        $jsonData = $searchData | ConvertTo-Json -Depth 10
        
        try {
            return Invoke-RestMethod -Uri "$($this.BaseUrl)/api/search" -Method Post -Body $jsonData -Headers $this.Headers
        }
        catch {
            $errorMsg = "搜索失败"
            if ($_.Exception.Response) {
                $statusCode = $_.Exception.Response.StatusCode.value__
                if ($statusCode -eq 401) {
                    $errorMsg = "认证失败，请检查用户名密码"
                } else {
                    $errorMsg += " (状态码: $statusCode)"
                }
            }
            throw $errorMsg
        }
    }
}

function Format-Results {
    param(
        [object]$Results,
        [int]$Limit = 10
    )
    
    if (-not $Results.merged_by_type -or $Results.merged_by_type.PSObject.Properties.Count -eq 0) {
        Write-Host "❌ 未找到相关资源" -ForegroundColor Red
        return
    }
    
    # 计算总链接数
    $totalLinks = 0
    $Results.merged_by_type.PSObject.Properties | ForEach-Object {
        $totalLinks += $_.Value.Count
    }
    
    Write-Host ""
    Write-Host "🎉 找到 $totalLinks 个资源链接" -ForegroundColor Green
    Write-Host ("=" * 60) -ForegroundColor Gray
    
    # 网盘类型名称映射
    $cloudNames = @{
        'baidu' = '百度网盘'
        'aliyun' = '阿里云盘'
        'quark' = '夸克网盘'
        'tianyi' = '天翼云盘'
        'uc' = 'UC网盘'
        'mobile' = '移动云盘'
        '115' = '115网盘'
        'pikpak' = 'PikPak'
        'xunlei' = '迅雷网盘'
        '123' = '123网盘'
        'magnet' = '磁力链接'
        'ed2k' = '电驴链接'
        'others' = '其他网盘'
    }
    
    $Results.merged_by_type.PSObject.Properties | ForEach-Object {
        $cloudType = $_.Name
        $links = $_.Value
        $cloudName = if ($cloudNames.ContainsKey($cloudType)) { $cloudNames[$cloudType] } else { $cloudType }
        
        Write-Host ""
        Write-Host "📁 $cloudName ($($links.Count) 个链接)" -ForegroundColor Yellow
        Write-Host ("-" * 40) -ForegroundColor Gray
        
        $displayLinks = $links | Select-Object -First $Limit
        for ($i = 0; $i -lt $displayLinks.Count; $i++) {
            $link = $displayLinks[$i]
            $index = "{0,2}" -f ($i + 1)
            
            Write-Host "$index. $($link.note)" -ForegroundColor White
            Write-Host "    🔗 $($link.url)" -ForegroundColor Cyan
            
            if ($link.password) {
                Write-Host "    🔑 提取码: $($link.password)" -ForegroundColor Magenta
            }
            
            if ($link.source) {
                Write-Host "    📍 来源: $($link.source)" -ForegroundColor DarkGray
            }
            
            if ($link.datetime) {
                Write-Host "    📅 时间: $($link.datetime)" -ForegroundColor DarkGray
            }
            
            Write-Host ""
        }
        
        if ($links.Count -gt $Limit) {
            Write-Host "    ... 还有 $($links.Count - $Limit) 个链接" -ForegroundColor DarkGray
            Write-Host ""
        }
    }
}

function Show-Help {
    Write-Host @"
PanSou API PowerShell 客户端

用法:
    .\powershell_client.ps1 -Keyword "搜索关键词" [选项]

参数:
    -Keyword        搜索关键词 (必填)
    -ApiUrl         API 服务地址 (默认: https://so.252035.xyz)
    -Username       用户名 (如果需要认证)
    -Password       密码 (如果需要认证)
    -CloudTypes     网盘类型数组 (如: @("baidu","aliyun"))
    -Plugins        插件数组 (如: @("labi","zhizhen"))
    -Source         数据源 (all/tg/plugin, 默认: all)
    -Limit          每种网盘显示的链接数量 (默认: 10)
    -Refresh        强制刷新缓存
    -Health         仅执行健康检查
    -Verbose        显示详细信息

示例:
    .\powershell_client.ps1 -Keyword "速度与激情"
    .\powershell_client.ps1 -Keyword "Python教程" -ApiUrl "http://localhost:8888"
    .\powershell_client.ps1 -Keyword "电影" -Username "admin" -Password "123456"
    .\powershell_client.ps1 -Keyword "资源" -CloudTypes @("baidu","aliyun") -Limit 5
    .\powershell_client.ps1 -Health
"@
}

# 主程序
try {
    # 显示帮助
    if ($Keyword -eq "help" -or $Keyword -eq "-help" -or $Keyword -eq "--help") {
        Show-Help
        exit 0
    }
    
    # 创建 API 客户端
    if ($Verbose) {
        Write-Host "🔗 连接到: $ApiUrl" -ForegroundColor Cyan
    }
    
    $api = [PanSouAPI]::new($ApiUrl)
    
    # 登录认证
    if ($Username -and $Password) {
        $api.Login($Username, $Password)
    }
    
    # 健康检查
    if ($Health) {
        Write-Host "🏥 执行健康检查..." -ForegroundColor Yellow
        $healthResult = $api.HealthCheck()
        
        Write-Host ""
        Write-Host "服务健康状态:" -ForegroundColor Green
        Write-Host "  状态: $($healthResult.status)" -ForegroundColor White
        Write-Host "  认证: $(if ($healthResult.auth_enabled) { '启用' } else { '禁用' })" -ForegroundColor White
        Write-Host "  插件: $(if ($healthResult.plugins_enabled) { '启用' } else { '禁用' })" -ForegroundColor White
        Write-Host "  插件数量: $($healthResult.plugin_count)" -ForegroundColor White
        Write-Host "  频道数量: $($healthResult.channels_count)" -ForegroundColor White
        
        if ($healthResult.plugins -and $healthResult.plugins.Count -gt 0) {
            Write-Host "  可用插件: $($healthResult.plugins -join ', ')" -ForegroundColor DarkGray
        }
        
        exit 0
    }
    
    # 准备搜索参数
    $searchOptions = @{
        src = $Source
        refresh = $Refresh.IsPresent
    }
    
    if ($CloudTypes -and $CloudTypes.Count -gt 0) {
        $searchOptions['cloud_types'] = $CloudTypes
    }
    
    if ($Plugins -and $Plugins.Count -gt 0) {
        $searchOptions['plugins'] = $Plugins
    }
    
    # 执行搜索
    if ($Verbose) {
        Write-Host "🔍 搜索关键词: $Keyword" -ForegroundColor Cyan
        Write-Host "📊 搜索参数:" -ForegroundColor Cyan
        $searchOptions.GetEnumerator() | ForEach-Object {
            Write-Host "  $($_.Key): $($_.Value)" -ForegroundColor DarkGray
        }
        Write-Host ""
    }
    
    Write-Host "🔍 正在搜索..." -ForegroundColor Yellow
    $results = $api.Search($Keyword, $searchOptions)
    
    # 显示结果
    Format-Results -Results $results -Limit $Limit
    
} catch {
    Write-Host "❌ 错误: $($_.Exception.Message)" -ForegroundColor Red
    
    if ($Verbose) {
        Write-Host ""
        Write-Host "详细错误信息:" -ForegroundColor DarkRed
        Write-Host $_.Exception.ToString() -ForegroundColor DarkRed
    }
    
    exit 1
}