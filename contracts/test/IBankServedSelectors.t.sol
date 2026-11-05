// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import {IBank} from "../src/precompiles/IBank.sol";

contract IBankServedSelectorsTest {
    function test_IBankDeclaresOnlyServedSelectors() public pure {
        bytes4 expected = bytes4(keccak256("send(address,address,string,uint256)"))
            ^ bytes4(keccak256("balance(address,string)")) ^ bytes4(keccak256("name(string)"))
            ^ bytes4(keccak256("symbol(string)")) ^ bytes4(keccak256("decimals(string)"))
            ^ bytes4(keccak256("supply(string)"));

        require(type(IBank).interfaceId == expected, "IBank declares unserved selectors");
    }
}
