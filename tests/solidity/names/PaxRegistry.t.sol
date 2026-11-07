// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.24;

import {PaxRegistry} from "../../../contracts/names/PaxRegistry.sol";

interface PaxRegistryVm {
    function prank(address account) external;
    function expectRevert(bytes4 selector) external;
}

contract PaxRegistryTest {
    PaxRegistryVm private constant vm = PaxRegistryVm(address(uint160(uint256(keccak256("hevm cheat code")))));

    function testRecordsFollowSubnodeOwnership() public {
        PaxRegistry registry = new PaxRegistry();
        address alice = address(0xA11CE);
        address bob = address(0xB0B);
        address operator = address(0xCAFE);
        address resolver = address(0x1234);
        bytes32 pax = registry.setSubnodeOwner(bytes32(0), keccak256("pax"), alice);
        require(registry.owner(bytes32(0)) == address(this), "root owner");
        require(pax == keccak256(abi.encodePacked(bytes32(0), keccak256("pax"))), "pax node");
        vm.prank(alice);
        bytes32 node = registry.setSubnodeOwner(pax, keccak256("alice"), alice);
        require(node == keccak256(abi.encodePacked(pax, keccak256("alice"))), "subnode hash");
        require(registry.owner(node) == alice, "subnode owner");
        require(registry.resolver(node) == address(0), "new subnode resolver");
        require(registry.ttl(node) == 0, "new subnode ttl");
        vm.expectRevert(PaxRegistry.Unauthorised.selector);
        registry.setResolver(node, resolver);
        vm.prank(alice);
        registry.setRecord(node, bob, resolver, 3600);
        require(
            registry.owner(node) == bob && registry.resolver(node) == resolver && registry.ttl(node) == 3600, "record"
        );
        vm.prank(alice);
        vm.expectRevert(PaxRegistry.Unauthorised.selector);
        registry.setTTL(node, 1);
        vm.prank(bob);
        registry.setApprovalForAll(operator, true);
        require(registry.isApprovedForAll(bob, operator), "approval missing");
        vm.prank(operator);
        registry.setResolver(node, address(0x5678));
        vm.prank(operator);
        registry.setTTL(node, 7200);
        require(registry.resolver(node) == address(0x5678) && registry.ttl(node) == 7200, "operator write");
        vm.prank(operator);
        registry.setSubnodeRecord(node, keccak256("child"), alice, resolver, 10);
        bytes32 child = keccak256(abi.encodePacked(node, keccak256("child")));
        require(
            registry.owner(child) == alice && registry.resolver(child) == resolver && registry.ttl(child) == 10,
            "child record"
        );
        vm.prank(bob);
        registry.setApprovalForAll(operator, false);
        vm.prank(operator);
        vm.expectRevert(PaxRegistry.Unauthorised.selector);
        registry.setOwner(node, operator);
        vm.prank(bob);
        registry.setOwner(node, alice);
        vm.prank(operator);
        vm.expectRevert(PaxRegistry.Unauthorised.selector);
        registry.setRecord(node, operator, address(0), 0);
        require(registry.owner(node) == alice, "transfer owner");
        require(!registry.isApprovedForAll(bob, operator), "approval not revoked");
    }
}
