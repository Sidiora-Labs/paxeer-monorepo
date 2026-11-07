// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.24;

import {PaxNameAvailability, IXWeb} from "../../../contracts/names/PaxNameAvailability.sol";
import {PaxRegistry} from "../../../contracts/names/PaxRegistry.sol";
import {PaxPublicResolver} from "../../../contracts/names/PaxPublicResolver.sol";
import {Governed} from "../../../contracts/security/Governed.sol";
import {IXWebConsumer} from "../../../contracts/src/precompiles/IXWeb.sol";

interface PaxAvailabilityVm {
    function etch(address account, bytes calldata code) external;
    function deal(address account, uint256 balance) external;
    function prank(address account) external;
    function roll(uint256 height) external;
    function expectRevert(bytes4 selector) external;
    function expectRevert(bytes calldata data) external;
    function expectEmit(bool topic1, bool topic2, bool topic3, bool data, address emitter) external;
}

contract RecordingReceiptXWeb is IXWeb {
    uint8 public lastKind;
    bytes public lastPayload;
    uint64 public lastCallbackGas;
    uint256 public lastValue;
    address public lastRequester;
    uint64 public lastRequestId;

    function request(uint8 kind, bytes calldata payload, uint64 callbackGas) external payable returns (uint64) {
        lastKind = kind;
        lastPayload = payload;
        lastCallbackGas = callbackGas;
        lastValue = msg.value;
        lastRequester = msg.sender;
        return ++lastRequestId;
    }

    function deliver(uint64 requestId, bytes32 contentDigest, uint32 fullLength, bytes calldata response) external {
        IXWebConsumer(lastRequester).onXWebResponse(requestId, contentDigest, fullLength, response);
    }

    function deliver64(uint64 requestId, bytes32 contentDigest, uint64 fullLength, bytes calldata response) external {
        PaxNameAvailability(lastRequester).onXWebResponse(requestId, contentDigest, fullLength, response);
    }
}

