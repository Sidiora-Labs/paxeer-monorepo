# Solidity API

## IDataFeedAdapter

Price feed interface consumed by OracleHub. The included PriceOracleAdapter and SidioraFeedAdapter
return USD prices with 18 decimal scaling. Missing or stale data is represented by a zero price or
confidence rather than a stale-data revert.

Confidence values are 0 for unavailable data, 1–3333 for low confidence, 3334–6666 for medium
confidence, and 6667–10000 for high confidence. Consumers also validate timestamps and the adapter’s
`maxStaleness` threshold.

Timestamps identify the source update. SidioraFeedAdapter uses the current block time for its
explicitly enabled spot reference; it also requires a fresh USDL/USD price from its configured
oracle. `sourceId` may read state; `adapterName` is pure.

Source: [IDataFeedAdapter.sol](../../../contracts/meta-ag/interfaces/IDataFeedAdapter.sol).

### FeedPrice

```solidity
struct FeedPrice {
  uint256 price;
  uint256 timestamp;
  uint256 confidence;
  bytes32 sourceId;
}
```

### getFeedPrice

```solidity
function getFeedPrice(address token) external view returns (struct IDataFeedAdapter.FeedPrice feed)
```

Get the latest price for a token from this adapter

_MUST NOT revert on stale data — return confidence=0 instead_

#### Parameters

| Name  | Type    | Description                |
| ----- | ------- | -------------------------- |
| token | address | The token address to price |

#### Return Values

| Name | Type                              | Description                                       |
| ---- | --------------------------------- | ------------------------------------------------- |
| feed | struct IDataFeedAdapter.FeedPrice | The full FeedPrice struct; price=0 if unsupported |

### supportsToken

```solidity
function supportsToken(address token) external view returns (bool supported)
```

Check whether this adapter supports a given token

#### Parameters

| Name  | Type    | Description                |
| ----- | ------- | -------------------------- |
| token | address | The token address to check |

#### Return Values

| Name      | Type | Description                              |
| --------- | ---- | ---------------------------------------- |
| supported | bool | True if this adapter can price the token |

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

### sourceId

```solidity
function sourceId() external view returns (bytes32 id)
```

Unique identifier for this adapter source (e.g., keccak256("PriceOracle.v1"))

#### Return Values

| Name | Type    | Description            |
| ---- | ------- | ---------------------- |
| id   | bytes32 | The bytes32 identifier |

### adapterName

```solidity
function adapterName() external pure returns (string name)
```

Human-readable name for this adapter

#### Return Values

| Name | Type   | Description             |
| ---- | ------ | ----------------------- |
| name | string | The adapter name string |

### maxStaleness

```solidity
function maxStaleness() external view returns (uint256 staleness)
```

Maximum age in seconds before this adapter's prices are considered stale

#### Return Values

| Name      | Type    | Description                    |
| --------- | ------- | ------------------------------ |
| staleness | uint256 | Staleness threshold in seconds |

### getSupportedTokens

```solidity
function getSupportedTokens() external view returns (address[] tokens)
```

Get all tokens this adapter currently supports

#### Return Values

| Name   | Type      | Description                        |
| ------ | --------- | ---------------------------------- |
| tokens | address[] | Array of supported token addresses |

### AdapterRegistered

```solidity
event AdapterRegistered(bytes32 indexed sourceId, address indexed adapter)
```

### AdapterPriceServed

```solidity
event AdapterPriceServed(bytes32 indexed sourceId, address indexed token, uint256 price, uint256 confidence)
```
