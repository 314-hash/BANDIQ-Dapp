import { expect } from "chai";
import hardhat from "hardhat";
const { ethers } = hardhat;

describe("BandiqBandwidthVerifier", function () {
  let verifierContract;
  let owner, verifierSigner, unauthorizedSigner, beneficiary;
  const chainId = 97453;
  const testNodeId = "bandiq-node-2d5cd8e1a89aac06c44c45d6e1908d61";

  // EIP-712 types
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
    [owner, verifierSigner, unauthorizedSigner, beneficiary] = await ethers.getSigners();

    const BandiqBandwidthVerifier = await ethers.getContractFactory("BandiqBandwidthVerifier");
    verifierContract = await BandiqBandwidthVerifier.deploy(verifierSigner.address, chainId);
    await verifierContract.waitForDeployment();
  });

  async function createSignedVoucher(signer, overrides = {}) {
    const latestBlock = await ethers.provider.getBlock("latest");
    const currentTime = latestBlock.timestamp;

    const voucher = {
      voucherId: overrides.voucherId || ethers.keccak256(ethers.toUtf8Bytes("voucher-" + Math.random())),
      nodeId: overrides.nodeId || testNodeId,
      beneficiary: overrides.beneficiary || beneficiary.address,
      verifiedBytes: overrides.verifiedBytes || 10737418240n, // 10 GB
      confidenceScore: overrides.confidenceScore || 980n, // 98.0%
      issuedAt: overrides.issuedAt || BigInt(currentTime),
      expiresAt: overrides.expiresAt || BigInt(currentTime + 86400),
      nonce: overrides.nonce || 1n
    };

    const domain = {
      name: "BANDIQ Proof-of-Bandwidth",
      version: "1",
      chainId: chainId,
      verifyingContract: await verifierContract.getAddress()
    };

    const signature = await signer.signTypedData(domain, types, voucher);
    return { voucher, signature };
  }

  it("should initialize with authorized verifier and 70% min confidence", async function () {
    expect(await verifierContract.authorizedVerifiers(verifierSigner.address)).to.be.true;
    expect(await verifierContract.minConfidenceScore()).to.equal(700);
  });

  it("should verify and consume a valid voucher signed by authorized verifier", async function () {
    const { voucher, signature } = await createSignedVoucher(verifierSigner);

    await expect(verifierContract.verifyAndConsumeVoucher(voucher, signature))
      .to.emit(verifierContract, "VoucherConsumed")
      .withArgs(
        voucher.voucherId,
        ethers.keccak256(ethers.toUtf8Bytes(voucher.nodeId)),
        voucher.nodeId,
        voucher.beneficiary,
        voucher.verifiedBytes,
        voucher.nonce,
        verifierSigner.address
      );

    expect(await verifierContract.redeemedVouchers(voucher.voucherId)).to.be.true;
  });

  it("should REJECT duplicate voucher redemption (Replay Attack Defense)", async function () {
    const { voucher, signature } = await createSignedVoucher(verifierSigner);

    // First consumption succeeds
    await verifierContract.verifyAndConsumeVoucher(voucher, signature);

    // Replay attempt fails immediately
    await expect(
      verifierContract.verifyAndConsumeVoucher(voucher, signature)
    ).to.be.revertedWith("BandiqBandwidthVerifier: Voucher already redeemed (Replay attack)");
  });

  it("should REJECT out-of-order or rolled-back sequence nonces", async function () {
    // Voucher 1 with nonce = 5
    const v1 = await createSignedVoucher(verifierSigner, { nonce: 5n });
    await verifierContract.verifyAndConsumeVoucher(v1.voucher, v1.signature);

    // Voucher 2 with lower nonce = 4 fails
    const v2 = await createSignedVoucher(verifierSigner, { nonce: 4n });
    await expect(
      verifierContract.verifyAndConsumeVoucher(v2.voucher, v2.signature)
    ).to.be.revertedWith("BandiqBandwidthVerifier: Nonce out of order");

    // Voucher 3 with equal nonce = 5 fails
    const v3 = await createSignedVoucher(verifierSigner, { nonce: 5n });
    await expect(
      verifierContract.verifyAndConsumeVoucher(v3.voucher, v3.signature)
    ).to.be.revertedWith("BandiqBandwidthVerifier: Nonce out of order");
  });

  it("should REJECT vouchers signed by unauthorized or forged keys", async function () {
    const { voucher, signature } = await createSignedVoucher(unauthorizedSigner);

    await expect(
      verifierContract.verifyAndConsumeVoucher(voucher, signature)
    ).to.be.revertedWith("BandiqBandwidthVerifier: Signer is not an authorized verifier");
  });

  it("should REJECT expired vouchers", async function () {
    const latestBlock = await ethers.provider.getBlock("latest");
    const pastTime = BigInt(latestBlock.timestamp - 100);

    const { voucher, signature } = await createSignedVoucher(verifierSigner, {
      expiresAt: pastTime
    });

    await expect(
      verifierContract.verifyAndConsumeVoucher(voucher, signature)
    ).to.be.revertedWith("BandiqBandwidthVerifier: Voucher has expired");
  });

  it("should REJECT vouchers with confidence score below threshold", async function () {
    const { voucher, signature } = await createSignedVoucher(verifierSigner, {
      confidenceScore: 650n // 65.0% < 70.0%
    });

    await expect(
      verifierContract.verifyAndConsumeVoucher(voucher, signature)
    ).to.be.revertedWith("BandiqBandwidthVerifier: Confidence score below threshold");
  });
});
