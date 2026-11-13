# Solidity API

## IPECOROrders

Interface for UUPS limit, stop-loss and stop-limit orders executed by keepers against PECORVault.
Placement pulls input into the vault and reserves the amount received; cancellation returns that
recorded amount to the owner. Execution releases the input reservation and sends priced output to
the owner.

`OrderRequest` supports a minimum output in output-token base units. Trigger and limit prices are
USD per whole nonstable token. A zero expiry means no expiry. The individual placement methods use a
zero output floor. The concrete [PECOROrders](../engine/PECOROrders.md) contract also exposes
output-floor updates, expiry refunds, pricing configuration and paginated discovery.

Source: [IPECOROrders.sol](../../../contracts/meta-ag/interfaces/IPECOROrders.sol).

### OrderType

```solidity
enum OrderType {
  LIMIT_BUY,
  LIMIT_SELL,
  STOP_LOSS,
  STOP_LIMIT_BUY,
  STOP_LIMIT_SELL
}
```

### OrderStatus

```solidity
enum OrderStatus {
  PENDING,
  ACTIVATED,
  EXECUTED,
  CANCELLED,
  EXPIRED
}
```

### LimitOrder

```solidity
struct LimitOrder {
  uint256 id;
  address user;
  address stablecoin;
  address token;
  uint256 amount;
  uint256 targetPrice;
  enum IPECOROrders.OrderType orderType;
  enum IPECOROrders.OrderStatus status;
  uint256 createdAt;
  uint256 expiresAt;
}
```

### StopLimitOrder

```solidity
struct StopLimitOrder {
  uint256 id;
  address user;
  address stablecoin;
  address token;
  uint256 amount;
  uint256 stopPrice;
  uint256 limitPrice;
  enum IPECOROrders.OrderType orderType;
  enum IPECOROrders.OrderStatus status;
  uint256 createdAt;
  uint256 expiresAt;
}
```

### initialize

```solidity
function initialize(address priceOracle, address vault, address tracker, address admin) external
```

Initialize the UUPS proxy

#### Parameters

| Name        | Type    | Description                                                        |
| ----------- | ------- | ------------------------------------------------------------------ |
| priceOracle | address | IPriceOracle address                                               |
| vault       | address | IPECORVault v2 address (must grant OPERATOR_ROLE to this contract) |
| tracker     | address | ITransactionTracker address (may be zero at bootstrap)             |
| admin       | address | Timelock — granted DEFAULT_ADMIN_ROLE                              |

### setKeeper

```solidity
function setKeeper(address keeper, bool authorized) external
```

Grant / revoke KEEPER_ROLE in one call.

### pause

```solidity
function pause() external
```

### unpause

```solidity
function unpause() external
```

### placeLimitBuy

```solidity
function placeLimitBuy(address stablecoin, address token, uint256 stablecoinAmount, uint256 targetPrice, uint256 expiresAt) external returns (uint256 orderId)
```

### placeLimitSell

```solidity
function placeLimitSell(address token, address stablecoin, uint256 tokenAmount, uint256 targetPrice, uint256 expiresAt) external returns (uint256 orderId)
```

### placeStopLoss

```solidity
function placeStopLoss(address token, address stablecoin, uint256 tokenAmount, uint256 triggerPrice, uint256 expiresAt) external returns (uint256 orderId)
```

### placeStopLimitBuy

```solidity
function placeStopLimitBuy(address stablecoin, address token, uint256 stablecoinAmount, uint256 stopPrice, uint256 limitPrice, uint256 expiresAt) external returns (uint256 orderId)
```

### placeStopLimitSell

```solidity
function placeStopLimitSell(address token, address stablecoin, uint256 tokenAmount, uint256 stopPrice, uint256 limitPrice, uint256 expiresAt) external returns (uint256 orderId)
```

### cancelLimitOrder

```solidity
function cancelLimitOrder(uint256 orderId) external
```

### cancelStopLimitOrder

```solidity
function cancelStopLimitOrder(uint256 orderId) external
```

### executeLimitOrder

```solidity
function executeLimitOrder(uint256 orderId) external
```

### executeStopLimitOrder

```solidity
function executeStopLimitOrder(uint256 orderId) external
```

### checkStopLimitActivation

```solidity
function checkStopLimitActivation(uint256 orderId) external
```

### batchExecuteLimitOrders

```solidity
function batchExecuteLimitOrders(uint256[] orderIds) external returns (uint256 executedCount)
```

### batchCheckAndExecuteStopLimits

```solidity
function batchCheckAndExecuteStopLimits(uint256[] orderIds) external returns (uint256 activatedCount, uint256 executedCount)
```

### getLimitOrder

```solidity
function getLimitOrder(uint256 orderId) external view returns (struct IPECOROrders.LimitOrder)
```

### getStopLimitOrder

```solidity
function getStopLimitOrder(uint256 orderId) external view returns (struct IPECOROrders.StopLimitOrder)
```

### getUserLimitOrders

```solidity
function getUserLimitOrders(address user) external view returns (uint256[])
```

### getUserStopLimitOrders

```solidity
function getUserStopLimitOrders(address user) external view returns (uint256[])
```

### canExecuteLimitOrder

```solidity
function canExecuteLimitOrder(uint256 orderId) external view returns (bool, string)
```

### getExecutableLimitOrders

```solidity
function getExecutableLimitOrders(uint256 maxCount) external view returns (uint256[] orderIds)
```

### getActivatableStopLimits

```solidity
function getActivatableStopLimits(uint256 maxCount) external view returns (uint256[] orderIds)
```

### getExecutableStopLimits

```solidity
function getExecutableStopLimits(uint256 maxCount) external view returns (uint256[] orderIds)
```

### LimitOrderCreated

```solidity
event LimitOrderCreated(uint256 indexed orderId, address indexed user, OrderType orderType, uint256 amount, uint256 targetPrice)
```

### LimitOrderExecuted

```solidity
event LimitOrderExecuted(uint256 indexed orderId, uint256 executionPrice)
```

### LimitOrderCancelled

```solidity
event LimitOrderCancelled(uint256 indexed orderId)
```

### StopLimitOrderCreated

```solidity
event StopLimitOrderCreated(uint256 indexed orderId, address indexed user, OrderType orderType, uint256 amount, uint256 stopPrice, uint256 limitPrice)
```

### StopLimitActivated

```solidity
event StopLimitActivated(uint256 indexed orderId, uint256 activationPrice)
```

### StopLimitExecuted

```solidity
event StopLimitExecuted(uint256 indexed orderId, uint256 executionPrice)
```

### StopLimitCancelled

```solidity
event StopLimitCancelled(uint256 indexed orderId)
```

### KeeperUpdated

```solidity
event KeeperUpdated(address indexed keeper, bool authorized)
```

### placeOrder

```solidity
function placeOrder(OrderRequest calldata request) external returns (uint256 orderId)
```

Places the requested order with its minimum delivered output. LIMIT_BUY, LIMIT_SELL and STOP_LOSS
use triggerPrice as their target; stop-limit orders also use limitPrice.

### OrderRequest

```solidity
struct OrderRequest {
  OrderType orderType;
  address stablecoin;
  address token;
  uint256 amount;
  uint256 triggerPrice;
  uint256 limitPrice;
  uint256 expiresAt;
  uint256 minimumOutput;
}
```

Trigger and limit prices use USD per whole nonstable token. minimumOutput uses output-token base
units. A zero expiresAt means no expiry.
