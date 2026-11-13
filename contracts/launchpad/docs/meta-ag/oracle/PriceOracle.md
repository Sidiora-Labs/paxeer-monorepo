# Solidity API

Source: [PriceOracle.sol](../../../contracts/meta-ag/oracle/PriceOracle.sol).

## PriceOracle

UUPS-upgradeable oracle with role-gated price publishing, per-token staleness limits, bounded price
history and time-weighted average prices. Prices use 18 decimals.

`RELAYER_ROLE` authorizes publishing. Administrators register tokens, configure bounds and relayers,
and set optional limits on price jumps. Administrators or `PAUSER_ROLE` holders can pause
publishing; only administrators can unpause. Reads remain available subject to their staleness
checks.

TWAP uses retained observations and requires a fresh latest price and history covering the requested
period. At most 1000 observations are retained per token; updates in the same block timestamp share
an observation.

### RELAYER_ROLE

```solidity
bytes32 RELAYER_ROLE
```

### PRICE_DECIMALS

```solidity
uint8 PRICE_DECIMALS
```

Price precision: 18 decimals.

### MAX_HISTORY_DEPTH

```solidity
uint256 MAX_HISTORY_DEPTH
```

### TWAPSnapshot

```solidity
struct TWAPSnapshot {
  uint256 cumulativePrice;
  uint256 timestamp;
}
```

### ZeroAddress

```solidity
error ZeroAddress()
```

### InvalidArrayLength

```solidity
error InvalidArrayLength()
```

### InvalidConfig

```solidity
error InvalidConfig()
```

### TokenAlreadyRegistered

```solidity
error TokenAlreadyRegistered()
```

### TokenNotConfigured

```solidity
error TokenNotConfigured()
```

### PriceOutOfBounds

```solidity
error PriceOutOfBounds()
```

### StalePrice

```solidity
error StalePrice()
```

### TwapWindowInvalid

```solidity
error TwapWindowInvalid()
```

### authorizedRelayers

```solidity
mapping(address => bool) authorizedRelayers
```

Relayer authorization mirror exposed as a public mapping.

Kept in sync with `RELAYER_ROLE` by role grants and revocations, including `setRelayer`.

### constructor

```solidity
constructor() public
```

### initialize

```solidity
function initialize(address admin) external
```

Initialize the UUPS proxy

#### Parameters

| Name  | Type    | Description                        |
| ----- | ------- | ---------------------------------- |
| admin | address | Account granted DEFAULT_ADMIN_ROLE |

### tokenRegistered

```solidity
modifier tokenRegistered(address token)
```

### \_authorizeUpgrade

```solidity
function _authorizeUpgrade(address) internal
```

### updatePrice

```solidity
function updatePrice(address token, uint256 price) external
```

Update the price of a single registered token. Requires `RELAYER_ROLE` and unpaused publishing. The
price must be nonzero, within configured bounds and within the optional `maxPriceJumpBps` limit.

#### Parameters

| Name  | Type    | Description                 |
| ----- | ------- | --------------------------- |
| token | address | The token address           |
| price | uint256 | The new price (18 decimals) |

### batchUpdatePrices

```solidity
function batchUpdatePrices(address[] tokens, uint256[] prices) external
```

Update prices for multiple tokens in a single transaction

Requires nonempty arrays of equal length. Unregistered tokens are skipped; an invalid price for a
registered token reverts the transaction. The emitted count is the number of applied updates.

#### Parameters

| Name   | Type      | Description                       |
| ------ | --------- | --------------------------------- |
| tokens | address[] | Array of token addresses          |
| prices | uint256[] | Array of new prices (18 decimals) |

### getPrice

```solidity
function getPrice(address token) external view returns (uint256 price)
```

Get the latest price for a token

_REVERTS if price is stale (older than maxStaleness for that token)_

#### Parameters

| Name  | Type    | Description                            |
| ----- | ------- | -------------------------------------- |
| token | address | The token address to get the price for |

#### Return Values

| Name  | Type    | Description                             |
| ----- | ------- | --------------------------------------- |
| price | uint256 | The token price in USD with 18 decimals |

### getPriceWithTimestamp

```solidity
function getPriceWithTimestamp(address token) external view returns (uint256 price, uint256 timestamp, uint256 roundId)
```

Get the latest price with full metadata

_REVERTS if price is stale_

#### Parameters

