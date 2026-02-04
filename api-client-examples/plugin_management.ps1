# 插件管理示例脚本 (PowerShell)
# 用于管理插件优先级和查看统计信息

$baseUrl = "http://localhost:8888/api"

# 颜色输出函数
function Write-ColorOutput($ForegroundColor) {
    $fc = $host.UI.RawUI.ForegroundColor
    $host.UI.RawUI.ForegroundColor = $ForegroundColor
    if ($args) {
        Write-Output $args
    }
    $host.UI.RawUI.ForegroundColor = $fc
}

# 1. 获取所有插件信息
function Get-AllPlugins {
    Write-ColorOutput Yellow "`n========== 所有插件信息 =========="
    $response = Invoke-RestMethod -Uri "$baseUrl/plugins" -Method Get
    $response.data | Format-Table -AutoSize
}

# 2. 获取插件统计信息
function Get-PluginStats {
    Write-ColorOutput Yellow "`n========== 插件统计信息 =========="
    $response = Invoke-RestMethod -Uri "$baseUrl/plugins/stats" -Method Get
    
    $stats = @()
    foreach ($key in $response.data.PSObject.Properties.Name) {
        $stat = $response.data.$key
        $stats += [PSCustomObject]@{
            插件名称 = $stat.plugin_name
            搜索次数 = $stat.total_searches
            成功次数 = $stat.success_searches
            总结果数 = $stat.total_results
            平均结果 = [math]::Round($stat.avg_results, 1)
            平均响应ms = [math]::Round($stat.avg_response_time, 1)
            自定义优先级 = if ($stat.custom_priority -eq 0) { "-" } else { $stat.custom_priority }
        }
    }
    
    $stats | Sort-Object -Property 平均结果 -Descending | Format-Table -AutoSize
}

# 3. 获取单个插件详细信息
function Get-PluginDetail {
    param([string]$pluginName)
    
    Write-ColorOutput Yellow "`n========== $pluginName 详细信息 =========="
    try {
        $response = Invoke-RestMethod -Uri "$baseUrl/plugins/stats/$pluginName" -Method Get
        $stat = $response.data
        
        Write-Output "插件名称: $($stat.plugin_name)"
        Write-Output "总搜索次数: $($stat.total_searches)"
        Write-Output "成功次数: $($stat.success_searches)"
        Write-Output "失败次数: $($stat.failed_searches)"
        Write-Output "总结果数: $($stat.total_results)"
        Write-Output "平均结果数: $([math]::Round($stat.avg_results, 2))"
        Write-Output "平均响应时间: $([math]::Round($stat.avg_response_time, 2))ms"
        Write-Output "最后搜索时间: $($stat.last_search_time)"
        Write-Output "自定义优先级: $(if ($stat.custom_priority -eq 0) { '默认' } else { $stat.custom_priority })"
    }
    catch {
        Write-ColorOutput Red "错误: $_"
    }
}

# 4. 设置插件优先级
function Set-PluginPriority {
    param(
        [string]$pluginName,
        [int]$priority
    )
    
    Write-ColorOutput Yellow "`n设置 $pluginName 优先级为 $priority"
    
    $body = @{
        plugin_name = $pluginName
        priority = $priority
    } | ConvertTo-Json
    
    try {
        $response = Invoke-RestMethod -Uri "$baseUrl/plugins/priority" -Method Post -Body $body -ContentType "application/json"
        Write-ColorOutput Green "✓ $($response.message)"
    }
    catch {
        Write-ColorOutput Red "✗ 设置失败: $_"
    }
}

# 5. 批量设置插件优先级
function Set-BatchPluginPriority {
    param([hashtable]$priorities)
    
    Write-ColorOutput Yellow "`n批量设置插件优先级"
    
    $body = @{
        priorities = $priorities
    } | ConvertTo-Json
    
    try {
        $response = Invoke-RestMethod -Uri "$baseUrl/plugins/priority/batch" -Method Post -Body $body -ContentType "application/json"
        Write-ColorOutput Green "✓ 成功: $($response.data.success_count) 个"
        if ($response.data.failed_count -gt 0) {
            Write-ColorOutput Red "✗ 失败: $($response.data.failed_count) 个"
            $response.data.failed_plugins | ForEach-Object { Write-Output "  - $_" }
        }
    }
    catch {
        Write-ColorOutput Red "✗ 批量设置失败: $_"
    }
}

# 6. 重置插件优先级
function Reset-PluginPriority {
    param([string]$pluginName)
    
    Write-ColorOutput Yellow "`n重置 $pluginName 优先级"
    
    try {
        $response = Invoke-RestMethod -Uri "$baseUrl/plugins/priority/$pluginName" -Method Delete
        Write-ColorOutput Green "✓ $($response.message)"
    }
    catch {
        Write-ColorOutput Red "✗ 重置失败: $_"
    }
}

# 7. 导出统计数据
function Export-PluginStats {
    param(
        [string]$format = "json",
        [string]$outputFile
    )
    
    Write-ColorOutput Yellow "`n导出统计数据 (格式: $format)"
    
    try {
        $url = "$baseUrl/plugins/stats/export?format=$format"
        
        if ($format -eq "csv") {
            Invoke-WebRequest -Uri $url -OutFile $outputFile
        }
        else {
            $response = Invoke-RestMethod -Uri $url -Method Get
            $response | ConvertTo-Json -Depth 10 | Out-File $outputFile
        }
        
        Write-ColorOutput Green "✓ 已导出到: $outputFile"
    }
    catch {
        Write-ColorOutput Red "✗ 导出失败: $_"
    }
}

