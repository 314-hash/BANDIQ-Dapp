# BANDIQ DePIN Smart Contracts Suite — Architecture & Security Audit

**BANDIQ: Bandwidth + Intelligence**  
*Decentralized Physical Infrastructure Network (DePIN) on Sidra Chain*  
*Target Chain ID:* `97453` (`0x17cad` in Hex)  
*Existing BNDQ Token Address:* `0x6d1537bEE4F1EE3f55F4BC9319Aa40252cd39133`  

---

## 1. Executive Summary & Design Principles

The BANDIQ smart contract suite provides on-chain coordination, cryptographic voucher validation, and escrow-based token settlement for decentralized physical bandwidth services.

### Core Protocol Tenets:
1. **Supply Preservation (Zero Minting):** No new BNDQ tokens can ever be minted. All token disbursements originate from buyer escrows (`BandiqUsageSettlement.sol`) or protocol fee/treasury deposits (`BandiqRewardDistributor.sol`).
2. **Off-Chain Verification Separation:** Physical network traffic cannot be measured directly on the blockchain. The off-chain **Proof-of-Bandwidth Verification Service** acts as an independent cryptographic witness, issuing single-claim EIP-712 settlement vouchers.
3. **Replay & Reentrancy Defenses:** Every voucher is tied to a unique `voucherId`, enforced monotonic nonces per node, and protected by non-reentrant state machine guards.

---

## 2. Smart Contracts Architecture

```
                                  [ Consumer ]
                                       |
                 (1) Lock BNDQ Tokens  |
                                       v
                           +------------------------+
                           | BandiqUsageSettlement  |
                           +------------------------+
                               |                |
           (3) Validate Status |                | (4) Release BNDQ Payment
                               v                |     (Excess Refunded to Consumer)
      +--------------------+   |                v
      | BandiqNodeRegistry |   |         [ Node Operator ]
      +--------------------+   |
                               | (2) Settle With Signed Voucher
                               v
               +-----------------------------+
               |   BandiqBandwidthVerifier   |
               +-----------------------------+
                               ^
                               | (EIP-712 Voucher with Nonce)
             [ Authorized Verifier Daemon ]
```

---

## 3. Contracts Breakdown

### 1. `BandiqNodeRegistry.sol`
- **Purpose:** On-chain registry linking operator EVM wallet addresses to off-chain Ed25519 node identities.
- **Key Features:**
  - Enforces uniqueness of Node IDs (`keccak256(nodeId)`).
  - Binds 32-byte Ed25519 public key to operator wallet.
  - Tracks node service tier (`RESIDENTIAL`, `COMMERCIAL`, `COMMUNITY_WIFI`, `ENTERPRISE_RELAY`).
  - Allows operators to update capacity and region.
  - Administrative authority to suspend nodes flagged for hardware counter fraud.

### 2. `BandiqBandwidthVerifier.sol`
- **Purpose:** Cryptographic gatekeeper verifying off-chain Proof-of-Bandwidth vouchers on Sidra Chain.
- **Key Features:**
  - Full **EIP-712 structured data signing**:
    ```solidity
    struct SettlementVoucher {
        bytes32 voucherId;
        string nodeId;
        address beneficiary;
        uint256 verifiedBytes;
        uint256 confidenceScore; // e.g. 985 = 98.5%
        uint256 issuedAt;
        uint256 expiresAt;
        uint256 nonce;
    }
    ```
  - **Single-Claim Replay Protection:** Records `redeemedVouchers[voucherId] = true`. Duplicate submissions immediately revert.
  - **Monotonic Nonce Check:** Enforces `voucher.nonce > nodeLatestNonce[nodeId]`.
  - **Freshness Window:** Rejects expired vouchers (`block.timestamp <= expiresAt`).
  - **Confidence Threshold:** Enforces minimum required statistical confidence (default 70.0%).

