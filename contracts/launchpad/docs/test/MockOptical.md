# Solidity API

Source: [MockOptical.sol](../../contracts/test/MockOptical.sol).

## MockOptical

Inherits the default hooks from `BaseOptical` and returns zero from `getFlags()`.

### constructor

```solidity
constructor(address _poolRegistry, address _owner)
```

## MockOpticalWithFlags

Test mock with configurable flags and hook behavior

### beforeSwapRejectNext

```solidity
bool public beforeSwapRejectNext
```

### beforeSwapDeltaNext

```solidity
int256 public beforeSwapDeltaNext
```

### beforeSwapCalled

```solidity
bool public beforeSwapCalled
```

### afterSwapCalled

```solidity
bool public afterSwapCalled
```

### beforeFeeDistCalled

```solidity
bool public beforeFeeDistCalled
```

### afterFeeDistCalled

```solidity
bool public afterFeeDistCalled
```

### lastPool

```solidity
address public lastPool
```

### lastSender

```solidity
address public lastSender
```

### lastIsBuy

```solidity
bool public lastIsBuy
```

### lastAmountIn

```solidity
uint256 public lastAmountIn
```

### lastAmountOut

```solidity
uint256 public lastAmountOut
```

### lastFeeAmount

```solidity
uint256 public lastFeeAmount
```

### constructor

```solidity
constructor(address _poolRegistry, address _owner, uint8 flags_)
```

### setFlags

```solidity
function setFlags(uint8 flags_) external
```

### setBeforeSwapReject

```solidity
function setBeforeSwapReject(bool reject) external
```

### setBeforeSwapDelta

```solidity
function setBeforeSwapDelta(int256 delta) external
```

### resetCallTrackers

```solidity
function resetCallTrackers() external
```

### getFlags

```solidity
function getFlags() external view returns (uint8)
```

Returns a bitmap of active hooks for this optical.

`beforeSwap` uses bit 0, `afterSwap` bit 1, `beforeFeeDistribution` bit 2 and
`afterFeeDistribution` bit 3. The pool checks the swap flags; the fee accumulator checks the
fee-distribution flags.

#### Return Values

| Name | Type  | Description |
| ---- | ----- | ----------- |
| [0]  | uint8 |             |

### beforeSwap

```solidity
function beforeSwap(address pool, address sender, bool isBuy, uint256 amountIn) external returns (bool proceed, int256 amountDelta)
```

### afterSwap

```solidity
function afterSwap(address pool, address sender, bool isBuy, uint256 amountIn, uint256 amountOut) external returns (bytes4)
```

### beforeFeeDistribution

```solidity
function beforeFeeDistribution(address pool, uint256 feeAmount) external returns (uint256 adjustedFee)
```

### afterFeeDistribution

```solidity
function afterFeeDistribution(address pool, uint256 feeAmount) external returns (bytes4)
```

## MockRevertingOptical

An optical whose `beforeSwap` and `afterSwap` hooks revert. Fee-distribution hooks inherit the
behavior of `BaseOptical`.

### constructor

```solidity
constructor(address _poolRegistry, address _owner, uint8 flags_)
```

### getFlags

```solidity
function getFlags() external view returns (uint8)
```

Returns a bitmap of active hooks for this optical.

`beforeSwap` uses bit 0, `afterSwap` bit 1, `beforeFeeDistribution` bit 2 and
`afterFeeDistribution` bit 3. The pool checks the swap flags; the fee accumulator checks the
fee-distribution flags.

#### Return Values

| Name | Type  | Description |
| ---- | ----- | ----------- |
| [0]  | uint8 |             |

### beforeSwap

```solidity
function beforeSwap(address, address, bool, uint256) external pure returns (bool, int256)
```

Called before a swap executes in the pool.

#### Parameters

| Name | Type    | Description |
| ---- | ------- | ----------- |
|      | address |             |
|      | address |             |
|      | bool    |             |
|      | uint256 |             |

#### Return Values

| Name | Type   | Description |
| ---- | ------ | ----------- |
| [0]  | bool   |             |
| [1]  | int256 |             |

### afterSwap

```solidity
function afterSwap(address, address, bool, uint256, uint256) external pure returns (bytes4)
```

Called after a swap completes in the pool.

#### Parameters

| Name | Type    | Description |
| ---- | ------- | ----------- |
|      | address |             |
|      | address |             |
|      | bool    |             |
|      | uint256 |             |
|      | uint256 |             |

#### Return Values

| Name | Type   | Description                                                  |
| ---- | ------ | ------------------------------------------------------------ |
| [0]  | bytes4 | selector The function selector to confirm execution (bytes4) |
