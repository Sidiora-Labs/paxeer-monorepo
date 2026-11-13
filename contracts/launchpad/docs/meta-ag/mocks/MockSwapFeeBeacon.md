# Solidity API

Source: [MockSwapFeeBeacon.sol](../../../contracts/meta-ag/mocks/MockSwapFeeBeacon.sol).

## MockSwapFeeBeacon

Test contract exposing a configurable `swapFeeBps()` value. `setRevert(true)` makes that getter
revert with `ForcedRevert`.

### ForcedRevert

```solidity
error ForcedRevert()
```

### constructor

```solidity
constructor(uint256 initialFeeBps_) public
```

### setFeeBps

```solidity
function setFeeBps(uint256 newFeeBps) external
```

### setRevert

```solidity
function setRevert(bool flag) external
```

### swapFeeBps

```solidity
function swapFeeBps() external view returns (uint256)
```
