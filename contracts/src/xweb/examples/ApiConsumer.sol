// SPDX-License-Identifier: MIT
pragma solidity ^0.8.27;

import {IXWebConsumer, XWEB_CONTRACT, XWEB_PRECOMPILE_ADDRESS} from "../../precompiles/IXWeb.sol";
import {XWebApi} from "../XWebApi.sol";

/// Asks one attestor to call a priced API with a credential only it can read,
/// and keeps the attested price.
contract ApiConsumer is IXWebConsumer {
    error WrongFee(uint256 supplied, uint256 expected);

    mapping(uint64 => bytes) public prices;

    function askPrice(bytes calldata envelope, address attestor) external payable returns (uint64) {
        uint256 fee = XWEB_CONTRACT.fee();
        if (msg.value != fee) revert WrongFee(msg.value, fee);
        return XWebApi.get("https://paxeer.app/api/v1/price?asset=PAX").select("/data/price").withCredential(envelope)
            .single(attestor).submit(200_000);
    }

    function onXWebResponse(uint64 requestId, bytes32, uint32, bytes calldata response) external {
        require(msg.sender == XWEB_PRECOMPILE_ADDRESS, "only xweb");
        prices[requestId] = response;
    }
}
