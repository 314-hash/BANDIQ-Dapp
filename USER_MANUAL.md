# BANDIQ DePIN Bandwidth Network — Comprehensive User Manual & Platform Integration Guide

**BANDIQ: Bandwidth + Intelligence**  
*Decentralized Physical Infrastructure Network (DePIN) on Sidra Chain*  
*Existing BNDQ Token Address:* `0x6d1537bEE4F1EE3f55F4BC9319Aa40252cd39133`  
*Target Network:* Sidra Chain (`Chain ID: 97453` / `0x17cad`)  

---

## Table of Contents
1. [Introduction to BANDIQ DePIN](#1-introduction-to-bandiq-depin)
2. [Architecture & Trust Boundaries](#2-architecture--trust-boundaries)
3. [Prerequisites & System Requirements](#3-prerequisites--system-requirements)
4. [Step-by-Step Node Agent Setup (Windows 10/11)](#4-step-by-step-node-agent-setup-windows-1011)
   - [Step 1: Identity Generation](#step-1-identity-generation)
   - [Step 2: Explicit Operator Consent](#step-2-explicit-operator-consent)
   - [Step 3: Starting the Bandwidth Agent](#step-3-starting-the-bandwidth-agent)
   - [Step 4: Operational Controls (Caps, Pause, Resume)](#step-4-operational-controls-caps-pause-resume)
5. [Connecting Your Wallet on Any Platform](#5-connecting-your-wallet-on-any-platform)
   - [Desktop Browsers (MetaMask, Rabby, Brave, OKX, Coinbase)](#desktop-browsers)
   - [Mobile Devices: iOS & Android (MetaMask Mobile, Trust Wallet)](#mobile-devices-ios--android)
   - [Universal WalletConnect Flow](#universal-walletconnect-flow)
   - [Zero-Trust Read-Only Mode (Any Device)](#zero-trust-read-only-mode)
   - [Configuring Sidra Chain Manually in Any Wallet](#configuring-sidra-chain-manually-in-any-wallet)
   - [Importing BNDQ Token into Your Wallet](#importing-bndq-token-into-your-wallet)
6. [The Proof-of-Bandwidth Verification System](#6-the-proof-of-bandwidth-verification-system)
   - [The 10 Integrity & Anti-Fraud Gates](#the-10-integrity--anti-fraud-gates)
   - [Synthetic Benchmarks vs. Real Customer Relays](#synthetic-benchmarks-vs-real-customer-relays)
   - [Cryptographic Settlement Vouchers](#cryptographic-settlement-vouchers)
7. [Using the BANDIQ DePIN Dashboard](#7-using-the-bandiq-depin-dashboard)
8. [Privacy, Safety & Ethical DePIN Tenets](#8-privacy-safety--ethical-depin-tenets)
9. [Troubleshooting, Audit Codes & FAQ](#9-troubleshooting-audit-codes--faq)

---

## 1. Introduction to BANDIQ DePIN

BANDIQ transforms standard residential and enterprise internet connections into a decentralized, measurable bandwidth sharing and intelligence network. Instead of relying on centralized CDN monopolies or speculative cloud mining, BANDIQ enables node operators to voluntarily contribute verifiable network throughput to authorized workloads and receive settlement in existing **$BNDQ** tokens.

### Key Design Principles:
1. **Physical Reality First:** Digital signatures and speed tests alone do *not* prove physical bandwidth delivery. All traffic must pass continuous telemetry verification, hardware counter correlation, and independent witness checks.
2. **Token Supply Preservation:** The deployed $BNDQ token (`0x6d1537bEE4F1EE3f55F4BC9319Aa40252cd39133`) has a fixed supply. No new tokens are ever minted for rewards. Reward settlements originate exclusively from buyer payment escrows and allocated network service pools.
3. **Operator Sovereignty:** Operators retain 100% control over their bandwidth cap, daily data quota, pause/resume state, and uninstallation.

---

## 2. Architecture & Trust Boundaries

BANDIQ separates concerns into four autonomous, secure layers:

```
+-----------------------------------------------------------------------------------+
| 1. Node Agent Layer (bandiq.exe - Localhost Only: 127.0.0.1:8725)                  |
|    - Ed25519 node keypair & EVM wallet pairing                                    |
|    - Windows IP Helper hardware counter telemetry (iphlpapi.dll)                  |
|    - Offline report spooling (.bandiq/spool) & automated drain                    |
+-----------------------------------------------------------------------------------+
                                        | (HTTPS / Ed25519 Signed JSON Reports)
                                        v
+-----------------------------------------------------------------------------------+
| 2. Proof-of-Bandwidth Verification Service (bandiq-verifier.exe :8085)            |
|    - Evaluates reports across 10 deterministic anti-fraud gates                    |
|    - Rejects replays, impossible speed spikes, and hardware counter fraud         |
|    - Issues cryptographically signed SettlementVouchers for valid served bytes    |
+-----------------------------------------------------------------------------------+
                                        | (Settlement Vouchers with Nonces)
                                        v
+-----------------------------------------------------------------------------------+
| 3. Smart Contracts & Settlement Layer (Sidra Chain - Chain ID: 97453)             |
|    - Existing BNDQ Token: 0x6d1537bEE4F1EE3f55F4BC9319Aa40252cd39133              |
|    - Verifies voucher signatures and redeems BNDQ escrow payments                 |
|    - Enforces non-reentrant single-claim nonces                                   |
+-----------------------------------------------------------------------------------+
                                        ^
                                        |
+-----------------------------------------------------------------------------------+
| 4. Universal Web Dashboard (Mobile-Ready Futuristic UI)                           |
|    - Connects to ANY wallet on ANY platform (MetaMask, Rabby, Trust, Mobile, WC)   |
|    - Live gauge, canvas topology visualizer, node controls, and verification tool |
+-----------------------------------------------------------------------------------+
```

---

## 3. Prerequisites & System Requirements

BANDIQ is engineered to run seamlessly on low-resource consumer computers:

- **Operating System:** Windows 10 or Windows 11 (64-bit).
- **Shell:** Windows PowerShell 5.1+ or PowerShell 7 Core.
- **CPU & Memory:** Minimum 1 CPU core, 512 MB RAM footprint.
- **Network Interface:** Standard Ethernet or Wi-Fi adapter.
- **Port Bindings:**
  - `127.0.0.1:8725` — Node Agent Admin API (strictly local loopback).
  - `0.0.0.0:8080` — Mock Backend & Web Dashboard.
  - `0.0.0.0:8085` — Proof-of-Bandwidth Verifier Daemon.
- **Wallet Requirement:** Any EVM-compatible wallet address to receive $BNDQ rewards.

---

## 4. Step-by-Step Node Agent Setup (Windows 10/11)

### Step 1: Identity Generation
Open PowerShell in the BANDIQ directory and initialize your node identity with your recipient EVM wallet address:

```powershell
.\bandiq.exe init -wallet "0xYourEthereumOrSidraWalletAddress"
```

*What happens:*
- An Ed25519 cryptographic keypair is generated and stored locally in `.bandiq/identity.json`.
- A deterministic Node ID is generated: `bandiq-node-<sha256(pubkey)[0:32]>`.
- The node configuration file `.bandiq/config.json` is initialized.

---

### Step 2: Explicit Operator Consent
Before the agent is permitted to conduct network measurements or relay traffic, the operator must grant explicit consent:

```powershell
.\bandiq.exe consent -accept
```

*Privacy Guarantee:* This records consent in `.bandiq/consent.json`. The agent will immediately abort execution if consent is revoked.

---

### Step 3: Starting the Bandwidth Agent
Start the node agent daemon:

```powershell
.\bandiq.exe start
```

Or run using the automated background launcher script:
```powershell
.\scripts\run-node.ps1
```

*Agent Actions:*
- Collects real hardware network adapter counters via Windows API (`GetIfEntry2`).
- Monitors host CPU and RAM via Windows kernel APIs (`GetSystemTimes`, `GlobalMemoryStatusEx`).
- Performs scheduled bandwidth measurements according to configured limits.
- Generates Ed25519 signed reports and posts them to the verifier.
- Automatically stores reports in `.bandiq/spool` during network outages and drains when restored.

---

### Step 4: Operational Controls (Caps, Pause, Resume)

You can manage your node in real time via the CLI or the Web Dashboard:

| Action | PowerShell Command |
| :--- | :--- |
| **Check Node Health & Status** | `.\bandiq.exe status` |
| **Trigger Immediate Speedtest** | `.\bandiq.exe test` |
| **Pause All Network Activity** | `.\bandiq.exe pause` |
| **Resume Node Service** | `.\bandiq.exe resume` |
| **Verify Sidra Chain RPC** | `.\bandiq.exe verify-chain` |
| **Clean Uninstall & Purge Keys** | `.\bandiq.exe uninstall` |

To modify data caps or speed throttles, adjust `.bandiq/config.json` or use the Web Dashboard sliders:
- `daily_data_cap_mb`: Maximum total traffic allowed per 24 hours (e.g. `1000` MB).
- `max_bandwidth_mbps`: Maximum throughput throttle (e.g. `50.0` Mbps).

---

## 5. Connecting Your Wallet on Any Platform

BANDIQ features a **Universal Multi-Platform Web3 Connector** that connects to any crypto wallet across Desktop, Mobile (iOS/Android), and Browser extensions.

### Desktop Browsers
*(MetaMask, Rabby Wallet, SafePal Extension, Brave Browser, OKX Wallet, Coinbase Wallet)*

1. Open your browser and navigate to the BANDIQ Dashboard: `http://127.0.0.1:8080/dashboard/`.
2. Click the **Connect Wallet** button in the top-right header or in the **BNDQ Ledger** tab.
3. The dashboard automatically detects any EIP-1193 injected wallet (MetaMask, Rabby, SafePal, etc.).
4. Click your preferred wallet provider (e.g. MetaMask, Rabby, or SafePal). Approve the connection prompt in your extension.
5. If your wallet is on Ethereum or another chain, click **Switch to Sidra Chain**; the dashboard will trigger a 1-click network configuration.

---

### Mobile Devices: iOS & Android
*(SafePal Mobile App, MetaMask Mobile, Trust Wallet, OKX Mobile, Rainbow)*

When accessing the dashboard on a smartphone or tablet:

#### Method A: Direct In-App DApp Browser (Recommended)
1. Open your mobile wallet app (SafePal, MetaMask, Trust Wallet, or OKX).
2. Tap the in-app **Browser / DApp / Discover** tab.
3. Enter your BANDIQ Dashboard URL (or live deployment `https://bandiq-dapp.vercel.app/`).
4. Tap **Connect Wallet** — connection is instantaneous with native biometric signature.

#### Method B: Universal Mobile Deep-Link
1. Open the dashboard in Safari, Chrome, or Firefox on your mobile device.
2. Tap **Connect Wallet**.
3. Select **Open in SafePal**, **Open in MetaMask Mobile**, or **Open in Trust Wallet**.
4. The dashboard triggers an OS-level universal deep link (`safepal://dapp?...`, `metamask://dapp/...` or `https://link.trustwallet.com/...`) opening the application directly.

---

### Universal WalletConnect Flow
*(Cross-Device Connection for over 300+ Wallets)*

1. On the dashboard wallet modal, select the **WalletConnect** tab.
2. A dynamic connection QR code and a direct copyable pairing URI (`wc:...`) will appear.
3. Open any supported wallet on your phone, tap **Scan QR Code**, and point your camera at the screen.
4. Confirm connection in your mobile wallet.

---

### Zero-Trust Read-Only Mode (Any Device)
If you are operating on a public workstation or using a hardware cold-storage wallet (Ledger / Trezor) and do not wish to connect private keys:

1. Click **Connect Wallet** and select the **Read-Only / Address Watcher** tab.
2. Paste any EVM address (e.g., `0x6d1537bEE4F1EE3f55F4BC9319Aa40252cd39133`).
3. Click **Watch Address**.
4. The dashboard loads your live Sidra Chain balance, historical node settlements, and network tier without requiring signature permissions.

---

### Configuring Sidra Chain Manually in Any Wallet

If your wallet does not automatically add Sidra Chain, you can manually enter the network parameters:

| Parameter | Value |
| :--- | :--- |
| **Network Name** | `Sidra Chain` |
| **New RPC URL** | `https://node.sidrachain.com` *(Local dev: `http://127.0.0.1:8080/rpc`)* |
| **Chain ID** | `97453` *(Hex: `0x17cad`)* |
| **Currency Symbol** | `KST` *(or `SIDRA`)* |
| **Block Explorer URL** | `https://explorer.sidrachain.com` |

---

### Importing BNDQ Token into Your Wallet

To display your $BNDQ balance inside MetaMask, Rabby, or Trust Wallet:

#### 1-Click Method (Via Dashboard):
- Open the **BNDQ Ledger** view and click the **"Add BNDQ to Wallet"** button. Your wallet will display an EIP-747 `wallet_watchAsset` prompt. Click **Add Token**.

#### Manual Method:
1. Open your wallet and select **Import Tokens / Custom Token**.
2. Ensure your active network is set to **Sidra Chain (97453)**.
3. Paste the Token Contract Address:
   ```
   0x6d1537bEE4F1EE3f55F4BC9319Aa40252cd39133
   ```
4. Token Symbol: `BNDQ`
5. Token Decimal: `18`
6. Click **Save / Add Custom Token**.

---

## 6. The Proof-of-Bandwidth Verification System

The BANDIQ Verifier (`bandiq-verifier.exe`) is an autonomous gatekeeper that sits between physical node measurements and the blockchain settlement layer.

### The 10 Integrity & Anti-Fraud Gates

Every measurement report must pass all 10 deterministic gates:

1. **Gate 1 — Schema Validation (`ERR_MALFORMED_REPORT`):** Validates strict JSON typing, required fields, and software version.
2. **Gate 2 — Cryptographic Authenticity (`ERR_INVALID_SIGNATURE`):** Validates the Ed25519 signature of the SHA-256 report hash using the node's registered public key.
3. **Gate 3 — Replay & Monotonicity Defense (`ERR_REPLAY_ATTACK`, `ERR_SEQUENCE_OUT_OF_ORDER`):** Verifies the `report_id` has never been seen before and that the sequence number is strictly increasing per node.
4. **Gate 4 — Timestamp Freshness (`ERR_TIMESTAMP_EXPIRED`, `ERR_FUTURE_TIMESTAMP`):** Rejects reports outside a strict $\pm 5$-minute drift window relative to verifier UTC.
5. **Gate 5 — Physical Plausibility (`ERR_IMPOSSIBLE_THROUGHPUT`):** Caps throughput at 10,000 Mbps and packet loss at $\le 100\%$.
6. **Gate 6 — Hardware Counter Delta Check (`ERR_COUNTER_INCONSISTENCY`):** Compares reported speedtest bytes against actual Windows adapter octets ($\Delta\text{In} + \Delta\text{Out} \ge \text{ClaimedBytes}$). Any discrepancy proves the report is synthetic/spoofed.
7. **Gate 7 — Identical Metric Spam Detection (`ERR_IDENTICAL_SAMPLE_SPAM`):** Flags automated scripts generating mathematically identical throughput, latency, and jitter samples across consecutive runs.
8. **Gate 8 — Rate Limiting (`ERR_RATE_LIMIT_EXCEEDED`):** Prevents spamming; enforces a minimum cooldown between reports (default 30 seconds).
9. **Gate 9 — Traffic Segregation:** Distinguishes active capacity speed tests (`CAPACITY_BENCHMARK`) from billable user relay traffic (`BILLABLE_RELAY`).
10. **Gate 10 — Statistical Scoring & Voucher Generation:** Computes a confidence score ($0.0 - 1.0$). If score $\ge 0.70$ and billable traffic was delivered, issues a cryptographically signed **Settlement Voucher**.

---

### Synthetic Benchmarks vs. Real Customer Relays

| Attribute | Active Bandwidth Test (Speedtest) | Customer Traffic Relay (Served Workload) |
| :--- | :--- | :--- |
| **Purpose** | Diagnostic capacity benchmark | Verifiable physical data relay |
| **Traffic Direction** | Node $\leftrightarrow$ Test Server | Consumer $\leftrightarrow$ Node $\leftrightarrow$ Origin |
| **Reward Eligibility** | **Unearned (0 BNDQ)** | **Earned ($BNDQ Rewards)** |
| **Classification** | `CAPACITY_BENCHMARK` | `BILLABLE_RELAY` |
| **Evidence Required** | Signed report + RTT probes | Signed report + Mutual Session Receipts |

---

### Cryptographic Settlement Vouchers

When legitimate billable traffic is verified, the verifier issues a canonical `SettlementVoucher`:

```json
{
  "voucher_id": "vouch-4ef81a20-09c1",
  "node_id": "bandiq-node-2d5cd8e1a89aac06c44c45d6e1908d61",
  "beneficiary_wallet": "0x6d1537bEE4F1EE3f55F4BC9319Aa40252cd39133",
  "verified_bytes": 10737418240,
  "confidence_score": 0.985,
  "issued_at": 1729141200,
  "expires_at": 1729227600,
  "nonce": 42,
  "voucher_signature": "0x7a89b...ed25519"
}
```

This voucher is submitted to the on-chain settlement contract, which unlocks payment without the blockchain having to re-verify physical traffic.

---

## 7. Using the BANDIQ DePIN Dashboard

The Web Dashboard runs locally on `http://127.0.0.1:8080/dashboard/` and features 8 specialized views:

1. **Network Overview:** Global stats, active nodes, verified traffic, and interactive canvas network topology visualizer.
2. **My Node Console:** Real-time Windows adapter telemetry, CPU/RAM meters, power controls (pause/resume), and bandwidth throttle sliders.
3. **Bandwidth Benchmark:** Real-time animated SVG gauge, chunked download/upload speed test, ping RTT, jitter, and packet loss.
4. **DePIN Node Explorer:** Live table of all network nodes with search, status filtering, and quality rankings.
5. **Bandwidth Market:** Peer-to-peer session marketplace where consumers request bandwidth and inspect active session receipts.
6. **Proof Verification Center:** In-browser cryptographic tool to paste any report JSON and inspect its Ed25519 signature and audit evaluation.
7. **BNDQ Ledger, Sidra DEX & PinetSwap Terminal:** Live wallet balance tracker, on-chain transaction history, 1-click token import, and live AMM decentralized exchange (swapping SDA <-> BNDQ directly on Sidra Chain with PinetSwap routing).
8. **Docs & Security:** Built-in searchable manual, operator consent rules, and data collection transparency.

---

## 8. Privacy, Safety & Ethical DePIN Tenets

- **Non-Custodial Architecture:** Your private keys and seed phrases never leave your wallet. The agent only knows your public address.
- **Never an Open Relay:** The node will never act as a free-for-all proxy or exit node for unauthorized traffic.
- **Zero Command Execution:** The node agent rejects any arbitrary shell commands, scripts, or executables sent from the network.
- **Zero Browsing Tracking:** No website history, DNS records, or personal file payloads are monitored or recorded.
- **Strict Loopback Admin:** The local management API is strictly bound to `127.0.0.1:8725` to prevent external LAN attacks.
- **Ethical DePIN Alignment:** Backed by real physical utility, fair market compensation, and transparent auditability without speculative promises.

---

## 9. Troubleshooting, Audit Codes & FAQ

### Verifier Error Codes Reference

| Error Code | Cause | Resolution |
| :--- | :--- | :--- |
| `ERR_UNREGISTERED_NODE` | Node public key not registered | Run `.\bandiq.exe init` and start agent |
| `ERR_INVALID_SIGNATURE` | Report hash signature failed | Check that `.bandiq/identity.json` is intact |
| `ERR_REPLAY_ATTACK` | Duplicate `report_id` submitted | Ensure reports are generated by official agent |
| `ERR_SEQUENCE_OUT_OF_ORDER` | Sequence number rolled back | Reset node identity or sync local clock |
| `ERR_TIMESTAMP_EXPIRED` | Node clock drifted by > 5 minutes | Synchronize Windows clock with Internet time |
| `ERR_IMPOSSIBLE_THROUGHPUT` | Throughput exceeds 10,000 Mbps | Check for corrupted network driver readings |
| `ERR_COUNTER_INCONSISTENCY` | Adapter octets do not match test data | Ensure traffic flows through monitored adapter |
| `ERR_IDENTICAL_SAMPLE_SPAM` | Consecutive identical speed values | Ensure node is executing real HTTP tests |
| `ERR_RATE_LIMIT_EXCEEDED` | Reports submitted faster than 30s | Relax test frequency in `.bandiq/config.json` |

---

### Frequently Asked Questions (FAQ)

**Q: Can I run BANDIQ on multiple PCs with the same wallet?**  
A: Yes. Each PC generates its own unique Node ID (`bandiq-node-...`), but all nodes can point to the same recipient EVM wallet address.

**Q: Why does my speedtest show 100 Mbps, but my earned BNDQ balance did not increase?**  
A: Active speed tests are classified as `CAPACITY_BENCHMARK` (unearned diagnostic tests). Earnings accrue when your node delivers actual relay service (`ServedBytes`) for verified sessions in the Bandwidth Market.

**Q: Does BANDIQ consume my internet bandwidth when I'm gaming or working?**  
A: You can set a strict bandwidth cap (e.g., 20 Mbps) or daily cap (e.g., 500 MB) in the dashboard, or simply click **Pause Node** whenever you need full bandwidth.

**Q: What happens if my internet disconnects during a test?**  
A: The agent stores unsubmitted measurement reports in `.bandiq/spool`. As soon as your internet connection is restored, the agent automatically drains the queue.

---

*BANDIQ DePIN Network — Bandwidth + Intelligence*  
*Sidra Chain Ecosystem | Existing Token: `0x6d1537bEE4F1EE3f55F4BC9319Aa40252cd39133`*
