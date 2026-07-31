param([string]$EnvPath = "D:\env\deepwiki.env")

if (-not (Test-Path $EnvPath)) {
    Write-Host "❌ 未找到配置文件: $EnvPath" -ForegroundColor Red
    exit 1
}

Write-Host "✅ 加载配置: $EnvPath" -ForegroundColor Green

Get-Content $EnvPath | ForEach-Object {
    if ($_ -match "^([^#][^=]+)=(.*)$") {
        $name = $matches[1].Trim()
        $value = $matches[2].Trim()
        Set-Item -Path "env:$name" -Value $value
        
        if ($name -like "*PASSWORD*" -or $name -like "*API_KEY*") {
            Write-Host "  $name = ********" -ForegroundColor Gray
        } else {
            Write-Host "  $name = $value" -ForegroundColor Gray
        }
    }
}

Write-Host "`n🚀 启动服务...`n" -ForegroundColor Green
& ".\deepwiki.exe"
