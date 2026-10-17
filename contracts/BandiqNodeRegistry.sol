// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

/**
 * @title BandiqNodeRegistry
 * @notice On-chain registry for BANDIQ DePIN Bandwidth Nodes on Sidra Chain (Chain ID: 97453).
 * Links operator EVM wallets with off-chain Ed25519 node identities, service profiles,
 * and maintains network eligibility states.
 */
contract BandiqNodeRegistry {
    enum ServiceType {
        RESIDENTIAL,
        COMMERCIAL,
        COMMUNITY_WIFI,
        ENTERPRISE_RELAY
    }

    enum NodeStatus {
        PENDING,
        ACTIVE,
        SUSPENDED,
        DEREGISTERED
    }

    struct NodeInfo {
        string nodeId;
        address operator;
        bytes32 ed25519PubKey;
        ServiceType serviceType;
        string region;            // e.g. "PH", "US-EAST", "EU-WEST"
        uint32 maxCapacityMbps;   // Operator declared or benchmarked capacity
        uint64 registeredAt;
        NodeStatus status;
        bool exists;
    }

    address public owner;
    bool public paused;

    // Node ID hash => NodeInfo
    mapping(bytes32 => NodeInfo) private nodes;

    // Operator address => Array of node ID hashes
    mapping(address => bytes32[]) private operatorNodes;

    // Total active node count
    uint256 public activeNodeCount;
    uint256 public totalRegisteredCount;

    // Events
    event NodeRegistered(
        string nodeId,
        bytes32 indexed nodeIdHash,
        address indexed operator,
        bytes32 ed25519PubKey,
        ServiceType serviceType,
        string region,
        uint32 maxCapacityMbps
    );

    event NodeStatusChanged(
        bytes32 indexed nodeIdHash,
        NodeStatus indexed previousStatus,
        NodeStatus indexed newStatus,
        string reason
    );

    event NodeUpdated(
        bytes32 indexed nodeIdHash,
        uint32 maxCapacityMbps,
        string region
    );

    event OwnershipTransferred(address indexed previousOwner, address indexed newOwner);
    event PauseToggled(bool isPaused);

    modifier onlyOwner() {
        require(msg.sender == owner, "BandiqNodeRegistry: Caller is not owner");
        _;
    }

    modifier whenNotPaused() {
        require(!paused, "BandiqNodeRegistry: Registry is paused");
        _;
    }

    constructor() {
        owner = msg.sender;
        emit OwnershipTransferred(address(0), msg.sender);
    }

    /**
     * @notice Register a new BANDIQ bandwidth node.
     * @param nodeId Human-readable Node ID string (e.g. "bandiq-node-2d5cd8...").
     * @param ed25519PubKey 32-byte Ed25519 public key used by the node agent for signing reports.
     * @param serviceType Node service tier (RESIDENTIAL, COMMERCIAL, COMMUNITY_WIFI, ENTERPRISE_RELAY).
     * @param region Consented geographical region (e.g. "PH", "APAC").
     * @param maxCapacityMbps Declared maximum bandwidth throttle.
     */
    function registerNode(
        string calldata nodeId,
        bytes32 ed25519PubKey,
        ServiceType serviceType,
        string calldata region,
        uint32 maxCapacityMbps
    ) external whenNotPaused {
        require(bytes(nodeId).length > 0, "BandiqNodeRegistry: Node ID cannot be empty");
        require(ed25519PubKey != bytes32(0), "BandiqNodeRegistry: Invalid Ed25519 public key");

        bytes32 idHash = keccak256(bytes(nodeId));
        require(!nodes[idHash].exists, "BandiqNodeRegistry: Node ID already registered");

        nodes[idHash] = NodeInfo({
            nodeId: nodeId,
            operator: msg.sender,
            ed25519PubKey: ed25519PubKey,
            serviceType: serviceType,
            region: region,
            maxCapacityMbps: maxCapacityMbps,
            registeredAt: uint64(block.timestamp),
            status: NodeStatus.ACTIVE,
            exists: true
        });

        operatorNodes[msg.sender].push(idHash);
        activeNodeCount++;
        totalRegisteredCount++;

        emit NodeRegistered(
            nodeId,
            idHash,
            msg.sender,
            ed25519PubKey,
            serviceType,
            region,
            maxCapacityMbps
        );
    }

    /**
     * @notice Update node capacity or region by the node operator.
     */
    function updateNode(
        string calldata nodeId,
        uint32 maxCapacityMbps,
        string calldata region
    ) external whenNotPaused {
        bytes32 idHash = keccak256(bytes(nodeId));
        NodeInfo storage node = nodes[idHash];

        require(node.exists, "BandiqNodeRegistry: Node not found");
        require(node.operator == msg.sender, "BandiqNodeRegistry: Only operator can update node");
        require(node.status != NodeStatus.SUSPENDED, "BandiqNodeRegistry: Suspended node cannot be updated");

        node.maxCapacityMbps = maxCapacityMbps;
        node.region = region;

        emit NodeUpdated(idHash, maxCapacityMbps, region);
    }

    /**
     * @notice Deactivate or deregister a node by its operator.
     */
    function deregisterNode(string calldata nodeId) external {
        bytes32 idHash = keccak256(bytes(nodeId));
        NodeInfo storage node = nodes[idHash];

        require(node.exists, "BandiqNodeRegistry: Node not found");
        require(node.operator == msg.sender || msg.sender == owner, "BandiqNodeRegistry: Unauthorized");
        require(node.status != NodeStatus.DEREGISTERED, "BandiqNodeRegistry: Already deregistered");

        NodeStatus prevStatus = node.status;
        node.status = NodeStatus.DEREGISTERED;

        if (prevStatus == NodeStatus.ACTIVE) {
            activeNodeCount--;
        }

        emit NodeStatusChanged(idHash, prevStatus, NodeStatus.DEREGISTERED, "Operator deregistered node");
    }

    /**
     * @notice Administrative control to suspend malicious or compromised nodes.
     */
    function setNodeStatus(
        string calldata nodeId,
        NodeStatus newStatus,
        string calldata reason
    ) external onlyOwner {
        bytes32 idHash = keccak256(bytes(nodeId));
        NodeInfo storage node = nodes[idHash];

        require(node.exists, "BandiqNodeRegistry: Node not found");
        NodeStatus prevStatus = node.status;
        require(prevStatus != newStatus, "BandiqNodeRegistry: Status unchanged");

        if (prevStatus == NodeStatus.ACTIVE && newStatus != NodeStatus.ACTIVE) {
            activeNodeCount--;
        } else if (prevStatus != NodeStatus.ACTIVE && newStatus == NodeStatus.ACTIVE) {
            activeNodeCount++;
        }

        node.status = newStatus;
        emit NodeStatusChanged(idHash, prevStatus, newStatus, reason);
    }

    /**
     * @notice Check whether a node is registered and active for receiving bandwidth settlement.
     */
    function isNodeEligible(string calldata nodeId) external view returns (bool) {
        bytes32 idHash = keccak256(bytes(nodeId));
        return nodes[idHash].exists && nodes[idHash].status == NodeStatus.ACTIVE;
    }

    /**
     * @notice Get full details for a node by ID.
     */
    function getNode(string calldata nodeId) external view returns (NodeInfo memory) {
        bytes32 idHash = keccak256(bytes(nodeId));
        require(nodes[idHash].exists, "BandiqNodeRegistry: Node not found");
        return nodes[idHash];
    }

    /**
     * @notice Get all node ID hashes owned by an operator.
     */
    function getOperatorNodes(address operator) external view returns (bytes32[] memory) {
        return operatorNodes[operator];
    }

    /**
     * @notice Admin pause/unpause registry functions.
     */
    function setPaused(bool _paused) external onlyOwner {
        paused = _paused;
        emit PauseToggled(_paused);
    }

    /**
     * @notice Transfer contract ownership.
     */
    function transferOwnership(address newOwner) external onlyOwner {
        require(newOwner != address(0), "BandiqNodeRegistry: New owner is zero address");
        emit OwnershipTransferred(owner, newOwner);
        owner = newOwner;
    }
}
