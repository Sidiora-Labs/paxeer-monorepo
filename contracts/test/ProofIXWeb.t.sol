// SPDX-License-Identifier: MIT
pragma solidity ^0.8.27;

import {Test} from "forge-std/Test.sol";
import {Vm} from "forge-std/Vm.sol";
import {IXWeb, XWEB_PRECOMPILE_ADDRESS} from "../src/precompiles/IXWeb.sol";
import {XWebConsumer} from "../src/xweb/XWebConsumer.sol";

/// Records only the EVM/native boundary; it does not implement xweb behaviour.
contract IXWebCalldataRecorder {
    Vm private constant VM = Vm(address(uint160(uint256(keccak256("hevm cheat code")))));

    fallback(bytes calldata input) external returns (bytes memory) {
        // The real consumer issues STATICCALL for views. Foundry storage
        // instrumentation retains those bytes without changing the call type.
        VM.store(address(this), bytes32(0), bytes32(input.length));
        for (uint256 i; i < input.length; i += 32) {
            bytes32 word;
            assembly ("memory-safe") {
                word := calldataload(add(input.offset, i))
            }
            VM.store(address(this), bytes32(1 + i / 32), word);
        }
        if (msg.sig == IXWeb.getKernelNetworkId.selector) return abi.encode(uint32(1));
        if (msg.sig == IXWeb.getProgramResult.selector) {
            return abi.encode(
                bytes32(hex"f8f995a5abc8e450e1af6753d023f6d54dfc48c9d935dfdb79149ce3a0786c88"),
                uint32(22),
                uint32(3),
                hex"4100fe"
            );
        }
        require(msg.sig == IXWeb.fulfilProgram.selector, "unexpected IXWeb selector");
        return bytes("");
    }
}

contract ProofIXWebTest is Test {
    bytes private constant OBSERVATION =
        hex"020000000000000000000000000000000000000000000000000000000000000001713200000000000000000000000000000000000000000000000000000000000000000000000000110262da65e513a2dc07e8c56bb6e148a96d6d259e496ec215d58c89522fa9649ce1f8f995a5abc8e450e1af6753d023f6d54dfc48c9d935dfdb79149ce3a0786c8800000016000000034100fe0273f7410dfe41b13773a2cac7aec955172a6fb46bf1cf5cf968836ec6868947936ada9883852b613ef69d87f3d219c3a49cd3f75c5cd21b0f28b783935bb177041ca78c17608f791dacdb345c507aa82d99c6b19628356157f1de602b853d4ea97379281a6836360579bc0eb4929e4aa629aaea4ad9e94186ce4544a660243667f61b";
    bytes private constant FULFIL_CALL =
        hex"2d4bf7ad00000000000000000000000000000000000000000000000000000000000000200000000000000000000000000000000000000000000000000000000000000118020000000000000000000000000000000000000000000000000000000000000001713200000000000000000000000000000000000000000000000000000000000000000000000000110262da65e513a2dc07e8c56bb6e148a96d6d259e496ec215d58c89522fa9649ce1f8f995a5abc8e450e1af6753d023f6d54dfc48c9d935dfdb79149ce3a0786c8800000016000000034100fe0273f7410dfe41b13773a2cac7aec955172a6fb46bf1cf5cf968836ec6868947936ada9883852b613ef69d87f3d219c3a49cd3f75c5cd21b0f28b783935bb177041ca78c17608f791dacdb345c507aa82d99c6b19628356157f1de602b853d4ea97379281a6836360579bc0eb4929e4aa629aaea4ad9e94186ce4544a660243667f61b0000000000000000";
    bytes private constant RESULT_CALL =
        hex"2b2e13fc71320000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000011";
    bytes private constant NETWORK_CALL = hex"fc0d26f4";
    bytes32 private constant PROGRAM_ID = hex"7132000000000000000000000000000000000000000000000000000000000000";

    function recordedCalldata() private view returns (bytes memory data) {
        uint256 length = uint256(vm.load(XWEB_PRECOMPILE_ADDRESS, bytes32(0)));
        data = new bytes(length);
        for (uint256 i; i < length; i++) {
            bytes32 word = vm.load(XWEB_PRECOMPILE_ADDRESS, bytes32(1 + i / 32));
            data[i] = word[i % 32];
        }
    }

    /// Proves: XWebConsumer / IXWeb -> 2d4bf7ad, 2b2e13fc, fc0d26f4 at 0x1019 -> Go Precompile.RunAndCalculateGas / PrecompileExecutor.Execute.
    /// Repairs contracts/src/xweb/XWebConsumer.sol: expose program calls on the
    /// real consumer and forward native void calls without an EXTCODESIZE check.
    /// The independently pinned observation is signed and executed in the Go half.
    function test_ProofIXWebConsumerCalldata() public {
        XWebConsumer consumer = new XWebConsumer();
        IXWebCalldataRecorder recorder = new IXWebCalldataRecorder();
        vm.etch(XWEB_PRECOMPILE_ADDRESS, address(recorder).code);
        assertEq(XWEB_PRECOMPILE_ADDRESS, address(0x1019));
        assertEq(bytes32(IXWeb.fulfilProgram.selector), bytes32(bytes4(hex"2d4bf7ad")));
        assertEq(bytes32(IXWeb.getProgramResult.selector), bytes32(bytes4(hex"2b2e13fc")));
        assertEq(bytes32(IXWeb.getKernelNetworkId.selector), bytes32(bytes4(hex"fc0d26f4")));

        consumer.submitProgramObservation(OBSERVATION);
        assertEq(recordedCalldata(), FULFIL_CALL, "IXWeb.fulfilProgram calldata");

        (bytes32 contentDigest, uint32 fullLength, uint32 responseLength, bytes memory response) =
            consumer.programResult(PROGRAM_ID, uint64(17));
        assertEq(recordedCalldata(), RESULT_CALL, "IXWeb.getProgramResult calldata");
        assertEq(contentDigest, bytes32(hex"f8f995a5abc8e450e1af6753d023f6d54dfc48c9d935dfdb79149ce3a0786c88"));
        assertEq(fullLength, 22);
        assertEq(responseLength, 3);
        assertEq(response, hex"4100fe");

        assertEq(consumer.kernelNetworkId(), 1);
        assertEq(recordedCalldata(), NETWORK_CALL, "IXWeb.getKernelNetworkId calldata");
    }
}
