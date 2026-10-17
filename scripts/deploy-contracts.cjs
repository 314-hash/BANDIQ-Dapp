const { ethers } = require("hardhat");
const fs = require("fs");
const path = require("path");

async function main() {
  console.log("==================================================================");
  console.log("Deploying BANDIQ DePIN Smart Contracts Suite to Sidra Chain");
  console.log("Target Chain ID: 97453 (Expected)");
  console.log("Existing BNDQ Token: 0x6d1537bEE4F1EE3f55F4BC9319Aa40252cd39133");
  console.log("==================================================================\n");

  const [deployer] = await ethers.getSigners();
  const network = await ethers.provider.getNetwork();
  console.log("Deployer Address:", deployer.address);
  console.log("Active Network Chain ID:", network.chainId.toString());

  // Existing BNDQ token address on Sidra Chain
  const BNDQ_TOKEN_ADDRESS = process.env.BNDQ_TOKEN_ADDRESS || "0x6d1537bEE4F1EE3f55F4BC9319Aa40252cd39133";
  const TREASURY_ADDRESS = process.env.TREASURY_ADDRESS || deployer.address;
  const VERIFIER_SIGNER_ADDRESS = process.env.VERIFIER_SIGNER_ADDRESS || deployer.address;

  // 1. Deploy Node Registry
  console.log("1. Deploying BandiqNodeRegistry...");
  const BandiqNodeRegistry = await ethers.getContractFactory("BandiqNodeRegistry");
  const nodeRegistry = await BandiqNodeRegistry.deploy();
  await nodeRegistry.waitForDeployment();
  const registryAddress = await nodeRegistry.getAddress();
  console.log("   -> BandiqNodeRegistry deployed at:", registryAddress);

  // 2. Deploy Bandwidth Verifier
  console.log("2. Deploying BandiqBandwidthVerifier...");
  const BandiqBandwidthVerifier = await ethers.getContractFactory("BandiqBandwidthVerifier");
  const bandwidthVerifier = await BandiqBandwidthVerifier.deploy(
    VERIFIER_SIGNER_ADDRESS,
    network.chainId
  );
  await bandwidthVerifier.waitForDeployment();
  const verifierAddress = await bandwidthVerifier.getAddress();
  console.log("   -> BandiqBandwidthVerifier deployed at:", verifierAddress);

  // 3. Deploy Usage Settlement
  console.log("3. Deploying BandiqUsageSettlement...");
  const BandiqUsageSettlement = await ethers.getContractFactory("BandiqUsageSettlement");
  const usageSettlement = await BandiqUsageSettlement.deploy(
    BNDQ_TOKEN_ADDRESS,
    registryAddress,
    verifierAddress,
    TREASURY_ADDRESS
  );
  await usageSettlement.waitForDeployment();
  const settlementAddress = await usageSettlement.getAddress();
  console.log("   -> BandiqUsageSettlement deployed at:", settlementAddress);

  // 4. Deploy Reward Distributor
  console.log("4. Deploying BandiqRewardDistributor...");
  const BandiqRewardDistributor = await ethers.getContractFactory("BandiqRewardDistributor");
  const rewardDistributor = await BandiqRewardDistributor.deploy(
    BNDQ_TOKEN_ADDRESS,
    registryAddress,
    VERIFIER_SIGNER_ADDRESS,
    network.chainId
  );
  await rewardDistributor.waitForDeployment();
  const distributorAddress = await rewardDistributor.getAddress();
  console.log("   -> BandiqRewardDistributor deployed at:", distributorAddress);

  const deploymentData = {
    network: network.name,
    chainId: network.chainId.toString(),
    timestamp: new Date().toISOString(),
    deployer: deployer.address,
    bndqTokenAddress: BNDQ_TOKEN_ADDRESS,
    treasuryAddress: TREASURY_ADDRESS,
    verifierSignerAddress: VERIFIER_SIGNER_ADDRESS,
    contracts: {
      BandiqNodeRegistry: registryAddress,
      BandiqBandwidthVerifier: verifierAddress,
      BandiqUsageSettlement: settlementAddress,
      BandiqRewardDistributor: distributorAddress
    }
  };

  const deploymentsDir = path.join(__dirname, "../deployments");
  if (!fs.existsSync(deploymentsDir)) {
    fs.mkdirSync(deploymentsDir, { recursive: true });
  }

  const outFilePath = path.join(deploymentsDir, `sidra_${network.chainId}.json`);
  fs.writeFileSync(outFilePath, JSON.stringify(deploymentData, null, 2));
  console.log("\nDeployment record saved to:", outFilePath);
}

main()
  .then(() => process.exit(0))
  .catch((error) => {
    console.error(error);
    process.exit(1);
  });
