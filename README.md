# BANDIQ DePIN Bandwidth Node Agent

**BANDIQ — Bandwidth + Intelligence**  
*The lightweight, verifiable, decentralized physical infrastructure (DePIN) bandwidth agent.*

[![Go Version](https://img.shields.io/badge/Go-1.22%2B-00ADD8?style=flat&logo=go)](https://go.dev/)
[![Network](https://img.shields.io/badge/Network-Sidra%20Chain%20(97453)-8A2BE2)](https://sidrachain.com)
[![Token](https://img.shields.io/badge/Token-%24BNDQ%20(0x6d15...3133)-F7931A)](https://sidrachain.com)
[![Platform](https://img.shields.io/badge/Platform-Windows%2010%2F11%20%7C%20Linux%20%7C%20macOS-0078D6)](https://microsoft.com/windows)
[![License](https://img.shields.io/badge/License-MIT-green)](LICENSE)

---

## 1. Overview

The **BANDIQ Bandwidth Node Agent** enables node operators to voluntarily contribute measurable network capacity, collect verifiable quality-of-service telemetry, and submit signed cryptographic attestations to the BANDIQ network backend.

### Blockchain & Token Configuration
* **Network:** Sidra Chain
* **Chain ID:** `97453` (verified via JSON-RPC `eth_chainId`)
* **Token Contract ($BNDQ):** `0x6d1537bEE4F1EE3f55F4BC9319Aa40252cd39133`
* **Token Economy Mode:** Uses existing token only. No replacement tokens deployed, no minting. Local measurement-only mode by default without unearned reward claims.

---

## 2. Research Analysis of Reference Projects

Before engineering the agent, we investigated five major open-source networking and monitoring architectures:

1. **iperf3 (`esnet/iperf`):** Focused on active socket throughput benchmarks. *Key lesson adopted:* Active measurement traffic generates artificial load; it **MUST** be strictly isolated from actual user payload traffic, throttled by bandwidth limits, and capped by daily quotas.
2. **LibreSpeed (`librespeed/speedtest`):** Lightweight HTTP/1.1 and HTTP/2 speed testing. *Key lesson adopted:* Avoids non-standard ports (like raw port 5201) that residential ISPs and routers block. Adopted chunked HTTP GET (download) and POST (upload) over standard web ports.
3. **Prometheus Node Exporter (`prometheus/node_exporter`):** Low-overhead passive OS telemetry. *Key lesson adopted:* Direct OS IP Helper API integration (`iphlpapi.dll` on Windows) to read physical interface byte and packet counters without external drivers or kernel modifications.
4. **go-libp2p (`libp2p/go-libp2p`):** Decentralized cryptographic peer identity. *Key lesson adopted:* Pure Ed25519 identity generation, deterministic `NodeID` derivation (`bandiq-node-<hash>`), and message signing decoupled from blockchain wallet private keys.
5. **OpenWISP Monitoring (`openwisp/openwisp-monitoring`):** Resilient edge daemon with local buffering. *Key lesson adopted:* Implemented an on-disk FIFO persistent queue (`internal/spool`) that spools signed reports during backend disconnections and flushes them sequentially upon reconnection.

*(See full technical breakdown in [docs/research_and_architecture.md](docs/research_and_architecture.md))*

---

## 3. Key Features

- [x] **Node Identity & Wallet Association:** Generates an Ed25519 keypair and deterministic Node ID, securely associating the operator's Sidra Chain EVM wallet address (`0x...`).
- [x] **Explicit Operator Consent:** Zero network measurement or traffic relaying runs without explicit opt-in confirmation (`bandiq consent grant`).
- [x] **Opt-In Active Benchmarking:** Configurable limits on test duration, data volume per test, test frequency, and daily data cap (default 1000 MB).
- [x] **Full QoS Telemetry:** Measures download/upload throughput (Mbps), latency RTT (min/avg/max), RFC 3550 statistical jitter, estimated packet loss, host uptime, CPU %, and memory usage.
- [x] **Strict Traffic Distinction:** Unambiguously distinguishes synthetic speedtest bytes (`active_test_bytes_in/out`) from useful relayed client bytes (`served_bytes_in/out`).
- [x] **Signed Measurement Reports:** Every report is serialized canonically, hashed (SHA-256), and signed with Ed25519.
- [x] **Replay Resistance:** Monotonically increasing sequence numbers, cryptographic UUIDv4 report IDs, and timestamp freshness windows (max 5 minutes).
- [x] **Persistent Offline Spool Queue:** Local on-disk queue buffers reports during network dropouts; flushes automatically when connectivity restores.
- [x] **Power Controls & Local Admin API:** Localhost loopback HTTP server (`127.0.0.1:8725`) providing health checks, status inspection, dynamic cap adjustment, pause, and resume.
- [x] **Clean Uninstaller:** Guided, operator-confirmed uninstallation that cleans keys and spooled files without modifying system registry, firewalls, or routers.

---

## 4. Safety & Privacy Guarantees

* **No Botnet, Hidden Proxy, or Open Relay:** In v0.1.0, traffic relaying is strictly disabled. The agent only conducts outbound synthetic measurements against designated test servers.
* **No Private Key or Seed Phrase Exposure:** The agent never asks for or stores the operator's wallet private key. Only the public `0x...` address is used.
* **No Remote Code Execution:** The agent does not execute arbitrary shell commands received from the backend.
* **No Browsing History or Payload Sniffing:** Passive interface counters are read as numerical summaries via OS APIs (`GetIfEntry2`).
* **Strict Loopback Admin:** Admin API binds exclusively to `127.0.0.1`.

*(See detailed threat analysis in [docs/security_and_privacy.md](docs/security_and_privacy.md))*

---

## 5. Quick Start (Windows 10/11 & PowerShell)

### Step 1: Run Setup
Open PowerShell in the project directory and run:

```powershell
.\scripts\setup.ps1
```
*This compiles `bandiq.exe` (agent) and `bandiq-mock-backend.exe` (mock verification backend), runs all unit tests, and verifies the environment.*

### Step 2: Initialize Node Identity
Initialize your node and associate your Sidra Chain EVM wallet address:

```powershell
.\bandiq.exe init --wallet 0x6d1537bEE4F1EE3f55F4BC9319Aa40252cd39133
```

### Step 3: Grant Explicit Operator Consent
Authorize network telemetry:

```powershell
.\bandiq.exe consent grant
```

### Step 4: Run the Mock Backend (Terminal 1)
Start the local verification backend and test server:

```powershell
.\scripts\run-mock-backend.ps1
```
*The mock backend provides speedtest endpoints, node registration, report verification with signature checking, and a simulated Sidra Chain RPC endpoint (Chain ID: 97453).*

### Step 5: Start the Node Agent (Terminal 2)
Launch the background agent daemon:

```powershell
.\scripts\run-node.ps1
```

---

## 6. Operator Command Line Reference

| Command | Description |
|---|---|
| `bandiq init --wallet <0x...>` | Generates Ed25519 keypair and associates Sidra Chain wallet |
| `bandiq consent grant` | Grants explicit operator consent to enable measurements |
| `bandiq consent revoke` | Revokes consent, immediately pausing all network activity |
| `bandiq consent status` | Displays current operator consent record |
| `bandiq start` | Starts the background agent daemon |
| `bandiq test [server_url]` | Executes an immediate on-demand test and prints results |
| `bandiq status` | Queries running agent's status, QoS metrics, and queue size |
| `bandiq pause` | Pauses scheduled measurements |
| `bandiq resume` | Resumes scheduled measurements |
| `bandiq verify-chain` | Tests Sidra Chain RPC connectivity and queries BNDQ token contract |
| `bandiq uninstall` | Safely removes node keys, config, and spooled data |

---

## 7. Operator Control Script (`control.ps1`)

Use `scripts/control.ps1` to interact with a running node agent:

```powershell
# View full node status and hardware metrics
.\scripts\control.ps1 status

# Check node health
.\scripts\control.ps1 health

# Trigger an immediate on-demand benchmark
.\scripts\control.ps1 test

# Pause and resume
.\scripts\control.ps1 pause
.\scripts\control.ps1 resume

# Dynamically adjust daily data cap (e.g. 2500 MB) and bandwidth limit (100 Mbps)
.\scripts\control.ps1 set-caps -DailyMB 2500 -SpeedMbps 100.0

# Inspect latest completed report
.\scripts\control.ps1 reports
```

---

## 8. Sample Signed Measurement Report

Below is an excerpt from a verified report generated by the agent ([examples/sample_signed_report.json](examples/sample_signed_report.json)):

```json
{
  "report_id": "c8a3f9e2-1b4d-4e89-a29d-7f9182390a44",
  "node_id": "bandiq-node-f73b889a012c4491de88f01b",
  "operator_wallet": "0x6d1537bee4f1ee3f55F4BC9319Aa40252cd39133",
  "sequence_number": 1,
  "timestamp": "2026-10-17T09:45:00.123456789Z",
  "software_version": "0.1.0",
  "measurement_method": "HTTP_SPEEDTEST_V1",
  "metrics": {
    "download_mbps": 94.25,
    "upload_mbps": 38.60,
    "latency_ms": 14.82,
    "jitter_ms": 1.45,
    "packet_loss_pct": 0.0,
    "active_test_bytes_in": 13107200,
    "active_test_bytes_out": 5242880,
    "served_bytes_in": 0,
    "served_bytes_out": 0,
    "test_duration_ms": 4850
  },
  "interface_counters": {
    "interface_name": "Ethernet",
    "bytes_sent": 255451748,
    "bytes_recv": 1423798221,
    "operational_status": "UP"
  },
  "signature": "3f98cba2290f114008bc27834591a2ee04bbda3948571029abce88492048aa77..."
}
```

---

## 9. Verification & Testing

Run all unit tests across all packages:

```powershell
go test -v ./...
```

Packages tested:
* `internal/identity`: Key generation, deterministic Node ID, Ed25519 signing/verification.
* `internal/consent`: Operator consent tracking, persistence, and revocation enforcement.
* `internal/config`: Schema validation, environment variable overrides, security checks.
* `internal/system`: Windows IP Helper API interface counters, CPU, RAM, and uptime.
* `internal/report`: Canonical JSON hashing, cryptographic signing, and tamper detection.
* `internal/spool`: On-disk FIFO report queue, persistence across restarts, capacity limits.
* `internal/measure`: Active bandwidth tests, traffic segregation, and daily cap enforcement.
* `internal/blockchain`: Sidra Chain JSON-RPC chain ID (97453) verification and BNDQ contract queries.
* `internal/client`: Authenticated node registration, report submission, and offline spool draining.
* `internal/admin`: Localhost administrative API and power controls.

---

## 10. License

This project is licensed under the MIT License — see the [LICENSE](LICENSE) file for details.
