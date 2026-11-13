# Solidity API

Source: [MockPoolForOptical.sol](../../contracts/test/MockPoolForOptical.sol).

## MockPoolForOptical

Minimal mock pool that returns configurable reserves for optical testing

### feeAccumulator

```solidity
address feeAccumulator
```

### opticalAddress

```solidity
address opticalAddress
```

### setFeeAccumulator

```solidity
function setFeeAccumulator(address value) external
```

### setOpticalAddress

```solidity
function setOpticalAddress(address value) external
```

### setReserves

```solidity
function setReserves(uint256 virtualUsdl, uint256 realUsdl, uint256 tokenRes) external
```

### setTokenAddress

```solidity
function setTokenAddress(address token) external
```

### getReserves

```solidity
function getReserves() external view returns (uint256, uint256, uint256)
```

### getEffectiveReserves

```solidity
function getEffectiveReserves() external view returns (uint256, uint256)
```

### tokenAddress

```solidity
function tokenAddress() external view returns (address)
```

### creationTimestamp

```solidity
function creationTimestamp() external view returns (uint256)
```
