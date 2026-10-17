# Run BANDIQ Proof-of-Bandwidth Verification Service
[CmdletBinding()]
param(
    [int]$Port = 8085,
    [double]$Confidence = 0.70,
    [int]$DailyCapGB = 10,
    [int]$MaxDriftMin = 5
)

$ErrorActionPreference = "Stop"

if (-not (Test-Path ".\bandiq-verifier.exe")) {
    Write-Host "[!] Binary bandiq-verifier.exe not found. Building..." -ForegroundColor Yellow
    if (Test-Path "C:\Program Files\Go\bin\go.exe") {
        $env:Path = "C:\Program Files\Go\bin;$env:Path"
    }
    go build -ldflags="-s -w" -o bandiq-verifier.exe ./cmd/bandiq-verifier
}

Write-Host "==========================================================" -ForegroundColor Cyan
Write-Host "     Starting BANDIQ Proof-of-Bandwidth Verifier         " -ForegroundColor Cyan
Write-Host "==========================================================" -ForegroundColor Cyan
Write-Host "Service Port:           $Port" -ForegroundColor Green
Write-Host "Confidence Threshold:   $Confidence" -ForegroundColor Green
Write-Host "Daily Cap:              $DailyCapGB GB / node" -ForegroundColor Green
Write-Host "Max Clock Drift:        ±$MaxDriftMin minutes" -ForegroundColor Green
Write-Host "Verification Endpoint:  http://localhost:$Port/api/v1/reports/verify" -ForegroundColor Yellow
Write-Host "Audit Decisions:        http://localhost:$Port/api/v1/audit/decisions`n" -ForegroundColor Yellow

& ".\bandiq-verifier.exe" -port $Port -confidence $Confidence -daily-cap-gb $DailyCapGB -max-drift-min $MaxDriftMin
