import { expect } from "chai";
import hardhat from "hardhat";
const { ethers } = hardhat;

describe("BandiqRewardDistributor", function () {
  let bndqToken, registry, distributor;
  let owner, authorizedSigner, unauthorizedSigner, operator;

  const chainId = 97453;
  const testNodeId = "bandiq-node-2d5cd8e1a89aac06c44c45d6e1908d61";
  const testPubKey = "0x" + "a1".repeat(32);

  const types = {
    RewardClaim: [
      { name: "epoch", type: "uint256" },
      { name: "nodeId", type: "string" },
      { name: "beneficiary", type: "address" },
      { name: "amountBndq", type: "uint256" },
      { name: "nonce", type: "uint256" }
    ]
  };

  beforeEach(async function () {
    [owner, authorizedSigner, unauthorizedSigner, operator] = await ethers.getSigners();

    // 1. Deploy Mock BNDQ Token
    const initialSupply = ethers.parseEther("1000000");
    const MockBNDQToken = await ethers.getContractFactory("MockBNDQToken");
    bndqToken = await MockBNDQToken.deploy(initialSupply);
    await bndqToken.waitForDeployment();

    // 2. Deploy Node Registry & register operator node
    const BandiqNodeRegistry = await ethers.getContractFactory("BandiqNodeRegistry");
    registry = await BandiqNodeRegistry.deploy();
    await registry.waitForDeployment();

    await registry.connect(operator).registerNode(testNodeId, testPubKey, 0, "PH", 50);

    // 3. Deploy Reward Distributor
    const BandiqRewardDistributor = await ethers.getContractFactory("BandiqRewardDistributor");
    distributor = await BandiqRewardDistributor.deploy(
      await bndqToken.getAddress(),
      await registry.getAddress(),
      authorizedSigner.address,
      chainId
    );
    await distributor.waitForDeployment();

    // 4. Fund Distributor Pool with 10,000 BNDQ from owner (Zero minting)
    await bndqToken.transfer(await distributor.getAddress(), ethers.parseEther("10000"));
  });

  async function createSignedClaim(signer, epoch, nodeId, beneficiary, amountBndq, nonce) {
    const domain = {
      name: "BANDIQ Reward Distributor",
      version: "1",
      chainId: chainId,
      verifyingContract: await distributor.getAddress()
    };

    const claim = {
      epoch,
      nodeId,
      beneficiary,
      amountBndq,
      nonce
    };

    const signature = await signer.signTypedData(domain, types, claim);
    return { claim, signature };
  }

  it("should initialize with funded balance and correct signer", async function () {
    expect(await distributor.getDistributorPoolBalance()).to.equal(ethers.parseEther("10000"));
    expect(await distributor.authorizedSigner()).to.equal(authorizedSigner.address);
  });

  it("should allow operator to claim verified epoch reward", async function () {
    const epoch = 1n;
    const amount = ethers.parseEther("250");
    const nonce = 1n;

    const { claim, signature } = await createSignedClaim(
      authorizedSigner,
      epoch,
      testNodeId,
      operator.address,
      amount,
      nonce
    );

    const operatorBalBefore = await bndqToken.balanceOf(operator.address);
    const poolBalBefore = await distributor.getDistributorPoolBalance();

    await expect(distributor.claimReward(claim, signature))
      .to.emit(distributor, "RewardClaimed")
      .withArgs(
        epoch,
        ethers.keccak256(ethers.toUtf8Bytes(testNodeId)),
        testNodeId,
        operator.address,
        amount,
        nonce
      );

    expect((await bndqToken.balanceOf(operator.address)) - operatorBalBefore).to.equal(amount);
    expect(poolBalBefore - (await distributor.getDistributorPoolBalance())).to.equal(amount);

    // Total supply must remain strictly constant
    expect(await bndqToken.totalSupply()).to.equal(ethers.parseEther("1000000"));
  });

  it("should REJECT double claims in the same epoch", async function () {
    const epoch = 2n;
    const amount = ethers.parseEther("100");
    const nonce = 2n;

    const { claim, signature } = await createSignedClaim(
      authorizedSigner,
      epoch,
      testNodeId,
      operator.address,
      amount,
      nonce
    );

    // First claim succeeds
    await distributor.claimReward(claim, signature);

    // Second claim fails
    await expect(distributor.claimReward(claim, signature)).to.be.revertedWith(
      "Reward already claimed for this epoch"
    );
  });

  it("should REJECT reward claims with unauthorized or forged signatures", async function () {
    const epoch = 3n;
    const amount = ethers.parseEther("500");
    const nonce = 3n;

    const { claim, signature: forgedSignature } = await createSignedClaim(
      unauthorizedSigner,
      epoch,
      testNodeId,
      operator.address,
      amount,
      nonce
    );

    await expect(distributor.claimReward(claim, forgedSignature)).to.be.revertedWith(
      "Invalid reward authorization signature"
    );
  });

  it("should REJECT claims exceeding available pool balance", async function () {
    const epoch = 4n;
    const excessiveAmount = ethers.parseEther("50000"); // 50,000 > 10,000 available
    const nonce = 4n;

    const { claim, signature } = await createSignedClaim(
      authorizedSigner,
      epoch,
      testNodeId,
      operator.address,
      excessiveAmount,
      nonce
    );

    await expect(distributor.claimReward(claim, signature)).to.be.revertedWith(
      "Reward pool balance insufficient"
    );
  });
});
