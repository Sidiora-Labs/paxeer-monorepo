// SPDX-License-Identifier: Apache-2.0
pragma solidity 0.8.30;

import {LayerXMirrorArchive} from "../LayerXMirrorArchive.sol";

interface Repair2231231Vm {
    struct Log {
        bytes32[] topics;
        bytes data;
        address emitter;
    }

    function addr(uint256 privateKey) external pure returns (address);
    function recordLogs() external;
    function getRecordedLogs() external returns (Log[] memory);
    function load(address target, bytes32 slot) external view returns (bytes32);
    function ffi(string[] calldata commandInput) external returns (bytes memory);
    function projectRoot() external view returns (string memory);
    function toString(bytes calldata value) external pure returns (string memory);
}

contract Repair2231231Test {
    Repair2231231Vm private constant vm = Repair2231231Vm(address(uint160(uint256(keccak256("hevm cheat code")))));

    function testManifestOpenedSevenFieldReceiptAndStoredCoordinate() public {
        LayerXMirrorArchive archive = new LayerXMirrorArchive(address(this));
        bytes memory payload = hex"0102030405";
        bytes32 commitment = keccak256("repair-22.31.231");
        uint32 network = type(uint32).max;
        uint64 height = type(uint64).max - 1;
        bytes32 blockHash = keccak256("committed-block-22.31.231");
        bytes32 digest = sha256(payload);
        bytes32 chain = keccak256(abi.encodePacked(bytes32(0), uint32(0), digest, uint32(payload.length)));

        vm.recordLogs();
        archive.begin(commitment, network, height, blockHash, uint64(payload.length), 1, digest, chain);
        Repair2231231Vm.Log[] memory logs = vm.getRecordedLogs();
        require(logs.length == 1 && logs[0].emitter == address(archive), "manifest receipt producer");
        require(logs[0].topics.length == 2 && logs[0].topics[1] == commitment, "indexed commitment");
        require(
            logs[0].topics[0] == keccak256("ManifestOpened(bytes32,uint32,uint64,bytes32,uint64,uint32,bytes32)"),
            "seven-field manifest topic"
        );
        require(
            logs[0].topics[0] != keccak256("ManifestOpened(bytes32,uint32,uint64,uint64,uint32,bytes32)"),
            "legacy topic accepted"
        );
        require(
            keccak256(logs[0].data)
                == keccak256(abi.encode(network, height, blockHash, uint64(payload.length), uint32(1), digest)),
            "manifest receipt fields"
        );
        bytes32 slot = keccak256(abi.encode(commitment, uint256(0)));
        require(
            uint256(vm.load(address(archive), slot)) == (uint256(network) | (uint256(height) << 32)),
            "stored committed coordinate"
        );
        require(vm.load(address(archive), bytes32(uint256(slot) + 1)) == blockHash, "stored committed block hash");
        (uint64 length, uint32 chunks, bytes32 storedDigest, bool finalized) = archive.manifest(commitment);
        require(length == payload.length && chunks == 1 && storedDigest == digest && !finalized, "stored manifest");

        vm.recordLogs();
        archive.begin(commitment, network, height, blockHash, uint64(payload.length), 1, digest, chain);
        require(vm.getRecordedLogs().length == 0, "recovery must use the original begin receipt");
        archive.append(commitment, 0, payload);
        archive.finalize(commitment);
        (,,, finalized) = archive.manifest(commitment);
        require(finalized && keccak256(archive.chunk(commitment, 0)) == keccak256(payload), "archive round trip");
    }

    function testManifestReceiptPublisherProcessRestartAndResumeChunks() public {
        address publisher = vm.addr(1);
        LayerXMirrorArchive archive = new LayerXMirrorArchive(publisher);
        string[] memory command = new string[](4);
        command[0] = "python3";
        command[1] = string.concat(vm.projectRoot(), "/test/repair2231231/run.py");
        command[2] = vm.toString(bytes.concat(type(LayerXMirrorArchive).creationCode, abi.encode(publisher)));
        command[3] = vm.toString(address(archive).code);
        bytes memory result = vm.ffi(command);
        (bytes32 commitment, bytes32 digest, bytes32 blockHash, uint64 height, uint32 chunks, bool recovered) =
            abi.decode(result, (bytes32, bytes32, bytes32, uint64, uint32, bool));
        require(commitment != bytes32(0) && digest != bytes32(0), "publisher archive identity");
        require(blockHash == bytes32(uint256(type(uint256).max / 15)) && height == 1000, "recorded producer coordinate");
        require(chunks > 1 && recovered, "publisher did not recover and resume chunk publication");
    }
}
