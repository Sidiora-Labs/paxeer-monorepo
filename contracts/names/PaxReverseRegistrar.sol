// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.24;

import {IPaxRegistry, IPaxPublicResolver} from "./IPaxNames.sol";

contract PaxReverseRegistrar {
    error InvalidConfiguration();
    error Unauthorised();

    bytes32 public constant ADDR_REVERSE_NODE =
        keccak256(abi.encodePacked(keccak256(abi.encodePacked(bytes32(0), keccak256("reverse"))), keccak256("addr")));
    IPaxRegistry public immutable registry;
    IPaxPublicResolver public immutable defaultResolver;
    address public immutable controller;

    constructor(address registryAddress, address resolverAddress, address controllerAddress) {
        if (registryAddress.code.length == 0 || resolverAddress.code.length == 0) revert InvalidConfiguration();
        registry = IPaxRegistry(registryAddress);
        defaultResolver = IPaxPublicResolver(resolverAddress);
        controller = controllerAddress;
    }

    function setName(string calldata newName) external returns (bytes32 reverseNode) {
        return _setNameForAddr(msg.sender, msg.sender, address(defaultResolver), newName);
    }

    function setNameForAddr(address account, address owner, address resolverAddress, string calldata newName)
        external
        returns (bytes32 reverseNode)
    {
        return _setNameForAddr(account, owner, resolverAddress, newName);
    }

    function _setNameForAddr(address account, address owner, address resolverAddress, string calldata newName)
        private
        returns (bytes32 reverseNode)
    {
        if (msg.sender != account && (controller == address(0) || msg.sender != controller)) {
            revert Unauthorised();
        }
        if (resolverAddress.code.length == 0) revert InvalidConfiguration();
        bytes32 label = _sha3HexAddress(account);
        reverseNode = keccak256(abi.encodePacked(ADDR_REVERSE_NODE, label));
        // Hold the node only while writing its owner-authorised name record.
        // The final subnode record belongs to the requested owner, atomically.
        registry.setSubnodeRecord(ADDR_REVERSE_NODE, label, address(this), resolverAddress, 0);
        IPaxPublicResolver(resolverAddress).setName(reverseNode, newName);
        registry.setSubnodeRecord(ADDR_REVERSE_NODE, label, owner, resolverAddress, 0);
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
