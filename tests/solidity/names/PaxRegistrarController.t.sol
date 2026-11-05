// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.24;

import {PaxRegistry} from "../../../contracts/names/PaxRegistry.sol";
import {PaxBaseRegistrar} from "../../../contracts/names/PaxBaseRegistrar.sol";
import {PaxRegistrarController} from "../../../contracts/names/PaxRegistrarController.sol";
import {Registration} from "../../../contracts/names/IPaxNames.sol";
import {Governed} from "../../../contracts/security/Governed.sol";

interface PaxNamesVm {
    function warp(uint256 timestamp) external;
    function prank(address account) external;
    function deal(address account, uint256 balance) external;
    function expectRevert(bytes4 selector) external;
    function load(address account, bytes32 slot) external view returns (bytes32);
}

contract PaxRegistrarControllerTest {
    PaxNamesVm private constant vm = PaxNamesVm(address(uint160(uint256(keccak256("hevm cheat code")))));
    address private constant USER = address(0xA11CE);
    address private constant BUYER = address(0xB0B);
    address private constant TREASURY = address(0xFEE);
    address private constant COUNCIL = address(0xCAFE);
    address private constant GOVERNANCE = address(uint160(uint256(keccak256("governance"))));
    uint256 private constant YEAR = 365 days;
    string private constant TUPLE = "(string,address,uint256,bytes32,address,bytes[],uint8,bytes32)";
    PaxRegistry private registry;
    PaxBaseRegistrar private registrar;
    PaxRegistrarController private controller;

    function setUp() public {
        registry = new PaxRegistry();
        registrar = new PaxBaseRegistrar(address(registry), GOVERNANCE, COUNCIL, 90 days);
        registry.setSubnodeOwner(bytes32(0), keccak256("pax"), address(registrar));
        controller = new PaxRegistrarController(
            address(registrar), address(registry), address(0), address(0), TREASURY, GOVERNANCE, COUNCIL
        );
        vm.prank(GOVERNANCE);
        registrar.addController(address(controller));
        vm.prank(GOVERNANCE);
        controller.setPrice(3, 500 ether);
        vm.prank(GOVERNANCE);
        controller.setPrice(4, 100 ether);
        vm.prank(GOVERNANCE);
        controller.setPrice(5, 5 ether);
        vm.deal(USER, 2000 ether);
        vm.warp(10 days);
    }

    function testCommitRegisterRenewFollowsTheWalletAbi() public {
        bytes memory tuple = _walletTuple("alice", USER, YEAR, keccak256("secret"), keccak256("referrer"));
        (bool made, bytes memory output) =
            address(controller).staticcall(abi.encodePacked(_selector("makeCommitment"), tuple));
        require(made && abi.decode(output, (bytes32)) == keccak256(tuple), "wallet commitment tuple");
        bytes32 commitment = abi.decode(output, (bytes32));
        _walletCall(abi.encodeWithSignature("commit(bytes32)", commitment), 0);
        (bool readAge, bytes memory age) = address(controller).staticcall(abi.encodeWithSignature("minCommitmentAge()"));
        require(readAge && abi.decode(age, (uint256)) == 60, "wallet minimum age");
        vm.warp(block.timestamp + abi.decode(age, (uint256)) + 1);
        uint256 registeredAt = block.timestamp;
        (uint256 base, uint256 premium) = _walletPrice("alice", YEAR);
        require(base == 5 ether && premium == 0 && _walletAvailable("alice"), "wallet quote");
        uint256 beforeBalance = USER.balance;
        _walletCall(abi.encodePacked(_selector("register"), tuple), base);
        uint256 id = uint256(keccak256("alice"));
        bytes32 node = keccak256(abi.encodePacked(registrar.PAX_NODE(), bytes32(id)));
        require(registrar.ownerOf(id) == USER && registrar.balanceOf(USER) == 1, "labelhash token");
        require(registry.owner(node) == USER, "registry owner");
        require(
            registrar.expiries(id) == registeredAt + YEAR && registrar.nameExpires(id) == registeredAt + YEAR, "expiry"
        );
        require(
            TREASURY.balance == base && USER.balance == beforeBalance - base && address(controller).balance == 0,
            "exact payment"
        );
        require(!_walletAvailable("alice") && controller.commitments(commitment) == 0, "commit consumed");
        uint256 expiry = registrar.nameExpires(id);
        beforeBalance = USER.balance;
        _walletCall(abi.encodeWithSignature("renew(string,uint256)", "alice", YEAR), base + 1 ether);
        require(registrar.nameExpires(id) == expiry + YEAR, "renew expiry");
        require(
            TREASURY.balance == base * 2 && USER.balance == beforeBalance - base && address(controller).balance == 0,
            "renew refund"
        );
        bytes memory secondTuple = _walletTuple("other", USER, YEAR, keccak256("other secret"), bytes32(0));
        _walletCall(abi.encodeWithSignature("commit(bytes32)", keccak256(secondTuple)), 0);
        vm.warp(block.timestamp + controller.minCommitmentAge());
        beforeBalance = USER.balance;
        _walletCall(abi.encodePacked(_selector("register"), secondTuple), base + 2 ether);
        require(
            USER.balance == beforeBalance - base && TREASURY.balance == base * 3 && address(controller).balance == 0,
            "registration refund"
        );
        vm.prank(BUYER);
        vm.expectRevert(PaxBaseRegistrar.Unauthorised.selector);
        registrar.transferFrom(USER, BUYER, id);
        vm.prank(USER);
        registrar.approve(BUYER, id);
        vm.prank(BUYER);
        registrar.safeTransferFrom(USER, BUYER, id);
        require(registrar.ownerOf(id) == BUYER && registry.owner(node) == BUYER, "transfer registry owner");
        require(
            registrar.getApproved(id) == address(0) && registrar.balanceOf(BUYER) == 1, "transfer approval and balance"
        );
        vm.prank(BUYER);
        registry.setOwner(node, USER);
        vm.prank(BUYER);
        registrar.reclaim(id, BUYER);
        require(registry.owner(node) == BUYER, "reclaim owner");
        vm.warp(registrar.nameExpires(id));
        vm.expectRevert(PaxBaseRegistrar.NonexistentToken.selector);
        registrar.ownerOf(id);
        require(!_walletAvailable("alice"), "grace unavailable");
        vm.warp(registrar.nameExpires(id) + registrar.GRACE_PERIOD() + 1);
        require(_walletAvailable("alice"), "released after grace");
        bytes memory replacement = _walletTuple("alice", USER, YEAR, keccak256("replacement"), bytes32(0));
        _walletCall(abi.encodeWithSignature("commit(bytes32)", keccak256(replacement)), 0);
        vm.warp(block.timestamp + controller.minCommitmentAge());
        _walletCall(abi.encodePacked(_selector("register"), replacement), base);
        require(
            registrar.ownerOf(id) == USER && registry.owner(node) == USER && registrar.balanceOf(BUYER) == 0,
            "reregistration"
        );
    }

    function testRegistrationRefusesEarlyRevealAndInvalidLabels() public {
        Registration memory registration =
            abi.decode(_walletTuple("alice", USER, YEAR, keccak256("age"), bytes32(0)), (Registration));
        bytes32 commitment = controller.makeCommitment(registration);
        vm.prank(USER);
        controller.commit(commitment);
        vm.prank(USER);
        vm.expectRevert(PaxRegistrarController.CommitmentTooNew.selector);
        controller.register{value: 5 ether}(registration);
        vm.prank(BUYER);
        vm.expectRevert(PaxRegistrarController.CommitmentExists.selector);
        controller.commit(commitment);
        vm.warp(block.timestamp + controller.maxCommitmentAge() + 1);
        vm.prank(USER);
        vm.expectRevert(PaxRegistrarController.CommitmentExpired.selector);
        controller.register{value: 5 ether}(registration);
        string[6] memory invalid = [
            string("ab"),
            "-abc",
            "abc-",
            "ABC",
            "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
            "ab_c"
        ];
        for (uint256 i; i < invalid.length; ++i) {
            registration.label = invalid[i];
            require(!controller.available(invalid[i]), "invalid available");
            bytes32 invalidCommitment = controller.makeCommitment(registration);
            vm.prank(USER);
            controller.commit(invalidCommitment);
            vm.warp(block.timestamp + controller.minCommitmentAge());
            vm.prank(USER);
            vm.expectRevert(PaxRegistrarController.InvalidLabel.selector);
            controller.register{value: 500 ether}(registration);
        }
        require(
            controller.available("abc") && controller.available("a-1")
                && controller.available("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
            "valid boundaries"
        );
        registration.label = "alice";
        vm.prank(USER);
        controller.commit(commitment);
        vm.warp(block.timestamp + controller.minCommitmentAge());
        vm.prank(USER);
        vm.expectRevert(PaxRegistrarController.InsufficientPayment.selector);
        controller.register{value: 5 ether - 1}(registration);
        require(controller.available("alice") && controller.commitments(commitment) != 0, "failed reveal changed state");
        vm.prank(USER);
        vm.expectRevert(PaxBaseRegistrar.ControllerOnly.selector);
        registrar.register(uint256(keccak256("alice")), USER, YEAR);
    }

    function testOnlyPaxNamesAreRegistrable() public {
        bytes32 pax = keccak256(abi.encodePacked(bytes32(0), keccak256("pax")));
        require(registrar.baseNode() == pax && registrar.PAX_NODE() == pax, "registrar parent must be pax");
        bytes32 labelhash = keccak256("alice");
        bytes32 otherParent = registry.setSubnodeOwner(bytes32(0), keccak256("other"), address(registrar));
        bytes32 otherNode = keccak256(abi.encodePacked(otherParent, labelhash));
        bytes memory foreignRegistration = abi.encodeWithSignature(
            "register(bytes32,uint256,address,uint256)", otherParent, uint256(labelhash), USER, YEAR
        );
        vm.prank(USER);
        (bool callerAccepted,) = address(registrar).call(foreignRegistration);
        require(!callerAccepted, "caller supplied another parent");
        vm.prank(address(controller));
        (bool controllerAccepted,) = address(registrar).call(foreignRegistration);
        require(!controllerAccepted, "controller supplied another parent");
        require(registrar.available(uint256(labelhash)), "foreign registration changed availability");
        require(registrar.expiries(uint256(labelhash)) == 0, "foreign registration changed expiry");
        require(registry.owner(otherNode) == address(0), "foreign registration created a record");

        bytes memory tuple = _walletTuple("alice", USER, YEAR, keccak256("pax-only"), bytes32(0));
        _walletCall(abi.encodeWithSignature("commit(bytes32)", keccak256(tuple)), 0);
        vm.warp(block.timestamp + controller.minCommitmentAge());
        (uint256 base,) = _walletPrice("alice", YEAR);
        _walletCall(abi.encodePacked(_selector("register"), tuple), base);
        bytes32 paxNode = keccak256(abi.encodePacked(pax, labelhash));
        require(registrar.ownerOf(uint256(labelhash)) == USER, "pax token owner");
        require(registry.owner(paxNode) == USER, "label must be registered under pax");
        require(registry.owner(otherNode) == address(0), "registration escaped pax parent");
        require(!controller.available("alice.other") && !controller.available("alice.pax"), "labels only");
    }

    function testPriceTiersAndReservedLabelsAreGovernanceSet() public {
        vm.prank(USER);
        vm.expectRevert(Governed.GovernanceOnly.selector);
        controller.setPrice(3, 1);
        vm.prank(USER);
        vm.expectRevert(Governed.GovernanceOnly.selector);
        controller.setReserved(keccak256("admin"), true);
        vm.prank(USER);
        vm.expectRevert(Governed.GovernanceOnly.selector);
        registrar.addController(USER);
        _govern(address(controller), abi.encodeCall(PaxRegistrarController.setPrice, (uint8(3), 600 ether)));
        _govern(address(controller), abi.encodeCall(PaxRegistrarController.setPrice, (uint8(4), 120 ether)));
        _govern(address(controller), abi.encodeCall(PaxRegistrarController.setPrice, (uint8(5), 6 ether)));
        (uint256 three, uint256 premium) = controller.rentPrice("abc", YEAR);
        (uint256 four,) = controller.rentPrice("abcd", YEAR / 2);
        (uint256 five,) = controller.rentPrice("abcde", YEAR);
        (uint256 fraction,) = controller.rentPrice("abcdef", 1 days);
        require(three == 600 ether && four == 60 ether && five == 6 ether && premium == 0, "tier prices");
        require(fraction == uint256(6 ether) * 1 days / YEAR, "prorated price");
        require(controller.available("admin"), "admin initially available");
        _govern(address(controller), abi.encodeCall(PaxRegistrarController.setReserved, (keccak256("admin"), true)));
        require(!controller.available("admin"), "reserved available");
        Registration memory registration =
            abi.decode(_walletTuple("admin", USER, YEAR, bytes32(0), bytes32(0)), (Registration));
        controller.commit(controller.makeCommitment(registration));
        vm.warp(block.timestamp + controller.minCommitmentAge());
        vm.prank(USER);
        vm.expectRevert(PaxRegistrarController.NameUnavailable.selector);
        controller.register{value: 6 ether}(registration);
        _govern(address(controller), abi.encodeCall(PaxRegistrarController.setReserved, (keccak256("admin"), false)));
        require(controller.available("admin"), "reservation not cleared");
        _govern(address(controller), abi.encodeCall(PaxRegistrarController.setCommitmentAges, (120, 2 days)));
        require(controller.minCommitmentAge() == 120 && controller.maxCommitmentAge() == 2 days, "governed ages");
        _govern(address(controller), abi.encodeCall(PaxRegistrarController.setTreasury, (BUYER)));
        require(controller.treasury() == BUYER, "governed treasury");
        _govern(address(registrar), abi.encodeCall(PaxBaseRegistrar.removeController, (address(controller))));
        require(!registrar.controllers(address(controller)), "controller not removed");
    }

    function _walletTuple(string memory label, address owner, uint256 duration, bytes32 secret, bytes32 referrer)
        private
        pure
        returns (bytes memory)
    {
        return abi.encodePacked(
            bytes32(uint256(32)),
            abi.encode(label, owner, duration, secret, address(0), new bytes[](0), uint8(0), referrer)
        );
    }

    function _selector(string memory method) private pure returns (bytes4) {
        return bytes4(keccak256(bytes(string.concat(method, "(", TUPLE, ")"))));
    }

    function _walletCall(bytes memory data, uint256 value) private {
        vm.prank(USER);
        (bool success, bytes memory result) = address(controller).call{value: value}(data);
        if (!success) assembly { revert(add(result, 32), mload(result)) }
    }

    function _walletPrice(string memory label, uint256 duration) private view returns (uint256 base, uint256 premium) {
        (bool success, bytes memory result) =
            address(controller).staticcall(abi.encodeWithSignature("rentPrice(string,uint256)", label, duration));
        require(success, "wallet rentPrice call");
        return abi.decode(result, (uint256, uint256));
    }

    function _walletAvailable(string memory label) private view returns (bool) {
        (bool success, bytes memory result) =
            address(controller).staticcall(abi.encodeWithSignature("available(string)", label));
        require(success, "wallet available call");
        return abi.decode(result, (bool));
    }

    function _govern(address target, bytes memory data) private {
        vm.prank(GOVERNANCE);
        (bool success,) = target.call(data);
        require(success, "governance call failed");
    }
}
