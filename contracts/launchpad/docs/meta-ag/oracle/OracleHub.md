# Solidity API

Source: [OracleHub.sol](../../../contracts/meta-ag/oracle/OracleHub.sol).

## OracleHub

UUPS-upgradeable price aggregator using prioritized `IDataFeedAdapter` sources. Administrators
manage adapters and configuration. An administrator or `PAUSER_ROLE` holder can pause; only an
administrator can unpause.

- `getPrice` returns the first valid feed in priority order.
- `getAggregatedPrice` computes a confidence-weighted median after filtering deviations relative to
  the first valid feed. It returns a zero result when no usable feed remains.
- `getTWAP` delegates to the configured primary `IPriceOracle` and propagates its errors.

Feed validation checks price, timestamp, confidence and the adapter's maximum staleness. Adapter
read errors are treated as unavailable feeds in the aggregated and best-source queries.

### BPS_DENOMINATOR

```solidity
uint256 BPS_DENOMINATOR
```

### MAX_ADAPTERS

```solidity
uint256 MAX_ADAPTERS
```

### ZeroAddress

```solidity
error ZeroAddress()
```

### InvalidConfig

```solidity
error InvalidConfig()
```

### MaxAdaptersReached

```solidity
error MaxAdaptersReached()
```

### AdapterAlreadyRegistered

```solidity
error AdapterAlreadyRegistered()
```

### AdapterNotFound

```solidity
error AdapterNotFound()
```

### NoActiveAdapters

```solidity
error NoActiveAdapters()
```

### SourceIdConflict

```solidity
error SourceIdConflict()
```

### deviationThresholdBps

```solidity
uint256 deviationThresholdBps
```

The current deviation threshold in BPS

### minConfidence

```solidity
uint256 minConfidence
```

The minimum confidence score threshold

### primaryOracle

```solidity
address primaryOracle
```

### constructor

```solidity
constructor() public
```

### initialize

```solidity
function initialize(address _primaryOracle, uint256 _deviationBps, uint256 _minConfidence, address admin) external
```

Initialize the UUPS proxy

#### Parameters

| Name            | Type    | Description                        |
| --------------- | ------- | ---------------------------------- |
| \_primaryOracle | address |                                    |
| \_deviationBps  | uint256 |                                    |
| \_minConfidence | uint256 |                                    |
| admin           | address | Account granted DEFAULT_ADMIN_ROLE |

### \_authorizeUpgrade

```solidity
function _authorizeUpgrade(address) internal
```

### pause

```solidity
function pause() external
```

Pause `getPrice`, `getAggregatedPrice`, `getPricesBatch` and `getTWAP`. `isPriceAvailable` returns
`(false, 0)` while paused; `getPriceFromSource` remains callable.

### unpause

```solidity
function unpause() external
```

Resume price queries after pause.

### setPrimaryOracle

```solidity
function setPrimaryOracle(address oracle) external
```

Set the primary oracle used for TWAP queries

#### Parameters

| Name   | Type    | Description                                     |
| ------ | ------- | ----------------------------------------------- |
| oracle | address | Address of the IPriceOracle-compatible contract |

### setDeviationThreshold

```solidity
function setDeviationThreshold(uint256 deviationBps) external
```

Set the max deviation allowed between adapters before circuit breaking

#### Parameters

| Name         | Type    | Description                           |
| ------------ | ------- | ------------------------------------- |
| deviationBps | uint256 | Max deviation in BPS (e.g., 500 = 5%) |

### setMinConfidence

```solidity
function setMinConfidence(uint256 _minConfidence) external
```

### registerAdapter

```solidity
function registerAdapter(address adapter, uint256 priority) external
```

Register a new data feed adapter

#### Parameters

| Name     | Type    | Description                                       |
| -------- | ------- | ------------------------------------------------- |
| adapter  | address | The IDataFeedAdapter-compliant contract address   |
| priority | uint256 | Priority order (0 = highest priority, used first) |

### deactivateAdapter

```solidity
function deactivateAdapter(bytes32 sid) external
```

Deactivate an adapter (does not remove, just disables)

#### Parameters

| Name | Type    | Description |
| ---- | ------- | ----------- |
| sid  | bytes32 |             |

### activateAdapter

```solidity
function activateAdapter(bytes32 sid) external
```

Reactivate a previously deactivated adapter

#### Parameters

| Name | Type    | Description |
| ---- | ------- | ----------- |
| sid  | bytes32 |             |

### setAdapterPriority

```solidity
function setAdapterPriority(bytes32 sid, uint256 newPriority) external
```

Update the priority of an existing adapter

#### Parameters

| Name        | Type    | Description                                  |
| ----------- | ------- | -------------------------------------------- |
| sid         | bytes32 |                                              |
| newPriority | uint256 | New priority value (lower = higher priority) |

### getPrice

```solidity
function getPrice(address token) external view returns (uint256 price)
```

Get the best available price for a token

_Uses highest-priority adapter that returns non-stale, non-zero price. Reverts if no valid price is
found from any adapter._