| Name  | Type    | Description       |
| ----- | ------- | ----------------- |
| token | address | The token address |

#### Return Values

| Name      | Type    | Description                           |
| --------- | ------- | ------------------------------------- |
| price     | uint256 | The token price (18 decimals)         |
| timestamp | uint256 | When the price was last updated       |
| roundId   | uint256 | The round number of this price update |

### getTWAP

```solidity
function getTWAP(address token, uint256 period) external view returns (uint256 twapPrice)
```

Get the Time-Weighted Average Price over a period

_Uses on-chain price accumulator. Reverts if insufficient history._

#### Parameters

| Name   | Type    | Description                                              |
| ------ | ------- | -------------------------------------------------------- |
| token  | address | The token address                                        |
| period | uint256 | The TWAP period in seconds (e.g., 300 for 5-minute TWAP) |

#### Return Values

| Name      | Type    | Description               |
| --------- | ------- | ------------------------- |
| twapPrice | uint256 | The TWAP with 18 decimals |

### isPriceStale

```solidity
function isPriceStale(address token) external view returns (bool stale)
```

Check if a token's price is stale

#### Parameters

| Name  | Type    | Description       |
| ----- | ------- | ----------------- |
| token | address | The token address |

#### Return Values

| Name  | Type | Description                                  |
| ----- | ---- | -------------------------------------------- |
| stale | bool | True if the price is older than maxStaleness |

### getPrices

```solidity
function getPrices(address[] tokens) external view returns (uint256[] prices, uint256[] timestamps, bool[] staleFlags)
```

Get prices for multiple tokens in a single call

_Does NOT revert on stale prices — returns staleness flags instead_

#### Parameters

| Name   | Type      | Description              |
| ------ | --------- | ------------------------ |
| tokens | address[] | Array of token addresses |

#### Return Values

| Name       | Type      | Description                                        |
| ---------- | --------- | -------------------------------------------------- |
| prices     | uint256[] | Array of prices (18 decimals), 0 if not registered |
| timestamps | uint256[] | Array of last update timestamps                    |
| staleFlags | bool[]    | Array of staleness booleans                        |

### getPriceHistory

```solidity
function getPriceHistory(address token, uint256 count) external view returns (struct IPriceOracle.PriceData[] history)
```

Get recent price history for a token

#### Parameters

| Name  | Type    | Description                         |
| ----- | ------- | ----------------------------------- |
| token | address | The token address                   |
| count | uint256 | Number of recent rounds to retrieve |

#### Return Values

| Name    | Type                            | Description                                    |
| ------- | ------------------------------- | ---------------------------------------------- |
| history | struct IPriceOracle.PriceData[] | Array of PriceData structs (most recent first) |

### getLatestRound

```solidity
function getLatestRound(address token) external view returns (struct IPriceOracle.PriceData data)
```

Get the latest round data for a token

#### Parameters

| Name  | Type    | Description       |
| ----- | ------- | ----------------- |
| token | address | The token address |

#### Return Values

| Name | Type                          | Description                 |
| ---- | ----------------------------- | --------------------------- |
| data | struct IPriceOracle.PriceData | The latest PriceData struct |

### getRoundData

```solidity
function getRoundData(address token, uint256 roundId) external view returns (struct IPriceOracle.PriceData data)
```

Get the price at a specific round

#### Parameters

| Name    | Type    | Description               |
| ------- | ------- | ------------------------- |
| token   | address | The token address         |
| roundId | uint256 | The round number to query |

#### Return Values

| Name | Type                          | Description                  |
| ---- | ----------------------------- | ---------------------------- |
| data | struct IPriceOracle.PriceData | The PriceData for that round |

### getTokenConfig

```solidity
function getTokenConfig(address token) external view returns (struct IPriceOracle.TokenConfig config)
```

Get the configuration for a token

#### Parameters

| Name  | Type    | Description       |
| ----- | ------- | ----------------- |
| token | address | The token address |

#### Return Values

| Name   | Type                            | Description            |
| ------ | ------------------------------- | ---------------------- |
| config | struct IPriceOracle.TokenConfig | The TokenConfig struct |

### getRegisteredTokens

```solidity
function getRegisteredTokens() external view returns (address[] tokens)
```

Get all registered token addresses

#### Return Values

| Name   | Type      | Description                         |
| ------ | --------- | ----------------------------------- |
| tokens | address[] | Array of registered token addresses |

