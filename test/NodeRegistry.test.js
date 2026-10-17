import { expect } from "chai";
import hardhat from "hardhat";
const { ethers } = hardhat;

describe("BandiqNodeRegistry", function () {
  let registry;
  let owner, operator1, operator2, nonOperator;

  const testNodeId1 = "bandiq-node-2d5cd8e1a89aac06c44c45d6e1908d61";
  const testNodeId2 = "bandiq-node-99a81ff3e4b09c231456bcde43210987";
  const testPubKey1 = "0x" + "a1".repeat(32);
  const testPubKey2 = "0x" + "b2".repeat(32);

  beforeEach(async function () {
    [owner, operator1, operator2, nonOperator] = await ethers.getSigners();

    const BandiqNodeRegistry = await ethers.getContractFactory("BandiqNodeRegistry");
    registry = await BandiqNodeRegistry.deploy();
    await registry.waitForDeployment();
  });

  it("should initialize with deployer as owner", async function () {
    expect(await registry.owner()).to.equal(owner.address);
    expect(await registry.activeNodeCount()).to.equal(0);
    expect(await registry.totalRegisteredCount()).to.equal(0);
  });

  it("should allow an operator to register a node successfully", async function () {
    await expect(
      registry.connect(operator1).registerNode(
        testNodeId1,
        testPubKey1,
        0, // RESIDENTIAL
        "PH",
        50 // 50 Mbps
      )
    )
      .to.emit(registry, "NodeRegistered")
      .withArgs(
        testNodeId1,
        ethers.keccak256(ethers.toUtf8Bytes(testNodeId1)),
        operator1.address,
        testPubKey1,
        0,
        "PH",
        50
      );

    expect(await registry.activeNodeCount()).to.equal(1);
    expect(await registry.totalRegisteredCount()).to.equal(1);
    expect(await registry.isNodeEligible(testNodeId1)).to.be.true;

    const node = await registry.getNode(testNodeId1);
    expect(node.nodeId).to.equal(testNodeId1);
    expect(node.operator).to.equal(operator1.address);
    expect(node.ed25519PubKey).to.equal(testPubKey1);
    expect(node.serviceType).to.equal(0);
    expect(node.region).to.equal("PH");
    expect(node.maxCapacityMbps).to.equal(50);
    expect(node.status).to.equal(1); // ACTIVE
  });

  it("should reject duplicate node ID registration", async function () {
    await registry.connect(operator1).registerNode(testNodeId1, testPubKey1, 0, "PH", 50);

    await expect(
      registry.connect(operator2).registerNode(testNodeId1, testPubKey2, 1, "US", 100)
    ).to.be.revertedWith("BandiqNodeRegistry: Node ID already registered");
  });

  it("should reject registration with empty ID or zero public key", async function () {
    await expect(
      registry.connect(operator1).registerNode("", testPubKey1, 0, "PH", 50)
    ).to.be.revertedWith("BandiqNodeRegistry: Node ID cannot be empty");

    await expect(
      registry.connect(operator1).registerNode(testNodeId1, ethers.ZeroHash, 0, "PH", 50)
    ).to.be.revertedWith("BandiqNodeRegistry: Invalid Ed25519 public key");
  });

  it("should allow operator to update capacity and region", async function () {
    await registry.connect(operator1).registerNode(testNodeId1, testPubKey1, 0, "PH", 50);

    await registry.connect(operator1).updateNode(testNodeId1, 100, "APAC");

    const node = await registry.getNode(testNodeId1);
    expect(node.maxCapacityMbps).to.equal(100);
    expect(node.region).to.equal("APAC");
  });

  it("should prevent non-operators from updating a node", async function () {
    await registry.connect(operator1).registerNode(testNodeId1, testPubKey1, 0, "PH", 50);

    await expect(
      registry.connect(nonOperator).updateNode(testNodeId1, 200, "EU")
    ).to.be.revertedWith("BandiqNodeRegistry: Only operator can update node");
  });

  it("should allow owner to suspend a fraudulent node", async function () {
    await registry.connect(operator1).registerNode(testNodeId1, testPubKey1, 0, "PH", 50);
    expect(await registry.isNodeEligible(testNodeId1)).to.be.true;

    await registry.connect(owner).setNodeStatus(testNodeId1, 2, "Hardware counter fraud detected"); // SUSPENDED

    expect(await registry.isNodeEligible(testNodeId1)).to.be.false;
    expect(await registry.activeNodeCount()).to.equal(0);

    const node = await registry.getNode(testNodeId1);
    expect(node.status).to.equal(2); // SUSPENDED
  });

  it("should allow operator to deregister their node", async function () {
    await registry.connect(operator1).registerNode(testNodeId1, testPubKey1, 0, "PH", 50);
    expect(await registry.activeNodeCount()).to.equal(1);

    await registry.connect(operator1).deregisterNode(testNodeId1);

    expect(await registry.isNodeEligible(testNodeId1)).to.be.false;
    expect(await registry.activeNodeCount()).to.equal(0);
  });
});
