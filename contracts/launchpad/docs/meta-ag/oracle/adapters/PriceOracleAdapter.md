# Solidity API

Source:
[PriceOracleAdapter.sol](../../../../contracts/meta-ag/oracle/adapters/PriceOracleAdapter.sol).

## PriceOracleAdapter

`IDataFeedAdapter` wrapper around an immutable `IPriceOracle` reference.

Confidence is computed from the latest round's age:

| Condition                                                          | Confidence |
| ------------------------------------------------------------------ | ---------- |
| Age exceeds the token's `maxStaleness`                             | 0          |
| Age is at most `heartbeatInterval` and within `maxStaleness`       | 9000       |
| Age is at most twice `heartbeatInterval` and within `maxStaleness` | 6000       |
| Otherwise, within `maxStaleness`                                   | 3000       |

Unregistered tokens and missing or future timestamps return a zero-price feed. Stale rounds retain
their price and timestamp with zero confidence. Errors from the configured oracle propagate.

### CONFIDENCE_FRESH

```solidity
uint256 CONFIDENCE_FRESH
```

### CONFIDENCE_AGING

```solidity
uint256 CONFIDENCE_AGING
```

### CONFIDENCE_NEAR_STALE

```solidity
uint256 CONFIDENCE_NEAR_STALE
```

### ZeroAddress

```solidity
error ZeroAddress()
```

### priceOracle

```solidity
contract IPriceOracle priceOracle
```

### constructor

```solidity
constructor(address priceOracle_) public
```

### sourceId

```solidity
function sourceId() external pure returns (bytes32)
```

Returns `keccak256("PaxeerPriceOracle.v1")`.

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

Returns `type(uint256).max`. Per-token staleness is enforced by setting feed confidence to zero.

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

Stale data returns zero confidence while retaining its price and timestamp. Oracle call errors
propagate to the caller.

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
