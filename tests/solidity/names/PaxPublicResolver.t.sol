// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.24;

import {PaxRegistry} from "../../../contracts/names/PaxRegistry.sol";
import {PaxPublicResolver} from "../../../contracts/names/PaxPublicResolver.sol";
import {PaxReverseRegistrar} from "../../../contracts/names/PaxReverseRegistrar.sol";
import {Governed} from "../../../contracts/security/Governed.sol";

interface PaxResolverVm {
    function prank(address account) external;
    function expectRevert(bytes4 selector) external;
    function expectEmit(bool topic1, bool topic2, bool topic3, bool data, address emitter) external;
    function load(address account, bytes32 slot) external view returns (bytes32);
    function toString(bytes calldata value) external pure returns (string memory);
    function serializeString(string calldata objectKey, string calldata valueKey, string calldata value)
        external
        returns (string memory);
    function serializeUint(string calldata objectKey, string calldata valueKey, uint256 value)
        external
        returns (string memory);
    function serializeJson(string calldata objectKey, string calldata value) external returns (string memory);
    function envOr(string calldata key, uint256 defaultValue) external view returns (uint256);
    function writeJson(string calldata json, string calldata path) external;
    function readFile(string calldata path) external view returns (string memory);
}

contract PaxPublicResolverTest {
    event AddrChanged(bytes32 indexed node, address a);
    event NameChanged(bytes32 indexed node, string name);
    event TextChanged(bytes32 indexed node, string indexed indexedKey, string key, string value);
    event ContenthashChanged(bytes32 indexed node, bytes hash);

    PaxResolverVm private constant vm = PaxResolverVm(address(uint160(uint256(keccak256("hevm cheat code")))));
    address private constant OWNER = address(0xA11CE);
    address private constant OPERATOR = address(0xB0B);
    address private constant STRANGER = address(0xBAD);
    address private constant GOVERNANCE = address(0x710E);
    address private constant COUNCIL = address(0xC011);
    address private constant CONTROLLER = address(0xC017);
    bytes32 private constant DIGEST = keccak256("pax manifest");
    string private constant VECTOR_PATH = "tests/vectors/names/resolver_layout_v1.json";
    PaxRegistry private registry;
    PaxPublicResolver private resolver;
    bytes32 private recordNode;

    function setUp() public {
        registry = new PaxRegistry();
        resolver = new PaxPublicResolver(address(registry), GOVERNANCE, COUNCIL);
        recordNode = registry.setSubnodeOwner(bytes32(0), keccak256("alice"), OWNER);
    }

    function testResolverLayoutVectorIsCurrent() public {
        string memory registryJson =
            vm.serializeString("registry", "creation_code", vm.toString(type(PaxRegistry).creationCode));
        vm.serializeString("resolver", "creation_code", vm.toString(type(PaxPublicResolver).creationCode));
        string memory resolverJson = vm.serializeString(
            "resolver", "constructor", "(address registry,address governanceTimelock,address emergencyAuthority)"
        );
        vm.serializeJson(
            "layout",
            string.concat('{"contracts":{"PaxPublicResolver":', resolverJson, ',"PaxRegistry":', registryJson, "}}")
        );
        vm.serializeUint("layout", "version", 1);
        vm.serializeUint("layout", "manifest_digest_slot", resolver.MANIFEST_DIGEST_SLOT());
        vm.serializeString("layout", "slot_formula", "keccak256(abi.encode(node, manifest_digest_slot))");
        string memory json = _prettyJson(vm.serializeString("layout", "contenthash_tag", "0x70617831"));
        if (vm.envOr("NAMES_WRITE_VECTOR", uint256(0)) == 1) vm.writeJson(json, VECTOR_PATH);
        require(keccak256(bytes(vm.readFile(VECTOR_PATH))) == keccak256(bytes(json)), "resolver layout vector changed");
    }

    function testAddrNameTextAndContenthashRecordsAreOwnerOnly() public {
        _expectWritesDenied(STRANGER);
        _expectWritesDenied(GOVERNANCE);
        _expectWritesDenied(COUNCIL);
        require(resolver.addr(recordNode) == address(0), "initial addr");
        require(bytes(resolver.name(recordNode)).length == 0, "initial name");
        require(bytes(resolver.text(recordNode, "url")).length == 0, "initial text");
        require(resolver.contenthash(recordNode).length == 0, "initial contenthash");

        vm.expectEmit(true, false, false, true, address(resolver));
        emit AddrChanged(recordNode, OWNER);
        vm.prank(OWNER);
        resolver.setAddr(recordNode, OWNER);
        vm.expectEmit(true, false, false, true, address(resolver));
        emit NameChanged(recordNode, "alice.pax");
        vm.prank(OWNER);
        resolver.setName(recordNode, "alice.pax");
        vm.expectEmit(true, true, false, true, address(resolver));
        emit TextChanged(recordNode, "url", "url", "https://alice.pax");
        vm.prank(OWNER);
        resolver.setText(recordNode, "url", "https://alice.pax");
        vm.expectEmit(true, false, false, true, address(resolver));
        emit ContenthashChanged(recordNode, _hash(DIGEST));
        vm.prank(OWNER);
        resolver.setContenthash(recordNode, _hash(DIGEST));
        _assertRecords(OWNER, "alice.pax", "https://alice.pax", DIGEST);

        vm.prank(OWNER);
        registry.setApprovalForAll(OPERATOR, true);
        bytes[] memory calls = _records(OPERATOR, "operator.pax", "operator", keccak256("operator manifest"));
        vm.prank(OPERATOR);
        bytes[] memory results = resolver.multicall(calls);
        require(results.length == 4, "multicall result count");
        for (uint256 i; i < results.length; ++i) {
            require(results[i].length == 0, "setter return data");
        }
        _assertRecords(OPERATOR, "operator.pax", "operator", keccak256("operator manifest"));
        vm.prank(OWNER);
        registry.setApprovalForAll(OPERATOR, false);
        _expectWritesDenied(OPERATOR);
        vm.prank(STRANGER);
        vm.expectRevert(PaxPublicResolver.Unauthorised.selector);
        resolver.multicall(calls);

        vm.prank(STRANGER);
        vm.expectRevert(Governed.GovernanceOnly.selector);
        resolver.setRegistrarController(CONTROLLER);
        vm.prank(GOVERNANCE);
        vm.expectRevert(PaxPublicResolver.InvalidConfiguration.selector);
        resolver.setRegistrarController(address(0));
        vm.prank(GOVERNANCE);
        resolver.setRegistrarController(CONTROLLER);
        require(resolver.registrarController() == CONTROLLER, "controller configured");
        vm.prank(GOVERNANCE);
        vm.expectRevert(PaxPublicResolver.InvalidConfiguration.selector);
        resolver.setRegistrarController(STRANGER);
        vm.prank(CONTROLLER);
        resolver.multicall(_records(OWNER, "controller.pax", "controller", DIGEST));
        _assertRecords(OWNER, "controller.pax", "controller", DIGEST);

        // A later invalid call must roll back earlier writes and bubble its error.
        calls = new bytes[](2);
        calls[0] = abi.encodeCall(resolver.setAddr, (recordNode, STRANGER));
        calls[1] = abi.encodeCall(resolver.setContenthash, (recordNode, hex"70617831"));
        vm.prank(OWNER);
        vm.expectRevert(PaxPublicResolver.PaxContenthashForm.selector);
        resolver.multicall(calls);
        require(resolver.addr(recordNode) == OWNER, "multicall atomicity");

        vm.prank(OWNER);
        registry.setOwner(recordNode, STRANGER);
        _expectWritesDenied(OWNER);
        vm.prank(STRANGER);
        resolver.setName(recordNode, "new-owner.pax");
        require(_equal(resolver.name(recordNode), "new-owner.pax"), "new owner write");
        require(resolver.supportsInterface(0x01ffc9a7), "ERC165");
        require(resolver.supportsInterface(0x3b3b57de), "addr interface");
        require(resolver.supportsInterface(0x691f3431), "name interface");
        require(resolver.supportsInterface(0x59d1d43c), "text interface");
        require(resolver.supportsInterface(0xbc1c58d1), "contenthash interface");
        require(!resolver.supportsInterface(0xffffffff), "invalid interface");
    }

    function testContenthashAcceptsOnlyThePaxeerTag() public {
        bytes memory hash = _hash(DIGEST);
        bytes memory shortHash = new bytes(35);
        for (uint256 i; i < shortHash.length; ++i) {
            shortHash[i] = hash[i];
        }
        vm.prank(OWNER);
        vm.expectRevert(PaxPublicResolver.PaxContenthashForm.selector);
        resolver.setContenthash(recordNode, shortHash);
        vm.prank(OWNER);
        vm.expectRevert(PaxPublicResolver.PaxContenthashForm.selector);
        resolver.setContenthash(recordNode, abi.encodePacked(bytes4(0x70617832), DIGEST));
        vm.prank(OWNER);
        vm.expectRevert(PaxPublicResolver.PaxContenthashForm.selector);
        resolver.setContenthash(recordNode, abi.encodePacked(hash, bytes1(0)));
        vm.prank(OWNER);
        vm.expectRevert(PaxPublicResolver.PaxContenthashForm.selector);
        resolver.setContenthash(recordNode, hex"70");
        vm.prank(OWNER);
        resolver.setContenthash(recordNode, hash);
        require(keccak256(resolver.contenthash(recordNode)) == keccak256(hash), "Paxeer form round trip");
        require(resolver.contenthash(recordNode).length == 36, "Paxeer form length");
        bytes32 slot = keccak256(abi.encode(recordNode, resolver.MANIFEST_DIGEST_SLOT()));
        require(vm.load(address(resolver), slot) == DIGEST, "manifest storage slot");
        vm.prank(OWNER);
        resolver.setContenthash(recordNode, "");
        require(resolver.contenthash(recordNode).length == 0, "clear contenthash");
        require(vm.load(address(resolver), slot) == bytes32(0), "clear digest word");
        vm.prank(OWNER);
        resolver.setContenthash(recordNode, _hash(bytes32(0)));
        require(resolver.contenthash(recordNode).length == 0, "zero digest is empty");
    }

    function testFreezeClearsAndBlocksTheContenthash() public {
        vm.prank(OWNER);
        resolver.setContenthash(recordNode, _hash(DIGEST));
        vm.prank(OWNER);
        resolver.setText(recordNode, "url", "before freeze");
        vm.prank(OWNER);
        vm.expectRevert(Governed.GovernanceOnly.selector);
        resolver.freeze(recordNode);
        vm.expectEmit(true, false, false, true, address(resolver));
        emit ContenthashChanged(recordNode, "");
        vm.prank(COUNCIL);
        resolver.freeze(recordNode);
        require(resolver.frozen(recordNode), "emergency freeze");
        require(resolver.contenthash(recordNode).length == 0, "freeze clears contenthash");
        bytes32 slot = keccak256(abi.encode(recordNode, resolver.MANIFEST_DIGEST_SLOT()));
        require(vm.load(address(resolver), slot) == bytes32(0), "freeze clears digest word");
        require(_equal(resolver.text(recordNode, "url"), "before freeze"), "freeze preserves existing text");
        vm.prank(OWNER);
        registry.setApprovalForAll(OPERATOR, true);
        vm.prank(GOVERNANCE);
        resolver.setRegistrarController(CONTROLLER);
        _expectFrozenWrites(OWNER);
        _expectFrozenWrites(OPERATOR);
        _expectFrozenWrites(CONTROLLER);
        vm.prank(COUNCIL);
        vm.expectRevert(Governed.GovernanceOnly.selector);
        resolver.unfreeze(recordNode);
        vm.prank(OWNER);
        vm.expectRevert(Governed.GovernanceOnly.selector);
        resolver.unfreeze(recordNode);
        vm.prank(GOVERNANCE);
        resolver.unfreeze(recordNode);
        require(!resolver.frozen(recordNode), "governance unfreeze");
        require(resolver.contenthash(recordNode).length == 0, "unfreeze does not restore digest");
        vm.prank(OWNER);
        resolver.setContenthash(recordNode, _hash(DIGEST));
        vm.prank(OWNER);
        resolver.setText(recordNode, "url", "after unfreeze");
        require(keccak256(resolver.contenthash(recordNode)) == keccak256(_hash(DIGEST)), "unfrozen contenthash");
        require(_equal(resolver.text(recordNode, "url"), "after unfreeze"), "unfrozen text");
        vm.prank(GOVERNANCE);
        resolver.freeze(recordNode);
        require(resolver.frozen(recordNode) && resolver.contenthash(recordNode).length == 0, "governance freeze");
    }

    function testReverseSetNameBindsTheCallerAddress() public {
        PaxReverseRegistrar reverse = new PaxReverseRegistrar(address(registry), address(resolver), address(0));
        bytes32 reverseParent = registry.setSubnodeOwner(bytes32(0), keccak256("reverse"), address(this));
        bytes32 addrReverse = registry.setSubnodeOwner(reverseParent, keccak256("addr"), address(reverse));
        address account = address(bytes20(hex"1234567890abcdef1234567890abcdef12345678"));
        bytes32 expected =
            keccak256(abi.encodePacked(addrReverse, keccak256("1234567890abcdef1234567890abcdef12345678")));
        require(reverse.node(account) == expected, "lowercase hex ENS reverse node");
        bytes32 zeroNode =
            keccak256(abi.encodePacked(addrReverse, keccak256("0000000000000000000000000000000000000000")));
        require(reverse.node(address(0)) == zeroNode, "leading zero hex label");
        vm.prank(account);
        bytes32 claimed = reverse.setName("alice.pax");
        require(claimed == expected, "returned reverse node");
        require(registry.owner(claimed) == account, "caller owns reverse node");
        require(registry.resolver(claimed) == address(resolver) && registry.ttl(claimed) == 0, "reverse record");
        require(_equal(resolver.name(claimed), "alice.pax"), "reverse resolver name");
        vm.prank(STRANGER);
        reverse.setName("mallory.pax");
        require(_equal(resolver.name(claimed), "alice.pax"), "other caller cannot overwrite reverse name");
        require(registry.owner(reverse.node(STRANGER)) == STRANGER, "other caller owns only own node");
        vm.prank(STRANGER);
        vm.expectRevert(PaxPublicResolver.Unauthorised.selector);
        resolver.setName(claimed, "stolen.pax");
        vm.prank(account);
        reverse.setName("updated.pax");
        require(_equal(resolver.name(claimed), "updated.pax"), "repeat reverse update");
        require(registry.owner(claimed) == account, "owner after repeated claim");
        vm.prank(account);
        resolver.setName(claimed, "direct.pax");
        require(_equal(resolver.name(claimed), "direct.pax"), "reverse owner can write directly");
    }

    function _expectWritesDenied(address caller) private {
        vm.prank(caller);
        vm.expectRevert(PaxPublicResolver.Unauthorised.selector);
        resolver.setAddr(recordNode, STRANGER);
        vm.prank(caller);
        vm.expectRevert(PaxPublicResolver.Unauthorised.selector);
        resolver.setName(recordNode, "stolen.pax");
        vm.prank(caller);
        vm.expectRevert(PaxPublicResolver.Unauthorised.selector);
        resolver.setText(recordNode, "url", "stolen");
        vm.prank(caller);
        vm.expectRevert(PaxPublicResolver.Unauthorised.selector);
        resolver.setContenthash(recordNode, _hash(DIGEST));
    }

    function _expectFrozenWrites(address caller) private {
        vm.prank(caller);
        vm.expectRevert(PaxPublicResolver.NodeFrozen.selector);
        resolver.setContenthash(recordNode, _hash(DIGEST));
        vm.prank(caller);
        vm.expectRevert(PaxPublicResolver.NodeFrozen.selector);
        resolver.setContenthash(recordNode, "");
        vm.prank(caller);
        vm.expectRevert(PaxPublicResolver.NodeFrozen.selector);
        resolver.setText(recordNode, "url", "blocked");
    }

    function _records(address account, string memory newName, string memory url, bytes32 digest)
        private
        view
        returns (bytes[] memory calls)
    {
        calls = new bytes[](4);
        calls[0] = abi.encodeCall(resolver.setAddr, (recordNode, account));
        calls[1] = abi.encodeCall(resolver.setName, (recordNode, newName));
        calls[2] = abi.encodeCall(resolver.setText, (recordNode, "url", url));
        calls[3] = abi.encodeCall(resolver.setContenthash, (recordNode, _hash(digest)));
    }

    function _assertRecords(address account, string memory newName, string memory url, bytes32 digest) private view {
        require(resolver.addr(recordNode) == account, "addr record");
        require(_equal(resolver.name(recordNode), newName), "name record");
        require(_equal(resolver.text(recordNode, "url"), url), "text record");
        require(keccak256(resolver.contenthash(recordNode)) == keccak256(_hash(digest)), "contenthash record");
    }

    function _hash(bytes32 digest) private pure returns (bytes memory) {
        return abi.encodePacked(bytes4(0x70617831), digest);
    }

    function _equal(string memory a, string memory b) private pure returns (bool) {
        return keccak256(bytes(a)) == keccak256(bytes(b));
    }

    // Match writeJson's two-space pretty printing, retaining literal contents.
    function _prettyJson(string memory compact) private pure returns (string memory) {
        bytes memory input = bytes(compact);
        bytes memory output = new bytes(input.length * 3);
        uint256 length;
        uint256 depth;
        bool quoted;
        bool escaped;
        for (uint256 i; i < input.length; ++i) {
            bytes1 c = input[i];
            if (quoted) {
                output[length++] = c;
                if (escaped) escaped = false;
                else if (c == 0x5c) escaped = true;
                else if (c == 0x22) quoted = false;
            } else if (c == 0x22) {
                quoted = true;
                output[length++] = c;
            } else if (c == "{" || c == "," || c == "}") {
                if (c == "}") --depth;
                else output[length++] = c;
                if (c == "{") ++depth;
                output[length++] = 0x0a;
                for (uint256 j; j < depth * 2; ++j) {
                    output[length++] = " ";
                }
                if (c == "}") output[length++] = c;
            } else {
                output[length++] = c;
                if (c == ":") output[length++] = " ";
            }
        }
        assembly ("memory-safe") { mstore(output, length) }
        return string(output);
    }
}