contract PaxNameAvailabilityTest {
    event ReceiptRequested(bytes32 indexed node, uint64 indexed requestId, bytes32 digest);
    event ReceiptRecorded(bytes32 indexed node, bytes32 digest, uint64 length, uint256 atBlock, bytes32 contentDigest);

    PaxAvailabilityVm private constant vm = PaxAvailabilityVm(address(uint160(uint256(keccak256("hevm cheat code")))));
    address private constant XWEB = 0x0000000000000000000000000000000000001019;
    address private constant GOVERNANCE = address(0x710E);
    address private constant COUNCIL = address(0xC011);
    address private constant STRANGER = address(0xBAD);
    bytes32 private constant DIGEST = keccak256("example manifest");
    string private constant BUCKET = "https://bucket.example/pax";
    uint256 private constant FEE = 1_000_003 * 1e12;

    PaxPublicResolver private resolver;
    PaxNameAvailability private consumer;
    RecordingReceiptXWeb private xweb;
    bytes32 private node;

    function setUp() public {
        PaxRegistry registry = new PaxRegistry();
        resolver = new PaxPublicResolver(address(registry), GOVERNANCE, COUNCIL);
        bytes32 pax = registry.setSubnodeOwner(bytes32(0), keccak256("pax"), address(this));
        node = registry.setSubnodeOwner(pax, keccak256("example"), address(this));
        registry.setResolver(node, address(resolver));
        resolver.setContenthash(node, abi.encodePacked(bytes4(0x70617831), DIGEST));
        consumer = new PaxNameAvailability(address(resolver), GOVERNANCE, COUNCIL, BUCKET);
        RecordingReceiptXWeb implementation = new RecordingReceiptXWeb();
        vm.etch(XWEB, address(implementation).code);
        xweb = RecordingReceiptXWeb(XWEB);
        vm.deal(address(this), 3 * FEE);
    }

    function testReceiptRequestCarriesTheManifestDigestAndRecordsTheCallback() public {
        require(keccak256(bytes(consumer.bucketBaseUrl())) == keccak256(bytes(BUCKET)), "initial bucket");
        vm.expectRevert(Governed.GovernanceOnly.selector);
        consumer.setBucketBaseUrl("https://other.example");
        vm.prank(COUNCIL);
        vm.expectRevert(Governed.GovernanceOnly.selector);
        consumer.setBucketBaseUrl("https://other.example");
        string memory bucket = "https://replacement.example/pax";
        vm.prank(GOVERNANCE);
        consumer.setBucketBaseUrl(bucket);
        require(keccak256(bytes(consumer.bucketBaseUrl())) == keccak256(bytes(bucket)), "governed bucket");

        vm.expectRevert(PaxNameAvailability.PaxContenthashForm.selector);
        consumer.requestReceipt(bytes32(uint256(123)));
        require(xweb.lastRequestId() == 0, "empty contenthash must not reach xweb");
        _assertAvailability(bytes32(0), 0, 0, bytes32(0));

        vm.expectEmit(true, true, false, true, address(consumer));
        emit ReceiptRequested(node, 1, DIGEST);
        uint64 requestId = consumer.requestReceipt{value: FEE}(node);
        require(requestId == 1 && xweb.lastRequestId() == requestId, "request id");
        require(xweb.lastKind() == 4, "storage receipt kind");
        require(keccak256(xweb.lastPayload()) == keccak256(abi.encodePacked(DIGEST, bytes(bucket))), "receipt payload");
        require(xweb.lastCallbackGas() == 200_000, "callback gas budget");
        require(xweb.lastRequester() == address(consumer), "consumer is requester");
        require(xweb.lastValue() == FEE && XWEB.balance == FEE, "fee forwarded");
        require(address(consumer).balance == 0 && address(this).balance == 2 * FEE, "no fee retained");
        require(consumer.pending(requestId) == node, "pending node");

        bytes32 nextDigest = keccak256("updated manifest");
        resolver.setContenthash(node, abi.encodePacked(bytes4(0x70617831), nextDigest));
        bytes memory response = bytes("canonical storage receipt");
        bytes32 contentDigest = sha256(response);
        vm.roll(500);
        vm.expectEmit(true, false, false, true, address(consumer));
        emit ReceiptRecorded(node, DIGEST, 4096, 500, contentDigest);
        xweb.deliver(requestId, contentDigest, 4096, response);
        _assertAvailability(DIGEST, 4096, 500, contentDigest);
        require(consumer.pending(requestId) == bytes32(0), "pending removed");
        vm.expectRevert(abi.encodeWithSelector(PaxNameAvailability.UnknownRequest.selector, requestId));
        xweb.deliver(requestId, contentDigest, 4096, response);
        vm.expectRevert(abi.encodeWithSelector(PaxNameAvailability.UnknownRequest.selector, uint64(99)));
        xweb.deliver(99, contentDigest, 4096, response);
        _assertAvailability(DIGEST, 4096, 500, contentDigest);

        requestId = consumer.requestReceipt{value: FEE}(node);
        require(requestId == 2 && consumer.pending(requestId) == node, "second request");
        require(
            keccak256(xweb.lastPayload()) == keccak256(abi.encodePacked(nextDigest, bytes(bucket))),
            "updated digest payload"
        );
        uint64 fullLength = uint64(type(uint32).max) + 1;
        vm.roll(501);
        xweb.deliver64(requestId, contentDigest, fullLength, response);
        _assertAvailability(nextDigest, fullLength, 501, contentDigest);
        require(consumer.pending(requestId) == bytes32(0), "second pending removed");
    }

    function testCallbackIsRefusedFromAnyOtherCaller() public {
        uint64 requestId = consumer.requestReceipt{value: FEE}(node);
        bytes32 contentDigest = keccak256("receipt");
        vm.prank(STRANGER);
        vm.expectRevert(PaxNameAvailability.XWebOnly.selector);
        consumer.onXWebResponse(requestId, contentDigest, uint64(4096), "receipt");
        vm.prank(STRANGER);
        vm.expectRevert(PaxNameAvailability.XWebOnly.selector);
        IXWebConsumer(address(consumer)).onXWebResponse(requestId, contentDigest, 4096, "receipt");
        _assertAvailability(bytes32(0), 0, 0, bytes32(0));
        require(consumer.pending(requestId) == node, "forged callbacks preserve pending");
        vm.roll(600);
        xweb.deliver(requestId, contentDigest, 4096, "receipt");
        _assertAvailability(DIGEST, 4096, 600, contentDigest);
    }

    function _assertAvailability(bytes32 digest, uint64 length, uint256 atBlock, bytes32 contentDigest) private view {
        (bytes32 actualDigest, uint64 actualLength, uint256 actualBlock) = consumer.availability(node);
        require(actualDigest == digest, "availability manifest digest");
        require(actualLength == length, "availability length");
        require(actualBlock == atBlock, "availability block");
        bytes32 actualContentDigest;
        (actualDigest, actualLength, actualBlock, actualContentDigest) = consumer.receipts(node);
        require(actualDigest == digest && actualLength == length && actualBlock == atBlock, "recorded receipt");
        require(actualContentDigest == contentDigest, "receipt content digest");
    }
}
