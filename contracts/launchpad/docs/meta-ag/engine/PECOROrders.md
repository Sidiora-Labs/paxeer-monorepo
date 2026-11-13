# Solidity API

## PECOROrders

UUPS order engine for limit buys, limit sells, stop-loss orders and stop-limit orders. The vault
must grant it `OPERATOR_ROLE`. Placement pulls funds directly from the user into the vault and
reserves the amount actually received. Cancellation and expiry release that reservation and return
funds to the owner.

`DEFAULT_ADMIN_ROLE` controls configuration, keepers, upgrades and unpausing. Default administration
or `PAUSER_ROLE` can pause. `KEEPER_ROLE` controls execution and stop-limit activation; `keepers`
mirrors role membership. Cancellation and expiry remain available while paused once escrow is ready.

Trigger and limit prices are USD per whole nonstable token. `placeOrder` accepts an `OrderRequest`
with a minimum output in output-token base units. The individual placement methods set that minimum
to zero; the owner can change it with `setOrderMinOutput` while the order is pending or activated.
Settlement applies configured fees and price impact, checks available vault liquidity and the output
floor, and sends output to the owner. Retained output is reserved as accrued fees.

Each item in a batch executes in a separate external self-call. An item that reverts emits
`OrderExecutionFailed` without undoing successful items. Stop-limit activation can persist even if
execution is not yet eligible. The isolated helper entrypoints accept only this contract as their
caller.

`getOrdersPage` scans an active array using an index cursor, with scan and result limits capped
at 100. Kinds 0, 1 and 2 select executable limit orders, activatable stop-limit orders and
executable stop-limit orders. Pin one block across pages: execution and refunds remove entries by
swapping with the final array element. The convenience discovery methods scan at most the first 100
active entries.

Initialization marks escrow reservations ready. An upgraded proxy with `escrowInitialized == false`
must be paused while default administration calls `migrateEscrowReservations` in batches of 1 to 100
active orders. Placement, execution and refunds require completed reservation initialization.

Tracker calls are optional and use a 500,000 gas limit. A failed configured call emits
`TrackerCallFailed` without reverting order settlement.

Source: [PECOROrders.sol](../../../contracts/meta-ag/engine/PECOROrders.sol).

### KEEPER_ROLE

```solidity
bytes32 KEEPER_ROLE
```

### BPS_DENOMINATOR

```solidity
uint256 BPS_DENOMINATOR
```

### ZeroAddress

```solidity
error ZeroAddress()
```

### ZeroAmount

```solidity
error ZeroAmount()
```

### InvalidPrice

```solidity
error InvalidPrice()
```

### InvalidExpiry

```solidity
error InvalidExpiry()
```

### InvalidPriceRange

```solidity
error InvalidPriceRange()
```

### NotAStablecoin

```solidity
error NotAStablecoin()
```

### TokenIsStablecoin

```solidity
error TokenIsStablecoin()
```

### NotOrderOwner

```solidity
error NotOrderOwner()
```

### OrderNotPending

```solidity
error OrderNotPending()
```

### OrderNotActivated

```solidity
error OrderNotActivated()
```

### OrderExpired

```solidity
error OrderExpired()
```

### OrderCannotCancel

```solidity
error OrderCannotCancel()
```

### PriceNotMet

```solidity
error PriceNotMet()
```

### InsufficientLiquidity

```solidity
error InsufficientLiquidity()
```

### priceOracle

```solidity
contract IPriceOracle priceOracle
```

### vault

```solidity
contract IPECORVault vault
```

### transactionTracker

```solidity
contract ITransactionTracker transactionTracker
```

### nextOrderId

```solidity
uint256 nextOrderId
```

Monotonically increasing order id. Starts at 1 so that `0` reliably means "not a valid order id".

### limitOrders

```solidity
mapping(uint256 => struct IPECOROrders.LimitOrder) limitOrders
```

### stopLimitOrders

```solidity
mapping(uint256 => struct IPECOROrders.StopLimitOrder) stopLimitOrders
```

### userLimitOrders

```solidity
mapping(address => uint256[]) userLimitOrders
```

### userStopLimitOrders

```solidity
mapping(address => uint256[]) userStopLimitOrders
```

### activeLimitOrderIds

```solidity
uint256[] activeLimitOrderIds
```

### activeStopLimitOrderIds

```solidity
uint256[] activeStopLimitOrderIds
```

### keepers

```solidity
mapping(address => bool) keepers
```

Mirror of KEEPER_ROLE membership for O(1) view.

### constructor

```solidity
constructor() public
```

### initialize

