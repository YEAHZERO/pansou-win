param(
    [string]$Root = "."
)

$ErrorActionPreference = "Stop"

$goFiles = Get-ChildItem -Path $Root -Recurse -File -Filter *.go |
    Where-Object { $_.FullName -notmatch "\\.gocache\\" }

$badCommentPattern = "^\s*//.*\?\?"
$replacementChar = [char]0xFFFD

$commentHits = @()
$replacementHits = @()

foreach ($file in $goFiles) {
    $lines = Get-Content -Path $file.FullName -Encoding utf8
    for ($i = 0; $i -lt $lines.Length; $i++) {
        $lineNo = $i + 1
        $line = $lines[$i]

        if ($line -match $badCommentPattern) {
            $commentHits += [PSCustomObject]@{
                File = $file.FullName
                Line = $lineNo
                Type = "CommentQuestionMarks"
                Text = $line.Trim()
            }
        }

        if ($line.Contains($replacementChar)) {
            $replacementHits += [PSCustomObject]@{
                File = $file.FullName
                Line = $lineNo
                Type = "ReplacementChar"
                Text = $line.Trim()
            }
        }
    }
}

Write-Host "Checked $($goFiles.Count) .go files"
Write-Host "Comment '??' hits: $($commentHits.Count)"
Write-Host "Replacement char hits: $($replacementHits.Count)"

if ($commentHits.Count -gt 0) {
    Write-Host "`n[CommentQuestionMarks]"
    $commentHits | ForEach-Object { "{0}:{1} {2}" -f $_.File, $_.Line, $_.Text } | Write-Host
}

if ($replacementHits.Count -gt 0) {
    Write-Host "`n[ReplacementChar]"
    $replacementHits | ForEach-Object { "{0}:{1} {2}" -f $_.File, $_.Line, $_.Text } | Write-Host
}

if ($commentHits.Count -gt 0 -or $replacementHits.Count -gt 0) {
    exit 1
}

exit 0
