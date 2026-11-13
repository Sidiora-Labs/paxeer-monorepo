# Solidity API

## ITransactionTracker

Interface for the Sidiora Meta-AG Transaction Tracker — event hub for PECOR-flow analytics

`EMITTER_ROLE` gates recording. Only `recordTrade` updates volume, trade counts and user totals; the
other recording methods emit events. `forceDaySnapshot` archives an earlier daily bucket, and
`getCurrentDayStats` returns an empty bucket for today if the stored day has ended.

Source: [ITransactionTracker.sol](../../../contracts/meta-ag/interfaces/ITransactionTracker.sol).

### DailyStats

```solidity
struct DailyStats {
  uint256 totalVolume;
  uint256 totalTrades;
  uint256 uniqueTraders;
  uint256 timestamp;
}
```

### TokenStats

```solidity
struct TokenStats {
  uint256 totalVolume;
  uint256 tradeCount;
  uint256 lastTradeTimestamp;
}
```

### UserStats

```solidity
struct UserStats {
  uint256 totalVolume;
  uint256 tradeCount;
  uint256 firstTradeTimestamp;
  uint256 lastTradeTimestamp;
}
```

### initialize

```solidity
function initialize(address admin) external
```

Initialize the UUPS proxy

#### Parameters

| Name  | Type    | Description                           |
| ----- | ------- | ------------------------------------- |
| admin | address | Timelock — granted DEFAULT_ADMIN_ROLE |

### setAuthorizedEmitter

```solidity
function setAuthorizedEmitter(address emitter, bool authorized) external
```

Grant / revoke EMITTER_ROLE to a contract address.

### forceDaySnapshot

```solidity
function forceDaySnapshot() external
```

Force a daily stats snapshot (e.g., ops-driven day rollover).

### recordTrade

```solidity
function recordTrade(address user, address tokenIn, address tokenOut, uint256 amountIn, uint256 amountOut, uint256 volumeUSD) external
```

### recordMarketTrade

```solidity
function recordMarketTrade(address user, address stablecoin, address token, uint256 stablecoinAmount, uint256 tokenAmount, bool isBuy, uint256 executionPrice) external
```

### recordLimitOrderPlaced

```solidity
function recordLimitOrderPlaced(uint256 orderId, address user, address token, address stablecoin, uint256 amount, uint256 targetPrice, bool isBuy) external
```

### recordLimitOrderExecuted

```solidity
function recordLimitOrderExecuted(uint256 orderId, address user, uint256 executionPrice) external
```

### recordLimitOrderCancelled

```solidity
function recordLimitOrderCancelled(uint256 orderId, address user) external
```

### recordStopLossPlaced

```solidity
function recordStopLossPlaced(uint256 orderId, address user, address token, uint256 tokenAmount, uint256 triggerPrice) external
```

### recordStopLossTriggered

```solidity
function recordStopLossTriggered(uint256 orderId, address user, uint256 triggerPrice, uint256 executionPrice, uint256 amountOut) external
```

### recordStopLimitPlaced

```solidity
function recordStopLimitPlaced(uint256 orderId, address user, address token, uint256 amount, uint256 stopPrice, uint256 limitPrice, bool isBuy) external
```

### recordStopLimitActivated

```solidity
function recordStopLimitActivated(uint256 orderId, uint256 activationPrice) external
```

### recordStopLimitExecuted

```solidity
function recordStopLimitExecuted(uint256 orderId, address user, uint256 executionPrice) external
```

### recordLiquidityAdded

```solidity
function recordLiquidityAdded(address token, address provider, uint256 amount, uint256 newReserve) external
```

### recordLiquidityRemoved

```solidity
function recordLiquidityRemoved(address token, address remover, uint256 amount, uint256 newReserve) external
```

### getDailyStats

```solidity
function getDailyStats(uint256 dayTimestamp) external view returns (struct ITransactionTracker.DailyStats)
```

### getTokenStats

```solidity
function getTokenStats(address token) external view returns (struct ITransactionTracker.TokenStats)
```

### getUserStats

```solidity
function getUserStats(address user) external view returns (struct ITransactionTracker.UserStats)
```

### getCurrentDayStats

```solidity
function getCurrentDayStats() external view returns (struct ITransactionTracker.DailyStats)
```

### TradeExecuted

```solidity
event TradeExecuted(address indexed user, address indexed tokenIn, address indexed tokenOut, uint256 amountIn, uint256 amountOut, uint256 volumeUSD, uint256 timestamp)
```

### MarketTrade

```solidity
event MarketTrade(address indexed user, address indexed stablecoin, address indexed token, uint256 stablecoinAmount, uint256 tokenAmount, bool isBuy, uint256 executionPrice, uint256 timestamp)
```

### LimitOrderPlaced

```solidity
event LimitOrderPlaced(uint256 indexed orderId, address indexed user, address indexed token, address stablecoin, uint256 amount, uint256 targetPrice, bool isBuy, uint256 timestamp)
```

### LimitOrderExecuted

```solidity
event LimitOrderExecuted(uint256 indexed orderId, address indexed user, uint256 executionPrice, uint256 timestamp)
```

### LimitOrderCancelled

```solidity
event LimitOrderCancelled(uint256 indexed orderId, address indexed user, uint256 timestamp)
```

### StopLossPlaced

```solidity
event StopLossPlaced(uint256 indexed orderId, address indexed user, address indexed token, uint256 tokenAmount, uint256 triggerPrice, uint256 timestamp)
```

### StopLossTriggered

```solidity
event StopLossTriggered(uint256 indexed orderId, address indexed user, uint256 triggerPrice, uint256 executionPrice, uint256 amountOut, uint256 timestamp)
```

### StopLimitPlaced

```solidity
event StopLimitPlaced(uint256 indexed orderId, address indexed user, address indexed token, uint256 amount, uint256 stopPrice, uint256 limitPrice, bool isBuy, uint256 timestamp)
```

### StopLimitActivated

```solidity
event StopLimitActivated(uint256 indexed orderId, uint256 activationPrice, uint256 timestamp)
```

### StopLimitExecuted

```solidity
event StopLimitExecuted(uint256 indexed orderId, address indexed user, uint256 executionPrice, uint256 timestamp)
```

### LiquidityAdded

```solidity
event LiquidityAdded(address indexed token, address indexed provider, uint256 amount, uint256 newReserve, uint256 timestamp)
```

### LiquidityRemoved

```solidity
event LiquidityRemoved(address indexed token, address indexed remover, uint256 amount, uint256 newReserve, uint256 timestamp)
```

### DailyStatsSnapshot

```solidity
event DailyStatsSnapshot(uint256 indexed dayTimestamp, uint256 totalVolume, uint256 totalTrades, uint256 uniqueTraders)
```

### TokenVolumeUpdate

```solidity
event TokenVolumeUpdate(address indexed token, uint256 dailyVolume, uint256 totalVolume, uint256 timestamp)
```

### VaultLiquidityUpdate

```solidity
event VaultLiquidityUpdate(address indexed token, uint256 reserve, uint256 timestamp)
```

### UserTradeMetrics

```solidity
event UserTradeMetrics(address indexed user, uint256 totalVolume, uint256 tradeCount, uint256 timestamp)
```

### AuthorizedEmitterUpdated

```solidity
event AuthorizedEmitterUpdated(address indexed emitter, bool authorized)
```
