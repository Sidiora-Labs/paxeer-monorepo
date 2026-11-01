// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.24;

import {NamesDeployment} from "../../../contracts/names/NamesDeployment.sol";
import {PaxRegistry} from "../../../contracts/names/PaxRegistry.sol";
import {PaxBaseRegistrar} from "../../../contracts/names/PaxBaseRegistrar.sol";
import {PaxPublicResolver} from "../../../contracts/names/PaxPublicResolver.sol";
import {PaxReverseRegistrar} from "../../../contracts/names/PaxReverseRegistrar.sol";
import {PaxRegistrarController} from "../../../contracts/names/PaxRegistrarController.sol";
import {Registration} from "../../../contracts/names/IPaxNames.sol";
import {Governed} from "../../../contracts/security/Governed.sol";

interface NamesEndToEndVm {
    function prank(address account) external;
    function warp(uint256 timestamp) external;
    function deal(address account, uint256 balance) external;
    function expectRevert(bytes4 selector) external;
    function load(address account, bytes32 slot) external view returns (bytes32);
}

contract NamesEndToEndTest {
    NamesEndToEndVm private constant vm = NamesEndToEndVm(address(uint160(uint256(keccak256("hevm cheat code")))));
    address private constant TIMELOCK = address(0x710E);
    address private constant EMERGENCY = address(0xCAFE);
    address private constant TREASURY = address(0xFEE);
    address private constant USER = address(0xA11CE);
    uint256 private constant YEAR = 365 days;
    uint256 private constant GRACE = 90 days;

    function testCommitRegisterPublishAndResolveThroughTheWiredSet() public {
        NamesDeployment.Addresses memory deployed = _deploy();
        PaxRegistry registry = PaxRegistry(deployed.registry);
        PaxBaseRegistrar registrar = PaxBaseRegistrar(deployed.registrar);
        PaxPublicResolver resolver = PaxPublicResolver(deployed.resolver);
        PaxReverseRegistrar reverseRegistrar = PaxReverseRegistrar(deployed.reverseRegistrar);
        PaxRegistrarController controller = PaxRegistrarController(deployed.controller);
        bytes32 labelhash = keccak256("example");
        bytes32 node = keccak256(abi.encodePacked(registrar.PAX_NODE(), labelhash));
        bytes[] memory data = new bytes[](1);
        data[0] = abi.encodeCall(PaxPublicResolver.setAddr, (node, USER));
        Registration memory registration = Registration({
            label: "example",
            owner: USER,
            duration: YEAR,
            secret: keccak256("example registration secret"),
            resolver: deployed.resolver,
            data: data,
            reverseRecord: 1,
            referrer: bytes32(0)
        });

        vm.warp(10 days);
        vm.deal(USER, 20 ether);
        require(controller.available("example"), "initial availability");
        bytes32 commitment = controller.makeCommitment(registration);
        vm.prank(USER);
        controller.commit(commitment);
        vm.warp(block.timestamp + controller.minCommitmentAge() + 1);
        (uint256 price, uint256 premium) = controller.rentPrice("example", YEAR);
        require(price == 5 ether && premium == 0, "five-plus character tier");
        uint256 registeredAt = block.timestamp;
        uint256 treasuryBefore = TREASURY.balance;
        vm.prank(USER);
        controller.register{value: price}(registration);

        require(registry.owner(node) == USER, "registered node owner");
        require(registrar.ownerOf(uint256(labelhash)) == USER, "registered token owner");
        require(registry.resolver(node) == deployed.resolver, "registered resolver");
        require(resolver.addr(node) == USER, "registration resolver data");
        bytes32 reverseNode = reverseRegistrar.node(USER);
        require(registry.owner(reverseNode) == USER, "reverse node owner");
        require(registry.resolver(reverseNode) == deployed.resolver, "reverse resolver");
        require(keccak256(bytes(resolver.name(reverseNode))) == keccak256("example.pax"), "reverse name");
        uint256 expiry = registeredAt + YEAR;
        require(registrar.nameExpires(uint256(labelhash)) == expiry, "registration expiry");
        require(TREASURY.balance == treasuryBefore + price, "registration payment");
        require(!controller.available("example"), "registered name unavailable");

        bytes32 digest = keccak256("published example manifest");
        bytes memory contenthash = abi.encodePacked(bytes4(0x70617831), digest);
        vm.prank(USER);
        resolver.setContenthash(node, contenthash);
        require(keccak256(resolver.contenthash(node)) == keccak256(contenthash), "published contenthash");
        bytes32 digestSlot = keccak256(abi.encode(node, resolver.MANIFEST_DIGEST_SLOT()));
        require(vm.load(deployed.resolver, digestSlot) == digest, "published manifest digest");

        vm.prank(USER);
        controller.renew{value: price}("example", YEAR);
        expiry += YEAR;
        require(registrar.nameExpires(uint256(labelhash)) == expiry, "renewed expiry");
        require(TREASURY.balance == treasuryBefore + 2 * price, "renewal payment");
        vm.warp(expiry + GRACE);
        require(!controller.available("example"), "grace boundary unavailable");
        vm.warp(expiry + GRACE + 1);
        require(controller.available("example"), "available after grace");

        vm.prank(TIMELOCK);
        resolver.freeze(node);
        require(resolver.frozen(node), "governance freeze");
        require(resolver.contenthash(node).length == 0, "freeze clears contenthash");
        require(vm.load(deployed.resolver, digestSlot) == bytes32(0), "freeze clears manifest digest");
    }

    function testDeploymentHandsControlToGovernance() public {
        NamesDeployment.Addresses memory deployed = _deploy();
        PaxRegistry registry = PaxRegistry(deployed.registry);
        PaxBaseRegistrar registrar = PaxBaseRegistrar(deployed.registrar);
        PaxPublicResolver resolver = PaxPublicResolver(deployed.resolver);
        PaxReverseRegistrar reverseRegistrar = PaxReverseRegistrar(deployed.reverseRegistrar);
        PaxRegistrarController controller = PaxRegistrarController(deployed.controller);

        require(registry.owner(bytes32(0)) == TIMELOCK, "root governance owner");
        require(registry.owner(registrar.PAX_NODE()) == deployed.registrar, "pax registrar owner");
        bytes32 reverseNode = keccak256(abi.encodePacked(bytes32(0), keccak256("reverse")));
        require(registry.owner(reverseNode) == TIMELOCK, "reverse parent governance owner");
        require(registry.owner(reverseRegistrar.ADDR_REVERSE_NODE()) == deployed.reverseRegistrar, "reverse owner");
        require(registrar.governance() == TIMELOCK && registrar.emergencyCouncil() == EMERGENCY, "registrar authority");
        require(resolver.governance() == TIMELOCK && resolver.emergencyCouncil() == EMERGENCY, "resolver authority");
        require(
            controller.governance() == TIMELOCK && controller.emergencyCouncil() == EMERGENCY, "controller authority"
        );
        require(registrar.controllers(deployed.controller), "controller allowed");
        require(resolver.registrarController() == deployed.controller, "resolver controller");
        require(address(registrar.registry()) == deployed.registry, "registrar registry");
        require(address(resolver.registry()) == deployed.registry, "resolver registry");
        require(address(reverseRegistrar.registry()) == deployed.registry, "reverse registry");
        require(address(reverseRegistrar.defaultResolver()) == deployed.resolver, "reverse default resolver");
        require(address(controller.registrar()) == deployed.registrar, "controller registrar");
        require(address(controller.registry()) == deployed.registry, "controller registry");
        require(controller.resolver() == deployed.resolver, "controller resolver");
        require(controller.reverseRegistrar() == deployed.reverseRegistrar, "controller reverse registrar");
        require(controller.treasury() == TREASURY, "treasury");
        require(registrar.GRACE_PERIOD() == GRACE, "grace period");
        require(controller.yearlyPrices(3) == 500 ether, "three character price");
        require(controller.yearlyPrices(4) == 100 ether, "four character price");
        require(controller.yearlyPrices(5) == 5 ether, "five-plus character price");

        vm.prank(USER);
        vm.expectRevert(Governed.GovernanceOnly.selector);
        registrar.addController(USER);
        vm.prank(USER);
        vm.expectRevert(Governed.GovernanceOnly.selector);
        controller.setPrice(3, 600 ether);
        require(!registrar.controllers(USER), "unauthorised controller unchanged");
        require(controller.yearlyPrices(3) == 500 ether, "unauthorised price unchanged");

        vm.prank(TIMELOCK);
        registrar.addController(USER);
        require(registrar.controllers(USER), "governance can add controller");
        vm.prank(TIMELOCK);
        controller.setPrice(3, 600 ether);
        require(controller.yearlyPrices(3) == 600 ether, "governance can set price");
    }

    function _deploy() private returns (NamesDeployment.Addresses memory) {
        uint256[3] memory prices = [uint256(500 ether), uint256(100 ether), uint256(5 ether)];
        return new NamesDeployment().deploy(TIMELOCK, EMERGENCY, TREASURY, prices, GRACE);
    }
}
