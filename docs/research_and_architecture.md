# BANDIQ DePIN Bandwidth Node Agent — Research & Architecture Analysis

## 1. Research of Reference Open-Source Projects

Before implementing the BANDIQ node agent, we investigated five major open-source network telemetry, monitoring, and P2P architectures. Below is the comparative analysis detailing their source structure, dependencies, licensing, security posture, and compatibility with a lightweight Windows 10/11 Go agent.

---

### A. iperf3 (`esnet/iperf`)
* **Primary Language & Focus:** C; active network throughput benchmarks (TCP, UDP, SCTP).
* **Architecture:** Client-server model using dedicated sockets to generate synthetic streams. Measures throughput, TCP window size, round-trip time, UDP jitter, and packet loss.
* **License:** 3-clause BSD.
* **Security & Platform Assessment:**
  - *Strengths:* Highly accurate wire-speed benchmarking and well-defined RFC metrics.
  - *Limitations for DePIN on Windows:* Requires running dedicated listener daemons on non-standard ports (default 5201). Residential ISPs and NAT gateways frequently block or throttle arbitrary TCP ports. Furthermore, compiled C binaries often require Cygwin/MSYS2 compatibility layers or raw socket privileges.
* **Takeaways for BANDIQ:**
  - Active synthetic measurement traffic MUST be strictly segregated from actual user-relayed payload bytes.
  - Test intervals, socket timeouts, and maximum duration must be strictly bounded so the agent never starves operator network bandwidth.

---

### B. LibreSpeed (`librespeed/speedtest`)
* **Primary Language & Focus:** JavaScript (frontend) with Go, PHP, or Node.js backends; lightweight HTTP-based speed testing.
* **Architecture:** Uses standard chunked HTTP/1.1 and HTTP/2 GET requests for download testing, chunked POST requests for upload testing, and HEAD/GET round-trip probes for latency and jitter.
* **License:** GNU LGPL v3.0.
* **Security & Platform Assessment:**
  - *Strengths:* 100% firewall and NAT traversal friendly because it operates over standard HTTP/HTTPS ports (80/443/8080). Pure application-layer implementation with no kernel driver requirements.
  - *Portability:* Excellent cross-platform compatibility across Windows, Linux, and macOS.
* **Takeaways for BANDIQ:**
  - Adopted HTTP chunked streaming for download and upload tests in `internal/measure`. This guarantees seamless execution behind residential NATs and firewalls without requiring router port-forwarding or UPnP modifications.

---

### C. Prometheus Node Exporter (`prometheus/node_exporter`)
* **Primary Language & Focus:** Go; passive host telemetry collector for Linux, Unix, and Windows (via `windows_exporter` / WMI).
* **Architecture:** Pull-based metric exposure. Reads low-level kernel interface counters, CPU utilization, memory allocation, and disk statistics.
* **License:** Apache 2.0.
* **Security & Platform Assessment:**
  - *Strengths:* Extremely lightweight, passive, and read-only. Does not generate artificial network traffic or modify network stack parameters.
  - *Portability:* On Windows, native IP Helper APIs (`iphlpapi.dll`) provide direct access to interface byte and packet counters without overhead.
* **Takeaways for BANDIQ:**
  - Implemented in `internal/system` using Windows IP Helper API (`GetIfEntry2` and `GetTickCount64`) to extract exact physical adapter counters (`InOctets`, `OutOctets`, `InUcastPkts`, `OutUcastPkts`, error/drop counters) with zero external CGo dependencies.

---

### D. go-libp2p (`libp2p/go-libp2p`)
* **Primary Language & Focus:** Go; modular peer-to-peer networking stack powering IPFS, Filecoin, and Ethereum consensus layers.
* **Architecture:** Decentralized peer identity derived from public key cryptography. Multiaddresses, encrypted transport (Noise, TLS 1.3), stream multiplexing, and distributed NAT traversal (STUN, TURN, AutoNAT).
* **License:** MIT / Apache 2.0.
* **Security & Platform Assessment:**
  - *Strengths:* Industry standard for cryptographic peer identity and tamper-proof wire messaging.
  - *Trade-offs:* Full go-libp2p dependency graph is large (~100+ packages), adding memory footprint and binary size which conflicts with low-resource Windows workstations.
* **Takeaways for BANDIQ:**
  - Adopted the libp2p cryptographic identity model without importing heavy dependency trees: Ed25519 keypair generation, deterministic `NodeID` derivation (`bandiq-node-<hash>`), and cryptographic message signing for all measurement proofs.

---

