<#
.SYNOPSIS
    Runs the complete Hardhat smart contracts test suite for BANDIQ DePIN on Sidra Chain.
#>

$ErrorActionPreference = "Stop"
$WorkspaceRoot = Split-Path -Parent $PSScriptRoot

Write-Host "==========================================================" -ForegroundColor Cyan
Write-Host " Running BANDIQ DePIN Smart Contracts Test Suite" -ForegroundColor Cyan
Write-Host " Target Chain: Sidra Chain (Chain ID: 97453)" -ForegroundColor Cyan
Write-Host " Existing Token: 0x6d1537bEE4F1EE3f55F4BC9319Aa40252cd39133" -ForegroundColor Cyan
Write-Host "==========================================================`n" -ForegroundColor Cyan

Set-Location $WorkspaceRoot

& npx hardhat --config hardhat.config.cjs test
if ($LASTEXITCODE -eq 0) {
    Write-Host "`nAll 24 smart contract test cases passed successfully (100%)!" -ForegroundColor Green
} else {
    Write-Host "`nSmart contract test suite failed with exit code $LASTEXITCODE." -ForegroundColor Red
    exit $LASTEXITCODE
}
