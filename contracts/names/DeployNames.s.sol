// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.24;

import {NamesDeployment} from "./NamesDeployment.sol";

interface DeployNamesVm {
    function envAddress(string calldata key) external view returns (address);
    function envUint(string calldata key) external view returns (uint256);
    function startBroadcast() external;
    function stopBroadcast() external;
    function writeJson(string calldata json, string calldata path) external;
    function toString(address value) external pure returns (string memory);
    function toString(uint256 value) external pure returns (string memory);
    function toString(bytes calldata value) external pure returns (string memory);
}

contract DeployNames {
    DeployNamesVm private constant vm = DeployNamesVm(address(uint160(uint256(keccak256("hevm cheat code")))));

    function run() external returns (NamesDeployment.Addresses memory deployed) {
        address governanceTimelock = vm.envAddress("NAMES_GOVERNANCE_TIMELOCK");
        address emergencyAuthority = vm.envAddress("NAMES_EMERGENCY_AUTHORITY");
        address treasury = vm.envAddress("NAMES_TREASURY");
        uint256[3] memory yearlyPrices =
            [vm.envUint("NAMES_PRICE_3"), vm.envUint("NAMES_PRICE_4"), vm.envUint("NAMES_PRICE_5")];
        uint256 gracePeriod = vm.envUint("NAMES_GRACE_SECONDS");

        vm.startBroadcast();
        NamesDeployment deployment = new NamesDeployment();
        deployed = deployment.deploy(governanceTimelock, emergencyAuthority, treasury, yearlyPrices, gracePeriod);
        vm.stopBroadcast();

        vm.writeJson(
            string.concat(
                '{"registry":"',
                vm.toString(deployed.registry),
                '","registrar":"',
                vm.toString(deployed.registrar),
                '","resolver":"',
                vm.toString(deployed.resolver),
                '","reverseRegistrar":"',
                vm.toString(deployed.reverseRegistrar),
                '","controller":"',
                vm.toString(deployed.controller),
                '","chainId":',
                vm.toString(block.chainid),
                "}"
            ),
            "build/names-deployment.json"
        );

        (address[] memory targets, bytes[] memory payloads) = deployment.governanceSetup(deployed, yearlyPrices);
        string memory targetsJson = "[";
        string memory payloadsJson = "[";
        for (uint256 i; i < targets.length; ++i) {
            string memory separator = i == 0 ? "" : ",";
            targetsJson = string.concat(targetsJson, separator, '"', vm.toString(targets[i]), '"');
            payloadsJson = string.concat(payloadsJson, separator, '"', vm.toString(payloads[i]), '"');
        }
        vm.writeJson(
            string.concat('{"targets":', targetsJson, '],"payloads":', payloadsJson, "]}"),
            "build/names-governance-setup.json"
        );
    }
}