#### Parameters

| Name  | Type    | Description       |
| ----- | ------- | ----------------- |
| token | address | The token address |

#### Return Values

| Name  | Type    | Description                     |
| ----- | ------- | ------------------------------- |
| price | uint256 | Token price in USD, 18 decimals |

### getAggregatedPrice

```solidity
function getAggregatedPrice(address token) external view returns (struct IOracleHub.AggregatedPrice result)
```

Get the aggregated price with full metadata

Aggregates valid feeds from active adapters using a confidence-weighted median. Adapter failures are
skipped; `sourceCount` is zero if no usable feed remains. Reverts while paused.

#### Parameters

| Name  | Type    | Description       |
| ----- | ------- | ----------------- |
| token | address | The token address |

#### Return Values

| Name   | Type                              | Description                |
| ------ | --------------------------------- | -------------------------- |
| result | struct IOracleHub.AggregatedPrice | The AggregatedPrice struct |

### getPriceFromSource

```solidity
function getPriceFromSource(address token, bytes32 sid) external view returns (struct IDataFeedAdapter.FeedPrice feed)
```

Read a registered adapter directly by source ID. This call does not filter inactive or stale feeds
and propagates adapter errors.

#### Parameters

| Name  | Type    | Description       |
| ----- | ------- | ----------------- |
| token | address | The token address |
| sid   | bytes32 |                   |

#### Return Values

| Name | Type                              | Description                              |
| ---- | --------------------------------- | ---------------------------------------- |
| feed | struct IDataFeedAdapter.FeedPrice | The FeedPrice from that specific adapter |

### getPricesBatch

```solidity
function getPricesBatch(address[] tokens) external view returns (uint256[] prices, uint256[] confidences)
```

Get prices for multiple tokens using best-available source

#### Parameters

| Name   | Type      | Description              |
| ------ | --------- | ------------------------ |
| tokens | address[] | Array of token addresses |

#### Return Values

| Name        | Type      | Description                                     |
| ----------- | --------- | ----------------------------------------------- |
| prices      | uint256[] | Array of prices (18 decimals); 0 if unavailable |
| confidences | uint256[] | Array of confidence scores (0–10000)            |

### getTWAP

```solidity
function getTWAP(address token, uint256 period) external view returns (uint256 twapPrice)
```

Get the TWAP price from the configured primary oracle.

Requires a nonzero period and an unpaused hub. Propagates errors from the primary oracle, including
stale prices and insufficient history.

#### Parameters

| Name   | Type    | Description            |
| ------ | ------- | ---------------------- |
| token  | address | The token address      |
| period | uint256 | TWAP period in seconds |

#### Return Values

| Name      | Type    | Description                 |
| --------- | ------- | --------------------------- |
| twapPrice | uint256 | The TWAP price, 18 decimals |

### isPriceAvailable

```solidity
function isPriceAvailable(address token) external view returns (bool available, uint256 bestConfidence)
```

Check if a valid (non-stale, non-zero) price exists for a token

#### Parameters

| Name  | Type    | Description       |
| ----- | ------- | ----------------- |
| token | address | The token address |

#### Return Values

| Name           | Type    | Description                                    |
| -------------- | ------- | ---------------------------------------------- |
| available      | bool    | True if at least one adapter has a valid price |
| bestConfidence | uint256 | The highest confidence score available         |

### getAdapters

```solidity
function getAdapters() external view returns (struct IOracleHub.AdapterInfo[] adapters)
```

Get all registered adapters

#### Return Values

| Name     | Type                            | Description                  |
| -------- | ------------------------------- | ---------------------------- |
| adapters | struct IOracleHub.AdapterInfo[] | Array of AdapterInfo structs |

### getAdapter

```solidity
function getAdapter(bytes32 sid) external view returns (struct IOracleHub.AdapterInfo info)
```

### adapterCount

```solidity
function adapterCount() external view returns (uint256 count)
```

Get the number of registered adapters

#### Return Values

| Name  | Type    | Description                             |
| ----- | ------- | --------------------------------------- |
| count | uint256 | Total adapter count (active + inactive) |

### \_sortAdaptersByPriority

```solidity
function _sortAdaptersByPriority() internal
```

_Insertion sort by priority ASC. O(n^2) — fine since MAX_ADAPTERS == 20._

### removeAdapter

```solidity
function removeAdapter(bytes32 sid) external
```

Administrator-only removal of an adapter, freeing its address, source ID and registry slot.

### AdapterRemoved

```solidity
event AdapterRemoved(bytes32 indexed sourceId, address indexed adapter)
```

### validatedFeed

```solidity
function validatedFeed(address adapter, address token) external view returns (struct IDataFeedAdapter.FeedPrice feed)
```

Callable only by the hub itself. Reads and validates one feed; reverts on invalid data.

### \_validatedFeed

```solidity
function _validatedFeed(address adapter, address token) internal view returns (bool valid, struct IDataFeedAdapter.FeedPrice feed)
```

Wraps the validation call and returns `valid=false` on failure.
