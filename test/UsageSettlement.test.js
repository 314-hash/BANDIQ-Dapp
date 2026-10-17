import { expect } from "chai";
import hardhat from "hardhat";
const { ethers } = hardhat;

describe("BandiqUsageSettlement", function () {
  let bndqToken, registry, verifierContract, settlement;
  let owner, verifierSigner, consumer, provider, treasury;

  const chainId = 97453;
  const testNodeId = "bandiq-node-2d5cd8e1a89aac06c44c45d6e1908d61";
  const testPubKey = "0x" + "a1".repeat(32);
  const BYTES_PER_GB = 1073741824n; // 1024^3 bytes

  const types = {
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

  beforeEach(async function () {
    [owner, verifierSigner, consumer, provider, treasury] = await ethers.getSigners();

    // 1. Deploy Mock BNDQ Token (1,000,000 BNDQ initial supply)
    const initialSupply = ethers.parseEther("1000000");
    const MockBNDQToken = await ethers.getContractFactory("MockBNDQToken");
    bndqToken = await MockBNDQToken.deploy(initialSupply);
    await bndqToken.waitForDeployment();

    // 2. Deploy Node Registry & register provider node
    const BandiqNodeRegistry = await ethers.getContractFactory("BandiqNodeRegistry");
    registry = await BandiqNodeRegistry.deploy();
    await registry.waitForDeployment();

    await registry.connect(provider).registerNode(testNodeId, testPubKey, 0, "PH", 100);

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

    // Fund consumer with 1,000 BNDQ
    await bndqToken.transfer(consumer.address, ethers.parseEther("1000"));
  });

  it("should enforce fixed supply (Zero new minting)", async function () {
    const supplyBefore = await bndqToken.totalSupply();
    expect(supplyBefore).to.equal(ethers.parseEther("1000000"));
  });

  it("should create consumer escrow successfully", async function () {
    const sessionId = ethers.keccak256(ethers.toUtf8Bytes("session-001"));
    const maxBytes = BYTES_PER_GB * 10n; // 10 GB
    const ratePerGb = ethers.parseEther("5"); // 5 BNDQ per GB
    const escrowAmount = ethers.parseEther("50"); // 50 BNDQ total

    // Consumer approves settlement contract
    await bndqToken.connect(consumer).approve(await settlement.getAddress(), escrowAmount);

    await expect(
      settlement.connect(consumer).createSessionEscrow(
        sessionId,
        testNodeId,
        provider.address,
        maxBytes,
        escrowAmount,
        ratePerGb,
        3600 // 1 hour
      )
    ).to.emit(settlement, "SessionEscrowCreated");

    expect(await bndqToken.balanceOf(await settlement.getAddress())).to.equal(escrowAmount);
    const session = await settlement.sessions(sessionId);
    expect(session.consumer).to.equal(consumer.address);
    expect(session.escrowAmountBndq).to.equal(escrowAmount);
    expect(session.status).to.equal(1); // ACTIVE
  });

  it("should settle verified usage, disburse provider payout, deduct protocol fee, and refund excess", async function () {
    const sessionId = ethers.keccak256(ethers.toUtf8Bytes("session-002"));
    const maxBytes = BYTES_PER_GB * 10n; // 10 GB max
    const ratePerGb = ethers.parseEther("10"); // 10 BNDQ per GB
    const escrowAmount = ethers.parseEther("100"); // 100 BNDQ deposited

    await bndqToken.connect(consumer).approve(await settlement.getAddress(), escrowAmount);
    await settlement.connect(consumer).createSessionEscrow(
      sessionId,
      testNodeId,
      provider.address,
      maxBytes,
      escrowAmount,
      ratePerGb,
      3600
    );

    // Node actually delivers 6 GB of traffic
    const deliveredBytes = BYTES_PER_GB * 6n;
    const latestBlock = await ethers.provider.getBlock("latest");
    const currentTime = latestBlock.timestamp;

    const voucher = {
      voucherId: sessionId,
      nodeId: testNodeId,
      beneficiary: provider.address,
      verifiedBytes: deliveredBytes,
      confidenceScore: 990n,
      issuedAt: BigInt(currentTime),
      expiresAt: BigInt(currentTime + 1800),
      nonce: 1n
    };

    const domain = {
      name: "BANDIQ Proof-of-Bandwidth",
      version: "1",
      chainId: chainId,
      verifyingContract: await verifierContract.getAddress()
    };

    const signature = await verifierSigner.signTypedData(domain, types, voucher);

    // Track balances before settlement
    const providerBalBefore = await bndqToken.balanceOf(provider.address);
    const consumerBalBefore = await bndqToken.balanceOf(consumer.address);
    const treasuryBalBefore = await bndqToken.balanceOf(treasury.address);

    // Settle session
    await settlement.settleSession(sessionId, voucher, signature);

    // Math check:
    // Gross = 6 GB * 10 BNDQ/GB = 60 BNDQ
    // Fee = 1% of 60 BNDQ = 0.6 BNDQ
    // Provider payout = 59.4 BNDQ
    // Consumer refund = 100 - 60 = 40 BNDQ
    const expectedGross = ethers.parseEther("60");
    const expectedFee = ethers.parseEther("0.6");
    const expectedProviderPayout = ethers.parseEther("59.4");
    const expectedRefund = ethers.parseEther("40");

    expect((await bndqToken.balanceOf(provider.address)) - providerBalBefore).to.equal(expectedProviderPayout);
    expect((await bndqToken.balanceOf(treasury.address)) - treasuryBalBefore).to.equal(expectedFee);
    expect((await bndqToken.balanceOf(consumer.address)) - consumerBalBefore).to.equal(expectedRefund);

    // Contract escrow balance should now be 0
    expect(await bndqToken.balanceOf(await settlement.getAddress())).to.equal(0);

    // Total token supply must remain strictly constant
    expect(await bndqToken.totalSupply()).to.equal(ethers.parseEther("1000000"));
  });

  it("should allow consumer to cancel and get 100% refund after session expires", async function () {
    const sessionId = ethers.keccak256(ethers.toUtf8Bytes("session-003"));
    const escrowAmount = ethers.parseEther("25");

    await bndqToken.connect(consumer).approve(await settlement.getAddress(), escrowAmount);
    await settlement.connect(consumer).createSessionEscrow(
      sessionId,
      testNodeId,
      provider.address,
      BYTES_PER_GB * 5n,
      escrowAmount,
      ethers.parseEther("5"),
      300 // 5 minutes
    );

    // Fast-forward time past expiration
    await ethers.provider.send("evm_increaseTime", [301]);
    await ethers.provider.send("evm_mine");

    const consumerBalBefore = await bndqToken.balanceOf(consumer.address);

    await settlement.connect(consumer).cancelExpiredSession(sessionId);

    expect((await bndqToken.balanceOf(consumer.address)) - consumerBalBefore).to.equal(escrowAmount);
  });
});