# 8. 推荐优先级设置（基于统计数据）
function Get-RecommendedPriorities {
    Write-ColorOutput Yellow "`n========== 推荐优先级设置 =========="
    
    $response = Invoke-RestMethod -Uri "$baseUrl/plugins/stats" -Method Get
    
    $stats = @()
    foreach ($key in $response.data.PSObject.Properties.Name) {
        $stat = $response.data.$key
        
        # 计算综合得分
        $avgResults = $stat.avg_results
        $avgTime = $stat.avg_response_time
        $successRate = if ($stat.total_searches -gt 0) { 
            $stat.success_searches / $stat.total_searches * 100 
        } else { 0 }
        
        # 得分计算：平均结果数 * 成功率 / 响应时间
        $score = if ($avgTime -gt 0) { 
            ($avgResults * $successRate) / $avgTime 
        } else { 0 }
        
        $stats += [PSCustomObject]@{
            插件名称 = $stat.plugin_name
            综合得分 = [math]::Round($score, 2)
            平均结果 = [math]::Round($avgResults, 1)
            成功率 = [math]::Round($successRate, 1)
            响应时间 = [math]::Round($avgTime, 1)
            推荐优先级 = ""
        }
    }
    
    # 排序并分配推荐优先级
    $sorted = $stats | Sort-Object -Property 综合得分 -Descending
    
    $tier1Count = [math]::Min(5, $sorted.Count)
    $tier2Count = [math]::Min(10, $sorted.Count - $tier1Count)
    
    for ($i = 0; $i -lt $sorted.Count; $i++) {
        if ($i -lt $tier1Count) {
            $sorted[$i].推荐优先级 = "1 (第一梯队)"
        }
        elseif ($i -lt ($tier1Count + $tier2Count)) {
            $sorted[$i].推荐优先级 = "2 (第二梯队)"
        }
        else {
            $sorted[$i].推荐优先级 = "3 (第三梯队)"
        }
    }
    
    $sorted | Format-Table -AutoSize
    
    Write-ColorOutput Cyan "`n提示: 使用 Apply-RecommendedPriorities 应用推荐设置"
}

# 9. 应用推荐的优先级设置
function Apply-RecommendedPriorities {
    Write-ColorOutput Yellow "`n应用推荐的优先级设置"
    
    $response = Invoke-RestMethod -Uri "$baseUrl/plugins/stats" -Method Get
    
    $stats = @()
    foreach ($key in $response.data.PSObject.Properties.Name) {
        $stat = $response.data.$key
        
        $avgResults = $stat.avg_results
        $avgTime = $stat.avg_response_time
        $successRate = if ($stat.total_searches -gt 0) { 
            $stat.success_searches / $stat.total_searches * 100 
        } else { 0 }
        
        $score = if ($avgTime -gt 0) { 
            ($avgResults * $successRate) / $avgTime 
        } else { 0 }
        
        $stats += @{
            name = $stat.plugin_name
            score = $score
        }
    }
    
    # 排序
    $sorted = $stats | Sort-Object -Property score -Descending
    
    # 构建优先级映射
    $priorities = @{}
    $tier1Count = [math]::Min(5, $sorted.Count)
    $tier2Count = [math]::Min(10, $sorted.Count - $tier1Count)
    
    for ($i = 0; $i -lt $sorted.Count; $i++) {
        if ($i -lt $tier1Count) {
            $priorities[$sorted[$i].name] = 1
        }
        elseif ($i -lt ($tier1Count + $tier2Count)) {
            $priorities[$sorted[$i].name] = 2
        }
        else {
            $priorities[$sorted[$i].name] = 3
        }
    }
    
    # 批量设置
    Set-BatchPluginPriority -priorities $priorities
}

# 主菜单
function Show-Menu {
    Write-ColorOutput Cyan @"

========================================
      插件管理工具
========================================
1. 查看所有插件信息
2. 查看插件统计信息
3. 查看单个插件详情
4. 设置插件优先级
5. 批量设置优先级
6. 重置插件优先级
7. 导出统计数据
8. 查看推荐优先级
9. 应用推荐优先级
0. 退出
========================================
"@
}

# 交互式主程序
function Start-Interactive {
    while ($true) {
        Show-Menu
        $choice = Read-Host "请选择操作"
        
        switch ($choice) {
            "1" { Get-AllPlugins }
            "2" { Get-PluginStats }
            "3" {
                $name = Read-Host "请输入插件名称"
                Get-PluginDetail -pluginName $name
            }
            "4" {
                $name = Read-Host "请输入插件名称"
                $priority = Read-Host "请输入优先级 (1-10)"
                Set-PluginPriority -pluginName $name -priority ([int]$priority)
            }
            "5" {
                Write-Output "示例: pioz=1,gying=1,hdr4k=2"
                $input = Read-Host "请输入优先级设置"
                $priorities = @{}
                $input.Split(',') | ForEach-Object {
                    $parts = $_.Split('=')
                    if ($parts.Count -eq 2) {
                        $priorities[$parts[0].Trim()] = [int]$parts[1].Trim()
                    }
                }
                Set-BatchPluginPriority -priorities $priorities
            }
            "6" {
                $name = Read-Host "请输入插件名称"
                Reset-PluginPriority -pluginName $name
            }
            "7" {
                $format = Read-Host "请输入格式 (json/csv)"
                $file = Read-Host "请输入输出文件名"
                Export-PluginStats -format $format -outputFile $file
            }
            "8" { Get-RecommendedPriorities }
            "9" { Apply-RecommendedPriorities }
            "0" { 
                Write-ColorOutput Green "再见!"
                return 
            }
            default { Write-ColorOutput Red "无效选择" }
        }
        
        Read-Host "`n按回车继续"
    }
}

# 如果直接运行脚本，启动交互模式
if ($MyInvocation.InvocationName -ne '.') {
    Start-Interactive
}
