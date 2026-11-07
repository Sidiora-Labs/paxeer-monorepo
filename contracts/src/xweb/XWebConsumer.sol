// SPDX-License-Identifier: MIT
pragma solidity ^0.8.27;

import {IXWeb, IXWebConsumer, XWEB_CONTRACT, XWEB_PRECOMPILE_ADDRESS} from "../precompiles/IXWeb.sol";

/// A contract that asks the xweb precompile for attested web data and keeps
/// the answer. Only the precompile may deliver an answer, only for a request
/// this contract issued, and only once.
contract XWebConsumer is IXWebConsumer {
    struct Answer {
        bool answered;
        bytes32 contentDigest;
        uint32 fullLength;
        bytes response;
    }

    error NotXWeb(address caller);
    error UnknownRequest(uint64 requestId);
    error AlreadyAnswered(uint64 requestId);
    error NotRefundable(uint64 requestId);
    error RefundTransferFailed(uint64 requestId);
    error UnexpectedProgramReturn();

    event Asked(uint64 indexed requestId, uint8 kind, bytes payload, uint64 callbackGas);
    event Answered(uint64 indexed requestId, bytes32 contentDigest, uint32 fullLength, bytes response);
    event Refunded(uint64 indexed requestId, address indexed payer, uint256 amount);

    mapping(uint64 => bool) public issued;
    mapping(uint64 => address) public payer;
    mapping(uint64 => uint256) public paid;
    mapping(uint64 => Answer) private answers;

    /// Requests kind (1 fetch, 2 search) of payload, forwarding msg.value as
    /// the fee, and returns the request id.
    function ask(uint8 kind, bytes calldata payload, uint64 callbackGas) external payable returns (uint64 requestId) {
        requestId = XWEB_CONTRACT.request{value: msg.value}(kind, payload, callbackGas);
        issued[requestId] = true;
        payer[requestId] = msg.sender;
        paid[requestId] = msg.value;
        emit Asked(requestId, kind, payload, callbackGas);
    }

    function refund(uint64 requestId) external {
        address recipient = payer[requestId];
        if (recipient == address(0)) revert NotRefundable(requestId);
        uint256 amount = paid[requestId];
        delete payer[requestId];
        delete paid[requestId];

        XWEB_CONTRACT.refund(requestId);
        (bool success,) = payable(recipient).call{value: amount}("");
        if (!success) revert RefundTransferFailed(requestId);
        emit Refunded(requestId, recipient, amount);
    }

    function onXWebResponse(uint64 requestId, bytes32 contentDigest, uint32 fullLength, bytes calldata response)
        external
    {
        if (msg.sender != XWEB_PRECOMPILE_ADDRESS) revert NotXWeb(msg.sender);
        if (!issued[requestId]) revert UnknownRequest(requestId);
        Answer storage answer = answers[requestId];
        if (answer.answered) revert AlreadyAnswered(requestId);
        answer.answered = true;
        answer.contentDigest = contentDigest;
        answer.fullLength = fullLength;
        answer.response = response;
        emit Answered(requestId, contentDigest, fullLength, response);
    }

    function answer(uint64 requestId) external view returns (Answer memory) {
        return answers[requestId];
    }

    /// Forwards a signed program observation to the native precompile.
    function submitProgramObservation(bytes calldata observation) external {
        // A typed void call checks EXTCODESIZE, but native precompiles have no code.
        (bool success, bytes memory result) =
            XWEB_PRECOMPILE_ADDRESS.call(abi.encodeCall(IXWeb.fulfilProgram, (observation)));
        if (!success) {
            assembly ("memory-safe") {
                revert(add(result, 32), mload(result))
            }
        }
        if (result.length != 0) revert UnexpectedProgramReturn();
    }

    function programResult(bytes32 programId, uint64 requestId)
        external
        view
        returns (bytes32 contentDigest, uint32 fullLength, uint32 responseLength, bytes memory response)
    {
        return XWEB_CONTRACT.getProgramResult(programId, requestId);
    }

    function kernelNetworkId() external view returns (uint32) {
        return XWEB_CONTRACT.getKernelNetworkId();
    }
}
