# Launch BANDIQ Web Dashboard
[CmdletBinding()]
param(
    [int]$Port = 8080,
    [switch]$NoBrowser
)

$ErrorActionPreference = "Stop"

if (-not (Test-Path ".\bandiq-mock-backend.exe")) {
    Write-Host "[!] Binary bandiq-mock-backend.exe not found. Running setup..." -ForegroundColor Yellow
    & ".\scripts\setup.ps1"
}

$url = "http://localhost:$Port/dashboard/"
Write-Host "==========================================================" -ForegroundColor Cyan
Write-Host "         Launching BANDIQ DePIN Web Dashboard             " -ForegroundColor Cyan
Write-Host "==========================================================" -ForegroundColor Cyan
Write-Host "Dashboard URL: $url" -ForegroundColor Green
Write-Host "Mobile Access: http://<YOUR-LOCAL-IP>:$Port/dashboard/`n" -ForegroundColor Yellow

if (-not $NoBrowser) {
    Start-Process $url
}

& ".\bandiq-mock-backend.exe" -port $Port
