// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.24;

import {Governed} from "../security/Governed.sol";
import {ReentrancyLock} from "../security/ReentrancyLock.sol";
import {
    Registration,
    IPaxRegistry,
    IPaxBaseRegistrar,
    IPaxRegistrarController,
    IPaxPublicResolver,
    IPaxReverseRegistrar
} from "./IPaxNames.sol";

contract PaxRegistrarController is Governed, ReentrancyLock, IPaxRegistrarController {
    error InvalidConfiguration();
    error InvalidLabel();
    error NameUnavailable();
    error InvalidRegistration();
    error InvalidRecord();
    error CommitmentExists();
    error CommitmentTooNew();
    error CommitmentExpired();
    error CommitmentMissing();
    error InsufficientPayment();
    error PaymentFailed();

    event PriceSet(uint8 indexed tier, uint256 pricePerYear);
    event ReservedSet(bytes32 indexed labelhash, bool reserved);
    event CommitmentAgesSet(uint256 minimumAge, uint256 maximumAge);
    event TreasurySet(address indexed treasury);
    event ResolversSet(address indexed resolver, address indexed reverseRegistrar);

    IPaxBaseRegistrar public immutable registrar;
    IPaxRegistry public immutable registry;
    address public resolver;
    address public reverseRegistrar;
    address public treasury;
    uint256 public minCommitmentAge = 60 seconds;
    uint256 public maxCommitmentAge = 24 hours;
    mapping(uint8 => uint256) public yearlyPrices;
    mapping(bytes32 => bool) public reserved;
    mapping(bytes32 => uint256) public commitments;
    mapping(bytes32 => bool) private committed;
    bool private resolversSet;

    constructor(
        address registrarAddress,
        address registryAddress,
        address resolverAddress,
        address reverseRegistrarAddress,
        address treasuryAddress,
        address governanceTimelock,
        address emergencyAuthority
    ) Governed(governanceTimelock, emergencyAuthority) {
        if (registrarAddress.code.length == 0 || registryAddress.code.length == 0 || treasuryAddress == address(0)) {
            revert InvalidConfiguration();
        }
        if (
            (resolverAddress != address(0) && resolverAddress.code.length == 0)
                || (reverseRegistrarAddress != address(0) && reverseRegistrarAddress.code.length == 0)
        ) revert InvalidConfiguration();
        registrar = IPaxBaseRegistrar(registrarAddress);
        registry = IPaxRegistry(registryAddress);
        resolver = resolverAddress;
        reverseRegistrar = reverseRegistrarAddress;
        treasury = treasuryAddress;
        resolversSet = resolverAddress != address(0) && reverseRegistrarAddress != address(0);
    }

    function setPrice(uint8 tier, uint256 pricePerYear) external onlyGovernance {
        if (tier < 3 || tier > 5) revert InvalidConfiguration();
        yearlyPrices[tier] = pricePerYear;
        emit PriceSet(tier, pricePerYear);
    }

    function setReserved(bytes32 labelhash, bool isReserved) external onlyGovernance {
        reserved[labelhash] = isReserved;
        emit ReservedSet(labelhash, isReserved);
    }

    function setTreasury(address newTreasury) external onlyGovernance {
        if (newTreasury == address(0)) revert InvalidConfiguration();
        treasury = newTreasury;
        emit TreasurySet(newTreasury);
    }

    function setCommitmentAges(uint256 minimumAge, uint256 maximumAge) external onlyGovernance {
        if (minimumAge == 0 || minimumAge >= maximumAge) revert InvalidConfiguration();
        minCommitmentAge = minimumAge;
        maxCommitmentAge = maximumAge;
        emit CommitmentAgesSet(minimumAge, maximumAge);
    }

    function setResolvers(address resolverAddress, address reverseRegistrarAddress) external onlyGovernance {
        if (
            resolversSet || resolverAddress.code.length == 0 || reverseRegistrarAddress.code.length == 0
                || (resolver != address(0) && resolver != resolverAddress)
                || (reverseRegistrar != address(0) && reverseRegistrar != reverseRegistrarAddress)
        ) revert InvalidConfiguration();
        resolver = resolverAddress;
        reverseRegistrar = reverseRegistrarAddress;
        resolversSet = true;
        emit ResolversSet(resolverAddress, reverseRegistrarAddress);
    }

    function rentPrice(string calldata label, uint256 duration) public view returns (uint256 base, uint256 premium) {
        if (!_valid(label)) revert InvalidLabel();
        uint256 length = bytes(label).length;
        uint8 tier = length == 3 ? 3 : length == 4 ? 4 : 5;
        base = yearlyPrices[tier] * duration / 365 days;
        premium = 0;
    }

    function available(string calldata label) public view returns (bool) {
        bytes32 labelhash = keccak256(bytes(label));
        return _valid(label) && !reserved[labelhash] && registrar.available(uint256(labelhash));
    }

    function makeCommitment(Registration calldata registration) public pure returns (bytes32) {
        return keccak256(abi.encode(registration));
    }

    function commit(bytes32 commitment) external {
        if (committed[commitment] && block.timestamp <= commitments[commitment] + maxCommitmentAge) {
            revert CommitmentExists();
        }
        commitments[commitment] = block.timestamp;
        committed[commitment] = true;
    }

    function register(Registration calldata registration) external payable nonReentrant {
        if (!_valid(registration.label)) revert InvalidLabel();
        if (!available(registration.label)) revert NameUnavailable();
        if (
            registration.owner == address(0) || registration.owner == address(this) || registration.duration == 0
                || registration.reverseRecord > 1
        ) revert InvalidRegistration();
        bytes32 commitment = makeCommitment(registration);
        if (!committed[commitment]) revert CommitmentMissing();
        uint256 age = block.timestamp - commitments[commitment];
        if (age < minCommitmentAge) revert CommitmentTooNew();
        if (age > maxCommitmentAge) revert CommitmentExpired();
        (uint256 cost,) = rentPrice(registration.label, registration.duration);
        if (msg.value < cost) revert InsufficientPayment();
        address recordResolver = registration.resolver;
        if (
            (recordResolver == address(0) && registration.data.length != 0)
                || (recordResolver != address(0) && recordResolver.code.length == 0)
                || (registration.reverseRecord == 1 && reverseRegistrar == address(0))
        ) revert InvalidConfiguration();
        delete commitments[commitment];
        delete committed[commitment];
        bytes32 labelhash = keccak256(bytes(registration.label));
        uint256 id = uint256(labelhash);
        uint256 expires = registrar.register(id, address(this), registration.duration);
        bytes32 node = keccak256(abi.encodePacked(registrar.PAX_NODE(), labelhash));
        registry.setResolver(node, recordResolver);
        registry.setTTL(node, 0);
        if (registration.data.length != 0) {
            for (uint256 i; i < registration.data.length; ++i) {
                bytes calldata record = registration.data[i];
                if (record.length < 36 || bytes32(record[4:36]) != node) revert InvalidRecord();
            }
            IPaxPublicResolver(recordResolver).multicall(registration.data);
        }
        registrar.transferFrom(address(this), registration.owner, id);
        if (registration.reverseRecord == 1) {
            address reverseResolver = recordResolver == address(0) ? resolver : recordResolver;
            if (reverseResolver == address(0)) revert InvalidConfiguration();
            IPaxReverseRegistrar(reverseRegistrar)
                .setNameForAddr(
                    msg.sender, registration.owner, reverseResolver, string.concat(registration.label, ".pax")
                );
        }
        _settle(cost);
        emit NameRegistered(registration.label, labelhash, registration.owner, cost, expires, registration.referrer);
    }

    function renew(string calldata label, uint256 duration) external payable nonReentrant {
        (uint256 cost,) = rentPrice(label, duration);
        if (msg.value < cost) revert InsufficientPayment();
        bytes32 labelhash = keccak256(bytes(label));
        uint256 expires = registrar.renew(uint256(labelhash), duration);
        _settle(cost);
        emit NameRenewed(label, labelhash, cost, expires);
    }

    function _settle(uint256 cost) private {
        if (cost != 0) {
            (bool paid,) = payable(treasury).call{value: cost}("");
            if (!paid) revert PaymentFailed();
        }
        uint256 refund = msg.value - cost;
        if (refund != 0) {
            (bool refunded,) = payable(msg.sender).call{value: refund}("");
            if (!refunded) revert PaymentFailed();
        }
    }

    function _valid(string calldata label) private pure returns (bool) {
        bytes calldata value = bytes(label);
        if (value.length < 3 || value.length > 63 || value[0] == 0x2d || value[value.length - 1] == 0x2d) return false;
        for (uint256 i; i < value.length; ++i) {
            bytes1 c = value[i];
            if (!((c >= 0x61 && c <= 0x7a) || (c >= 0x30 && c <= 0x39) || c == 0x2d)) return false;
        }
        return true;
    }
}
