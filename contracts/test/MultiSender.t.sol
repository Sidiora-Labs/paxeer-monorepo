// SPDX-License-Identifier: UNLICENSED
pragma solidity ^0.8.0;

import {Test} from "forge-std/Test.sol";
import {MultiSender} from "../src/MultiSender.sol";

contract MultiSenderTest is Test {
    function test_batchTransferRejectsOverpayment() public {
        MultiSender multiSender = new MultiSender();
        address[] memory recipients = new address[](2);
        recipients[0] = makeAddr("alice");
        recipients[1] = makeAddr("bob");
        uint256[] memory amounts = new uint256[](2);
        amounts[0] = 1 ether;
        amounts[1] = 2 ether;
        vm.deal(address(this), 10 ether);

        vm.expectRevert("Incorrect amount sent");
        multiSender.batchTransfer{value: 3 ether + 1}(recipients, amounts);
        vm.expectRevert("Incorrect amount sent");
        multiSender.batchTransferEqualAmount{value: 4 ether + 1}(recipients, 2 ether);
        vm.expectRevert("Incorrect amount sent");
        multiSender.batchTransfer{value: 3 ether - 1}(recipients, amounts);
        vm.expectRevert("Incorrect amount sent");
        multiSender.batchTransferEqualAmount{value: 4 ether - 1}(recipients, 2 ether);

        assertEq(address(this).balance, 10 ether);
        assertEq(recipients[0].balance, 0);
        assertEq(recipients[1].balance, 0);
        assertEq(address(multiSender).balance, 0);

        multiSender.batchTransfer{value: 3 ether}(recipients, amounts);
        assertEq(recipients[0].balance, 1 ether);
        assertEq(recipients[1].balance, 2 ether);
        assertEq(address(this).balance, 7 ether);
        assertEq(address(multiSender).balance, 0);

        multiSender.batchTransferEqualAmount{value: 4 ether}(recipients, 2 ether);
        assertEq(recipients[0].balance, 3 ether);
        assertEq(recipients[1].balance, 4 ether);
        assertEq(address(this).balance, 3 ether);
        assertEq(address(multiSender).balance, 0);
    }
}
