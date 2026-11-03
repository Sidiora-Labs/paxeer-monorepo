// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

address constant LAYERX_CHAINS_PRECOMPILE_ADDRESS = 0x000000000000000000000000000000000000101E;

interface ILayerXChains {
    event ChainCreated(uint32 indexed id, string name, address indexed operator, uint64 chainId);
    event ChainStatusChanged(uint32 indexed id, uint8 status);
    event ChainFunded(uint32 indexed id, address indexed from, uint256 amount);
    event ChainOperatorChanged(uint32 indexed id, address indexed operator);

    function createChain(string calldata name, uint64 gasAllowance, uint64 byteAllowance, uint16 feeShareBps) external payable returns (uint32 id);
    function fundChain(uint32 id) external payable;
    function pauseChain(uint32 id) external;
    function resumeChain(uint32 id) external;
    function retireChain(uint32 id) external;
    function governancePause(uint32 id, bool paused) external;
    function setOperator(uint32 id, address operator) external;
    function getChain(uint32 chain) external view returns (uint32 id, string memory name, address operator, uint8 status, uint64 chainId, uint64 gasAllowance, uint64 byteAllowance, uint16 feeShareBps, uint256 bond, uint256 baseFeePerGas, uint64 createdHeight);
    function chainByName(string calldata name) external view returns (uint32);
    function chainCount() external view returns (uint32);
    function chainIdOf(uint32 id) external pure returns (uint64);
    function params() external view returns (address authority, uint256 minBond, uint256 creationFee, uint64 minGasAllowance, uint64 maxGasAllowance, uint64 totalGasAllowanceLeft, uint32 maxChains);
}
