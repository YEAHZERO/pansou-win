# Test Pioz plugin search functionality
$keywords = @("星辰未眠", "太奶奶")

foreach ($keyword in $keywords) {
    Write-Host "Testing keyword: $keyword"
    Write-Host "------------------------"
    
    try {
        $body = @{"keyword" = $keyword; "page" = 1} | ConvertTo-Json
        $response = Invoke-WebRequest -Uri "http://localhost:8889/api/search" -Method POST -Headers @{"Content-Type"="application/json"} -Body $body
        $content = $response.Content
        Write-Host "Search result:"
        Write-Host $content
        Write-Host ""
    } catch {
        Write-Host "Error: $($_.Exception.Message)"
        Write-Host ""
    }
}
