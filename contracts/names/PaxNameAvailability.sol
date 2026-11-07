// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.24;

import {Governed} from "../security/Governed.sol";
import {IPaxPublicResolver} from "./IPaxNames.sol";

interface IXWeb {
    function request(uint8 kind, bytes calldata payload, uint64 callbackGas) external payable returns (uint64);
}

contract PaxNameAvailability is Governed {
    error InvalidConfiguration();
    error PaxContenthashForm();
    error XWebOnly();
    error UnknownRequest(uint64 requestId);

    struct Receipt {
        bytes32 digest;
        uint64 length;
        uint256 atBlock;
        bytes32 contentDigest;
    }

    struct RequestedManifest {
        bytes32 digest;
        bool exists;
    }

    event ReceiptRequested(bytes32 indexed node, uint64 indexed requestId, bytes32 digest);
    event ReceiptRecorded(bytes32 indexed node, bytes32 digest, uint64 length, uint256 atBlock, bytes32 contentDigest);

    address public constant XWEB = 0x0000000000000000000000000000000000001019;
    uint64 public constant CALLBACK_GAS = 200_000;
    bytes4 private constant CONTENTHASH_TAG = 0x70617831;

    IPaxPublicResolver public immutable resolver;
    string public bucketBaseUrl;
    mapping(uint64 => bytes32) public pending;
    mapping(uint64 => RequestedManifest) private requestedManifests;
    mapping(bytes32 => Receipt) public receipts;

    constructor(address resolverAddress, address governanceTimelock, address emergencyAuthority, string memory baseUrl)
        Governed(governanceTimelock, emergencyAuthority)
    {
        if (resolverAddress.code.length == 0) revert InvalidConfiguration();
        resolver = IPaxPublicResolver(resolverAddress);
        bucketBaseUrl = baseUrl;
    }

    function setBucketBaseUrl(string calldata baseUrl) external onlyGovernance {
        bucketBaseUrl = baseUrl;
    }

    function requestReceipt(bytes32 node) external payable returns (uint64 requestId) {
        bytes memory hash = resolver.contenthash(node);
        if (hash.length != 36 || bytes4(hash) != CONTENTHASH_TAG) revert PaxContenthashForm();
        bytes32 digest;
        assembly ("memory-safe") {
            digest := mload(add(hash, 36))
        }
        requestId =
            IXWeb(XWEB).request{value: msg.value}(4, abi.encodePacked(digest, bytes(bucketBaseUrl)), CALLBACK_GAS);
        pending[requestId] = node;
        requestedManifests[requestId] = RequestedManifest(digest, true);
        emit ReceiptRequested(node, requestId, digest);
    }

    function onXWebResponse(uint64 requestId, bytes32 contentDigest, uint64 fullLength, bytes calldata response)
        external
    {
        _recordReceipt(requestId, contentDigest, fullLength);
    }

    function onXWebResponse(uint64 requestId, bytes32 contentDigest, uint32 fullLength, bytes calldata response)
        external
    {
        _recordReceipt(requestId, contentDigest, fullLength);
    }

    function availability(bytes32 node) external view returns (bytes32 digest, uint64 length, uint256 atBlock) {
        Receipt storage receipt = receipts[node];
        return (receipt.digest, receipt.length, receipt.atBlock);
    }

    function _recordReceipt(uint64 requestId, bytes32 contentDigest, uint64 fullLength) private {
        if (msg.sender != XWEB) revert XWebOnly();
        RequestedManifest memory manifest = requestedManifests[requestId];
        if (!manifest.exists) revert UnknownRequest(requestId);
        bytes32 node = pending[requestId];
        receipts[node] = Receipt(manifest.digest, fullLength, block.number, contentDigest);
        delete pending[requestId];
        delete requestedManifests[requestId];
        emit ReceiptRecorded(node, manifest.digest, fullLength, block.number, contentDigest);
    }
}
