// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.24;

import {Governed} from "../security/Governed.sol";
import {IPaxRegistry, IPaxBaseRegistrar} from "./IPaxNames.sol";

interface IPaxERC721Receiver {
    function onERC721Received(address operator, address from, uint256 id, bytes calldata data) external returns (bytes4);
}

contract PaxBaseRegistrar is Governed, IPaxBaseRegistrar {
    error ControllerOnly();
    error InvalidRegistration();
    error Unavailable();
    error NonexistentToken();
    error Unauthorised();
    error InvalidRecipient();

    event Transfer(address indexed from, address indexed to, uint256 indexed tokenId);
    event Approval(address indexed owner, address indexed approved, uint256 indexed tokenId);
    event ApprovalForAll(address indexed owner, address indexed operator, bool approved);
    event ControllerAdded(address indexed controller);
    event ControllerRemoved(address indexed controller);
    event NameRegistered(uint256 indexed id, address indexed owner, uint256 expires);
    event NameRenewed(uint256 indexed id, uint256 expires);

    string public constant name = "Pax Names";
    string public constant symbol = "PAX";
    bytes32 public constant PAX_NODE = keccak256(abi.encodePacked(bytes32(0), keccak256("pax")));
    IPaxRegistry public immutable registry;
    uint256 public immutable GRACE_PERIOD;
    mapping(uint256 => uint256) public expiries;
    mapping(address => bool) public controllers;
    mapping(uint256 => address) private owners;
    mapping(address => uint256) private balances;
    mapping(uint256 => address) private approvals;
    mapping(address => mapping(address => bool)) private operators;

    constructor(address registryAddress, address governanceTimelock, address emergencyAuthority, uint256 gracePeriod)
        Governed(governanceTimelock, emergencyAuthority)
    {
        if (registryAddress.code.length == 0) revert InvalidRegistration();
        registry = IPaxRegistry(registryAddress);
        GRACE_PERIOD = gracePeriod;
    }

    modifier onlyController() {
        if (!controllers[msg.sender]) revert ControllerOnly();
        _;
    }

    function supportsInterface(bytes4 interfaceId) external pure returns (bool) {
        return interfaceId == 0x01ffc9a7 || interfaceId == 0x80ac58cd;
    }

    function baseNode() external pure returns (bytes32) {
        return PAX_NODE;
    }

    function addController(address controller) external onlyGovernance {
        if (controller == address(0)) revert InvalidRecipient();
        controllers[controller] = true;
        emit ControllerAdded(controller);
    }

    function removeController(address controller) external onlyGovernance {
        delete controllers[controller];
        emit ControllerRemoved(controller);
    }

    function nameExpires(uint256 id) external view returns (uint256) {
        return expiries[id];
    }

    function available(uint256 id) public view returns (bool) {
        return expiries[id] == 0 || block.timestamp > expiries[id] + GRACE_PERIOD;
    }

    function ownerOf(uint256 id) public view returns (address account) {
        account = owners[id];
        if (account == address(0) || block.timestamp >= expiries[id]) revert NonexistentToken();
    }

    function balanceOf(address account) external view returns (uint256) {
        if (account == address(0)) revert InvalidRecipient();
        return balances[account];
    }

    function getApproved(uint256 id) public view returns (address) {
        ownerOf(id);
        return approvals[id];
    }

    function isApprovedForAll(address account, address operator) public view returns (bool) {
        return operators[account][operator];
    }

    function approve(address operator, uint256 id) external {
        address account = ownerOf(id);
        if (operator == account || (msg.sender != account && !operators[account][msg.sender])) revert Unauthorised();
        approvals[id] = operator;
        emit Approval(account, operator, id);
    }

    function setApprovalForAll(address operator, bool approved) external {
        if (operator == msg.sender) revert Unauthorised();
        operators[msg.sender][operator] = approved;
        emit ApprovalForAll(msg.sender, operator, approved);
    }

    function transferFrom(address from, address to, uint256 id) public {
        address account = ownerOf(id);
        if (account != from || !_authorised(account, id)) revert Unauthorised();
        if (to == address(0)) revert InvalidRecipient();
        delete approvals[id];
        balances[from]--;
        balances[to]++;
        owners[id] = to;
        registry.setSubnodeOwner(PAX_NODE, bytes32(id), to);
        emit Transfer(from, to, id);
    }

    function safeTransferFrom(address from, address to, uint256 id) external {
        safeTransferFrom(from, to, id, "");
    }

    function safeTransferFrom(address from, address to, uint256 id, bytes memory data) public {
        transferFrom(from, to, id);
        if (to.code.length != 0) {
            if (
                IPaxERC721Receiver(to).onERC721Received(msg.sender, from, id, data)
                    != IPaxERC721Receiver.onERC721Received.selector
            ) {
                revert InvalidRecipient();
            }
        }
    }

    function register(uint256 id, address newOwner, uint256 duration)
        external
        onlyController
        returns (uint256 expires)
    {
        if (!available(id)) revert Unavailable();
        if (newOwner == address(0) || duration == 0) revert InvalidRegistration();
        expires = block.timestamp + duration;
        if (expires > type(uint256).max - GRACE_PERIOD) revert InvalidRegistration();
        address oldOwner = owners[id];
        if (oldOwner != address(0)) {
            balances[oldOwner]--;
            delete approvals[id];
            emit Transfer(oldOwner, address(0), id);
        }
        owners[id] = newOwner;
        balances[newOwner]++;
        expiries[id] = expires;
        registry.setSubnodeOwner(PAX_NODE, bytes32(id), newOwner);
        emit Transfer(address(0), newOwner, id);
        emit NameRegistered(id, newOwner, expires);
    }

    function renew(uint256 id, uint256 duration) external onlyController returns (uint256 expires) {
        if (expiries[id] == 0 || available(id) || duration == 0) revert InvalidRegistration();
        expires = expiries[id] + duration;
        if (expires > type(uint256).max - GRACE_PERIOD) revert InvalidRegistration();
        expiries[id] = expires;
        emit NameRenewed(id, expires);
    }

    function reclaim(uint256 id, address newOwner) external {
        if (!_authorised(ownerOf(id), id)) revert Unauthorised();
        registry.setSubnodeOwner(PAX_NODE, bytes32(id), newOwner);
    }

    function _authorised(address account, uint256 id) private view returns (bool) {
        return msg.sender == account || approvals[id] == msg.sender || operators[account][msg.sender];
    }
}
