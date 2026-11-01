// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.24;

import {PaxRegistry} from "./PaxRegistry.sol";
import {PaxBaseRegistrar} from "./PaxBaseRegistrar.sol";
import {PaxPublicResolver} from "./PaxPublicResolver.sol";
import {PaxReverseRegistrar} from "./PaxReverseRegistrar.sol";
import {PaxRegistrarController} from "./PaxRegistrarController.sol";

contract NamesDeployment {
    struct Addresses {
        address registry;
        address registrar;
        address resolver;
        address reverseRegistrar;
        address controller;
    }

    event NamesDeployed(
        address registry, address registrar, address resolver, address reverseRegistrar, address controller
    );

    function deploy(
        address governanceTimelock,
        address emergencyAuthority,
        address treasury,
        uint256[3] calldata yearlyPrices,
        uint256 gracePeriod
    ) external returns (Addresses memory deployed) {
        PaxRegistry registry = new PaxRegistry();
        PaxBaseRegistrar registrar =
            new PaxBaseRegistrar(address(registry), governanceTimelock, emergencyAuthority, gracePeriod);
        PaxPublicResolver resolver = new PaxPublicResolver(address(registry), governanceTimelock, emergencyAuthority);
        PaxReverseRegistrar reverseRegistrar = new PaxReverseRegistrar(address(registry), address(resolver));
        PaxRegistrarController controller = new PaxRegistrarController(
            address(registrar),
            address(registry),
            address(resolver),
            address(reverseRegistrar),
            treasury,
            governanceTimelock,
            emergencyAuthority
        );

        registry.setSubnodeOwner(bytes32(0), keccak256("pax"), address(registrar));
        bytes32 reverseNode = registry.setSubnodeOwner(bytes32(0), keccak256("reverse"), address(this));
        registry.setSubnodeOwner(reverseNode, keccak256("addr"), address(reverseRegistrar));
        registrar.addController(address(controller));
        resolver.setRegistrarController(address(controller));
        for (uint8 i; i < 3; ++i) {
            controller.setPrice(i + 3, yearlyPrices[i]);
        }
        registry.setOwner(reverseNode, governanceTimelock);
        registry.setOwner(bytes32(0), governanceTimelock);

        deployed = Addresses(
            address(registry), address(registrar), address(resolver), address(reverseRegistrar), address(controller)
        );
        emit NamesDeployed(
            deployed.registry, deployed.registrar, deployed.resolver, deployed.reverseRegistrar, deployed.controller
        );
    }
}
