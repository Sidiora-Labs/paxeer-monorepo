// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.24;

import {Governed} from "../security/Governed.sol";
import {IPaxRegistry, IPaxPublicResolver} from "./IPaxNames.sol";

contract PaxPublicResolver is Governed, IPaxPublicResolver {
    error Unauthorised();
    error InvalidConfiguration();
    error PaxContenthashForm();
    error NodeFrozen();

    event AddrChanged(bytes32 indexed node, address a);
    event NameChanged(bytes32 indexed node, string name);
    event TextChanged(bytes32 indexed node, string indexed indexedKey, string key, string value);
    event ContenthashChanged(bytes32 indexed node, bytes hash);
    event RegistrarControllerSet(address indexed controller);
    event Frozen(bytes32 indexed node);
    event Unfrozen(bytes32 indexed node);

    // Governed has only immutables. Keep this mapping first: external manifest
    // readers derive its node slots from the published layout vector.
    mapping(bytes32 => bytes32) private manifestDigests;

    uint256 public constant MANIFEST_DIGEST_SLOT = 0;
    bytes4 private constant CONTENTHASH_TAG = 0x70617831;
    IPaxRegistry public immutable registry;
    address public registrarController;
    mapping(bytes32 => address) private addresses;
    mapping(bytes32 => string) private names;
    mapping(bytes32 => mapping(string => string)) private texts;
    mapping(bytes32 => bool) public frozen;

    constructor(address registryAddress, address governanceTimelock, address emergencyAuthority)
        Governed(governanceTimelock, emergencyAuthority)
    {
        if (registryAddress.code.length == 0) revert InvalidConfiguration();
        registry = IPaxRegistry(registryAddress);
    }

    modifier authorised(bytes32 node) {
        address owner = registry.owner(node);
        if (msg.sender != owner && msg.sender != registrarController && !registry.isApprovedForAll(owner, msg.sender)) {
            revert Unauthorised();
        }
        _;
    }

    function setRegistrarController(address controller) external onlyGovernance {
        if (controller == address(0) || registrarController != address(0)) revert InvalidConfiguration();
        registrarController = controller;
        emit RegistrarControllerSet(controller);
    }

    function addr(bytes32 node) external view returns (address) {
        return addresses[node];
    }

    function setAddr(bytes32 node, address account) external authorised(node) {
        addresses[node] = account;
        emit AddrChanged(node, account);
    }

    function name(bytes32 node) external view returns (string memory) {
        return names[node];
    }

    function setName(bytes32 node, string calldata newName) external authorised(node) {
        names[node] = newName;
        emit NameChanged(node, newName);
    }

    function text(bytes32 node, string calldata key) external view returns (string memory) {
        return texts[node][key];
    }

    function setText(bytes32 node, string calldata key, string calldata value) external authorised(node) {
        if (frozen[node]) revert NodeFrozen();
        texts[node][key] = value;
        emit TextChanged(node, key, key, value);
    }

    function contenthash(bytes32 node) external view returns (bytes memory) {
        bytes32 digest = manifestDigests[node];
        return digest == bytes32(0) ? bytes("") : abi.encodePacked(CONTENTHASH_TAG, digest);
    }

    function setContenthash(bytes32 node, bytes calldata hash) external authorised(node) {
        if (frozen[node]) revert NodeFrozen();
        bytes32 digest;
        if (hash.length != 0) {
            if (hash.length != 36 || bytes4(hash[:4]) != CONTENTHASH_TAG) revert PaxContenthashForm();
            digest = bytes32(hash[4:36]);
        }
        manifestDigests[node] = digest;
        emit ContenthashChanged(node, hash);
    }

    function freeze(bytes32 node) external {
        if (msg.sender != governance && msg.sender != emergencyCouncil) revert GovernanceOnly();
        frozen[node] = true;
        delete manifestDigests[node];
        emit ContenthashChanged(node, "");
        emit Frozen(node);
    }

    function unfreeze(bytes32 node) external onlyGovernance {
        delete frozen[node];
        emit Unfrozen(node);
    }

    function supportsInterface(bytes4 interfaceId) external pure returns (bool) {
        return interfaceId == 0x01ffc9a7 || interfaceId == 0x3b3b57de || interfaceId == 0x691f3431
            || interfaceId == 0x59d1d43c || interfaceId == 0xbc1c58d1;
    }

    function multicall(bytes[] calldata data) external returns (bytes[] memory results) {
        results = new bytes[](data.length);
        for (uint256 i; i < data.length; ++i) {
            (bool success, bytes memory result) = address(this).delegatecall(data[i]);
            if (!success) {
                assembly ("memory-safe") {
                    revert(add(result, 32), mload(result))
                }
            }
            results[i] = result;
        }
    }
}
