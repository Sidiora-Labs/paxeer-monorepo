// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.24;

import {SidioraProxyTimelock, ISidioraProxyUpgrade} from "../../../contracts/governance/SidioraProxyTimelock.sol";
import {ERC1967Proxy} from "../../../contracts/launchpad/contracts/meta-ag/proxy/ERC1967Proxy.sol";
import {Treasury} from "../../../contracts/launchpad/contracts/protocol/Treasury.sol";
import {AccessControl} from "../../../contracts/launchpad/contracts/base/AccessControl.sol";

interface SidioraProxyTimelockVm {
    function prank(address account) external;
    function warp(uint256 timestamp) external;
    function expectRevert(bytes4 selector) external;
    function load(address target, bytes32 slot) external view returns (bytes32);
}

contract SidioraProxyTimelockTest {
    SidioraProxyTimelockVm private constant vm =
        SidioraProxyTimelockVm(address(uint160(uint256(keccak256("hevm cheat code")))));
    address private constant PROPOSER = address(0xA11CE);
    address private constant EXECUTOR = address(0xE0EC);
    address private constant OUTSIDER = address(0xB0B);
    uint64 private constant DELAY = 2 days;
    bytes32 private constant SALT = keccak256("treasury upgrade");
    bytes32 private constant IMPLEMENTATION_SLOT = 0x360894a13ba1a3210667c828492db98dca3e2076cc3735a920a3ca505d382bbc;

    SidioraProxyTimelock private timelock;
    Treasury private originalImplementation;
    Treasury private nextImplementation;
    Treasury private treasury;
    bytes private upgradeData;

    function setUp() public {
        vm.warp(1_000_000);
        originalImplementation = new Treasury();
        nextImplementation = new Treasury();
        ERC1967Proxy proxy = new ERC1967Proxy(
            address(originalImplementation), abi.encodeCall(Treasury.initialize, (address(0), address(this)))
        );
        treasury = Treasury(address(proxy));
        timelock = new SidioraProxyTimelock(address(proxy), PROPOSER, EXECUTOR, PROPOSER, DELAY, 1 days, 7 days);
        treasury.grantRole(treasury.DEFAULT_ADMIN_ROLE(), address(timelock));
        upgradeData = abi.encodeCall(
            ISidioraProxyUpgrade.upgradeToAndCall,
            (
                address(nextImplementation),
                abi.encodeCall(AccessControl.grantRole, (treasury.DEPOSITOR_ROLE(), OUTSIDER))
            )
        );
    }

    function test_schedule_recordsOperationIdAndReadyTime() public {
        uint256 nonce = timelock.operationNonce();
        uint256 scheduledAt = block.timestamp;
        bytes32 expectedId = sha256(
            abi.encode(
                block.chainid, address(timelock), address(treasury), uint256(0), sha256(upgradeData), SALT, nonce
            )
        );
        vm.prank(PROPOSER);
        bytes32 id = timelock.schedule(address(treasury), 0, upgradeData, SALT, DELAY);
        require(id == expectedId, "operation id");
        require(timelock.readyAt(id) == scheduledAt + DELAY, "ready time");
        require(timelock.operationNonce() == nonce + 1, "nonce not advanced");
        require(!timelock.completed(id), "scheduled operation completed");
        require(
            timelock.operationId(address(treasury), 0, upgradeData, SALT, nonce) == id, "scheduled operation lookup"
        );
        _assertNotApplied();
    }

    function test_schedule_proposerOnly() public {
        vm.prank(OUTSIDER);
        vm.expectRevert(SidioraProxyTimelock.Unauthorized.selector);
        timelock.schedule(address(treasury), 0, upgradeData, SALT, DELAY);
        vm.prank(EXECUTOR);
        vm.expectRevert(SidioraProxyTimelock.Unauthorized.selector);
        timelock.schedule(address(treasury), 0, upgradeData, SALT, DELAY);
        bytes32 id = timelock.operationId(address(treasury), 0, upgradeData, SALT, 0);
        require(timelock.readyAt(id) == 0, "unauthorized schedule recorded");
        require(timelock.operationNonce() == 0, "unauthorized schedule consumed nonce");
        require(!timelock.completed(id), "unauthorized operation completed");
    }

    function test_schedule_rejectsDelayBelowMinimum() public {
        vm.prank(PROPOSER);
        vm.expectRevert(SidioraProxyTimelock.InvalidOperation.selector);
        timelock.schedule(address(treasury), 0, upgradeData, SALT, DELAY - 1);
        bytes32 id = timelock.operationId(address(treasury), 0, upgradeData, SALT, 0);
        require(timelock.readyAt(id) == 0, "short delay recorded");
        require(timelock.operationNonce() == 0, "short delay consumed nonce");
    }

    function test_execute_rejectsBeforeDelayAppliesAtReadyTimeAndRejectsReplay() public {
        bytes32 id = _schedule();
        uint64 ready = timelock.readyAt(id);
        vm.warp(ready - 1);
        vm.prank(EXECUTOR);
        vm.expectRevert(SidioraProxyTimelock.OperationNotReady.selector);
        timelock.execute(address(treasury), 0, upgradeData, SALT, 0);
        require(!timelock.completed(id), "early execute completed");
        require(timelock.readyAt(id) == ready, "early execute changed ready time");
        _assertNotApplied();

        vm.warp(ready);
        vm.prank(EXECUTOR);
        bytes memory result = timelock.execute(address(treasury), 0, upgradeData, SALT, 0);
        require(result.length == 0, "upgrade return data");
        require(timelock.completed(id), "execute not completed");
        _assertApplied();

        vm.prank(EXECUTOR);
        vm.expectRevert(SidioraProxyTimelock.OperationNotReady.selector);
        timelock.execute(address(treasury), 0, upgradeData, SALT, 0);
        require(timelock.completed(id), "replay changed completion");
        _assertApplied();
    }

    function test_execute_executorOnly() public {
        bytes32 id = _schedule();
        vm.warp(timelock.readyAt(id));
        vm.prank(PROPOSER);
        vm.expectRevert(SidioraProxyTimelock.Unauthorized.selector);
        timelock.execute(address(treasury), 0, upgradeData, SALT, 0);
        vm.prank(OUTSIDER);
        vm.expectRevert(SidioraProxyTimelock.Unauthorized.selector);
        timelock.execute(address(treasury), 0, upgradeData, SALT, 0);
        require(!timelock.completed(id), "unauthorized execute completed");
        _assertNotApplied();
    }

    function test_cancel_proposerWithGuardianRoleOnlyAndPreventsExecution() public {
        bytes32 id = _schedule();
        uint64 ready = timelock.readyAt(id);
        vm.prank(OUTSIDER);
        vm.expectRevert(SidioraProxyTimelock.Unauthorized.selector);
        timelock.cancel(id);
        vm.prank(EXECUTOR);
        vm.expectRevert(SidioraProxyTimelock.Unauthorized.selector);
        timelock.cancel(id);
        require(timelock.readyAt(id) == ready, "unauthorized cancel changed ready time");

        vm.prank(PROPOSER);
        timelock.cancel(id);
        require(timelock.readyAt(id) == 0, "cancel did not clear ready time");
        require(!timelock.completed(id), "cancel marked completed");
        vm.warp(ready);
        vm.prank(EXECUTOR);
        vm.expectRevert(SidioraProxyTimelock.OperationNotReady.selector);
        timelock.execute(address(treasury), 0, upgradeData, SALT, 0);
        require(!timelock.completed(id), "cancelled operation completed");
        _assertNotApplied();
    }

    function test_cancel_executedOperationCannotBeCancelled() public {
        bytes32 id = _schedule();
        uint64 ready = timelock.readyAt(id);
        vm.warp(ready);
        vm.prank(EXECUTOR);
        timelock.execute(address(treasury), 0, upgradeData, SALT, 0);
        vm.prank(PROPOSER);
        vm.expectRevert(SidioraProxyTimelock.OperationNotReady.selector);
        timelock.cancel(id);
        require(timelock.completed(id), "cancel changed completion");
        require(timelock.readyAt(id) == ready, "cancel erased executed operation");
        _assertApplied();
    }

    function test_cancel_distinctGuardianOnly() public {
        SidioraProxyTimelock separateRoles =
            new SidioraProxyTimelock(address(treasury), PROPOSER, EXECUTOR, OUTSIDER, DELAY, 1 days, 7 days);
        vm.prank(PROPOSER);
        bytes32 id = separateRoles.schedule(address(treasury), 0, upgradeData, SALT, DELAY);
        uint64 ready = separateRoles.readyAt(id);
        vm.prank(PROPOSER);
        vm.expectRevert(SidioraProxyTimelock.Unauthorized.selector);
        separateRoles.cancel(id);
        require(separateRoles.readyAt(id) == ready, "proposer without guardian cancelled");
        vm.prank(OUTSIDER);
        separateRoles.cancel(id);
        require(separateRoles.readyAt(id) == 0, "guardian did not cancel");
        require(!separateRoles.completed(id), "guardian cancel marked completed");
        _assertNotApplied();
    }

    function _schedule() private returns (bytes32 id) {
        vm.prank(PROPOSER);
        id = timelock.schedule(address(treasury), 0, upgradeData, SALT, DELAY);
    }

    function _assertNotApplied() private view {
        require(
            vm.load(address(treasury), IMPLEMENTATION_SLOT)
                == bytes32(uint256(uint160(address(originalImplementation)))),
            "implementation changed before execution"
        );
        require(!treasury.hasRole(treasury.DEPOSITOR_ROLE(), OUTSIDER), "migration applied before execution");
    }

    function _assertApplied() private view {
        require(
            vm.load(address(treasury), IMPLEMENTATION_SLOT) == bytes32(uint256(uint160(address(nextImplementation)))),
            "implementation not upgraded"
        );
        require(treasury.hasRole(treasury.DEPOSITOR_ROLE(), OUTSIDER), "migration not applied");
        require(treasury.hasRole(treasury.DEFAULT_ADMIN_ROLE(), address(timelock)), "upgrade lost admin");
    }
}
