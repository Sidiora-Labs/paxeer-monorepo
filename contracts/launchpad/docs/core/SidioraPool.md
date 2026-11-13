# Solidity API

Source: [SidioraPool.sol](../../contracts/core/SidioraPool.sol).

## IOpticalMinimal

_Minimal interface for optical hooks used by pool_

### beforeSwap

```solidity
function beforeSwap(address pool, address sender, bool isBuy, uint256 amountIn) external returns (bool proceed, int256 amountDelta)
```

### afterSwap

```solidity
function afterSwap(address pool, address sender, bool isBuy, uint256 amountIn, uint256 amountOut) external returns (bytes4)
```

### getFlags

```solidity
function getFlags() external view returns (uint8)
```

## SidioraPool

Core AMM engine. Constant product with virtual reserves and dynamic fees.

Beacon proxy instances hold per-pool reserves. Fees are charged in the input asset: buy fees go to
FeeAccumulator in USDL, while sell fees remain in the pool as tokens. The initialized virtual USDL
reserve provides pricing depth; sell output is bounded by real USDL.

### VirtualFloorBreached

```solidity
error VirtualFloorBreached()
```

### tokenAddress

```solidity
address tokenAddress
```

Get the pool's token address

### usdlAddress

```solidity
address usdlAddress
```

### opticalAddress

```solidity
address opticalAddress
```

Get the pool's optical address

### feeAccumulator

```solidity
address feeAccumulator
```

### eventEmitter

```solidity
address eventEmitter
```

### protocolConfig

```solidity
address protocolConfig
```

### guardian

```solidity
address guardian
```

### virtualUsdlReserve

```solidity
uint256 virtualUsdlReserve
```

### realUsdlBalance

```solidity
uint256 realUsdlBalance
```

### tokenReserve

```solidity
uint256 tokenReserve
```

### creationTimestamp

```solidity
uint256 creationTimestamp
```

Get the pool's creation timestamp

### cumulativeVolume

```solidity
uint256 cumulativeVolume
```

Returns cumulative USDL volume: paid input on buys plus output on sells.

### priceSnapshots

```solidity
uint256[8] priceSnapshots
```

### snapshotIndex

```solidity
uint256 snapshotIndex
```

### snapshotCount

```solidity
uint256 snapshotCount
```

### accumulatedUsdlFees

```solidity
uint256 accumulatedUsdlFees
```

### accumulatedTokenFees

```solidity
uint256 accumulatedTokenFees
```

### constructor

```solidity
constructor()
```

### initialize

```solidity
function initialize( address _tokenAddress, address _usdlAddress, address _opticalAddress, address _feeAccumulator, address _eventEmitter, address _protocolConfig, address _guardian, uint256 _virtualUsdlReserve, uint256 _tokenReserve ) external override initializer
```

Initialize pool state (called once by factory via beacon proxy)

### swap

```solidity
function swap( uint256 amountIn, uint256 minAmountOut, bool isBuy, address recipient, uint256 deadline ) external override nonReentrant whenNotPaused returns (uint256 amountOut)
```

Executes a swap using input already transferred to the pool. The input balance must cover
`amountIn` above the booked reserve. Direct calls identify `msg.sender` as the trader; the
recipient receives the output.

#### Parameters

| Name         | Type    | Description                           |
| ------------ | ------- | ------------------------------------- |
| amountIn     | uint256 | Amount of input token                 |
| minAmountOut | uint256 | Minimum output (slippage protection)  |
| isBuy        | bool    | True = USDL→Token, False = Token→USDL |
| recipient    | address | Address to receive output tokens      |
| deadline     | uint256 | Transaction deadline timestamp        |

#### Return Values

| Name      | Type    | Description          |
| --------- | ------- | -------------------- |
| amountOut | uint256 | Actual output amount |

### syncReserves

```solidity
function syncReserves() external override nonReentrant returns (uint256 usdlBal, uint256 tokenBal)
```

Re-read actual token balances (used by LP_REWARDS strategy)

#### Return Values

| Name     | Type    | Description               |
| -------- | ------- | ------------------------- |
| usdlBal  | uint256 | Actual USDL balance       |
| tokenBal | uint256 | Actual pool-token balance |

### pause

```solidity
function pause() external
```

Only the current guardian can pause swaps.

### unpause

```solidity
function unpause() external
```

Only the current guardian can resume swaps.

### paused

```solidity
function paused() external view returns (bool)
```

### getReserves

```solidity
function getReserves() external view override returns ( uint256 virtualUsdl, uint256 realUsdl, uint256 tokenRes )
```

Get current reserves

### getEffectiveReserves

```solidity
function getEffectiveReserves() external view override returns ( uint256 effectiveUsdl, uint256 tokenRes )
```

Get effective reserves (virtual + real USDL, token)

### getPrice

```solidity
function getPrice() external view override returns (uint256)
```

Returns the current token price in USDL with 18 decimal places:
`(virtualUsdlReserve + realUsdlBalance) * 1e18 / tokenReserve`. Returns zero when the token reserve
is zero.

### getPoolInfo

```solidity
function getPoolInfo() external view override returns (PoolInfo memory)
```

Get full pool info struct

### getPriceSnapshots

```solidity
function getPriceSnapshots() external view override returns (uint256[8] memory)
```

Returns up to eight price snapshots in chronological order, with unused entries zero-filled.

### \_calculateFee

```solidity
function _calculateFee() internal view returns (uint256)
```

### \_currentPrice

```solidity
function _currentPrice() internal view returns (uint256)
```

### \_updatePriceSnapshot

```solidity
function _updatePriceSnapshot(uint256 price, uint256 usdlVolume) internal
```

### \_getBalance

```solidity
function _getBalance(address token) internal view returns (uint256)
```

### swapFor

```solidity
function swapFor(uint256 amountIn, uint256 minAmountOut, bool isBuy, address trader, address recipient, uint256 deadline) external override nonReentrant whenNotPaused returns (uint256)
```

Requires the factory’s `ROUTER_ROLE` and preserves trader identity separately from the output
recipient.

### currentFeeBps

```solidity
function currentFeeBps() external view override returns (uint256)
```

Returns the current dynamic fee in basis points.

### transferGuardian

```solidity
function transferGuardian(address nextGuardian) external
```

The current guardian proposes a nonzero successor.

### acceptGuardian

```solidity
function acceptGuardian() external
```

Only the pending guardian accepts the transfer.
