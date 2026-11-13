# Solidity API

## IMetaAGRouter

UUPS router that selects among registered IProtocolAdapter contracts or executes through a
user-selected adapter. Users approve this router for input tokens. The router pulls the exact input,
approves the selected adapter, measures the received output and resets its adapter allowance to
zero. Swaps enforce minimum output amounts and accept a zero deadline as no expiry.

When oracle sanity checking is enabled, the router compares received output with a
decimal-normalized expectation from the hub’s input and output USD prices. If either price is
unavailable or zero, or the computed expectation rounds to zero, the comparison is skipped. Hub
query errors propagate. If a valid comparison exceeds `maxOracleSanityDeviation`, the transaction
reverts.

Multi-hop swaps require two to five connected hops and query each adapter with the actual
intermediate amount. Each hop and the final output have minimum amount checks. `DEFAULT_ADMIN_ROLE`
controls adapters, oracle configuration, upgrades and unpausing; default administration or
`PAUSER_ROLE` can pause.

Source: [IMetaAGRouter.sol](../../../contracts/meta-ag/interfaces/IMetaAGRouter.sol).

### AdapterEntry

```solidity
struct AdapterEntry {
  bytes32 adapterId;
  address adapter;
  bool active;
  string name;
}
```

### BestQuote

```solidity
struct BestQuote {
  uint256 amountOut;
  uint256 priceImpactBps;
  uint256 feeBps;
  uint256 feeAmount;
  bytes32 adapterId;
  address adapter;
  bytes adapterData;
  bool found;
}
```

### HopParams

```solidity
struct HopParams {
  address tokenIn;
  address tokenOut;
  bytes32 adapterId;
  uint256 minAmountOut;
}
```

### initialize

```solidity
function initialize(address oracleHub, uint256 maxSanityDeviation, address admin) external
```

Initialize the UUPS proxy

#### Parameters

| Name               | Type    | Description                                        |
| ------------------ | ------- | -------------------------------------------------- |
| oracleHub          | address | IOracleHub address used for output sanity checks   |
| maxSanityDeviation | uint256 | Maximum output deviation in basis points (0–10000) |
| admin              | address | Timelock — granted DEFAULT_ADMIN_ROLE              |

### registerAdapter

```solidity
function registerAdapter(address adapter) external
```

Register a new adapter (reads adapter.adapterId() for collision check). Caller must have
DEFAULT_ADMIN_ROLE (Timelock).

### deactivateAdapter

```solidity
function deactivateAdapter(bytes32 adapterId) external
```

### activateAdapter

```solidity
function activateAdapter(bytes32 adapterId) external
```

### setOracleHub

```solidity
function setOracleHub(address hub) external
```

### setOracleSanityDeviation

```solidity
function setOracleSanityDeviation(uint256 bps) external
```

### setOracleSanityEnabled

```solidity
function setOracleSanityEnabled(bool enabled) external
```

### pause

```solidity
function pause() external
```

### unpause

```solidity
function unpause() external
```

### getBestQuote

```solidity
function getBestQuote(address tokenIn, address tokenOut, uint256 amountIn) external view returns (struct IMetaAGRouter.BestQuote best)
```

Best quote across all active adapters (no side effects).

### getAllQuotes

```solidity
function getAllQuotes(address tokenIn, address tokenOut, uint256 amountIn) external view returns (struct IProtocolAdapter.QuoteResult[] quotes, bytes32[] adapterIds, string[] names)
```

Every adapter's quote for a pair. Parallel arrays.

#### Return Values

| Name       | Type                                  | Description                           |
| ---------- | ------------------------------------- | ------------------------------------- |
| quotes     | struct IProtocolAdapter.QuoteResult[] | Array of IProtocolAdapter.QuoteResult |
| adapterIds | bytes32[]                             | Array of adapter IDs                  |
| names      | string[]                              | Array of adapter names                |

### swapBestRoute

```solidity
function swapBestRoute(address tokenIn, address tokenOut, uint256 amountIn, uint256 amountOutMin, uint256 deadline) external returns (uint256 amountOut)
```

Swap using the best available adapter (highest amountOut).

### swapViaAdapter

```solidity
function swapViaAdapter(bytes32 adapterId, address tokenIn, address tokenOut, uint256 amountIn, uint256 amountOutMin, uint256 deadline) external returns (uint256 amountOut)
```

Swap via a specific adapter (user-specified, bypasses best-route selection).

### swapMultiHop

```solidity
function swapMultiHop(struct IMetaAGRouter.HopParams[] hops, uint256 amountIn, uint256 amountOutMin, uint256 deadline) external returns (uint256 amountOut)
```

Execute a multi-hop swap across multiple adapters. Re-queries each hop.

### getAdapters

```solidity
function getAdapters() external view returns (struct IMetaAGRouter.AdapterEntry[])
```

### getAdapter

```solidity
function getAdapter(bytes32 adapterId) external view returns (struct IMetaAGRouter.AdapterEntry)
```

### adapterCount

```solidity
function adapterCount() external view returns (uint256)
```

### isAdapterActive

```solidity
function isAdapterActive(bytes32 adapterId) external view returns (bool)
```

### oracleHub

```solidity
function oracleHub() external view returns (address)
```

### maxOracleSanityDeviation

```solidity
function maxOracleSanityDeviation() external view returns (uint256)
```

### oracleSanityEnabled

```solidity
function oracleSanityEnabled() external view returns (bool)
```

### AdapterRegistered

```solidity
event AdapterRegistered(bytes32 indexed adapterId, address indexed adapter, string name)
```

### AdapterDeactivated

```solidity
event AdapterDeactivated(bytes32 indexed adapterId)
```

### AdapterActivated

```solidity
event AdapterActivated(bytes32 indexed adapterId)
```

### OracleHubUpdated

```solidity
event OracleHubUpdated(address indexed hub)
```

### OracleSanityDeviationUpdated

```solidity
event OracleSanityDeviationUpdated(uint256 bps)
```

### OracleSanityEnabledUpdated

```solidity
event OracleSanityEnabledUpdated(bool enabled)
```

### BestRouteSwap

```solidity
event BestRouteSwap(address indexed user, address indexed tokenIn, address indexed tokenOut, uint256 amountIn, uint256 amountOut, bytes32 adapterId)
```

### MultiHopSwap

```solidity
event MultiHopSwap(address indexed user, address indexed tokenIn, address indexed tokenOut, uint256 amountIn, uint256 amountOut, uint256 hops)
```

### OracleSanityCheckFailed

```solidity
event OracleSanityCheckFailed(address indexed tokenOut, uint256 expectedPrice, uint256 actualAmountOut, uint256 deviationBps)
```

### removeAdapter

```solidity
function removeAdapter(bytes32 adapterId) external
```

Requires default administration and an inactive adapter. Removes its ID and address registration,
reclaiming adapter capacity.