```solidity
function initialize(address priceOracle_, address vault_, address tracker_, address admin_) external
```

Initialize the UUPS proxy

#### Parameters

| Name          | Type    | Description |
| ------------- | ------- | ----------- |
| priceOracle\_ | address |             |
| vault\_       | address |             |
| tracker\_     | address |             |
| admin\_       | address |             |

### \_authorizeUpgrade

```solidity
function _authorizeUpgrade(address) internal
```

### setKeeper

```solidity
function setKeeper(address keeper, bool authorized) external
```

Grant / revoke KEEPER_ROLE in one call.

_Rotates KEEPER_ROLE and mirrors the flag in `keepers` for O(1) reads._

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
function placeLimitBuy(address stablecoin, address token, uint256 amount, uint256 target, uint256 expiry) external returns (uint256)
```

### placeLimitSell

```solidity
function placeLimitSell(address token, address stablecoin, uint256 amount, uint256 target, uint256 expiry) external returns (uint256)
```

### placeStopLoss

```solidity
function placeStopLoss(address token, address stablecoin, uint256 amount, uint256 trigger, uint256 expiry) external returns (uint256)
```

### placeStopLimitBuy

```solidity
function placeStopLimitBuy(address stablecoin, address token, uint256 amount, uint256 stop, uint256 limit, uint256 expiry) external returns (uint256)
```

### placeStopLimitSell

```solidity
function placeStopLimitSell(address token, address stablecoin, uint256 amount, uint256 stop, uint256 limit, uint256 expiry) external returns (uint256)
```

### cancelLimitOrder

```solidity
function cancelLimitOrder(uint256 id) external
```

### cancelStopLimitOrder

```solidity
function cancelStopLimitOrder(uint256 id) external
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

### getActiveLimitOrderCount

```solidity
function getActiveLimitOrderCount() external view returns (uint256)
```

### getActiveStopLimitOrderCount

```solidity
function getActiveStopLimitOrderCount() external view returns (uint256)
```

### canExecuteLimitOrder