### isAuthorizedRelayer

```solidity
function isAuthorizedRelayer(address relayer) external view returns (bool authorized)
```

Returns membership in `RELAYER_ROLE`.

#### Parameters

| Name    | Type    | Description          |
| ------- | ------- | -------------------- |
| relayer | address | The address to check |

#### Return Values

| Name       | Type | Description                                      |
| ---------- | ---- | ------------------------------------------------ |
| authorized | bool | True if the relayer is authorized to push prices |

### getCurrentRound

```solidity
function getCurrentRound(address token) external view returns (uint256)
```

### registerToken

```solidity
function registerToken(address token, uint256 heartbeatInterval, uint256 deviationThresholdBps, uint256 minPrice, uint256 maxPrice, uint256 maxStaleness) external
```

Register a new token with its oracle configuration

#### Parameters

| Name                  | Type    | Description                                                                   |
| --------------------- | ------- | ----------------------------------------------------------------------------- |
| token                 | address | The token address to register                                                 |
| heartbeatInterval     | uint256 | Heartbeat interval in seconds, reported by status and confidence calculations |
| deviationThresholdBps | uint256 | Stored deviation setting (0–10000 BPS)                                        |
| minPrice              | uint256 | Lower price bound (18 decimals)                                               |
| maxPrice              | uint256 | Upper price bound (18 decimals)                                               |
| maxStaleness          | uint256 | Seconds after which price is stale                                            |

### updateTokenConfig

```solidity
function updateTokenConfig(address token, struct IPriceOracle.TokenConfig config) external
```

Update the configuration of an existing token

#### Parameters

| Name   | Type                            | Description         |
| ------ | ------------------------------- | ------------------- |
| token  | address                         | The token address   |
| config | struct IPriceOracle.TokenConfig | The new TokenConfig |

### setRelayer

```solidity
function setRelayer(address relayer, bool authorized) external
```

Set the authorization status of a relayer

#### Parameters

| Name       | Type    | Description                        |
| ---------- | ------- | ---------------------------------- |
| relayer    | address | The relayer address                |
| authorized | bool    | True to authorize, false to revoke |

### pause

```solidity
function pause() external
```

Emergency pause; blocks price pushes.

### unpause

```solidity
function unpause() external
```

Resume price pushes after pause.

### \_updatePrice

```solidity
function _updatePrice(address token, uint256 price) internal
```

### \_updateTWAPAccumulator

```solidity
function _updateTWAPAccumulator(address token) internal
```

### \_takeTWAPSnapshot

```solidity
function _takeTWAPSnapshot(address token) internal
```

### MAX_TOKEN_PAGE_SIZE

```solidity
uint256 MAX_TOKEN_PAGE_SIZE
```

Maximum registered-token page size: 100.

### registeredTokenCount

```solidity
function registeredTokenCount() external view returns (uint256)
```

### getRegisteredTokensPage

```solidity
function getRegisteredTokensPage(uint256 cursor, uint256 limit) external view returns (address[] tokens, uint256 nextCursor)
```

Returns a page of registered tokens, capped at 100 entries.

### PriceStatus

```solidity
struct PriceStatus {
  bool registered;
  bool publishingPaused;
  bool fresh;
  bool heartbeatOverdue;
  bool twapReady;
  uint256 price;
  uint256 timestamp;
  uint256 roundId;
  uint256 oldestObservationTimestamp;
  uint256 retainedObservationCount;
}
```

### getPriceStatus

```solidity
function getPriceStatus(address token, uint256 period) external view returns (struct PriceOracle.PriceStatus status)
```

Reports registration, publishing pause, freshness, heartbeat and TWAP readiness for the requested
period, together with the latest price and retained observation metadata.

### maxPriceJumpBps

```solidity
mapping(address => uint256) maxPriceJumpBps
```

Maximum permitted change from the previous price; zero disables the jump limit.

### setMaxPriceJumpBps

```solidity
function setMaxPriceJumpBps(address token, uint256 limitBps) external
```

Administrator-only setter for a registered token. Accepts 0–10000 BPS.

### PriceJumpExceeded

```solidity
error PriceJumpExceeded(address token, uint256 previousPrice, uint256 proposedPrice, uint256 limitBps)
```

### MaxPriceJumpUpdated

```solidity
event MaxPriceJumpUpdated(address indexed token, uint256 previousLimitBps, uint256 limitBps)
```