### E. OpenWISP Monitoring (`openwisp/openwisp-monitoring`)
* **Primary Language & Focus:** Python / Shell; network management and monitoring daemon for OpenWrt and edge appliances.
* **Architecture:** Edge agent periodically collects telemetry, pushes metrics to a centralized controller, and utilizes a local persistent queue to buffer metrics during network outages.
* **License:** GPL v3.
* **Security & Platform Assessment:**
  - *Strengths:* Battle-tested offline resilience and spool-and-forward architecture. Prevents data loss during edge connectivity interruptions.
  - *Replay Protection:* Enforces monotonically increasing sequence numbers and server-validated timestamps.
* **Takeaways for BANDIQ:**
  - Built `internal/spool`: a persistent, on-disk FIFO spool queue in `.bandiq/spool/` that automatically buffers signed reports during backend downtime and flushes them sequentially with backoff when connectivity restores.

---

## 2. BANDIQ System Architecture

```
+-----------------------------------------------------------------------------------+
|                            BANDIQ Node Agent (Host)                               |
|                                                                                   |
|  +--------------------+    +--------------------+    +-------------------------+  |
|  | internal/identity  |    |  internal/consent  |    |     internal/system     |  |
|  | - Ed25519 Keypair  |    | - Explicit Opt-In  |    | - Windows IP Helper API |  |
|  | - Node ID Derived  |    | - Safety Gate      |    | - CPU, RAM, Uptime      |  |
|  | - Cryptographic    |    | - Relay Forbidden  |    | - Adapters & Counters   |  |
|  |   Signatures       |    |   by default       |    +-------------------------+  |
|  +--------------------+    +--------------------+                 |               |
|            |                         |                            v               |
|            v                         v               +-------------------------+  |
|  +----------------------------------------------+    |     internal/report     |  |
|  |             internal/measure                 |--->| - Canonical JSON Hash   |  |
|  | - Latency / RTT Probes (HTTP HEAD)           |    | - Ed25519 Signed Proof  |  |
|  | - RFC 3550 Statistical Jitter                |    | - Replay Sequence & ID  |  |
|  | - Chunked Download & Upload Throughput       |    | - Distinguishes Active  |  |
|  | - Daily Data Cap & Bandwidth Throttler       |    |   vs Served Bytes       |  |
|  +----------------------------------------------+    +-------------------------+  |
|                                                                   |               |
|  +------------------------+                          Offline?     v               |
|  |     internal/admin     |                       +----------------------------+  |
|  | - Bound to 127.0.0.1   |                       |      internal/spool        |  |
|  | - Power & Caps API     |                       | - On-disk FIFO Queue       |  |
|  | - Health Status        |                       | - Atomic Write & Commit    |  |
|  +------------------------+                       +----------------------------+  |
|                                                                   | Online        |
|                                                                   v               |
+-------------------------------------------------------------------|---------------+
                                                                    |
                                        HTTP POST /api/v1/reports/submit
                                        Headers: X-BANDIQ-Signature, Sequence
                                                                    |
                                                                    v
+-----------------------------------------------------------------------------------+
|                        BANDIQ Verification Backend                                |
|                                                                                   |
|  - Validates Node Registration & Public Key                                       |
|  - Replay Defense: Enforces Sequence > LastSequence & Max Timestamp Drift < 5 min |
|  - Verifies Ed25519 Cryptographic Signature over Canonical Payload                |
|  - Stores Proof-of-Bandwidth Attestation for Reward Accounting                    |
+-----------------------------------------------------------------------------------+
                                    |
                                    v
+-----------------------------------------------------------------------------------+
|                        Sidra Chain (Chain ID: 97453)                              |
|                                                                                   |
|  - BNDQ Token Contract: 0x6d1537bEE4F1EE3f55F4BC9319Aa40252cd39133                |
|  - Node Operator Wallet Association                                               |
+-----------------------------------------------------------------------------------+
```

---

## 3. Traffic Segregation Model

In DePIN network architectures, distinguishing between **synthetic benchmarking traffic** and **useful user workload (relayed) traffic** is paramount to prevent reward spoofing or bandwidth exhaustion:

| Metric Field | Definition | How Collected | Reward Classification |
|---|---|---|---|
| `active_test_bytes_in` | Synthetic test bytes downloaded | HTTP chunked speedtest probe | Network capacity benchmark (unearned / overhead) |
| `active_test_bytes_out`| Synthetic test bytes uploaded | HTTP chunked POST probe | Network capacity benchmark (unearned / overhead) |
| `served_bytes_in` | Real client request bytes relayed | Proxy/Relay service (disabled in v0.1.0) | Earned bandwidth service |
| `served_bytes_out` | Real client response bytes served | Proxy/Relay service (disabled in v0.1.0) | Earned bandwidth service |
| `interface_counters` | Total hardware adapter bytes | OS IP Helper API | Passive verification sanity check |
