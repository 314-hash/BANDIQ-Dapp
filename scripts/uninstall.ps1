# Clean Uninstallation Script for BANDIQ Node Agent
[CmdletBinding()]
param(
    [string]$DataDir = ".bandiq",
    [switch]$Force
)

$ErrorActionPreference = "Stop"

Write-Host "==========================================================" -ForegroundColor Cyan
Write-Host "         BANDIQ Node Agent Clean Uninstaller             " -ForegroundColor Cyan
Write-Host "==========================================================" -ForegroundColor Cyan

Write-Host "This will remove the local data directory and all stored keys:" -ForegroundColor Yellow
Write-Host "  Directory: $(Resolve-Path $DataDir -ErrorAction SilentlyContinue)" -ForegroundColor White
Write-Host "  - Node Identity Keypair (node_key.json)" -ForegroundColor Gray
Write-Host "  - Local Configuration (config.json)" -ForegroundColor Gray
Write-Host "  - Operator Consent Records (consent.json)" -ForegroundColor Gray
Write-Host "  - Offline Spool Queue (spool/)" -ForegroundColor Gray
Write-Host "`nNote: This script DOES NOT modify firewall rules, router, or Windows startup." -ForegroundColor Green

if (-not $Force) {
    $confirm = Read-Host "`nAre you sure you want to permanently remove these files? (type 'yes' to proceed)"
    if ($confirm.ToLower() -ne "yes") {
        Write-Host "[-] Uninstallation cancelled." -ForegroundColor Yellow
        exit 0
    }
}

if (Test-Path $DataDir) {
    Remove-Item -Recurse -Force $DataDir
    Write-Host "[+] Removed data directory '$DataDir'." -ForegroundColor Green
} else {
    Write-Host "[i] Data directory '$DataDir' not found." -ForegroundColor Gray
}

# Check if binaries exist and offer removal
if (Test-Path ".\bandiq.exe") {
    $delBin = Read-Host "Do you also want to remove compiled binaries (bandiq.exe, bandiq-mock-backend.exe)? (y/n)"
    if ($delBin.ToLower() -eq "y") {
        Remove-Item -Force ".\bandiq.exe" -ErrorAction SilentlyContinue
        Remove-Item -Force ".\bandiq-mock-backend.exe" -ErrorAction SilentlyContinue
        Write-Host "[+] Removed compiled binaries." -ForegroundColor Green
    }
}

Write-Host "`n==========================================================" -ForegroundColor Green
Write-Host "           BANDIQ Clean Uninstallation Complete          " -ForegroundColor Green
Write-Host "==========================================================" -ForegroundColor Green
