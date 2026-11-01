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
        uint256[3] calldata,
        uint256 gracePeriod
    ) external returns (Addresses memory deployed) {
        PaxRegistry registry = new PaxRegistry();
        PaxBaseRegistrar registrar =
            new PaxBaseRegistrar(address(registry), governanceTimelock, emergencyAuthority, gracePeriod);
        PaxPublicResolver resolver = new PaxPublicResolver(address(registry), governanceTimelock, emergencyAuthority);
        PaxRegistrarController controller = new PaxRegistrarController(
            address(registrar),
            address(registry),
            address(resolver),
            address(0),
            treasury,
            governanceTimelock,
            emergencyAuthority
        );
        PaxReverseRegistrar reverseRegistrar =
            new PaxReverseRegistrar(address(registry), address(resolver), address(controller));

        registry.setSubnodeOwner(bytes32(0), keccak256("pax"), address(registrar));
        bytes32 reverseNode = registry.setSubnodeOwner(bytes32(0), keccak256("reverse"), address(this));
        registry.setSubnodeOwner(reverseNode, keccak256("addr"), address(reverseRegistrar));
        registry.setOwner(reverseNode, governanceTimelock);
        registry.setOwner(bytes32(0), governanceTimelock);

        deployed = Addresses(
            address(registry), address(registrar), address(resolver), address(reverseRegistrar), address(controller)
        );
        emit NamesDeployed(
            deployed.registry, deployed.registrar, deployed.resolver, deployed.reverseRegistrar, deployed.controller
        );
    }

    function governanceSetup(Addresses calldata deployed, uint256[3] calldata yearlyPrices)
        external
        pure
        returns (address[] memory targets, bytes[] memory payloads)
    {
        targets = new address[](6);
        payloads = new bytes[](6);
        targets[0] = deployed.registrar;
        payloads[0] = abi.encodeCall(PaxBaseRegistrar.addController, (deployed.controller));
        targets[1] = deployed.resolver;
        payloads[1] = abi.encodeCall(PaxPublicResolver.setRegistrarController, (deployed.controller));
        for (uint8 i; i < 3; ++i) {
            targets[i + 2] = deployed.controller;
            payloads[i + 2] = abi.encodeCall(PaxRegistrarController.setPrice, (i + 3, yearlyPrices[i]));
        }
        targets[5] = deployed.controller;
        payloads[5] =
            abi.encodeCall(PaxRegistrarController.setResolvers, (deployed.resolver, deployed.reverseRegistrar));
    }
}
