# Run BANDIQ Mock Verification Backend & Speedtest Server
[CmdletBinding()]
param(
    [int]$Port = 8080
)

$ErrorActionPreference = "Stop"

if (-not (Test-Path ".\bandiq-mock-backend.exe")) {
    Write-Host "[!] Binary bandiq-mock-backend.exe not found. Running setup..." -ForegroundColor Yellow
    & ".\scripts\setup.ps1"
}

Write-Host "Starting BANDIQ Mock Backend on port $Port..." -ForegroundColor Cyan
Write-Host "Endpoints:" -ForegroundColor White
Write-Host "  - Speedtest Ping:      http://127.0.0.1:$Port/api/v1/ping"
Write-Host "  - Speedtest Download:  http://127.0.0.1:$Port/api/v1/speedtest/download?size=5"
Write-Host "  - Speedtest Upload:    http://127.0.0.1:$Port/api/v1/speedtest/upload"
Write-Host "  - Node Registration:   http://127.0.0.1:$Port/api/v1/nodes/register"
Write-Host "  - Report Submission:   http://127.0.0.1:$Port/api/v1/reports/submit"
Write-Host "  - Inspect Reports:     http://127.0.0.1:$Port/api/v1/reports"
Write-Host "  - Sidra Mock RPC:      http://127.0.0.1:$Port/rpc (Chain ID: 97453)`n"

& ".\bandiq-mock-backend.exe" -port $Port
