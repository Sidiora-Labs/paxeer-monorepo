# Solidity API

## TransactionTracker

UUPS analytics contract for authorized emitters. `EMITTER_ROLE` controls all recording methods, and
`DEFAULT_ADMIN_ROLE` controls emitter membership, snapshots and upgrades. `authorizedEmitters`
mirrors role membership, including changes made through inherited role administration.

`recordTrade` updates daily, token and user totals. Each token side receives `volumeUSD / 2`,
rounded down, and one trade count. If both sides name the same token, that token receives both
credits and two trade counts. Daily and user totals receive the full volume, and each distinct
user is counted once per day. Other recording methods emit their corresponding events without
changing trade totals. A configured producer must call the tracker for activity to appear here.

Daily buckets use Unix timestamps floored to one day. `recordTrade` and `forceDaySnapshot` roll a
bucket forward when the day changes; a snapshot call in the same day leaves the bucket unchanged.
`getCurrentDayStats` returns an empty bucket dated today if the stored bucket belongs to an earlier
day.

Source: [TransactionTracker.sol](../../../contracts/meta-ag/analytics/TransactionTracker.sol).

### EMITTER_ROLE

```solidity
bytes32 EMITTER_ROLE
```

### ZeroAddress

```solidity
error ZeroAddress()
```

### authorizedEmitters

```solidity
mapping(address => bool) authorizedEmitters
```

Mirror of `EMITTER_ROLE` membership (kept for cheap O(1) bool view).

### currentDayTimestamp

```solidity
uint256 currentDayTimestamp
```

Start-of-day Unix timestamp (floored to 1 day) for the active bucket.

### currentDayStats

```solidity
struct ITransactionTracker.DailyStats currentDayStats
```

Aggregated stats for the currently-accumulating day.

### dailyStats

```solidity
mapping(uint256 => struct ITransactionTracker.DailyStats) dailyStats
```

Historical daily stats keyed by day-floor timestamp.

### tokenStats

```solidity
mapping(address => struct ITransactionTracker.TokenStats) tokenStats
```

Per-token lifetime totals.

### userStats

```solidity
mapping(address => struct ITransactionTracker.UserStats) userStats
```

Per-user lifetime totals.

### constructor

```solidity
constructor() public
```

### initialize

```solidity
function initialize(address admin) external
```

Initialize the UUPS proxy with a nonzero administrator.

#### Parameters

| Name  | Type    | Description                              |
| ----- | ------- | ---------------------------------------- |
| admin | address | Administrator granted DEFAULT_ADMIN_ROLE |

### \_authorizeUpgrade

```solidity
function _authorizeUpgrade(address) internal
```

### setAuthorizedEmitter

```solidity
function setAuthorizedEmitter(address emitter, bool authorized) external
```

Grant or revoke `EMITTER_ROLE` for a nonzero emitter address.

### forceDaySnapshot

```solidity
function forceDaySnapshot() external
```

Archive the stored day and start today’s bucket if the day has changed. Requires default
administration.

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
function getCurrentDayStats() external view returns (DailyStats memory stats)
```

### \_grantRole

```solidity
function _grantRole(bytes32 role, address account) internal returns (bool changed)
```

### \_revokeRole

```solidity
function _revokeRole(bytes32 role, address account) internal returns (bool changed)
```
