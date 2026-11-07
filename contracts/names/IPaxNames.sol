// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.24;

struct Registration {
    string label;
    address owner;
    uint256 duration;
    bytes32 secret;
    address resolver;
    bytes[] data;
    uint8 reverseRecord;
    bytes32 referrer;
}

interface IPaxRegistry {
    event NewOwner(bytes32 indexed node, bytes32 indexed label, address owner);
    event Transfer(bytes32 indexed node, address owner);
    event NewResolver(bytes32 indexed node, address resolver);
    event NewTTL(bytes32 indexed node, uint64 ttl);
    event ApprovalForAll(address indexed owner, address indexed operator, bool approved);

    function owner(bytes32 node) external view returns (address);
    function resolver(bytes32 node) external view returns (address);
    function ttl(bytes32 node) external view returns (uint64);
    function setOwner(bytes32 node, address newOwner) external;
    function setResolver(bytes32 node, address newResolver) external;
    function setTTL(bytes32 node, uint64 newTTL) external;
    function setSubnodeOwner(bytes32 node, bytes32 label, address newOwner) external returns (bytes32);
    function setRecord(bytes32 node, address newOwner, address newResolver, uint64 newTTL) external;
    function setSubnodeRecord(bytes32 node, bytes32 label, address newOwner, address newResolver, uint64 newTTL)
        external;
    function setApprovalForAll(address operator, bool approved) external;
    function isApprovedForAll(address account, address operator) external view returns (bool);
}

interface IPaxBaseRegistrar {
    function PAX_NODE() external view returns (bytes32);
    function baseNode() external view returns (bytes32);
    function GRACE_PERIOD() external view returns (uint256);
    function expiries(uint256 id) external view returns (uint256);
    function nameExpires(uint256 id) external view returns (uint256);
    function available(uint256 id) external view returns (bool);
    function register(uint256 id, address newOwner, uint256 duration) external returns (uint256);
    function renew(uint256 id, uint256 duration) external returns (uint256);
    function reclaim(uint256 id, address newOwner) external;
    function addController(address controller) external;
    function removeController(address controller) external;
    function ownerOf(uint256 id) external view returns (address);
    function balanceOf(address account) external view returns (uint256);
    function approve(address operator, uint256 id) external;
    function getApproved(uint256 id) external view returns (address);
    function setApprovalForAll(address operator, bool approved) external;
    function isApprovedForAll(address account, address operator) external view returns (bool);
    function transferFrom(address from, address to, uint256 id) external;
    function safeTransferFrom(address from, address to, uint256 id) external;
    function safeTransferFrom(address from, address to, uint256 id, bytes calldata data) external;
}

interface IPaxRegistrarController {
    event NameRegistered(
        string label, bytes32 indexed labelhash, address indexed owner, uint256 cost, uint256 expires, bytes32 referrer
    );
    event NameRenewed(string label, bytes32 indexed labelhash, uint256 cost, uint256 expires);

    function rentPrice(string calldata label, uint256 duration) external view returns (uint256 base, uint256 premium);
    function available(string calldata label) external view returns (bool);
    function makeCommitment(Registration calldata registration) external pure returns (bytes32);
    function commit(bytes32 commitment) external;
    function minCommitmentAge() external view returns (uint256);
    function maxCommitmentAge() external view returns (uint256);
    function register(Registration calldata registration) external payable;
    function renew(string calldata label, uint256 duration) external payable;
    function setPrice(uint8 tier, uint256 pricePerYear) external;
    function setReserved(bytes32 labelhash, bool reserved) external;
    function setCommitmentAges(uint256 minimumAge, uint256 maximumAge) external;
    function setTreasury(address newTreasury) external;
    function setResolvers(address resolver, address reverseRegistrar) external;
}

interface IPaxPublicResolver {
    function addr(bytes32 node) external view returns (address);
    function setAddr(bytes32 node, address account) external;
    function name(bytes32 node) external view returns (string memory);
    function setName(bytes32 node, string calldata newName) external;
    function text(bytes32 node, string calldata key) external view returns (string memory);
    function setText(bytes32 node, string calldata key, string calldata value) external;
    function contenthash(bytes32 node) external view returns (bytes memory);
    function setContenthash(bytes32 node, bytes calldata hash) external;
    function multicall(bytes[] calldata data) external returns (bytes[] memory);
}

interface IPaxReverseRegistrar {
    function setName(string calldata name) external returns (bytes32);
    function setNameForAddr(address account, address owner, address resolver, string calldata name)
        external
        returns (bytes32);
    function node(address account) external pure returns (bytes32);
}
