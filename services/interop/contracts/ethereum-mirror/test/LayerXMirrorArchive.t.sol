// SPDX-License-Identifier: Apache-2.0
pragma solidity 0.8.30;

import {LayerXMirrorArchive} from "../LayerXMirrorArchive.sol";

interface ArchiveVm {
    struct Log {
        bytes32[] topics;
        bytes data;
        address emitter;
    }

    function load(address target, bytes32 slot) external view returns (bytes32);
    function recordLogs() external;
    function getRecordedLogs() external returns (Log[] memory);
}

contract MirrorUnauthorizedCaller {
    function begin(LayerXMirrorArchive archive) external {
        archive.begin(
            keccak256("unauthorized-commitment"),
            1,
            1,
            keccak256("checkpoint"),
            1,
            1,
            keccak256("archive"),
            keccak256("chain")
        );
    }
}

contract LayerXMirrorArchiveTest {
    ArchiveVm private constant vm = ArchiveVm(address(uint160(uint256(keccak256("hevm cheat code")))));

    function payloadOfLength(uint256 length) private pure returns (bytes memory payload) {
        payload = new bytes(length);
        for (uint256 i = 0; i < length; ++i) {
            payload[i] = bytes1(uint8(i));
        }
    }

    function chunkOf(bytes memory payload, uint256 offset, uint256 size) private pure returns (bytes memory chunk) {
        uint256 length = payload.length - offset;
        if (length > size) length = size;
        chunk = new bytes(length);
        for (uint256 i = 0; i < length; ++i) {
            chunk[i] = payload[offset + i];
        }
    }

    function openAndAppend(
        LayerXMirrorArchive archive,
        bytes32 commitment,
        bytes memory payload,
        uint256 chunkBytes,
        bytes32 digest
    ) private {
        uint32 count = uint32((payload.length + chunkBytes - 1) / chunkBytes);
        bytes32 chain;
        for (uint32 i = 0; i < count; ++i) {
            bytes memory chunk = chunkOf(payload, uint256(i) * chunkBytes, chunkBytes);
            chain = keccak256(abi.encodePacked(chain, i, sha256(chunk), uint32(chunk.length)));
        }
        archive.begin(commitment, 1, 7, keccak256("block"), uint64(payload.length), count, digest, chain);
        for (uint32 i = 0; i < count; ++i) {
            bytes memory chunk = chunkOf(payload, uint256(i) * chunkBytes, chunkBytes);
            uint256 remainingGas = gasleft();
            archive.append(commitment, i, chunk);
            require(remainingGas - gasleft() < 29_000_000, "append exceeds normal transaction budget");
            archive.append(commitment, i, chunk);
            require(keccak256(archive.chunk(commitment, i)) == keccak256(chunk), "stored chunk differs");
            archive.begin(commitment, 1, 7, keccak256("block"), uint64(payload.length), count, digest, chain);
        }
    }

    function requireRefusal(bool success, bytes memory reason, bytes4 expected) private pure {
        require(!success && reason.length >= 4, "operation did not refuse");
        bytes4 selector;
        assembly ("memory-safe") {
            selector := mload(add(reason, 32))
        }
        require(selector == expected, "unexpected refusal");
    }

    function testRejectsIncorrectArchiveDigestWithCorrectChunkChain() public {
        LayerXMirrorArchive archive = new LayerXMirrorArchive(address(this));
        bytes memory payload = payloadOfLength(193);
        bytes32 commitment = keccak256("incorrect-digest");
        bytes32 wrongDigest = bytes32(uint256(sha256(payload)) ^ 1);
        openAndAppend(archive, commitment, payload, 63, wrongDigest);
        (bool success, bytes memory reason) = address(archive).call(abi.encodeCall(archive.finalize, (commitment)));
        requireRefusal(success, reason, LayerXMirrorArchive.ArchiveDigestMismatch.selector);
        (,, bytes32 storedDigest, bool finalized) = archive.manifest(commitment);
        require(storedDigest == wrongDigest && !finalized, "invalid archive finalized");
        (success, reason) = address(archive).call(abi.encodeCall(archive.finalize, (commitment)));
        requireRefusal(success, reason, LayerXMirrorArchive.ArchiveDigestMismatch.selector);
    }

    function testMultiChunkPaddingBoundariesAndRetries() public {
        LayerXMirrorArchive archive = new LayerXMirrorArchive(address(this));
        uint256[13] memory lengths = [uint256(1), 55, 56, 63, 64, 65, 119, 120, 127, 128, 129, 193, 1025];
        for (uint256 i = 0; i < lengths.length; ++i) {
            bytes memory payload = payloadOfLength(lengths[i]);
            bytes32 commitment = keccak256(abi.encodePacked("padding", lengths[i]));
            openAndAppend(archive, commitment, payload, 31, sha256(payload));
            archive.finalize(commitment);
            archive.finalize(commitment);
            (,, bytes32 digest, bool finalized) = archive.manifest(commitment);
            require(digest == sha256(payload) && finalized, "SHA256 padding mismatch");
        }
    }

    function testMaximumChunkAndRemainder() public {
        LayerXMirrorArchive archive = new LayerXMirrorArchive(address(this));
        bytes memory payload = payloadOfLength(archive.MAX_CHUNK_BYTES() + 63);
        bytes32 commitment = keccak256("maximum-chunk");
        openAndAppend(archive, commitment, payload, archive.MAX_CHUNK_BYTES(), sha256(payload));
        archive.finalize(commitment);
        (,, bytes32 digest, bool finalized) = archive.manifest(commitment);
        require(digest == sha256(payload) && finalized, "maximum chunk digest mismatch");
    }

    function testRefusedAppendDoesNotAdvanceDigest() public {
        LayerXMirrorArchive archive = new LayerXMirrorArchive(address(this));
        bytes memory first = payloadOfLength(63);
        bytes memory second = payloadOfLength(65);
        bytes memory payload = bytes.concat(first, second);
        bytes32 commitment = keccak256("failed-append");
        bytes32 chain = keccak256(abi.encodePacked(bytes32(0), uint32(0), sha256(first), uint32(first.length)));
        chain = keccak256(abi.encodePacked(chain, uint32(1), sha256(second), uint32(second.length)));
        archive.begin(commitment, 1, 7, keccak256("block"), uint64(payload.length), 2, sha256(payload), chain);
        (bool emptySuccess, bytes memory emptyReason) =
            address(archive).call(abi.encodeCall(archive.append, (commitment, 0, bytes(""))));
        requireRefusal(emptySuccess, emptyReason, LayerXMirrorArchive.InvalidManifest.selector);
        (bool success, bytes memory reason) =
            address(archive).call(abi.encodeCall(archive.append, (commitment, 1, second)));
        requireRefusal(success, reason, LayerXMirrorArchive.ChunkOrder.selector);
        archive.append(commitment, 0, first);
        (success, reason) = address(archive).call(abi.encodeCall(archive.append, (commitment, 0, second)));
        requireRefusal(success, reason, LayerXMirrorArchive.ChunkConflict.selector);
        (success, reason) = address(archive).call(abi.encodeCall(archive.finalize, (commitment)));
        requireRefusal(success, reason, LayerXMirrorArchive.IncompleteArchive.selector);
        bytes memory oversize = payloadOfLength(archive.MAX_CHUNK_BYTES() + 1);
        (success, reason) = address(archive).call(abi.encodeCall(archive.append, (commitment, 1, oversize)));
        requireRefusal(success, reason, LayerXMirrorArchive.InvalidManifest.selector);
        archive.append(commitment, 0, first);
        archive.append(commitment, 1, second);
        archive.finalize(commitment);
        (,, bytes32 digest, bool finalized) = archive.manifest(commitment);
        require(digest == sha256(payload) && finalized, "refusal advanced digest");
    }

    function testChunkCreationFailureIsAtomic() public {
        LayerXMirrorArchive archive = new LayerXMirrorArchive(address(this));
        bytes memory payload = payloadOfLength(archive.MAX_CHUNK_BYTES());
        bytes32 commitment = keccak256("creation-failure");
        bytes32 digest = sha256(payload);
        bytes32 chain = keccak256(abi.encodePacked(bytes32(0), uint32(0), digest, uint32(payload.length)));
        archive.begin(commitment, 1, 7, keccak256("block"), uint64(payload.length), 1, digest, chain);
        (bool success,) = address(archive).call{gas: 100_000}(abi.encodeCall(archive.append, (commitment, 0, payload)));
        require(!success, "underfunded chunk creation succeeded");
        require(archive.chunk(commitment, 0).length == 0, "failed creation published a chunk");
        archive.append(commitment, 0, payload);
        archive.finalize(commitment);
        (,, bytes32 storedDigest, bool finalized) = archive.manifest(commitment);
        require(storedDigest == digest && finalized, "failed creation advanced archive hash");
        require(keccak256(archive.chunk(commitment, 0)) == keccak256(payload), "split chunk retrieval mismatch");
    }

    function testArchiveRoundTripAndIdempotence() public {
        LayerXMirrorArchive archive = new LayerXMirrorArchive(address(this));
        bytes memory payload = hex"0102030405";
        bytes32 commitment = keccak256("commitment");
        bytes32 digest = sha256(payload);
        bytes32 expectedChain = keccak256(abi.encodePacked(bytes32(0), uint32(0), digest, uint32(payload.length)));

        archive.begin(commitment, 1, 7, keccak256("checkpoint"), uint64(payload.length), 1, digest, expectedChain);
        archive.begin(commitment, 1, 7, keccak256("checkpoint"), uint64(payload.length), 1, digest, expectedChain);
        archive.append(commitment, 0, payload);
        archive.append(commitment, 0, payload);
        archive.finalize(commitment);
        archive.finalize(commitment);

        (uint64 totalBytes, uint32 totalChunks, bytes32 archiveDigest, bool finalized) = archive.manifest(commitment);
        require(totalBytes == payload.length, "total bytes");
        require(totalChunks == 1, "total chunks");
        require(archiveDigest == digest, "archive digest");
        require(finalized, "not finalized");
        require(keccak256(archive.chunk(commitment, 0)) == keccak256(payload), "chunk mismatch");
    }

    function testOnlyPublisherCanOpenArchive() public {
        LayerXMirrorArchive archive = new LayerXMirrorArchive(address(this));
        MirrorUnauthorizedCaller caller = new MirrorUnauthorizedCaller();
        (bool success, bytes memory reason) =
            address(caller).call(abi.encodeCall(MirrorUnauthorizedCaller.begin, (archive)));
        require(!success && reason.length >= 4, "unauthorized publication accepted");
        bytes4 selector;
        assembly ("memory-safe") {
            selector := mload(add(reason, 32))
        }
        require(selector == LayerXMirrorArchive.InvalidPublisher.selector, "wrong refusal");
    }

    function testBeginRecordsCommittedHeightAndBlockHash() public {
        LayerXMirrorArchive archive = new LayerXMirrorArchive(address(this));
        bytes32 commitment = keccak256("committed-height-and-hash");
        uint64 height = type(uint64).max - 42;
        bytes32 blockHash = keccak256("committed-block");
        bytes memory payload = hex"0102030405";
        bytes32 digest = sha256(payload);
        bytes32 chain = keccak256(abi.encodePacked(bytes32(0), uint32(0), digest, uint32(payload.length)));
        require(
            archive.begin.selector
                == bytes4(keccak256("begin(bytes32,uint32,uint64,bytes32,uint64,uint32,bytes32,bytes32)")),
            "begin selector changed"
        );
        (bool success, bytes memory reason) =
            address(archive).call(abi.encodeCall(archive.append, (commitment, 0, payload)));
        requireRefusal(success, reason, LayerXMirrorArchive.InvalidManifest.selector);

        vm.recordLogs();
        archive.begin(commitment, 125, height, blockHash, uint64(payload.length), 1, digest, chain);
        ArchiveVm.Log[] memory logs = vm.getRecordedLogs();
        require(logs.length == 1, "manifest event count");
        require(logs[0].emitter == address(archive), "manifest event emitter");
        require(logs[0].topics.length == 2, "manifest event topics");
        require(
            logs[0].topics[0] == keccak256("ManifestOpened(bytes32,uint32,uint64,bytes32,uint64,uint32,bytes32)"),
            "manifest event signature"
        );
        require(logs[0].topics[1] == commitment, "manifest event commitment");
        require(
            keccak256(logs[0].data)
                == keccak256(abi.encode(uint32(125), height, blockHash, uint64(payload.length), uint32(1), digest)),
            "manifest event fields"
        );
        bytes32 manifestSlot = keccak256(abi.encode(commitment, uint256(0)));
        uint256 coordinates = uint256(vm.load(address(archive), manifestSlot));
        require(uint32(coordinates) == 125, "stored network");
        require(uint64(coordinates >> 32) == height, "stored committed height");
        require(vm.load(address(archive), bytes32(uint256(manifestSlot) + 1)) == blockHash, "stored block hash");

        archive.begin(commitment, 125, height, blockHash, uint64(payload.length), 1, digest, chain);
        archive.append(commitment, 0, payload);
        require(keccak256(archive.chunk(commitment, 0)) == keccak256(payload), "committed chunk mismatch");
        archive.finalize(commitment);
        (uint64 totalBytes, uint32 totalChunks, bytes32 storedDigest, bool finalized) = archive.manifest(commitment);
        require(totalBytes == payload.length && totalChunks == 1, "manifest bounds");
        require(storedDigest == digest && finalized, "committed archive not finalized");
    }

    function testBeginRejectsZeroBlockHash() public {
        LayerXMirrorArchive archive = new LayerXMirrorArchive(address(this));
        bytes32 commitment = keccak256("zero-block-hash");
        bytes memory payload = hex"0102030405";
        bytes32 digest = sha256(payload);
        bytes32 chain = keccak256(abi.encodePacked(bytes32(0), uint32(0), digest, uint32(payload.length)));
        (bool success, bytes memory reason) = address(archive)
            .call(
                abi.encodeCall(
                    archive.begin, (commitment, 125, 7, bytes32(0), uint64(payload.length), 1, digest, chain)
                )
            );
        requireRefusal(success, reason, LayerXMirrorArchive.InvalidManifest.selector);
        (uint64 totalBytes, uint32 totalChunks, bytes32 storedDigest, bool finalized) = archive.manifest(commitment);
        require(
            totalBytes == 0 && totalChunks == 0 && storedDigest == bytes32(0) && !finalized, "invalid manifest stored"
        );
        (success, reason) = address(archive).call(abi.encodeCall(archive.append, (commitment, 0, payload)));
        requireRefusal(success, reason, LayerXMirrorArchive.InvalidManifest.selector);
        require(archive.chunk(commitment, 0).length == 0, "uncommitted chunk stored");
    }

    function testBeginRejectsDifferentBlockHashOnReopen() public {
        LayerXMirrorArchive archive = new LayerXMirrorArchive(address(this));
        bytes32 commitment = keccak256("reopen-block-hash");
        bytes32 blockHash = keccak256("committed-block");
        bytes memory payload = hex"0102030405";
        bytes32 digest = sha256(payload);
        bytes32 chain = keccak256(abi.encodePacked(bytes32(0), uint32(0), digest, uint32(payload.length)));
        archive.begin(commitment, 125, 7, blockHash, uint64(payload.length), 1, digest, chain);
        (bool success, bytes memory reason) = address(archive)
            .call(
                abi.encodeCall(
                    archive.begin,
                    (commitment, 125, 7, keccak256("different-block"), uint64(payload.length), 1, digest, chain)
                )
            );
        requireRefusal(success, reason, LayerXMirrorArchive.ManifestConflict.selector);
        (success, reason) = address(archive)
            .call(
                abi.encodeCall(archive.begin, (commitment, 125, 8, blockHash, uint64(payload.length), 1, digest, chain))
            );
        requireRefusal(success, reason, LayerXMirrorArchive.ManifestConflict.selector);
        archive.begin(commitment, 125, 7, blockHash, uint64(payload.length), 1, digest, chain);
        archive.append(commitment, 0, payload);
        archive.finalize(commitment);
        (,, bytes32 storedDigest, bool finalized) = archive.manifest(commitment);
        require(storedDigest == digest && finalized, "conflict changed committed manifest");
        bytes32 manifestSlot = keccak256(abi.encode(commitment, uint256(0)));
        require(
            vm.load(address(archive), bytes32(uint256(manifestSlot) + 1)) == blockHash, "conflict changed block hash"
        );
    }

    function testMirrorRejectsValue() public {
        LayerXMirrorArchive archive = new LayerXMirrorArchive(address(this));
        (bool success,) = address(archive).call{value: 1}("");
        require(!success, "value accepted");
    }

    receive() external payable {}
}
