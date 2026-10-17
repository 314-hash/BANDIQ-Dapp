// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "./interfaces/IERC20.sol";
import "./BandiqNodeRegistry.sol";

/**
 * @title BandiqRewardDistributor
 * @notice Epoch-based reward distributor for BANDIQ DePIN Bandwidth nodes on Sidra Chain.
 * Distributes rewards exclusively from treasury funding and protocol fees — NEVER MINTS SUPPLY.
 */
contract BandiqRewardDistributor {
    struct RewardClaim {
        uint256 epoch;
        string nodeId;
        address beneficiary;
        uint256 amountBndq;
        uint256 nonce;
    }

    IERC20 public immutable bndqToken;
    BandiqNodeRegistry public immutable nodeRegistry;

    address public owner;
    address public authorizedSigner;
    bool public paused;

    // EIP-712 Domain Separator & Typehash
    bytes32 public constant EIP712_DOMAIN_TYPEHASH = keccak256(
        "EIP712Domain(string name,string version,uint256 chainId,address verifyingContract)"
    );

    bytes32 public constant REWARD_CLAIM_TYPEHASH = keccak256(
        "RewardClaim(uint256 epoch,string nodeId,address beneficiary,uint256 amountBndq,uint256 nonce)"
    );

    bytes32 public immutable DOMAIN_SEPARATOR;

    // Double claim defense: epoch => nodeHash => hasClaimed
    mapping(uint256 => mapping(bytes32 => bool)) public epochNodeClaimed;

    // Cumulative stats
    uint256 public totalRewardsDistributed;
    uint256 public totalClaimsCount;

    // Events
    event RewardClaimed(
        uint256 indexed epoch,
        bytes32 indexed nodeIdHash,
        string nodeId,
        address indexed beneficiary,
        uint256 amountBndq,
        uint256 nonce
    );

    event FundsDeposited(address indexed depositor, uint256 amount);
    event AuthorizedSignerUpdated(address indexed oldSigner, address indexed newSigner);
    event OwnershipTransferred(address indexed previousOwner, address indexed newOwner);

    modifier onlyOwner() {
        require(msg.sender == owner, "BandiqRewardDistributor: Caller is not owner");
        _;
    }

    modifier whenNotPaused() {
        require(!paused, "BandiqRewardDistributor: Distributor is paused");
        _;
    }

    constructor(
        address _bndqToken,
        address _nodeRegistry,
        address _authorizedSigner,
        uint256 chainId
    ) {
        require(_bndqToken != address(0), "Invalid token address");
        require(_nodeRegistry != address(0), "Invalid registry address");
        require(_authorizedSigner != address(0), "Invalid signer address");

        bndqToken = IERC20(_bndqToken);
        nodeRegistry = BandiqNodeRegistry(_nodeRegistry);
        authorizedSigner = _authorizedSigner;
        owner = msg.sender;

        DOMAIN_SEPARATOR = keccak256(
            abi.encode(
                EIP712_DOMAIN_TYPEHASH,
                keccak256(bytes("BANDIQ Reward Distributor")),
                keccak256(bytes("1")),
                chainId,
                address(this)
            )
        );

        emit OwnershipTransferred(address(0), msg.sender);
    }

    /**
     * @notice Construct the full EIP-712 digest for a reward claim.
     */
    function getClaimDigest(RewardClaim calldata claim) public view returns (bytes32) {
        bytes32 structHash = keccak256(
            abi.encode(
                REWARD_CLAIM_TYPEHASH,
                claim.epoch,
                keccak256(bytes(claim.nodeId)),
                claim.beneficiary,
                claim.amountBndq,
                claim.nonce
            )
        );

        return keccak256(
            abi.encodePacked(
                "\x19\x01",
                DOMAIN_SEPARATOR,
                structHash
            )
        );
    }

    /**
     * @dev Internal signature recovery helper to keep stack depth optimal.
     */
    function _recoverSigner(
        bytes32 digest,
        bytes calldata signature
    ) internal pure returns (address) {
        require(signature.length == 65, "Invalid signature length");
        bytes32 r;
        bytes32 s;
        uint8 v;
        assembly {
            r := calldataload(signature.offset)
            s := calldataload(add(signature.offset, 32))
            v := byte(0, calldataload(add(signature.offset, 64)))
        }
        if (v < 27) v += 27;
        return ecrecover(digest, v, r, s);
    }

    /**
     * @notice Operators claim their verified epoch rewards by presenting an authorized cryptographic attestation.
     */
    function claimReward(
        RewardClaim calldata claim,
        bytes calldata signature
    ) external whenNotPaused {
        require(claim.beneficiary != address(0), "Invalid beneficiary");
        require(claim.amountBndq > 0, "Reward amount must be > 0");

        bytes32 nodeHash = keccak256(bytes(claim.nodeId));
        require(!epochNodeClaimed[claim.epoch][nodeHash], "Reward already claimed for this epoch");
        require(nodeRegistry.isNodeEligible(claim.nodeId), "Node is suspended or ineligible");

        // Verify available contract balance (No minting!)
        require(bndqToken.balanceOf(address(this)) >= claim.amountBndq, "Reward pool balance insufficient");

        // Verify signature
        bytes32 digest = getClaimDigest(claim);
        address recoveredSigner = _recoverSigner(digest, signature);
        require(recoveredSigner == authorizedSigner, "Invalid reward authorization signature");

        // Record claimed
        epochNodeClaimed[claim.epoch][nodeHash] = true;
        totalRewardsDistributed += claim.amountBndq;
        totalClaimsCount++;

        // Transfer reward from existing funded balance
        require(bndqToken.transfer(claim.beneficiary, claim.amountBndq), "Reward transfer failed");

        emit RewardClaimed(claim.epoch, nodeHash, claim.nodeId, claim.beneficiary, claim.amountBndq, claim.nonce);
    }

    /**
     * @notice Query available reward balance in the contract.
     */
    function getDistributorPoolBalance() external view returns (uint256) {
        return bndqToken.balanceOf(address(this));
    }

    /**
     * @notice Update authorized signer for reward allocations.
     */
    function setAuthorizedSigner(address newSigner) external onlyOwner {
        require(newSigner != address(0), "Zero address");
        emit AuthorizedSignerUpdated(authorizedSigner, newSigner);
        authorizedSigner = newSigner;
    }

    /**
     * @notice Emergency circuit breaker toggle.
     */
    function setPaused(bool _paused) external onlyOwner {
        paused = _paused;
    }

    /**
     * @notice Transfer contract ownership.
     */
    function transferOwnership(address newOwner) external onlyOwner {
        require(newOwner != address(0), "Zero address");
        emit OwnershipTransferred(owner, newOwner);
        owner = newOwner;
    }
}
