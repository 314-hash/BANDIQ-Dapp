import { expect } from "chai";
import hardhat from "hardhat";
const { ethers } = hardhat;

describe("BANDIQ DePIN — Phase 4 End-to-End Integration Tests", function () {
  let bndqToken, registry, verifierContract, settlement, distributor;
  let owner, verifierSigner, consumer, provider, treasury, unauthorizedSigner;

  const chainId = 97453;
  const testNodeId = "bandiq-node-2d5cd8e1a89aac06c44c45d6e1908d61";
  const testPubKey = "0x" + "bb".repeat(32);
  const BYTES_PER_GB = 1073741824n; // 1024^3 bytes

  const voucherTypes = {
    SettlementVoucher: [
      { name: "voucherId", type: "bytes32" },
      { name: "nodeId", type: "string" },
      { name: "beneficiary", type: "address" },
      { name: "verifiedBytes", type: "uint256" },
      { name: "confidenceScore", type: "uint256" },
      { name: "issuedAt", type: "uint256" },
      { name: "expiresAt", type: "uint256" },
      { name: "nonce", type: "uint256" }
    ]
  };

  const rewardTypes = {
    RewardClaim: [
      { name: "epoch", type: "uint256" },
      { name: "nodeId", type: "string" },
      { name: "beneficiary", type: "address" },
      { name: "amountBndq", type: "uint256" },
      { name: "nonce", type: "uint256" }
    ]
  };

  beforeEach(async function () {
    [owner, verifierSigner, consumer, provider, treasury, unauthorizedSigner] = await ethers.getSigners();

    // 1. Deploy Mock BNDQ Token (1,000,000 BNDQ initial supply)
    const initialSupply = ethers.parseEther("1000000");
    const MockBNDQToken = await ethers.getContractFactory("MockBNDQToken");
    bndqToken = await MockBNDQToken.deploy(initialSupply);
    await bndqToken.waitForDeployment();

    // 2. Deploy Node Registry
    const BandiqNodeRegistry = await ethers.getContractFactory("BandiqNodeRegistry");
    registry = await BandiqNodeRegistry.deploy();
    await registry.waitForDeployment();

    // 3. Deploy Bandwidth Verifier
    const BandiqBandwidthVerifier = await ethers.getContractFactory("BandiqBandwidthVerifier");
    verifierContract = await BandiqBandwidthVerifier.deploy(verifierSigner.address, chainId);
    await verifierContract.waitForDeployment();

    // 4. Deploy Usage Settlement Contract
    const BandiqUsageSettlement = await ethers.getContractFactory("BandiqUsageSettlement");
    settlement = await BandiqUsageSettlement.deploy(
      await bndqToken.getAddress(),
      await registry.getAddress(),
      await verifierContract.getAddress(),
      treasury.address
    );
    await settlement.waitForDeployment();

    // 5. Deploy Reward Distributor Contract
    const BandiqRewardDistributor = await ethers.getContractFactory("BandiqRewardDistributor");
    distributor = await BandiqRewardDistributor.deploy(
      await bndqToken.getAddress(),
      await registry.getAddress(),
      verifierSigner.address,
      chainId
    );
    await distributor.waitForDeployment();

    // Fund consumer with 5,000 BNDQ and Distributor pool with 50,000 BNDQ
    await bndqToken.transfer(consumer.address, ethers.parseEther("5000"));
    await bndqToken.transfer(await distributor.getAddress(), ethers.parseEther("50000"));
  });

  describe("1. Wallet connection and correct network detection", function () {
    it("should confirm target chain ID matches Sidra Chain (97453)", async function () {
      const network = await ethers.provider.getNetwork();
      expect(network.chainId).to.equal(97453n);
    });

    it("should verify token metadata matches deployed $BNDQ token", async function () {
      expect(await bndqToken.name()).to.equal("BANDIQ Token");
      expect(await bndqToken.symbol()).to.equal("BNDQ");
      expect(await bndqToken.decimals()).to.equal(18n);
    });
  });

  describe("2. Node registration and operator identity", function () {
    it("should register node and link operator EVM wallet with Ed25519 identity", async function () {
      await expect(
        registry.connect(provider).registerNode(testNodeId, testPubKey, 0, "PH", 100)
      )
        .to.emit(registry, "NodeRegistered")
        .withArgs(testNodeId, ethers.keccak256(ethers.toUtf8Bytes(testNodeId)), provider.address, testPubKey, 0, "PH", 100);

      const nodeInfo = await registry.getNode(testNodeId);
      expect(nodeInfo.operator).to.equal(provider.address);
      expect(nodeInfo.ed25519PubKey).to.equal(testPubKey);
      expect(nodeInfo.status).to.equal(1); // ACTIVE
      expect(await registry.isNodeEligible(testNodeId)).to.be.true;
    });

    it("should prevent duplicate node ID registrations", async function () {
      await registry.connect(provider).registerNode(testNodeId, testPubKey, 0, "PH", 100);
      await expect(
        registry.connect(consumer).registerNode(testNodeId, testPubKey, 1, "US", 200)
      ).to.be.revertedWith("BandiqNodeRegistry: Node ID already registered");
    });
  });

  describe("3. Measurement submission and signature validation", function () {
    beforeEach(async function () {
      await registry.connect(provider).registerNode(testNodeId, testPubKey, 0, "PH", 100);
    });

    it("should validate and consume an authentic EIP-712 settlement voucher", async function () {
      const block = await ethers.provider.getBlock("latest");
      const voucher = {
        voucherId: ethers.keccak256(ethers.toUtf8Bytes("voucher-e2e-001")),
        nodeId: testNodeId,
        beneficiary: provider.address,
        verifiedBytes: 2147483648n, // 2 GB
        confidenceScore: 950n,       // 95.0%
        issuedAt: BigInt(block.timestamp),
        expiresAt: BigInt(block.timestamp + 3600),
        nonce: 1n
      };

      const domain = {
        name: "BANDIQ Proof-of-Bandwidth",
        version: "1",
        chainId: chainId,
        verifyingContract: await verifierContract.getAddress()
      };

      const signature = await verifierSigner.signTypedData(domain, voucherTypes, voucher);
      const recoveredSigner = await verifierContract.recoverSigner(voucher, signature);
      expect(recoveredSigner).to.equal(verifierSigner.address);

      expect(await verifierContract.verifyAndConsumeVoucher.staticCall(voucher, signature)).to.be.true;
    });
  });

  describe("4. Rejection of fabricated, stale, duplicate, and replayed reports", function () {
    beforeEach(async function () {
      await registry.connect(provider).registerNode(testNodeId, testPubKey, 0, "PH", 100);
    });

    it("should REJECT vouchers signed by unauthorized or forged keys", async function () {
      const block = await ethers.provider.getBlock("latest");
      const voucher = {
        voucherId: ethers.keccak256(ethers.toUtf8Bytes("voucher-forge-001")),
        nodeId: testNodeId,
        beneficiary: provider.address,
        verifiedBytes: 1073741824n,
        confidenceScore: 900n,
        issuedAt: BigInt(block.timestamp),
        expiresAt: BigInt(block.timestamp + 3600),
        nonce: 1n
      };

      const domain = {
        name: "BANDIQ Proof-of-Bandwidth",
        version: "1",
        chainId: chainId,
        verifyingContract: await verifierContract.getAddress()
      };

      // Signed by unauthorized attacker
      const forgedSignature = await unauthorizedSigner.signTypedData(domain, voucherTypes, voucher);
      await expect(
        verifierContract.verifyAndConsumeVoucher(voucher, forgedSignature)
      ).to.be.revertedWith("BandiqBandwidthVerifier: Signer is not an authorized verifier");
    });

    it("should REJECT duplicate voucher redemption (Replay attack)", async function () {
      const block = await ethers.provider.getBlock("latest");
      const voucher = {
        voucherId: ethers.keccak256(ethers.toUtf8Bytes("voucher-replay-001")),
        nodeId: testNodeId,
        beneficiary: provider.address,
        verifiedBytes: 1073741824n,
        confidenceScore: 900n,
        issuedAt: BigInt(block.timestamp),
        expiresAt: BigInt(block.timestamp + 3600),
        nonce: 1n
      };

      const domain = {
        name: "BANDIQ Proof-of-Bandwidth",
        version: "1",
        chainId: chainId,
        verifyingContract: await verifierContract.getAddress()
      };

      const signature = await verifierSigner.signTypedData(domain, voucherTypes, voucher);
      await verifierContract.verifyAndConsumeVoucher(voucher, signature);

      // Attempt replay
      await expect(
        verifierContract.verifyAndConsumeVoucher(voucher, signature)
      ).to.be.revertedWith("BandiqBandwidthVerifier: Voucher already redeemed (Replay attack)");
    });

    it("should REJECT expired vouchers", async function () {
      const block = await ethers.provider.getBlock("latest");
      const voucher = {
        voucherId: ethers.keccak256(ethers.toUtf8Bytes("voucher-expired-001")),
        nodeId: testNodeId,
        beneficiary: provider.address,
        verifiedBytes: 1073741824n,
        confidenceScore: 900n,
        issuedAt: BigInt(block.timestamp - 7200),
        expiresAt: BigInt(block.timestamp - 3600), // Expired 1 hour ago
        nonce: 1n
      };

      const domain = {
        name: "BANDIQ Proof-of-Bandwidth",
        version: "1",
        chainId: chainId,
        verifyingContract: await verifierContract.getAddress()
      };

      const signature = await verifierSigner.signTypedData(domain, voucherTypes, voucher);
      await expect(
        verifierContract.verifyAndConsumeVoucher(voucher, signature)
      ).to.be.revertedWith("BandiqBandwidthVerifier: Voucher has expired");
    });
  });

  describe("5. Usage-session completion and settlement eligibility", function () {
    const sessionId = ethers.keccak256(ethers.toUtf8Bytes("session-e2e-001"));
    const escrowAmount = ethers.parseEther("50");     // 50 BNDQ
    const ratePerGB = ethers.parseEther("10");        // 10 BNDQ per GB
    const maxBytes = 5n * BYTES_PER_GB;               // 5 GB max

    beforeEach(async function () {
      await registry.connect(provider).registerNode(testNodeId, testPubKey, 0, "PH", 100);
      await bndqToken.connect(consumer).approve(await settlement.getAddress(), escrowAmount);
      await settlement.connect(consumer).createSessionEscrow(
        sessionId,
        testNodeId,
        provider.address,
        maxBytes,
        escrowAmount,
        ratePerGB,
        86400
      );
    });

    it("should lock tokens into escrow and emit SessionEscrowCreated", async function () {
      const session = await settlement.sessions(sessionId);
      expect(session.status).to.equal(1); // ACTIVE
      expect(session.escrowAmountBndq).to.equal(escrowAmount);
      expect(session.consumer).to.equal(consumer.address);
      expect(session.providerBeneficiary).to.equal(provider.address);
    });

    it("should settle verified usage, disburse provider earnings, collect protocol fee, and refund remaining escrow", async function () {
      const deliveredBytes = 2n * BYTES_PER_GB; // 2 GB delivered
      const block = await ethers.provider.getBlock("latest");

      const voucher = {
        voucherId: ethers.keccak256(ethers.toUtf8Bytes("voucher-settle-001")),
        nodeId: testNodeId,
        beneficiary: provider.address,
        verifiedBytes: deliveredBytes,
        confidenceScore: 980n,
        issuedAt: BigInt(block.timestamp),
        expiresAt: BigInt(block.timestamp + 3600),
        nonce: 1n
      };

      const domain = {
        name: "BANDIQ Proof-of-Bandwidth",
        version: "1",
        chainId: chainId,
        verifyingContract: await verifierContract.getAddress()
      };

      const signature = await verifierSigner.signTypedData(domain, voucherTypes, voucher);

      const providerBefore = await bndqToken.balanceOf(provider.address);
      const treasuryBefore = await bndqToken.balanceOf(treasury.address);
      const consumerBefore = await bndqToken.balanceOf(consumer.address);

      await expect(
        settlement.connect(provider).settleSession(sessionId, voucher, signature)
      )
        .to.emit(settlement, "SessionSettled");

      // Gross: 2 GB * 10 BNDQ = 20 BNDQ
      // Protocol fee: 1% of 20 = 0.2 BNDQ
      // Provider payout: 20 - 0.2 = 19.8 BNDQ
      // Consumer refund: 50 - 20 = 30 BNDQ
      const providerAfter = await bndqToken.balanceOf(provider.address);
      const treasuryAfter = await bndqToken.balanceOf(treasury.address);
      const consumerAfter = await bndqToken.balanceOf(consumer.address);

      expect(providerAfter - providerBefore).to.equal(ethers.parseEther("19.8"));
      expect(treasuryAfter - treasuryBefore).to.equal(ethers.parseEther("0.2"));
      expect(consumerAfter - consumerBefore).to.equal(ethers.parseEther("30.0"));

      const sessionAfter = await settlement.sessions(sessionId);
      expect(sessionAfter.status).to.equal(2); // SETTLED
    });
  });

  describe("6. Duplicate-claim prevention and reward caps", function () {
    const epoch = 1n;
    const rewardAmount = ethers.parseEther("25");

    beforeEach(async function () {
      await registry.connect(provider).registerNode(testNodeId, testPubKey, 0, "PH", 100);
    });

    it("should allow operator to claim verified epoch reward", async function () {
      const claim = {
        epoch: epoch,
        nodeId: testNodeId,
        beneficiary: provider.address,
        amountBndq: rewardAmount,
        nonce: 1n
      };

      const domain = {
        name: "BANDIQ Reward Distributor",
        version: "1",
        chainId: chainId,
        verifyingContract: await distributor.getAddress()
      };

      const signature = await verifierSigner.signTypedData(domain, rewardTypes, claim);
      const providerBefore = await bndqToken.balanceOf(provider.address);

      await expect(distributor.connect(provider).claimReward(claim, signature))
        .to.emit(distributor, "RewardClaimed")
        .withArgs(epoch, ethers.keccak256(ethers.toUtf8Bytes(testNodeId)), testNodeId, provider.address, rewardAmount, 1n);

      const providerAfter = await bndqToken.balanceOf(provider.address);
      expect(providerAfter - providerBefore).to.equal(rewardAmount);
    });

    it("should REJECT duplicate claim in the same epoch", async function () {
      const claim = {
        epoch: epoch,
        nodeId: testNodeId,
        beneficiary: provider.address,
        amountBndq: rewardAmount,
        nonce: 1n
      };

      const domain = {
        name: "BANDIQ Reward Distributor",
        version: "1",
        chainId: chainId,
        verifyingContract: await distributor.getAddress()
      };

      const signature = await verifierSigner.signTypedData(domain, rewardTypes, claim);
      await distributor.connect(provider).claimReward(claim, signature);

      // Attempt duplicate claim
      await expect(
        distributor.connect(provider).claimReward(claim, signature)
      ).to.be.revertedWith("Reward already claimed for this epoch");
    });

    it("should REJECT claims exceeding available pool balance (No Minting Guard)", async function () {
      const hugeReward = ethers.parseEther("100000"); // Exceeds 50,000 BNDQ funded pool
      const claim = {
        epoch: epoch,
        nodeId: testNodeId,
        beneficiary: provider.address,
        amountBndq: hugeReward,
        nonce: 1n
      };

      const domain = {
        name: "BANDIQ Reward Distributor",
        version: "1",
        chainId: chainId,
        verifyingContract: await distributor.getAddress()
      };

      const signature = await verifierSigner.signTypedData(domain, rewardTypes, claim);

      await expect(
        distributor.connect(provider).claimReward(claim, signature)
      ).to.be.revertedWith("Reward pool balance insufficient");
    });
  });

  describe("7. Pausing, revoked permissions, and failed token transfers", function () {
    it("should respect pause controls in BandiqNodeRegistry", async function () {
      await registry.setPaused(true);
      await expect(
        registry.connect(provider).registerNode(testNodeId, testPubKey, 0, "PH", 100)
      ).to.be.revertedWith("BandiqNodeRegistry: Registry is paused");

      await registry.setPaused(false);
      await expect(
        registry.connect(provider).registerNode(testNodeId, testPubKey, 0, "PH", 100)
      ).to.emit(registry, "NodeRegistered");
    });

    it("should reject vouchers when verifier authorization is revoked", async function () {
      await registry.connect(provider).registerNode(testNodeId, testPubKey, 0, "PH", 100);
      const block = await ethers.provider.getBlock("latest");
      const voucher = {
        voucherId: ethers.keccak256(ethers.toUtf8Bytes("voucher-revoked-001")),
        nodeId: testNodeId,
        beneficiary: provider.address,
        verifiedBytes: 1073741824n,
        confidenceScore: 900n,
        issuedAt: BigInt(block.timestamp),
        expiresAt: BigInt(block.timestamp + 3600),
        nonce: 1n
      };

      const domain = {
        name: "BANDIQ Proof-of-Bandwidth",
        version: "1",
        chainId: chainId,
        verifyingContract: await verifierContract.getAddress()
      };

      const signature = await verifierSigner.signTypedData(domain, voucherTypes, voucher);

      // Revoke verifier authorization
      await verifierContract.setVerifierAuthorization(verifierSigner.address, false);

      await expect(
        verifierContract.verifyAndConsumeVoucher(voucher, signature)
      ).to.be.revertedWith("BandiqBandwidthVerifier: Signer is not an authorized verifier");
    });

    it("should fail escrow creation if token approval / allowance is insufficient", async function () {
      await registry.connect(provider).registerNode(testNodeId, testPubKey, 0, "PH", 100);
      const sessionId = ethers.keccak256(ethers.toUtf8Bytes("session-no-allowance"));

      // Do NOT approve BNDQ
      await expect(
        settlement.connect(consumer).createSessionEscrow(
          sessionId,
          testNodeId,
          provider.address,
          BYTES_PER_GB,
          ethers.parseEther("10"),
          ethers.parseEther("10"),
          3600
        )
      ).to.be.revertedWith("Allowance exceeded");
    });
  });

  describe("8. Dashboard transaction history and explorer links", function () {
    it("should verify Sidra Explorer URL formatting for transactions and contracts", function () {
      const explorerBase = "https://explorer.sidrachain.com";
      const ledgerBase = "https://ledger.sidrachain.com";
      const sampleTxHash = "0x8c99e8fa21eb10b68f5dc7d9b02482b4e8916afac47dd9897f3429155bf2d612";
      const sampleAddress = "0x19b39b7de32F0d690D9286F8E17B593EE4019160";

      const txUrl = `${explorerBase}/tx/${sampleTxHash}`;
      const addrUrl = `${explorerBase}/address/${sampleAddress}`;
      const ledgerTxUrl = `${ledgerBase}/tx/${sampleTxHash}`;

      expect(txUrl).to.include("https://explorer.sidrachain.com/tx/0x");
      expect(addrUrl).to.include("https://explorer.sidrachain.com/address/0x");
      expect(ledgerTxUrl).to.include("https://ledger.sidrachain.com/tx/0x");
    });
  });
});
