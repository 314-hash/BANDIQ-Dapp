<#
.SYNOPSIS
    BANDIQ DePIN 2.0 — Live Sidra Chain Deployment Pipeline
    Deploys compiled smart contracts to the live Sidra Chain (Chain ID: 97453).

.DESCRIPTION
    1. Connects to live Sidra RPC (https://node.sidrachain.com).
    2. Validates Chain ID (97453 / 0x17cad) and checks existing BNDQ token contract.
    3. Ingests deployer private key securely via environment variable or secure prompt.
    4. Checks deployer native SDA balance to ensure sufficient gas.
    5. Deploys BandiqNodeRegistry, BandiqBandwidthVerifier, BandiqUsageSettlement, and BandiqRewardDistributor.
    6. Generates deployments/sidra_live_97453.json and updates dashboard/contracts.js.

.PARAMETER PrivateKey
    Optional private key for the deployer account (0x...).
    Recommended: Leave blank and set $env:PRIVATE_KEY, or let the script prompt you securely.

.PARAMETER RpcUrl
    Sidra Chain RPC endpoint (Default: https://node.sidrachain.com).

.PARAMETER BndqToken
    Existing BNDQ Token Address (Fixed: 0x6d1537bEE4F1EE3f55F4BC9319Aa40252cd39133).

.PARAMETER DryRun
    If specified, runs preflight checks and simulates deployment on hardhat without broadcasting real transactions.
#>

param(
    [string]$PrivateKey = "",
    [string]$RpcUrl = "https://node.sidrachain.com",
    [string]$BndqToken = "0x6d1537bEE4F1EE3f55F4BC9319Aa40252cd39133",
    [switch]$DryRun = $false,
    [switch]$SkipBalanceCheck = $false
)

$ErrorActionPreference = "Stop"
$WorkspaceRoot = Split-Path -Parent $PSScriptRoot

Write-Host "==================================================================" -ForegroundColor Cyan
Write-Host " BANDIQ DePIN -- Live Sidra Chain Deployment Pipeline" -ForegroundColor Cyan
Write-Host " Target Chain: Sidra Chain (Chain ID: 97453 / 0x17cad)" -ForegroundColor Cyan
Write-Host " Target RPC:   $RpcUrl" -ForegroundColor Cyan
Write-Host " BNDQ Token:   $BndqToken (Fixed Deployed Supply)" -ForegroundColor Cyan
Write-Host "==================================================================" -ForegroundColor Cyan
Write-Host ""

Set-Location $WorkspaceRoot

# -------------------------------------------------------------------------
# Preflight Check 1: RPC Connectivity and Chain ID Verification
# -------------------------------------------------------------------------
Write-Host "[1/5] Verifying Sidra Chain RPC connectivity..." -ForegroundColor Yellow
try {
    $chainPayload = '{"jsonrpc":"2.0","method":"eth_chainId","params":[],"id":1}'
    $chainResp = Invoke-RestMethod -Uri $RpcUrl -Method Post -Body $chainPayload -ContentType "application/json" -TimeoutSec 15
    $receivedChainHex = $chainResp.result
    $receivedChainDec = [Convert]::ToInt64($receivedChainHex, 16)
    
    if ($receivedChainDec -ne 97453) {
        Write-Host "[-] ERROR: RPC returned Chain ID $receivedChainDec, expected 97453" -ForegroundColor Red
        Write-Host "[-] Deployment aborted to protect funds." -ForegroundColor Red
        exit 1
    }
    Write-Host "    [OK] Connected! Live Chain ID: $receivedChainDec ($receivedChainHex)" -ForegroundColor Green

    # Query latest block number
    $blockPayload = '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":2}'
    $blockResp = Invoke-RestMethod -Uri $RpcUrl -Method Post -Body $blockPayload -ContentType "application/json" -TimeoutSec 10
    $latestBlockHex = $blockResp.result
    $latestBlockDec = [Convert]::ToInt64($latestBlockHex, 16)
    Write-Host "    [OK] Latest Sidra Block: $latestBlockDec ($latestBlockHex)" -ForegroundColor Green
} catch {
    Write-Host "[-] ERROR: Failed to connect to Sidra RPC at $RpcUrl." -ForegroundColor Red
    Write-Host "    Details: " + $_ -ForegroundColor Red
    exit 1
}

# -------------------------------------------------------------------------
# Preflight Check 2: Verify Existing BNDQ Token Contract on Sidra Chain
# -------------------------------------------------------------------------
Write-Host ""
Write-Host "[2/5] Inspecting deployed BNDQ token on Sidra Chain..." -ForegroundColor Yellow
try {
    $codePayload = '{"jsonrpc":"2.0","method":"eth_getCode","params":["' + $BndqToken + '","latest"],"id":3}'
    $codeResp = Invoke-RestMethod -Uri $RpcUrl -Method Post -Body $codePayload -ContentType "application/json" -TimeoutSec 15
    if (-not $codeResp.result -or $codeResp.result -eq "0x") {
        Write-Host "[-] ERROR: No contract bytecode found at $BndqToken on Sidra Chain!" -ForegroundColor Red
        exit 1
    }
    $byteCount = ($codeResp.result.Length - 2) / 2
    Write-Host "    [OK] Verified deployed token at $BndqToken [$byteCount bytes bytecode]" -ForegroundColor Green
} catch {
    Write-Host "[-] Warning: Unable to query token bytecode directly." -ForegroundColor Yellow
}

# -------------------------------------------------------------------------
# Preflight Check 3: Secure Deployer Private Key Ingestion
# -------------------------------------------------------------------------
Write-Host ""
Write-Host "[3/5] Ingesting deployer credentials securely..." -ForegroundColor Yellow

$effectiveKey = ""
if ($PrivateKey -ne "") {
    $effectiveKey = $PrivateKey
    Write-Host "    [OK] Private key received via script parameter." -ForegroundColor Green
} elseif ($env:PRIVATE_KEY -and $env:PRIVATE_KEY.Trim() -ne "") {
    $effectiveKey = $env:PRIVATE_KEY.Trim()
    Write-Host "    [OK] Private key detected from `$env:PRIVATE_KEY environment variable." -ForegroundColor Green
} elseif (Test-Path "$WorkspaceRoot\.env") {
    $envLines = Get-Content "$WorkspaceRoot\.env"
    foreach ($line in $envLines) {
        if ($line -match "^PRIVATE_KEY\s*=\s*(.+)$") {
            $effectiveKey = $matches[1].Trim().Trim('"').Trim("'")
            Write-Host "    [OK] Private key extracted securely from local .env file." -ForegroundColor Green
            break
        }
    }
}

if (-not $effectiveKey -and -not $DryRun) {
    Write-Host ""
    Write-Host "[?] Deployer Private Key is required for live Sidra deployment." -ForegroundColor Cyan
    Write-Host "    (Input is hidden and will not be saved to disk or shell history)" -ForegroundColor Gray
    $secureInput = Read-Host -Prompt "Enter Deployer Private Key (0x...)" -AsSecureString
    $bstr = [System.Runtime.InteropServices.Marshal]::SecureStringToBSTR($secureInput)
    $effectiveKey = [System.Runtime.InteropServices.Marshal]::PtrToStringAuto($bstr)
    [System.Runtime.InteropServices.Marshal]::ZeroFreeBSTR($bstr)
}

if ($effectiveKey -ne "" -and -not $effectiveKey.StartsWith("0x")) {
    $effectiveKey = "0x" + $effectiveKey
}

if ($effectiveKey -ne "") {
    $env:PRIVATE_KEY = $effectiveKey
}

# -------------------------------------------------------------------------
# Preflight Check 4: Check Gas Balance and Address
# -------------------------------------------------------------------------
Write-Host ""
Write-Host "[4/5] Checking deployer account and gas balance..." -ForegroundColor Yellow

if ($DryRun) {
    Write-Host "    [DRY-RUN MODE ENABLED] Skipping live gas deduction." -ForegroundColor Magenta
    Write-Host "    Simulating deployment against hardhat network (Chain ID: 97453)..." -ForegroundColor Magenta
} else {
    if (-not $env:PRIVATE_KEY -or $env:PRIVATE_KEY.Length -lt 64) {
        Write-Host "[-] ERROR: Invalid or missing private key. Aborting live deployment." -ForegroundColor Red
        exit 1
    }

    try {
        $deployerAddress = (& node -e "const { ethers } = require('ethers'); const w = new ethers.Wallet(process.env.PRIVATE_KEY); console.log(w.address);").Trim()
        Write-Host "    [OK] Deployer Address: $deployerAddress" -ForegroundColor Green
        Write-Host "         Explorer: https://explorer.sidrachain.com/address/$deployerAddress" -ForegroundColor DarkCyan

        # Check native SDA balance
        $balPayload = '{"jsonrpc":"2.0","method":"eth_getBalance","params":["' + $deployerAddress + '","latest"],"id":4}'
        $balResp = Invoke-RestMethod -Uri $RpcUrl -Method Post -Body $balPayload -ContentType "application/json" -TimeoutSec 10
        $balWeiHex = $balResp.result
        $balWei = [Convert]::ToInt64($balWeiHex, 16)
        $balSda = [math]::Round($balWei / 1e18, 4)
        Write-Host "    [OK] Deployer SDA Gas Balance: $balSda SDA [$balWeiHex wei]" -ForegroundColor Green

        if ($balWei -eq 0 -and -not $SkipBalanceCheck) {
            Write-Host ""
            Write-Host "[!] WARNING: Deployer balance is 0 SDA!" -ForegroundColor Red
            Write-Host "    To deploy contracts to Sidra Chain, your account needs native SDA for gas fees." -ForegroundColor Yellow
            Write-Host "    Fund your wallet address: $deployerAddress" -ForegroundColor Yellow
            $choice = Read-Host "Do you want to proceed anyway? (y/N)"
            if ($choice.ToLower() -ne "y") {
                Write-Host "Deployment cancelled. Fund your account with SDA and try again." -ForegroundColor Red
                exit 1
            }
        }
    } catch {
        Write-Host "    [!] Notice: Could not query exact balance. Proceeding..." -ForegroundColor Gray
    }
}

# -------------------------------------------------------------------------
# Execution Step 5: Execute Smart Contract Deployment
# -------------------------------------------------------------------------
Write-Host ""
Write-Host "[5/5] Deploying smart contracts suite..." -ForegroundColor Yellow

$env:SIDRA_RPC_URL = $RpcUrl
$env:BNDQ_TOKEN_ADDRESS = $BndqToken
$env:LIVE_DEPLOY = "true"
$env:UPDATE_DASHBOARD = "true"

$targetNetwork = if ($DryRun) { "hardhat" } else { "sidraLive" }
Write-Host "    Running Hardhat deployment on network: $targetNetwork" -ForegroundColor DarkCyan
Write-Host ""

& npx hardhat --config hardhat.config.cjs run scripts/deploy-contracts.cjs --network $targetNetwork

if ($LASTEXITCODE -eq 0) {
    Write-Host ""
    Write-Host "==================================================================" -ForegroundColor Green
    Write-Host " BANDIQ DEPIN SMART CONTRACTS DEPLOYMENT SUCCESSFUL!" -ForegroundColor Green
    Write-Host "==================================================================" -ForegroundColor Green

    $deploymentRecord = "$WorkspaceRoot\deployments\sidra_live_97453.json"
    if (Test-Path $deploymentRecord) {
        $json = Get-Content $deploymentRecord -Raw | ConvertFrom-Json
        Write-Host ""
        Write-Host "--- Deployed Contracts on Sidra Chain ---" -ForegroundColor White
        Write-Host "  BandiqNodeRegistry:      $($json.contracts.BandiqNodeRegistry)" -ForegroundColor White
        Write-Host "  BandiqBandwidthVerifier: $($json.contracts.BandiqBandwidthVerifier)" -ForegroundColor White
        Write-Host "  BandiqUsageSettlement:   $($json.contracts.BandiqUsageSettlement)" -ForegroundColor White
        Write-Host "  BandiqRewardDistributor: $($json.contracts.BandiqRewardDistributor)" -ForegroundColor White
        Write-Host ""
        Write-Host "  Dashboard Contracts Lib: Updated (dashboard/contracts.js)" -ForegroundColor Green
    }

    Write-Host ""
    Write-Host "Next Recommended Steps:" -ForegroundColor Yellow
    Write-Host " 1. Verify deployed addresses on Sidra Explorer: https://explorer.sidrachain.com" -ForegroundColor Gray
    Write-Host " 2. Run dashboard to interact with live contracts: .\scripts\run-dashboard.ps1" -ForegroundColor Gray
    Write-Host " 3. Test voucher redemption and bandwidth escrow on live Sidra Chain." -ForegroundColor Gray
} else {
    Write-Host ""
    Write-Host "[-] Deployment failed with exit code $LASTEXITCODE." -ForegroundColor Red
    exit $LASTEXITCODE
}
