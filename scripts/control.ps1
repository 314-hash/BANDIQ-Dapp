# Operator Control Script for Running BANDIQ Node Agent
[CmdletBinding()]
param(
    [Parameter(Position=0, Mandatory=$true)]
    [ValidateSet("status", "pause", "resume", "test", "set-caps", "health", "reports")]
    [string]$Action,

    [int]$DailyMB = 0,
    [double]$SpeedMbps = 0.0,
    [string]$AdminURL = "http://127.0.0.1:8725"
)

$ErrorActionPreference = "Stop"

switch ($Action) {
    "status" {
        Write-Host "Querying node status from $AdminURL/status..." -ForegroundColor Cyan
        try {
            $resp = Invoke-RestMethod -Uri "$AdminURL/status" -Method Get
            $resp | ConvertTo-Json -Depth 6
        } catch {
            Write-Host "[-] Failed to connect to BANDIQ agent at $AdminURL. Is the agent running?" -ForegroundColor Red
        }
    }

    "health" {
        try {
            $resp = Invoke-RestMethod -Uri "$AdminURL/health" -Method Get
            Write-Host "[+] Health Status: $($resp.status) (Paused: $($resp.paused))" -ForegroundColor Green
        } catch {
            Write-Host "[-] Agent offline or unreachable at $AdminURL" -ForegroundColor Red
        }
    }

    "pause" {
        Write-Host "Sending pause signal to agent..." -ForegroundColor Yellow
        try {
            $resp = Invoke-RestMethod -Uri "$AdminURL/pause" -Method Post -ContentType "application/json"
            Write-Host "[+] $($resp.message)" -ForegroundColor Green
        } catch {
            Write-Host "[-] Failed to pause agent: $_" -ForegroundColor Red
        }
    }

    "resume" {
        Write-Host "Sending resume signal to agent..." -ForegroundColor Cyan
        try {
            $resp = Invoke-RestMethod -Uri "$AdminURL/resume" -Method Post -ContentType "application/json"
            Write-Host "[+] $($resp.message)" -ForegroundColor Green
        } catch {
            Write-Host "[-] Failed to resume agent: $_" -ForegroundColor Red
        }
    }

    "test" {
        Write-Host "Triggering on-demand benchmark on running agent..." -ForegroundColor Cyan
        try {
            $resp = Invoke-RestMethod -Uri "$AdminURL/trigger" -Method Post -ContentType "application/json"
            Write-Host "[+] Benchmark completed!" -ForegroundColor Green
            Write-Host "  Report ID:     $($resp.report_id)"
            Write-Host "  Download:      $($resp.metrics.download_mbps) Mbps"
            Write-Host "  Upload:        $($resp.metrics.upload_mbps) Mbps"
            Write-Host "  Latency:       $($resp.metrics.latency_ms) ms"
            Write-Host "  Active Bytes:  $($resp.metrics.active_test_bytes_in) in / $($resp.metrics.active_test_bytes_out) out"
            Write-Host "  Served Bytes:  $($resp.metrics.served_bytes_in) in / $($resp.metrics.served_bytes_out) out"
        } catch {
            Write-Host "[-] Benchmark trigger failed: $_" -ForegroundColor Red
        }
    }

    "reports" {
        try {
            $resp = Invoke-RestMethod -Uri "$AdminURL/reports/latest" -Method Get
            $resp | ConvertTo-Json -Depth 6
        } catch {
            Write-Host "[-] No completed reports found or agent offline." -ForegroundColor Red
        }
    }

    "set-caps" {
        if ($DailyMB -le 0 -and $SpeedMbps -le 0) {
            Write-Host "[-] Please provide -DailyMB <MB> and/or -SpeedMbps <Mbps>" -ForegroundColor Red
            exit 1
        }
        $body = @{}
        if ($DailyMB -gt 0) { $body["daily_data_cap_mb"] = $DailyMB }
        if ($SpeedMbps -gt 0) { $body["max_bandwidth_limit_mbps"] = $SpeedMbps }
        $json = $body | ConvertTo-Json

        Write-Host "Updating bandwidth caps..." -ForegroundColor Cyan
        try {
            $resp = Invoke-RestMethod -Uri "$AdminURL/caps" -Method Post -Body $json -ContentType "application/json"
            Write-Host "[+] Caps updated successfully!" -ForegroundColor Green
            $resp.limits | ConvertTo-Json
        } catch {
            Write-Host "[-] Failed to update caps: $_" -ForegroundColor Red
        }
    }
}