### 3. `BandiqUsageSettlement.sol`
- **Purpose:** Non-custodial escrow and payment distribution for bandwidth service sessions.
- **Key Features:**
  - **Buyer Escrow:** Consumer locks $BNDQ tokens to reserve bandwidth capacity (`createSessionEscrow`).
  - **Voucher Settlement:** Verifies voucher with `BandiqBandwidthVerifier`, checks node status with `BandiqNodeRegistry`, and disburses funds based on actual verified bytes delivered.
  - **Automatic Refunds:** Any unspent escrowed tokens are immediately returned to the consumer.
  - **Protocol Fee:** Configurable fee (e.g. 1%) allocated to treasury to sustain community incentives.
  - **Expired Session Cancellation:** Allows 100% escrow refund if session times out without delivery.

### 4. `BandiqRewardDistributor.sol`
- **Purpose:** Periodic epoch rewards for high-uptime, compliant nodes.
- **Key Features:**
  - **Strictly Non-Minting:** Operates purely on funded balance deposited by treasury or protocol fee accrual.
  - **Double-Claim Prevention:** Tracks `epochNodeClaimed[epoch][nodeHash]`.
  - **EIP-712 Claim Signatures:** Verified against authorized reward signer.

---

## 4. Threat Matrix & Security Analysis

| Threat Vector | Attack Scenario | Mitigation in BANDIQ Contracts |
| :--- | :--- | :--- |
| **Voucher Replay Attack** | Attacker resubmits a previously redeemed voucher to drain escrow | `redeemedVouchers[voucherId]` check reverts duplicate claims immediately. |
| **Nonce Rollback** | Node attempts to submit older unspent vouchers | `nodeLatestNonce[nodeId]` enforces strictly increasing sequence numbers. |
| **Signature Forgery** | Malicious node fabricates their own bandwidth report voucher | EIP-712 domain separation and `authorizedVerifiers[recoveredSigner]` check. |
| **Reentrancy Attack** | Malicious receiver reenters settlement during payout | `nonReentrant` mutex guard applied to all settlement functions. |
| **Token Supply Inflation** | Contract mints extra BNDQ to pay rewards | No `mint()` function exists. All contracts interact via standard `IERC20`. |
| **Sybil Front-Running** | Third party tries to steal voucher payout | Voucher explicitly binds beneficiary wallet: `require(voucher.beneficiary == session.providerBeneficiary)`. |
| **Abandoned Escrow** | Provider disappears without delivering bandwidth | Consumer can cancel and receive 100% refund after `expiresAt`. |

---

## 5. Test Suite Verification (100% Passing)

```text
  BandiqBandwidthVerifier
    ✔ should initialize with authorized verifier and 70% min confidence
    ✔ should verify and consume a valid voucher signed by authorized verifier
    ✔ should REJECT duplicate voucher redemption (Replay Attack Defense)
    ✔ should REJECT out-of-order or rolled-back sequence nonces
    ✔ should REJECT vouchers signed by unauthorized or forged keys
    ✔ should REJECT expired vouchers
    ✔ should REJECT vouchers with confidence score below threshold

  BandiqNodeRegistry
    ✔ should initialize with deployer as owner
    ✔ should allow an operator to register a node successfully
    ✔ should reject duplicate node ID registration
    ✔ should reject registration with empty ID or zero public key
    ✔ should allow operator to update capacity and region
    ✔ should prevent non-operators from updating a node
    ✔ should allow owner to suspend a fraudulent node
    ✔ should allow operator to deregister their node

  BandiqRewardDistributor
    ✔ should initialize with funded balance and correct signer
    ✔ should allow operator to claim verified epoch reward
    ✔ should REJECT double claims in the same epoch
    ✔ should REJECT reward claims with unauthorized or forged signatures
    ✔ should REJECT claims exceeding available pool balance

  BandiqUsageSettlement
    ✔ should enforce fixed supply (Zero new minting)
    ✔ should create consumer escrow successfully
    ✔ should settle verified usage, disburse provider payout, deduct protocol fee, and refund excess
    ✔ should allow consumer to cancel and get 100% refund after session expires

  24 passing (4s)
```

---

## 6. How to Deploy to Sidra Chain

Run the PowerShell deployment script:

```powershell
.\scripts\deploy-contracts.ps1 -Network sidra -RpcUrl "https://node.sidrachain.com" -PrivateKey "0xYourDeployerPrivateKey"
```

Deployment metadata is automatically written to `deployments/sidra_97453.json`.
