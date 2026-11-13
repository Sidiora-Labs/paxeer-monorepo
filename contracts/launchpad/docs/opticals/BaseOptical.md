# Solidity API

Source: [BaseOptical.sol](../../contracts/opticals/BaseOptical.sol).

## BaseOptical

Base for non-upgradeable opticals. Provides no-op implementations of the four `IOptical` hooks and
returns zero active flags by default. Derived opticals override the hooks they use.

Swap callbacks require the attached registered pool as caller. Fee callbacks require that pool's fee
accumulator. Pool validation checks both registration and attachment to this optical. The registry
and owner addresses are immutable; derived contracts may expose mutable configuration.

### NotPool

```solidity
error NotPool()
```

### NotOwner

```solidity
error NotOwner()
```

### poolRegistry

```solidity
address poolRegistry
```

The PoolRegistry address for validating pool callers

### owner

```solidity
address owner
```

The deployer/owner of this optical (for configuration)

### constructor

```solidity
constructor(address _poolRegistry, address _owner)
```

### onlyPool

```solidity
modifier onlyPool()
```

_Modifier to restrict calls to registered, attached pools. Validates that the caller is registered
in PoolRegistry and attached to this optical._

### onlyOwner

```solidity
modifier onlyOwner()
```

_Modifier to restrict calls to the optical owner._

### beforeSwap

```solidity
function beforeSwap(address pool, address, bool, uint256) external virtual returns (bool proceed, int256 amountDelta)
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

| Name        | Type   | Description                                                                                      |
| ----------- | ------ | ------------------------------------------------------------------------------------------------ |
| proceed     | bool   | Whether the swap should continue (false = reject)                                                |
| amountDelta | int256 | Signed input adjustment; SidioraPool accepts zero or negative values and rejects positive values |

### afterSwap

```solidity
function afterSwap(address pool, address, bool, uint256, uint256) external virtual returns (bytes4)
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

| Name | Type   | Description                            |
| ---- | ------ | -------------------------------------- |
| [0]  | bytes4 | Function selector confirming execution |

### beforeFeeDistribution

```solidity
function beforeFeeDistribution(address pool, uint256 feeAmount) external virtual returns (uint256 adjustedFee)
```

Called before fee distribution occurs.

#### Parameters

| Name      | Type    | Description                              |
| --------- | ------- | ---------------------------------------- |
| pool      | address | Registered pool attached to this optical |
| feeAmount | uint256 | The fee amount about to be distributed   |

#### Return Values

| Name        | Type    | Description                                      |
| ----------- | ------- | ------------------------------------------------ |
| adjustedFee | uint256 | The adjusted fee amount after optical processing |

### afterFeeDistribution

```solidity
function afterFeeDistribution(address pool, uint256) external virtual returns (bytes4)
```

Called after fee distribution occurs.

#### Parameters

| Name | Type    | Description |
| ---- | ------- | ----------- |
|      | address |             |
|      | uint256 |             |

#### Return Values

| Name | Type   | Description                            |
| ---- | ------ | -------------------------------------- |
| [0]  | bytes4 | Function selector confirming execution |

### getFlags

```solidity
function getFlags() external view virtual returns (uint8)
```

Returns a bitmap of active hooks for this optical.

_Bit 0: beforeSwap, Bit 1: afterSwap, Bit 2: beforeFeeDistribution, Bit 3: afterFeeDistribution Pool
checks this to skip unused callbacks (gas optimization)._

#### Return Values

| Name | Type  | Description |
| ---- | ----- | ----------- |
| [0]  | uint8 |             |

### \_isRegisteredPool

```solidity
function _isRegisteredPool(address pool) internal view returns (bool)
```

Checks that PoolRegistry metadata has a nonzero creator and names this optical.

### \_getPoolReserves

```solidity
function _getPoolReserves(address pool) internal view returns (uint256 virtualUsdl, uint256 realUsdl, uint256 tokenRes)
```

_Helper to read pool reserves for opticals that need pricing data._

### onlyAttachedPool

```solidity
modifier onlyAttachedPool(address pool)
```

Requires the caller to be the specified registered pool attached to this optical.

### onlyFeeAccumulator

```solidity
modifier onlyFeeAccumulator(address pool)
```

Requires a registered pool attached to this optical and its fee accumulator as caller.

### \_poolMetadata

```solidity
function _poolMetadata(address pool) internal view returns (struct IPoolRegistry.PoolMetadata)
```

Returns metadata after validating pool registration and optical attachment.
