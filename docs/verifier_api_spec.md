# BANDIQ Proof-of-Bandwidth Verifier — REST API Specification

Base URL: `http://localhost:8085` (or configured verifier host)  
Content-Type: `application/json`

---

## 1. Health & Status

### `GET /api/v1/health`
Returns verifier service operational status and verifier cryptographic identity.

**Response (200 OK):**
```json
{
  "status": "HEALTHY",
  "service": "BANDIQ Proof-of-Bandwidth Verifier",
  "verifier_identity": "bandiq-verifier-root-alpha",
  "timestamp": "2026-10-17T11:45:00Z"
}
```

---

## 2. Node Registration

### `POST /api/v1/nodes/register`
Registers a node identity and associates its public key before reports can be verified.

**Request Body:**
```json
{
  "node_id": "bandiq-node-2d5cd8e1a89aac06c44c45d6e1908d61",
  "operator_wallet": "0x6d1537bEE4F1EE3f55F4BC9319Aa40252cd39133",
  "public_key_hex": "f85938d325696512969682bb227b40c098a3bfae4d8d12a1b86dde90719aeb94",
  "software_version": "0.1.0"
}
```

**Response (201 Created):**
```json
{
  "status": "registered",
  "node_id": "bandiq-node-2d5cd8e1a89aac06c44c45d6e1908d61"
}
```

---

## 3. Report Verification & Settlement Voucher Issuance

### `POST /api/v1/reports/verify`
Submits a cryptographically signed measurement report to be evaluated through the 10 sequential verification gates.

**Request Body:**
Full signed `MeasurementReport` JSON:
```json
{
  "report_id": "a1b2c3d4-e5f6-4a1b-8c9d-0e1f2a3b4c5d",
  "node_id": "bandiq-node-2d5cd8e1a89aac06c44c45d6e1908d61",
  "operator_wallet": "0x6d1537bEE4F1EE3f55F4BC9319Aa40252cd39133",
  "sequence_number": 12,
  "timestamp": "2026-10-17T11:45:00.000Z",
  "software_version": "0.1.0",
  "test_server_id": "http://127.0.0.1:8080",
  "measurement_method": "HTTP_SPEEDTEST_V1",
  "metrics": {
    "download_mbps": 142.60,
    "upload_mbps": 52.80,
    "latency_ms": 16.40,
    "min_latency_ms": 14.10,
    "max_latency_ms": 19.80,
    "jitter_ms": 1.25,
    "packet_loss_pct": 0.0,
    "active_test_bytes_in": 15728640,
    "active_test_bytes_out": 5242880,
    "served_bytes_in": 0,
    "served_bytes_out": 0,
    "test_duration_ms": 5200
  },
  "interface_counters": {
    "interface_name": "Ethernet",
    "bytes_sent": 512400900,
    "bytes_recv": 1890200400,
    "operational_status": "UP"
  },
  "system_metrics": {
    "cpu_percent": 18.5
  },
  "signature": "8a7b6c5d4e3f..."
}
```

**Response (200 OK — Accepted):**
```json
{
  "report_id": "a1b2c3d4-e5f6-4a1b-8c9d-0e1f2a3b4c5d",
  "node_id": "bandiq-node-2d5cd8e1a89aac06c44c45d6e1908d61",
  "operator_wallet": "0x6d1537bEE4F1EE3f55F4BC9319Aa40252cd39133",
  "status": "ACCEPTED",
  "rejection_code": "NONE",
  "confidence_score": 0.95,
  "verified_throughput_mbps": 142.60,
  "verified_bytes": 0,
  "is_reward_eligible": false,
  "settlement_voucher": {
    "voucher_id": "e4f3a2b1-9c8d-4e5f-6a7b-8c9d0e1f2a3b",
    "epoch_id": 482,
    "node_id": "bandiq-node-2d5cd8e1a89aac06c44c45d6e1908d61",
    "operator_wallet": "0x6d1537bEE4F1EE3f55F4BC9319Aa40252cd39133",
    "verified_bytes": 0,
    "qos_score": 9500,
    "nonce": 1849,
    "issued_at": "2026-10-17T11:45:05Z",
    "expires_at": "2026-10-18T11:45:05Z",
    "verifier_identity": "bandiq-verifier-root-alpha",
    "signature": "7d6e5f4a..."
  },
  "evaluated_at": "2026-10-17T11:45:05Z",
  "details": "verification passed (Confidence: 0.95)"
}
```

**Response (409 Conflict — Replay Attack):**
```json
{
  "report_id": "a1b2c3d4-e5f6-4a1b-8c9d-0e1f2a3b4c5d",
  "node_id": "bandiq-node-2d5cd8e1a89aac06c44c45d6e1908d61",
  "status": "REJECTED",
  "rejection_code": "ERR_DUPLICATE_REPORT_ID",
  "confidence_score": 0.0,
  "details": "replay attack detected: report_id a1b2c3d4... has already been evaluated"
}
```

---

## 4. Audit Trail & Decisions

### `GET /api/v1/audit/decisions?limit=50`
Returns the recent verification log with machine-readable decisions.

---

## 5. Dispute & Appeal System

### `POST /api/v1/disputes`
Files a formal challenge for a rejected or pending report.

**Request Body:**
```json
{
  "report_id": "f9e8d7c6-b5a4-4f9e-8d7c-6b5a4f9e8d7c",
  "node_id": "bandiq-node-2d5cd8e1a89aac06c44c45d6e1908d61",
  "operator_wallet": "0x6d1537bEE4F1EE3f55F4BC9319Aa40252cd39133",
  "reason": "ISP line maintenance caused transient latency spike during benchmark"
}
```

**Response (201 Created):**
```json
{
  "dispute_id": "disp-1792208000000000",
  "report_id": "f9e8d7c6-b5a4-4f9e-8d7c-6b5a4f9e8d7c",
  "status": "OPEN",
  "submitted_at": "2026-10-17T11:46:00Z"
}
```
