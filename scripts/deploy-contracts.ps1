<#
.SYNOPSIS
    Deploys the BANDIQ DePIN Smart Contracts Suite to Sidra Chain or Local Hardhat network.
#>

param(
    [string]$Network = "hardhat",
    [string]$RpcUrl = "http://127.0.0.1:8080/rpc",
    [string]$BndqToken = "0x6d1537bEE4F1EE3f55F4BC9319Aa40252cd39133",
    [string]$PrivateKey = ""
)

$ErrorActionPreference = "Stop"
$WorkspaceRoot = Split-Path -Parent $PSScriptRoot

Write-Host "==========================================================" -ForegroundColor Cyan
Write-Host " Deploying BANDIQ DePIN Smart Contracts Suite" -ForegroundColor Cyan
Write-Host " Network: $Network" -ForegroundColor Cyan
Write-Host " BNDQ Token: $BndqToken" -ForegroundColor Cyan
Write-Host "==========================================================`n" -ForegroundColor Cyan

Set-Location $WorkspaceRoot

$env:SIDRA_RPC_URL = $RpcUrl
$env:BNDQ_TOKEN_ADDRESS = $BndqToken
if ($PrivateKey -ne "") {
    $env:PRIVATE_KEY = $PrivateKey
}

& npx hardhat --config hardhat.config.cjs run scripts/deploy-contracts.cjs --network $Network
if ($LASTEXITCODE -eq 0) {
    Write-Host "`nDeployment successfully finished!" -ForegroundColor Green
} else {
    Write-Host "`nDeployment failed with exit code $LASTEXITCODE." -ForegroundColor Red
    exit $LASTEXITCODE
}