```solidity
function canExecuteLimitOrder(uint256 id) external view returns (bool, string memory)
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

### \_executeLimitOrder

```solidity
function _executeLimitOrder(uint256 orderId) internal
```

### \_executeStopLimitOrder

```solidity
function _executeStopLimitOrder(uint256 orderId) internal
```

### \_validatePair

```solidity
function _validatePair(address stablecoin, address token) internal view
```

### \_validateExpiry

```solidity
function _validateExpiry(uint256 expiresAt) internal view
```

### \_addToActiveLimitOrders

```solidity
function _addToActiveLimitOrders(uint256 orderId) internal
```

### \_addToActiveStopLimitOrders

```solidity
function _addToActiveStopLimitOrders(uint256 orderId) internal
```

### \_removeFromActiveLimitOrders

```solidity
function _removeFromActiveLimitOrders(uint256 orderId) internal
```

### \_removeFromActiveStopLimitOrders

```solidity
function _removeFromActiveStopLimitOrders(uint256 orderId) internal
```

### OnlySelf

```solidity
error OnlySelf()
```

### LiabilityMigrationRequired

```solidity
error LiabilityMigrationRequired()
```

### MigrationAlreadyComplete

```solidity
error MigrationAlreadyComplete()
```

### InvalidPricing

```solidity
error InvalidPricing()
```

### InsufficientOutput

```solidity
error InsufficientOutput()
```

### OrderExecutionFailed

```solidity
event OrderExecutionFailed(uint256 indexed orderId, bool stopLimit)
```

### OrderExpiredAndRefunded

```solidity
event OrderExpiredAndRefunded(uint256 indexed orderId, address indexed user, uint256 amount)
```

### TrackerCallFailed

```solidity
event TrackerCallFailed(address indexed tracker, bytes4 indexed selector)
```

### TransactionTrackerUpdated

```solidity
event TransactionTrackerUpdated(address indexed tracker)
```

### OrderMinimumOutputUpdated

```solidity
event OrderMinimumOutputUpdated(uint256 indexed orderId, uint256 minimum)
```

### PricingConfigUpdated

```solidity
event PricingConfigUpdated(uint256 swapFeeBps, uint256 tier1FeeBps, uint256 tier2FeeBps, bool impactEnabled, uint256 scalarBps)
```

### OrderFeesCollected

```solidity
event OrderFeesCollected(address indexed token, address indexed collector, uint256 amount)
```

### liabilitiesInitialized

```solidity
modifier liabilitiesInitialized()
```

### placeOrder

```solidity
function placeOrder(OrderRequest calldata request) external returns (uint256)
```

Places the request atomically with its minimum output. OrderRequest is declared by
[IPECOROrders.sol](../../../contracts/meta-ag/interfaces/IPECOROrders.sol).

### \_placeLimit

```solidity
function _placeLimit(address stablecoin, address token, uint256 amount, uint256 target, uint256 expiry, OrderType kind, uint256 minimum) internal returns (uint256 id)
```

### \_placeStopLimit

```solidity
function _placeStopLimit(address stablecoin, address token, uint256 amount, uint256 stop, uint256 limit, uint256 expiry, OrderType kind, uint256 minimum) internal returns (uint256 id)
```

### orderExecutionQuote

```solidity
function orderExecutionQuote(uint256 id, bool stop) external view returns (address output, uint256 gross, uint256 net)
```

### getPricingConfig

```solidity
function getPricingConfig() external view returns (PECOROrderPricing memory)
```

Returns PECOROrderPricing from
[PECOROrderViews.sol](../../../contracts/meta-ag/engine/PECOROrderViews.sol): fee, tier1, tier2,
scalar and impact.

### orderExecutionPaused

```solidity
function orderExecutionPaused() external view returns (bool)
```

### getOrdersPage

```solidity
function getOrdersPage(uint256 cursor, uint256 maxScanned, uint256 maxCount, uint8 kind) public view returns (uint256[] memory ids, uint256 next)
```

### \_settle

```solidity
function _settle(uint256 id, address user, address input, address output, uint256 amount) internal returns (uint256 net)
```

### \_price

```solidity
function _price(address input, address output, uint256 amount) internal view returns (uint256 gross, uint256 net)
```

### executeLimitOrderIsolated

```solidity
function executeLimitOrderIsolated(uint256 id) external
```

### activateAndExecuteStopLimitIsolated

```solidity
function activateAndExecuteStopLimitIsolated(uint256 id) external returns (bool activated, bool executed)
```

### executeStopLimitOrderIsolated

```solidity
function executeStopLimitOrderIsolated(uint256 id) external
```

### setTransactionTracker

```solidity
function setTransactionTracker(address tracker) external
```

### setPricingConfig

```solidity
function setPricingConfig(uint256 fee, uint256 tier1, uint256 tier2, bool impact, uint256 scalar) external
```

Requires default administration. The base and tier rates must sum to at most 200 basis points, tier2
must be at least tier1, and scalar must be at most 500.

### setFeeCollector

```solidity
function setFeeCollector(address collector) external
```

### collectFees

```solidity
function collectFees(address token) external
```

Only feeCollector may collect a nonzero accrued amount. Releases the reserved amount and transfers
it to feeCollector.

### setOrderMinOutput

```solidity
function setOrderMinOutput(uint256 id, uint256 minimum) external
```

### expireLimitOrder

```solidity
function expireLimitOrder(uint256 id) external
```

Anyone may refund an expired limit or stop-loss order to its owner. Expiry must be nonzero and no
later than the current block timestamp.

### expireStopLimitOrder

```solidity
function expireStopLimitOrder(uint256 id) external
```

Anyone may refund an expired pending or activated stop-limit order to its owner.

### refundOrderIsolated

```solidity
function refundOrderIsolated(uint256 id, bool stop, bool expired, address caller) external
```

### \_track

```solidity
function _track(bytes memory payload) internal
```

### liabilitiesReady

```solidity
function liabilitiesReady() external view returns (bool)
```

Returns escrowInitialized.

### migrateEscrowReservations

```solidity
function migrateEscrowReservations(uint256 maxCount) external
```

### \_grantRole

```solidity
function _grantRole(bytes32 role, address account) internal returns (bool changed)
```

### \_revokeRole

```solidity
function _revokeRole(bytes32 role, address account) internal returns (bool changed)
```

### orderMinOutput

```solidity
mapping(uint256 => uint256) orderMinOutput
```

### swapFeeBps

```solidity
uint256 swapFeeBps
```

### tier1FeeBps

```solidity
uint256 tier1FeeBps
```

### tier2FeeBps

```solidity
uint256 tier2FeeBps
```

### priceImpactScalarBps

```solidity
uint256 priceImpactScalarBps
```

### priceImpactEnabled

```solidity
bool priceImpactEnabled
```

### feeCollector

```solidity
address feeCollector
```

### accruedFees

```solidity
mapping(address => uint256) accruedFees
```

### escrowInitialized

```solidity
bool escrowInitialized
```

### limitMigrationCursor

```solidity
uint256 limitMigrationCursor
```

### stopMigrationCursor

```solidity
uint256 stopMigrationCursor
```
