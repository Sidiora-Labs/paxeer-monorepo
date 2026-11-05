// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.24;

import {IPaxRegistry} from "./IPaxNames.sol";

contract PaxRegistry is IPaxRegistry {
    error Unauthorised();

    struct Record {
        address owner;
        address resolver;
        uint64 ttl;
    }

    mapping(bytes32 => Record) private records;
    mapping(address => mapping(address => bool)) private operators;

    constructor() {
        records[bytes32(0)].owner = msg.sender;
        emit Transfer(bytes32(0), msg.sender);
    }

    modifier authorised(bytes32 node) {
        address account = records[node].owner;
        if (msg.sender != account && !operators[account][msg.sender]) revert Unauthorised();
        _;
    }

    function owner(bytes32 node) external view returns (address) {
        return records[node].owner;
    }

    function resolver(bytes32 node) external view returns (address) {
        return records[node].resolver;
    }

    function ttl(bytes32 node) external view returns (uint64) {
        return records[node].ttl;
    }

    function setOwner(bytes32 node, address newOwner) public authorised(node) {
        records[node].owner = newOwner;
        emit Transfer(node, newOwner);
    }

    function setResolver(bytes32 node, address newResolver) public authorised(node) {
        records[node].resolver = newResolver;
        emit NewResolver(node, newResolver);
    }

    function setTTL(bytes32 node, uint64 newTTL) public authorised(node) {
        records[node].ttl = newTTL;
        emit NewTTL(node, newTTL);
    }

    function setSubnodeOwner(bytes32 node, bytes32 label, address newOwner)
        public
        authorised(node)
        returns (bytes32 subnode)
    {
        subnode = keccak256(abi.encodePacked(node, label));
        records[subnode].owner = newOwner;
        emit NewOwner(node, label, newOwner);
    }

    function setRecord(bytes32 node, address newOwner, address newResolver, uint64 newTTL) external authorised(node) {
        records[node] = Record(newOwner, newResolver, newTTL);
        emit Transfer(node, newOwner);
        emit NewResolver(node, newResolver);
        emit NewTTL(node, newTTL);
    }

    function setSubnodeRecord(bytes32 node, bytes32 label, address newOwner, address newResolver, uint64 newTTL)
        external
        authorised(node)
    {
        bytes32 subnode = setSubnodeOwner(node, label, newOwner);
        records[subnode].resolver = newResolver;
        records[subnode].ttl = newTTL;
        emit NewResolver(subnode, newResolver);
        emit NewTTL(subnode, newTTL);
    }

    function setApprovalForAll(address operator, bool approved) external {
        operators[msg.sender][operator] = approved;
        emit ApprovalForAll(msg.sender, operator, approved);
    }

    function isApprovedForAll(address account, address operator) external view returns (bool) {
        return operators[account][operator];
    }
}
