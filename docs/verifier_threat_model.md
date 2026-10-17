# BANDIQ Proof-of-Bandwidth Verifier — Threat Model & Verification Architecture

## 1. Core Principle: The Physical Trust Boundary

> **A digital signature proves that a specific key signed a message. It does NOT prove that physical electrons or photons traversed the internet.**

In a DePIN bandwidth network, the node software is open-source and runs on hardware controlled by potentially adversarial operators. Therefore, the verification service cannot assume that signed reports are honest. The verifier’s role is to act as an adversarial gateway that evaluates physical plausibility, multi-point consistency, and economic barriers before any claim can reach the Sidra Chain BNDQ settlement contracts.

---

## 2. Threat Vectors & Defense Mechanisms

### Threat Vector 1: Sybil Identities (Virtual Machine / Container Farming)
* **Attack Scenario:** An attacker spins up 500 Docker containers or lightweight VMs on a single home PC with one 100 Mbps broadband connection, registering 500 distinct Ed25519 node identities to claim 500x rewards.
* **Impact:** Bandwidth capacity is diluted; the network pays for phantom redundancy.
* **Verifier Defenses:**
  1. **IP Subnet & Autonomous System (AS) Clustering:** The verifier clusters nodes by external IP prefix (`/24` for IPv4, `/48` for IPv6) and AS number. Multiple nodes behind the same IP share a single aggregate bandwidth quota.
  2. **Simultaneous Concurrency Challenge:** The verifier issues concurrent challenge benchmarks. If 10 virtual nodes claim 100 Mbps each simultaneously on a 100 Mbps pipe, the aggregate speed will collapse to 10 Mbps each, exposing the physical bottleneck.
  3. **Hardware Fingerprint Entropy:** Reading physical MAC addresses, CPU model identifiers, and adapter types via Windows IP Helper API to detect duplicated virtualized environments.

---

### Threat Vector 2: Colluding Test Endpoints
* **Attack Scenario:** A node operator deploys their own rogue speedtest server on a local gigabit switch or high-speed VPS. The node agent benchmarks exclusively against this colluding server, reporting fictitious 10 Gbps speeds with 0 ms latency.
* **Impact:** Artificially inflated bandwidth metrics used to siphon rewards.
* **Verifier Defenses:**
  1. **Authorized Verifier Endpoint Whitelist:** The verifier only accepts benchmarks conducted against protocol-authorized, independently monitored measurement servers.
  2. **Multi-Point Triangulation (Witness Probes):** The verifier dispatches independent probes from distributed witness nodes to test the candidate node.
  3. **Measurement Uncertainty Bounds:** The verifier penalizes tests with suspiciously zero jitter ($< 0.05\text{ ms}$) or latency below the theoretical speed-of-light propagation limit for the declared geographical distance.

---

### Threat Vector 3: Self-Generated / Loopback Traffic
* **Attack Scenario:** An attacker generates continuous dummy HTTP traffic across loopback (`127.0.0.1`) or a local LAN IP, causing interface byte counters to surge without delivering external public internet transit.
* **Impact:** Hardware interface counters look genuine, but no real customer bandwidth was contributed.
* **Verifier Defenses:**
  1. **Strict Mutual Session Receipts:** For billable traffic (`served_bytes`), the verifier requires **bilateral cryptographic session receipts** signed by both the provider node and the independent consumer.
     $$\text{Receipt}_{\text{Provider}} == \text{Receipt}_{\text{Consumer}}$$
     If provider claims 5 GB served but consumer signed receipts only confirm 50 MB, the claim is rejected for fraud.
  2. **Default Route Adapter Filtering:** The agent collector explicitly binds counter queries to the adapter with the active default gateway (`IsDefaultRoute == true`), ignoring virtual loopback devices.

---

### Threat Vector 4: Compromised Node Software
* **Attack Scenario:** A developer forks the Go agent source code, removes the measurement engine, and hardcodes the reporting routine to emit forged reports claiming 500 Mbps and 50 GB served daily.
* **Impact:** Zero bandwidth provided; full rewards claimed.
* **Verifier Defenses:**
  1. **Hardware Counter Delta Gate (Gate 6):** Even if the agent code is patched to lie about throughput, it cannot falsify the Windows kernel's physical adapter counters (`GetIfEntry2`). If reported test bytes exceed the delta of bytes that actually crossed the physical network card, the verifier immediately rejects the report (`ERR_COUNTER_INCONSISTENCY`).
  2. **Entropy & Micro-Jitter Analysis (Gate 7):** Hardcoded or synthetic metrics lack natural Gaussian distribution and Poisson arrival jitter. Consecutive reports matching to four decimal places trigger instant fraud flags (`ERR_IDENTICAL_SAMPLE_SPAM`).
  3. **Challenge Probes:** The verifier periodically sends unannounced challenge payloads to the node's listening port to measure actual return throughput.

---

### Threat Vector 5: Inaccurate Clocks & NTP Drift
* **Attack Scenario:** A node operator’s clock is misconfigured by several hours, or an attacker deliberately alters the system clock to replay expired reports or pre-date future epochs.
* **Impact:** Replay protection and epoch accounting fail.
* **Verifier Defenses:**
  1. **Strict Temporal Freshness Window (Gate 4):** Reports with timestamps deviating by more than $\pm 5\text{ minutes}$ from the verifier's GPS/NTP synchronized clock are rejected (`ERR_TIMESTAMP_STALE` / `ERR_TIMESTAMP_FUTURE`).
  2. **Strict Sequence Monotonicity (Gate 3):** Timestamps cannot move backward relative to the node’s previous accepted report.

---

## 3. Staged Rollout Strategy Before Production Rewards

Deploying on-chain BNDQ rewards without an empirical verification baseline invites immediate economic exploitation. BANDIQ follows a phased deployment:

```
┌────────────────────────────────────────────────────────────────────────┐
│ Phase 1: Observation & Anomaly Baselining (Current)                    │
│ - Local measurement-only mode.                                         │
│ - Verifier operates in audit mode (accepting/rejecting into logs).      │
│ - Real Sidra Chain token rewards NOT active yet.                       │
│ - Machine-learning baselines established for natural home connection   │
│   jitter, diurnal usage curves, and typical ISP latency.               │
└───────────────────────────────────┬────────────────────────────────────┘
                                    │
                                    ▼
┌────────────────────────────────────────────────────────────────────────┐
│ Phase 2: Multi-Witness Quorum & Challenge Probes                       │
│ - Verifier introduces decentralized witness nodes.                     │
│ - Challenge-response active probes sent to verify node availability.   │
│ - Dispute and appeal system tested with node operators.                │
└───────────────────────────────────┬────────────────────────────────────┘
                                    │
                                    ▼
┌────────────────────────────────────────────────────────────────────────┐
│ Phase 3: Canary Settlement on Sidra Testnet                            │
│ - Verifiers issue cryptographic Settlement Vouchers.                   │
│ - BandiqBandwidthVerifier.sol & BandiqRewardDistributor.sol deployed   │
│   on Sidra testnet.                                                    │
│ - Mock settlement reconciliations run for 30 consecutive epochs.       │
└───────────────────────────────────┬────────────────────────────────────┘
                                    │
                                    ▼
┌────────────────────────────────────────────────────────────────────────┐
│ Phase 4: Production Mainnet Settlement                                 │
│ - Existing BNDQ token contract (0x6d15...3133) connected.              │
│ - Vouchers verified on Sidra Chain Mainnet (Chain ID: 97453).          │
│ - Rewards disbursed strictly from verified customer fees and treasury. │
└────────────────────────────────────────────────────────────────────────┘
```
