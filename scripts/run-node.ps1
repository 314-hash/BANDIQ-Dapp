# Run BANDIQ DePIN Bandwidth Node Agent Daemon
[CmdletBinding()]
param(
    [string]$ConfigPath = "",
    [string]$DataDir = ".bandiq"
)

$ErrorActionPreference = "Stop"

if (-not (Test-Path ".\bandiq.exe")) {
    Write-Host "[!] Binary bandiq.exe not found. Running setup..." -ForegroundColor Yellow
    & ".\scripts\setup.ps1"
}

# Ensure PATH has Go if needed
if (Test-Path "C:\Program Files\Go\bin\go.exe") {
    $env:Path = "C:\Program Files\Go\bin;$env:Path"
}

# Check if node has been initialized
$keyPath = Join-Path $DataDir "node_key.json"
if (-not (Test-Path $keyPath)) {
    Write-Host "[!] Node key not found at '$keyPath'. Running initialization..." -ForegroundColor Yellow
    & ".\bandiq.exe" init --wallet "0x6d1537bEE4F1EE3f55F4BC9319Aa40252cd39133" --data-dir $DataDir
    & ".\bandiq.exe" consent grant
}

$cmdArgs = @("start")
if ($ConfigPath -ne "") {
    $cmdArgs += @("--config", $ConfigPath)
}
if ($DataDir -ne "") {
    $cmdArgs += @("--data-dir", $DataDir)
}

Write-Host "Launching BANDIQ Node Agent..." -ForegroundColor Cyan
& ".\bandiq.exe" @cmdArgs
