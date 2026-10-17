// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "./interfaces/IERC20.sol";
import "./BandiqNodeRegistry.sol";
import "./BandiqBandwidthVerifier.sol";

/**
 * @title BandiqUsageSettlement
 * @notice Bandwidth consumer escrow and provider settlement contract on Sidra Chain.
 * Settles verified bandwidth delivery using the existing $BNDQ token
 * (0x6d1537bEE4F1EE3f55F4BC9319Aa40252cd39133) without minting new supply.
 */
contract BandiqUsageSettlement {
    enum SessionStatus {
        NONE,
        ACTIVE,
        SETTLED,
        CANCELLED
    }

    struct BandwidthSession {
        bytes32 sessionId;
        string nodeId;
        address consumer;
        address providerBeneficiary;
        uint256 maxBytes;
        uint256 escrowAmountBndq;
        uint256 ratePerGigabyteBndq; // Price in BNDQ per 1 GB (1e9 bytes)
        uint64 createdAt;
        uint64 expiresAt;
        SessionStatus status;
    }

    IERC20 public immutable bndqToken;
    BandiqNodeRegistry public immutable nodeRegistry;
    BandiqBandwidthVerifier public immutable verifierContract;

    address public owner;
    address public treasury;
    uint256 public protocolFeeBps; // 100 = 1.0%
    bool public paused;

    // Mutex guard for reentrancy defense
    uint256 private _status;
    uint256 private constant _NOT_ENTERED = 1;
    uint256 private constant _ENTERED = 2;

    // Sessions registry: sessionId => BandwidthSession
    mapping(bytes32 => BandwidthSession) public sessions;

    // Total metrics tracked on-chain
    uint256 public totalSettledBytes;
    uint256 public totalSettledBndq;
    uint256 public totalSessionsCreated;

    // 1 Gigabyte in bytes
    uint256 public constant BYTES_PER_GB = 1073741824; // 1024^3 bytes

    // Events
    event SessionEscrowCreated(
        bytes32 indexed sessionId,
        string indexed nodeId,
        address indexed consumer,
        address providerBeneficiary,
        uint256 escrowAmountBndq,
        uint256 ratePerGigabyteBndq,
        uint64 expiresAt
    );

    event SessionSettled(
        bytes32 indexed sessionId,
        string indexed nodeId,
        address indexed provider,
        uint256 deliveredBytes,
        uint256 providerPaidBndq,
        uint256 consumerRefundBndq,
        uint256 protocolFeeBndq
    );

    event SessionCancelled(bytes32 indexed sessionId, address indexed consumer, uint256 refundedBndq);
    event ProtocolFeeUpdated(uint256 oldFeeBps, uint256 newFeeBps);
    event TreasuryUpdated(address indexed oldTreasury, address indexed newTreasury);
    event OwnershipTransferred(address indexed previousOwner, address indexed newOwner);

    modifier nonReentrant() {
        require(_status != _ENTERED, "ReentrancyGuard: reentrant call");
        _status = _ENTERED;
        _;
        _status = _NOT_ENTERED;
    }

    modifier onlyOwner() {
        require(msg.sender == owner, "BandiqUsageSettlement: Caller is not owner");
        _;
    }

    modifier whenNotPaused() {
        require(!paused, "BandiqUsageSettlement: Contract is paused");
        _;
    }

    constructor(
        address _bndqToken,
        address _nodeRegistry,
        address _verifierContract,
        address _treasury
    ) {
        require(_bndqToken != address(0), "Invalid token address");
        require(_nodeRegistry != address(0), "Invalid registry address");
        require(_verifierContract != address(0), "Invalid verifier address");
        require(_treasury != address(0), "Invalid treasury address");

        bndqToken = IERC20(_bndqToken);
        nodeRegistry = BandiqNodeRegistry(_nodeRegistry);
        verifierContract = BandiqBandwidthVerifier(_verifierContract);

        owner = msg.sender;
        treasury = _treasury;
        protocolFeeBps = 100; // 1% protocol fee
        _status = _NOT_ENTERED;

        emit OwnershipTransferred(address(0), msg.sender);
    }

    /**
     * @notice Consumers lock $BNDQ tokens into escrow for authorized bandwidth service.
     * @param sessionId Unique session identifier.
     * @param nodeId Registered target node ID providing the service.
     * @param providerBeneficiary Recipient wallet address of the node operator.
     * @param maxBytes Maximum expected byte transfer.
     * @param escrowAmountBndq Total BNDQ deposited to cover session cost.
     * @param ratePerGigabyteBndq Agreed rate per Gigabyte in BNDQ wei.
     * @param durationSeconds Session validity period (e.g. 86400 for 24h).
     */
    function createSessionEscrow(
        bytes32 sessionId,
        string calldata nodeId,
        address providerBeneficiary,
        uint256 maxBytes,
        uint256 escrowAmountBndq,
        uint256 ratePerGigabyteBndq,
        uint64 durationSeconds
    ) external nonReentrant whenNotPaused {
        require(sessionId != bytes32(0), "Invalid session ID");
        require(bytes(nodeId).length > 0, "Invalid node ID");
        require(providerBeneficiary != address(0), "Invalid provider address");
        require(escrowAmountBndq > 0, "Escrow amount must be > 0");
        require(ratePerGigabyteBndq > 0, "Rate must be > 0");
        require(durationSeconds >= 300, "Min session duration is 5 mins");
        require(sessions[sessionId].status == SessionStatus.NONE, "Session already exists");

        // Verify target node is active in the registry
        require(nodeRegistry.isNodeEligible(nodeId), "Target node is not active or eligible");

        // Transfer BNDQ from consumer to this contract
        bool success = bndqToken.transferFrom(msg.sender, address(this), escrowAmountBndq);
        require(success, "BNDQ escrow transferFrom failed");

        sessions[sessionId] = BandwidthSession({
            sessionId: sessionId,
            nodeId: nodeId,
            consumer: msg.sender,
            providerBeneficiary: providerBeneficiary,
            maxBytes: maxBytes,
            escrowAmountBndq: escrowAmountBndq,
            ratePerGigabyteBndq: ratePerGigabyteBndq,
            createdAt: uint64(block.timestamp),
            expiresAt: uint64(block.timestamp + durationSeconds),
            status: SessionStatus.ACTIVE
        });

        totalSessionsCreated++;

        emit SessionEscrowCreated(
            sessionId,
            nodeId,
            msg.sender,
            providerBeneficiary,
            escrowAmountBndq,
            ratePerGigabyteBndq,
            uint64(block.timestamp + durationSeconds)
        );
    }

    /**
     * @notice Settle an active session by presenting a verified Proof-of-Bandwidth voucher.
     * Reconciles delivered bytes, distributes provider earnings, and refunds unspent escrow.
     */
    function settleSession(
        bytes32 sessionId,
        BandiqBandwidthVerifier.SettlementVoucher calldata voucher,
        bytes calldata signature
    ) external nonReentrant whenNotPaused {
        BandwidthSession storage session = sessions[sessionId];
        require(session.status == SessionStatus.ACTIVE, "Session is not active");
        require(voucher.beneficiary == session.providerBeneficiary, "Beneficiary mismatch");

        // Verify node is still active
        require(nodeRegistry.isNodeEligible(session.nodeId), "Node is suspended or ineligible");

        // Verify and consume the cryptographic voucher (checks signatures, expiration, nonces, and replays)
        bool verified = verifierContract.verifyAndConsumeVoucher(voucher, signature);
        require(verified, "Voucher verification failed");

        // Calculate payable amount based on verified bytes delivered
        uint256 deliveredBytes = voucher.verifiedBytes;
        if (session.maxBytes > 0 && deliveredBytes > session.maxBytes) {
            deliveredBytes = session.maxBytes;
        }

        uint256 grossPayment = (deliveredBytes * session.ratePerGigabyteBndq) / BYTES_PER_GB;
        if (grossPayment > session.escrowAmountBndq) {
            grossPayment = session.escrowAmountBndq;
        }

        uint256 fee = (grossPayment * protocolFeeBps) / 10000;
        uint256 providerPayout = grossPayment - fee;
        uint256 consumerRefund = session.escrowAmountBndq - grossPayment;

        // Mark session as settled
        session.status = SessionStatus.SETTLED;
        totalSettledBytes += deliveredBytes;
        totalSettledBndq += grossPayment;

        // Disburse payments safely
        if (providerPayout > 0) {
            require(bndqToken.transfer(session.providerBeneficiary, providerPayout), "Provider transfer failed");
        }
        if (fee > 0) {
            require(bndqToken.transfer(treasury, fee), "Treasury fee transfer failed");
        }
        if (consumerRefund > 0) {
            require(bndqToken.transfer(session.consumer, consumerRefund), "Consumer refund transfer failed");
        }

        emit SessionSettled(
            sessionId,
            session.nodeId,
            session.providerBeneficiary,
            deliveredBytes,
            providerPayout,
            consumerRefund,
            fee
        );
    }

    /**
     * @notice Cancel an expired session and refund escrowed tokens back to the consumer.
     */
    function cancelExpiredSession(bytes32 sessionId) external nonReentrant {
        BandwidthSession storage session = sessions[sessionId];
        require(session.status == SessionStatus.ACTIVE, "Session not active");
        require(block.timestamp > session.expiresAt, "Session has not expired yet");
        require(
            msg.sender == session.consumer || msg.sender == owner,
            "Only consumer or owner can cancel"
        );

        uint256 refundAmount = session.escrowAmountBndq;
        session.status = SessionStatus.CANCELLED;

        require(bndqToken.transfer(session.consumer, refundAmount), "Refund transfer failed");
        emit SessionCancelled(sessionId, session.consumer, refundAmount);
    }

    /**
     * @notice Configure protocol fee basis points (max 500 = 5.0%).
     */
    function setProtocolFeeBps(uint256 newFeeBps) external onlyOwner {
        require(newFeeBps <= 500, "Max fee is 5%");
        uint256 oldFee = protocolFeeBps;
        protocolFeeBps = newFeeBps;
        emit ProtocolFeeUpdated(oldFee, newFeeBps);
    }

    /**
     * @notice Update treasury wallet for receiving protocol service fees.
     */
    function setTreasury(address newTreasury) external onlyOwner {
        require(newTreasury != address(0), "Zero address");
        emit TreasuryUpdated(treasury, newTreasury);
        treasury = newTreasury;
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
