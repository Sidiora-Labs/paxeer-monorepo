# Solidity API

## MockFeedAdapterBase

Configurable feed adapter contracts for OracleHub tests. The base stores prices, timestamps,
confidence values, support flags and a staleness threshold. Its abstract `sourceId` and
`adapterName` methods are pure; concrete A, B and C contracts return fixed identifiers and names.

`setPrice` stores a feed and marks the token supported. `setSupported` changes support
independently. The token list retains addresses after support is disabled; `setPrice` appends the
token again if its support flag is false, so the list can contain duplicates. `getFeedPrice` returns
the stored feed without freshness or support validation, and can be configured to revert with
`setRevertOnGet`. `getFeedPrices` returns stored feeds without consulting that revert flag.

Source: [MockFeedAdapter.sol](../../../contracts/meta-ag/mocks/MockFeedAdapter.sol).

### \_maxStaleness

```solidity
uint256 _maxStaleness
```

### \_supportedTokens

```solidity
address[] _supportedTokens
```

### \_feeds

```solidity
mapping(address => struct IDataFeedAdapter.FeedPrice) _feeds
```

### \_supports

```solidity
mapping(address => bool) _supports
```

### revertOnGet

```solidity
bool revertOnGet
```

### constructor

```solidity
constructor(uint256 maxStaleness_)
```

### sourceId

```solidity
function sourceId() external pure virtual returns (bytes32)
```

Identifier supplied by the concrete adapter's pure implementation.

#### Return Values

| Name | Type    | Description |
| ---- | ------- | ----------- |
| [0]  | bytes32 |             |

### adapterName

```solidity
function adapterName() external pure virtual returns (string)
```

Human-readable name for this adapter

#### Return Values

| Name | Type   | Description |
| ---- | ------ | ----------- |
| [0]  | string |             |

### setPrice

```solidity
function setPrice(address token, uint256 price, uint256 timestamp, uint256 confidence) external
```

### setSupported

```solidity
function setSupported(address token, bool supported) external
```

### setMaxStaleness

```solidity
function setMaxStaleness(uint256 s) external
```

### setRevertOnGet

```solidity
function setRevertOnGet(bool v) external
```

### getFeedPrice

```solidity
function getFeedPrice(address token) external view returns (struct IDataFeedAdapter.FeedPrice feed)
```

Get the latest price for a token from this adapter

Returns the stored feed unchanged. Reverts when `revertOnGet` is true.

#### Parameters

| Name  | Type    | Description                |
| ----- | ------- | -------------------------- |
| token | address | The token address to price |

#### Return Values

| Name | Type                              | Description                                                         |
| ---- | --------------------------------- | ------------------------------------------------------------------- |
| feed | struct IDataFeedAdapter.FeedPrice | The stored FeedPrice struct, or zero fields if no feed has been set |

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

### maxStaleness

```solidity
function maxStaleness() external view returns (uint256)
```

Returns the configured staleness threshold; consumers apply it to the stored timestamps

#### Return Values

| Name | Type    | Description |
| ---- | ------- | ----------- |
| [0]  | uint256 |             |

### getSupportedTokens

```solidity
function getSupportedTokens() external view returns (address[])
```

Get the recorded token list, including addresses whose support was later disabled

#### Return Values

| Name | Type      | Description |
| ---- | --------- | ----------- |
| [0]  | address[] |             |

### \_sourceIdInternal

```solidity
function _sourceIdInternal() internal pure virtual returns (bytes32)
```

## MockFeedAdapterA

Mock feed adapter keyed to `keccak256("MockFeedAdapter.A.v1")`, with name `"MockFeedAdapter.A.v1"`.

### constructor

```solidity
constructor(uint256 maxStaleness_) public
```

### sourceId

```solidity
function sourceId() external pure returns (bytes32)
```

Returns `keccak256("MockFeedAdapter.A.v1")`.

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

### \_sourceIdInternal

```solidity
function _sourceIdInternal() internal pure returns (bytes32)
```

## MockFeedAdapterB

Mock feed adapter keyed to `keccak256("MockFeedAdapter.B.v1")`, with name `"MockFeedAdapter.B.v1"`.

### constructor

```solidity
constructor(uint256 maxStaleness_) public
```

### sourceId

```solidity
function sourceId() external pure returns (bytes32)
```

Returns `keccak256("MockFeedAdapter.B.v1")`.

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

### \_sourceIdInternal

```solidity
function _sourceIdInternal() internal pure returns (bytes32)
```

## MockFeedAdapterC

Mock feed adapter keyed to `keccak256("MockFeedAdapter.C.v1")`, with name `"MockFeedAdapter.C.v1"`.

### constructor

```solidity
constructor(uint256 maxStaleness_) public
```

### sourceId

```solidity
function sourceId() external pure returns (bytes32)
```

Returns `keccak256("MockFeedAdapter.C.v1")`.

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

### \_sourceIdInternal

```solidity
function _sourceIdInternal() internal pure returns (bytes32)
```
