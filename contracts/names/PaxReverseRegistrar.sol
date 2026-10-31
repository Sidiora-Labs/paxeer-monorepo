// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.24;

import {IPaxRegistry, IPaxPublicResolver} from "./IPaxNames.sol";

contract PaxReverseRegistrar {
    error InvalidConfiguration();

    bytes32 public constant ADDR_REVERSE_NODE =
        keccak256(abi.encodePacked(keccak256(abi.encodePacked(bytes32(0), keccak256("reverse"))), keccak256("addr")));
    IPaxRegistry public immutable registry;
    IPaxPublicResolver public immutable defaultResolver;

    constructor(address registryAddress, address resolverAddress) {
        if (registryAddress.code.length == 0 || resolverAddress.code.length == 0) revert InvalidConfiguration();
        registry = IPaxRegistry(registryAddress);
        defaultResolver = IPaxPublicResolver(resolverAddress);
    }

    function setName(string calldata newName) external returns (bytes32 reverseNode) {
        bytes32 label = _sha3HexAddress(msg.sender);
        reverseNode = keccak256(abi.encodePacked(ADDR_REVERSE_NODE, label));
        // Hold the node only while writing its owner-authorised name record.
        // The final subnode record belongs to the caller, atomically.
        registry.setSubnodeRecord(ADDR_REVERSE_NODE, label, address(this), address(defaultResolver), 0);
        defaultResolver.setName(reverseNode, newName);
        registry.setSubnodeRecord(ADDR_REVERSE_NODE, label, msg.sender, address(defaultResolver), 0);
    }

    function node(address account) external pure returns (bytes32) {
        return keccak256(abi.encodePacked(ADDR_REVERSE_NODE, _sha3HexAddress(account)));
    }

    function _sha3HexAddress(address account) private pure returns (bytes32) {
        bytes16 alphabet = "0123456789abcdef";
        bytes memory label = new bytes(40);
        uint160 value = uint160(account);
        for (uint256 i; i < 40; ++i) {
            label[39 - i] = alphabet[value & 0xf];
            value >>= 4;
        }
        return keccak256(label);
    }
}
