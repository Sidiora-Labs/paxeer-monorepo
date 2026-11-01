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

    function testReverseSetNameForAddrRefusesCallersOtherThanTheAccountAndController() public {
        NamesDeployment.Addresses memory deployed = _deploy();
        PaxRegistry registry = PaxRegistry(deployed.registry);
        PaxPublicResolver resolver = PaxPublicResolver(deployed.resolver);
        PaxReverseRegistrar reverseRegistrar = PaxReverseRegistrar(deployed.reverseRegistrar);
        address stranger = address(0xBAD);
        bytes32 reverseNode = reverseRegistrar.node(USER);

        vm.prank(USER);
        require(reverseRegistrar.setName("original.pax") == reverseNode, "returned reverse node");
        vm.prank(stranger);
        vm.expectRevert(PaxReverseRegistrar.Unauthorised.selector);
        reverseRegistrar.setNameForAddr(USER, stranger, deployed.resolver, "stolen.pax");
        require(keccak256(bytes(resolver.name(reverseNode))) == keccak256("original.pax"), "reverse name unchanged");
        require(registry.owner(reverseNode) == USER, "reverse owner unchanged");
        require(registry.resolver(reverseNode) == deployed.resolver, "reverse resolver unchanged");

        vm.prank(stranger);
        reverseRegistrar.setName("stranger.pax");
        require(keccak256(bytes(resolver.name(reverseNode))) == keccak256("original.pax"), "setName binds caller");
        bytes32 strangerNode = reverseRegistrar.node(stranger);
        require(registry.owner(strangerNode) == stranger, "stranger owns own reverse node");
        require(keccak256(bytes(resolver.name(strangerNode))) == keccak256("stranger.pax"), "stranger reverse name");

        vm.prank(USER);
        vm.expectRevert(PaxReverseRegistrar.InvalidConfiguration.selector);
        reverseRegistrar.setNameForAddr(USER, USER, stranger, "invalid.pax");
        require(keccak256(bytes(resolver.name(reverseNode))) == keccak256("original.pax"), "invalid resolver unchanged");

        vm.prank(USER);
        require(reverseRegistrar.setName("updated.pax") == reverseNode, "updated reverse node");
        require(keccak256(bytes(resolver.name(reverseNode))) == keccak256("updated.pax"), "caller updates own name");
        require(registry.owner(reverseNode) == USER, "caller retains reverse node");
        require(registry.resolver(reverseNode) == deployed.resolver, "caller uses default resolver");
        require(keccak256(bytes(resolver.name(strangerNode))) == keccak256("stranger.pax"), "other reverse unchanged");
    }

    function testDeploymentHandsControlToGovernance() public {
        NamesDeployment.Addresses memory deployed;
        {
            NamesDeployment deployment = new NamesDeployment();
            uint256[3] memory prices = [uint256(500 ether), uint256(100 ether), uint256(5 ether)];
            require(address(deployment) != TIMELOCK, "factory differs from governance");
            deployed = deployment.deploy(TIMELOCK, EMERGENCY, TREASURY, prices, GRACE);
            require(
                !PaxBaseRegistrar(deployed.registrar).controllers(deployed.controller), "controller awaits governance"
            );
            require(
                PaxPublicResolver(deployed.resolver).registrarController() == address(0), "resolver awaits governance"
            );
            require(
                PaxRegistrarController(deployed.controller).reverseRegistrar() == address(0),
                "reverse registrar awaits governance"
            );
            (address[] memory targets, bytes[] memory payloads) = deployment.governanceSetup(deployed, prices);
            require(targets.length == 6 && payloads.length == 6, "six governance setup calls");
            require(targets[0] == deployed.registrar && targets[1] == deployed.resolver, "setup registrar and resolver");
            require(
                keccak256(payloads[0])
                    == keccak256(abi.encodeCall(PaxBaseRegistrar.addController, (deployed.controller))),
                "setup adds controller"
            );
            require(
                keccak256(payloads[1])
                    == keccak256(abi.encodeCall(PaxPublicResolver.setRegistrarController, (deployed.controller))),
                "setup sets resolver controller"
            );
            for (uint8 i; i < 3; ++i) {
                require(PaxRegistrarController(deployed.controller).yearlyPrices(i + 3) == 0, "price awaits governance");
                require(targets[i + 2] == deployed.controller, "setup price target");
                require(
                    keccak256(payloads[i + 2])
                        == keccak256(abi.encodeCall(PaxRegistrarController.setPrice, (i + 3, prices[i]))),
                    "setup price payload"
                );
            }
            require(targets[5] == deployed.controller, "setup resolvers target");
            require(
                keccak256(payloads[5])
                    == keccak256(
                        abi.encodeCall(
                            PaxRegistrarController.setResolvers, (deployed.resolver, deployed.reverseRegistrar)
                        )
                    ),
                "setup resolvers payload"
            );
            _assertGovernanceSetupRejected(address(deployment), targets, payloads);
            _assertGovernanceSetupRejected(USER, targets, payloads);
            _executeGovernanceSetup(targets, payloads);
        }
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
        require(reverseRegistrar.controller() == deployed.controller, "reverse controller");
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
        NamesDeployment deployment = new NamesDeployment();
        NamesDeployment.Addresses memory deployed = deployment.deploy(TIMELOCK, EMERGENCY, TREASURY, prices, GRACE);
        (address[] memory targets, bytes[] memory payloads) = deployment.governanceSetup(deployed, prices);
        _executeGovernanceSetup(targets, payloads);
        return deployed;
    }

    function _assertGovernanceSetupRejected(address caller, address[] memory targets, bytes[] memory payloads) private {
        bytes32 governanceOnly = keccak256(abi.encodeWithSelector(Governed.GovernanceOnly.selector));
        for (uint256 i; i < targets.length; ++i) {
            vm.prank(caller);
            (bool success, bytes memory result) = targets[i].call(payloads[i]);
            require(!success && keccak256(result) == governanceOnly, "unauthorised setup governance only");
        }
    }

    function _executeGovernanceSetup(address[] memory targets, bytes[] memory payloads) private {
        for (uint256 i; i < targets.length; ++i) {
            vm.prank(TIMELOCK);
            (bool success,) = targets[i].call(payloads[i]);
            require(success, "timelock governance setup");
        }
    }
}
