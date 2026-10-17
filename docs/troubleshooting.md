# BANDIQ DePIN Bandwidth Node Agent — Troubleshooting Guide

This guide covers common operational issues, diagnostics, error messages, and their resolutions.

---

## 1. Quick Diagnostics Commands

Check agent health and status using PowerShell:

```powershell
# Check if agent daemon is running and view status
.\bandiq.exe status

# Or query using the control script
.\scripts\control.ps1 status

# Query basic health endpoint directly
Invoke-RestMethod -Uri http://127.0.0.1:8725/health
```

---

## 2. Common Issues & Resolutions

### Issue 1: "ATTENTION: Operator consent has not been granted! Network tests are paused."
* **Cause:** By design, the BANDIQ agent enforces explicit operator consent before running any network measurement routines.
* **Resolution:**
  Grant consent using the CLI:
  ```powershell
  .\bandiq.exe consent grant
  ```
  Verify status:
  ```powershell
  .\bandiq.exe consent status
  ```

---

### Issue 2: "Node identity not found. Please run 'bandiq init --wallet <address>' first."
* **Cause:** The agent requires a local Ed25519 cryptographic keypair and operator wallet association in `.bandiq/node_key.json`.
* **Resolution:**
  Initialize your node with your Sidra Chain EVM wallet:
  ```powershell
  .\bandiq.exe init --wallet 0x6d1537bEE4F1EE3f55F4BC9319Aa40252cd39133
  ```

---

### Issue 3: "Sidra Chain RPC unreachable or verification skipped (operating in simulated verification mode)"
* **Cause:** The configured Sidra Chain RPC (`https://rpc.sidrachain.com`) may be experiencing public rate-limiting, latency, or the local environment is running offline.
* **Resolution:**
  1. Verify your RPC connectivity using:
     ```powershell
     .\bandiq.exe verify-chain
     ```
  2. For local testing, you can use the built-in mock RPC server provided by `bandiq-mock-backend.exe`:
     ```powershell
     # In Terminal 1:
     .\scripts\run-mock-backend.ps1

     # In Terminal 2, configure .env or config.json to point to http://127.0.0.1:8080/rpc:
     $env:BANDIQ_SIDRA_RPC_URL="http://127.0.0.1:8080/rpc"
     .\scripts\run-node.ps1
     ```

---

### Issue 4: "Backend service unreachable: spooled offline"
* **Cause:** The agent cannot reach the BANDIQ verification backend (`http://127.0.0.1:8080`).
* **Expected Behavior:** This is normal during network outages! The agent automatically spools signed measurement reports into `.bandiq/spool/` so no earned measurement proofs are lost.
* **Resolution:**
  1. Verify the mock backend is running:
     ```powershell
     .\scripts\run-mock-backend.ps1
     ```
  2. Once the backend is online, the agent's spool draining worker automatically flushes queued reports in FIFO order. Check queued count:
     ```powershell
     .\scripts\control.ps1 status
     ```

---

### Issue 5: "security violation: admin bind address ... is not a loopback address"
* **Cause:** The configuration attempted to bind the admin port to `0.0.0.0` or a non-loopback IP.
* **Resolution:** For security, the administrative interface MUST bind to `127.0.0.1` or `localhost`. Keep `bind_address: "127.0.0.1:8725"` in `config.json`.

---

### Issue 6: "daily data cap exceeded: benchmark skipped"
* **Cause:** The agent reached the operator-configured daily bandwidth quota (`daily_data_cap_mb`).
* **Resolution:**
  If you wish to increase your daily cap, you can update it on the running agent without restarting:
  ```powershell
  .\scripts\control.ps1 set-caps -DailyMB 2500
  ```

---

### Issue 7: How to Cleanly Uninstall the Agent
* **Resolution:**
  Run the uninstallation script:
  ```powershell
  .\scripts\uninstall.ps1
  ```
  This will remove `.bandiq` and stored keypairs without affecting firewall, router, or system settings.
