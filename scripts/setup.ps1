# PowerShell Setup Script for BANDIQ DePIN Bandwidth Node Agent
[CmdletBinding()]
param()

$ErrorActionPreference = "Stop"

Write-Host "==========================================================" -ForegroundColor Cyan
Write-Host "    BANDIQ DePIN Bandwidth Node Agent Setup (Windows)    " -ForegroundColor Cyan
Write-Host "==========================================================" -ForegroundColor Cyan

# 1. Check Go Toolchain
$goCmd = Get-Command "go" -ErrorAction SilentlyContinue
if (-not $goCmd) {
    if (Test-Path "C:\Program Files\Go\bin\go.exe") {
        Write-Host "[+] Found Go in 'C:\Program Files\Go\bin'. Adding to current PATH session..." -ForegroundColor Yellow
        $env:Path = "C:\Program Files\Go\bin;$env:Path"
    } else {
        Write-Host "[-] Go compiler not found! Please install Go 1.22+ or install via 'winget install GoLang.Go'." -ForegroundColor Red
        exit 1
    }
}

$goVer = go version
Write-Host "[+] Go toolchain ready: $goVer" -ForegroundColor Green

# 2. Run Test Suite
Write-Host "`n[+] Running unit tests..." -ForegroundColor Cyan
go test ./...
if ($LASTEXITCODE -ne 0) {
    Write-Host "[-] Unit tests failed!" -ForegroundColor Red
    exit 1
}
Write-Host "[+] All unit tests passed!" -ForegroundColor Green

# 3. Build Binaries
Write-Host "`n[+] Building bandiq.exe (Agent)..." -ForegroundColor Cyan
go build -ldflags="-s -w" -o bandiq.exe ./cmd/bandiq
if ($LASTEXITCODE -ne 0) {
    Write-Host "[-] Failed to build bandiq.exe" -ForegroundColor Red
    exit 1
}

Write-Host "[+] Building bandiq-mock-backend.exe (Verification Backend & Test Server)..." -ForegroundColor Cyan
go build -ldflags="-s -w" -o bandiq-mock-backend.exe ./cmd/bandiq-mock-backend
if ($LASTEXITCODE -ne 0) {
    Write-Host "[-] Failed to build bandiq-mock-backend.exe" -ForegroundColor Red
    exit 1
}

Write-Host "[+] Building bandiq-verifier.exe (Proof-of-Bandwidth Verifier Engine)..." -ForegroundColor Cyan
go build -ldflags="-s -w" -o bandiq-verifier.exe ./cmd/bandiq-verifier
if ($LASTEXITCODE -ne 0) {
    Write-Host "[-] Failed to build bandiq-verifier.exe" -ForegroundColor Red
    exit 1
}

# 4. Initialize Local Directory
if (-not (Test-Path ".bandiq")) {
    New-Item -ItemType Directory -Force -Path ".bandiq" | Out-Null
    Write-Host "[+] Created local .bandiq directory." -ForegroundColor Green
}

Write-Host "`n==========================================================" -ForegroundColor Green
Write-Host "              BANDIQ Setup Complete!                      " -ForegroundColor Green
Write-Host "==========================================================" -ForegroundColor Green
Write-Host "Quick Start Instructions:" -ForegroundColor White
Write-Host " 1. Initialize your node identity:" -ForegroundColor Yellow
Write-Host "    .\bandiq.exe init --wallet 0x6d1537bEE4F1EE3f55F4BC9319Aa40252cd39133" -ForegroundColor Gray
Write-Host " 2. Explicitly grant operator consent:" -ForegroundColor Yellow
Write-Host "    .\bandiq.exe consent grant" -ForegroundColor Gray
Write-Host " 3. Run mock backend (in a separate terminal):" -ForegroundColor Yellow
Write-Host "    .\scripts\run-mock-backend.ps1" -ForegroundColor Gray
Write-Host " 4. Start the node agent daemon:" -ForegroundColor Yellow
Write-Host "    .\scripts\run-node.ps1" -ForegroundColor Gray
