# Solidity API

Source:
[SidioraFeedAdapter.sol](../../../../contracts/meta-ag/oracle/adapters/SidioraFeedAdapter.sol).

## SidioraFeedAdapter

`IDataFeedAdapter` wrapper for Sidiora pool spot prices. The administrator configures the pool
registry, liquidity threshold, known-token list and USDL-to-USD reference oracle.

Feeds are disabled by default. `configureSpotReference` enables them with an oracle and USDL token.
A feed multiplies the pool's USDL price by the reference oracle's USD price and scales by `1e18`.
Missing pools, zero prices and failed pool or reference-oracle reads return an empty feed.
Successful feeds use the current block timestamp and these confidence bands:

| Real USDL reserves                        | Confidence |
| ----------------------------------------- | ---------- |
| At least `minLiquidityThreshold`          | 7000       |
| At least one quarter of the threshold     | 4000       |
| Below one quarter, or reserves read fails | 1500       |

The adapter is not upgradeable. `getSupportedTokens` enumerates the administrator-maintained
known-token list; `supportsToken` checks for a pool in the configured registry.

### CONFIDENCE_HIGH

```solidity
uint256 CONFIDENCE_HIGH
```

### CONFIDENCE_MEDIUM

```solidity
uint256 CONFIDENCE_MEDIUM
```

### CONFIDENCE_LOW

```solidity
uint256 CONFIDENCE_LOW
```

### ZeroAddress

```solidity
error ZeroAddress()
```

### poolRegistry

```solidity
contract IPoolRegistry poolRegistry
```

### minLiquidityThreshold

```solidity
uint256 minLiquidityThreshold
```

### constructor

```solidity
constructor(address poolRegistry_, uint256 minLiquidityThreshold_, address admin_) public
```

### setPoolRegistry

```solidity
function setPoolRegistry(address registry_) external
```

### setMinLiquidityThreshold

```solidity
function setMinLiquidityThreshold(uint256 threshold_) external
```

### registerKnownToken

```solidity
function registerKnownToken(address token) external
```

### registerKnownTokens

```solidity
function registerKnownTokens(address[] tokens) external
```

### sourceId

```solidity
function sourceId() external pure returns (bytes32)
```

Returns `keccak256("SidioraAMM.v1")`.

#### Return Values

| Name | Type    | Description |
| ---- | ------- | ----------- |
| [0]  | bytes32 |             |

### adapterName

```solidity
function adapterName() external pure returns (string)
```

Human-readable name for this adapter

#### Return Values

| Name | Type   | Description |
| ---- | ------ | ----------- |
| [0]  | string |             |

### maxStaleness

```solidity
function maxStaleness() external pure returns (uint256)
```

Returns 120 seconds. Successful feeds are stamped with the current block timestamp.

#### Return Values

| Name | Type    | Description |
| ---- | ------- | ----------- |
| [0]  | uint256 |             |

### supportsToken

```solidity
function supportsToken(address token) external view returns (bool)
```

Check whether this adapter supports a given token

#### Parameters

| Name  | Type    | Description                |
| ----- | ------- | -------------------------- |
| token | address | The token address to check |

#### Return Values

| Name | Type | Description |
| ---- | ---- | ----------- |
| [0]  | bool |             |

### getSupportedTokens

```solidity
function getSupportedTokens() external view returns (address[] tokens)
```

Get all tokens this adapter currently supports

#### Return Values

| Name   | Type      | Description                        |
| ------ | --------- | ---------------------------------- |
| tokens | address[] | Array of supported token addresses |

### getFeedPrice

```solidity
function getFeedPrice(address token) external view returns (struct IDataFeedAdapter.FeedPrice feed)
```

Get the latest price for a token from this adapter

Returns a zero-price, zero-confidence feed when spot references are disabled or required data is
unavailable.

#### Parameters

| Name  | Type    | Description                |
| ----- | ------- | -------------------------- |
| token | address | The token address to price |

#### Return Values

| Name | Type                              | Description                                       |
| ---- | --------------------------------- | ------------------------------------------------- |
| feed | struct IDataFeedAdapter.FeedPrice | The full FeedPrice struct; price=0 if unsupported |

### getFeedPrices

```solidity
function getFeedPrices(address[] tokens) external view returns (struct IDataFeedAdapter.FeedPrice[] feeds)
```

Get prices for multiple tokens in one call

#### Parameters

| Name   | Type      | Description              |
| ------ | --------- | ------------------------ |
| tokens | address[] | Array of token addresses |

#### Return Values

| Name  | Type                                | Description                                           |
| ----- | ----------------------------------- | ----------------------------------------------------- |
| feeds | struct IDataFeedAdapter.FeedPrice[] | Array of FeedPrice structs (parallel to tokens array) |

### \_readFeed

```solidity
function _readFeed(address token) internal view returns (struct IDataFeedAdapter.FeedPrice feed)
```

### \_getPool

```solidity
function _getPool(address token) internal view returns (address pool)
```

### \_safeGetPrice

```solidity
function _safeGetPrice(address pool) internal view returns (uint256 price)
```

### \_computeConfidence

```solidity
function _computeConfidence(address pool) internal view returns (uint256)
```

### \_registerKnownToken

```solidity
function _registerKnownToken(address token) internal
```

### InvalidConfig

```solidity
error InvalidConfig()
```

### spotReferenceEnabled

```solidity
bool spotReferenceEnabled
```

### usdlUsdOracle

```solidity
contract IPriceOracle usdlUsdOracle
```

### usdlToken

```solidity
address usdlToken
```

### configureSpotReference

```solidity
function configureSpotReference(bool enabled, address usdOracle, address usdl) external
```

Administrator-only setter. Enabling requires an oracle with code and a nonzero USDL token address.
