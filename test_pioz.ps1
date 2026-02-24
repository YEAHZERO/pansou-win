# Test script for pioz.go
$keywords = @("球神归来", "缅北黑玫瑰", "神豪")
$baseUrl = "http://localhost:8889/api/search"

$allResults = @()

foreach ($keyword in $keywords) {
    Write-Host "Testing keyword: $keyword"
    try {
        $url = "$baseUrl?q=$keyword"
        
        $response = Invoke-RestMethod -Uri $url -Method Get -TimeoutSec 30
        
        Write-Host "Found $($response.results.Count) results for: $keyword"
        
        foreach ($result in $response.results) {
            $allResults += @{
                keyword = $keyword
                title = $result.title
                content = $result.content
                links = $result.links
                uniqueId = $result.uniqueId
            }
        }
        
        Start-Sleep -Seconds 2
    }
    catch {
        Write-Host "Error for keyword '$keyword': $_"
    }
}

# Save all results to JSON
$outputFile = "pioz_all_results.json"
$allResults | ConvertTo-Json -Depth 10 | Out-File -FilePath $outputFile -Encoding UTF8
Write-Host "All results saved to: $outputFile"
Write-Host "Total results: $($allResults.Count)"