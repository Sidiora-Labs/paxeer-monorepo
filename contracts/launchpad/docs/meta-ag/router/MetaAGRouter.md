# Solidity API

Source: [MetaAGRouter.sol](../../../contracts/meta-ag/router/MetaAGRouter.sol).

## MetaAGRouter

UUPS-upgradeable router that selects the highest-output quote from active `IProtocolAdapter`
contracts, supports explicit adapter selection and executes paths of two to five hops.

Swaps enforce nonzero deadlines, slippage and exact input transfers. Output amounts are measured from
recipient balance changes. Adapter allowances are reset to zero before approval and after execution.
Each hop is quoted using the actual intermediate output.

The optional oracle sanity check accounts for token decimals and bounds deviation from the
oracle-implied output. It returns without checking when either price is reported unavailable.
Administrators manage adapters, configuration and upgrades. Administrators or `PAUSER_ROLE` holders
can pause swaps; only administrators can unpause.

### BPS_DENOMINATOR

```solidity
uint256 BPS_DENOMINATOR
```

### MAX_ADAPTERS

```solidity
uint256 MAX_ADAPTERS
```

### MAX_HOPS

```solidity
uint256 MAX_HOPS
```

### ZeroAddress

```solidity
error ZeroAddress()
```

### ZeroAmount

```solidity
error ZeroAmount()
```

### SameToken

```solidity
error SameToken()
```

### DeadlineExpired

```solidity
error DeadlineExpired()
```

### InvalidBps

```solidity
error InvalidBps()
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

### AdapterInactive

```solidity
error AdapterInactive()
```

### NoAdaptersAvailable

```solidity
error NoAdaptersAvailable()
```

### BestQuoteUnavailable

```solidity
error BestQuoteUnavailable()
```

### QuoteUnavailable

```solidity
error QuoteUnavailable()
```

### SlippageTooHigh

```solidity
error SlippageTooHigh()
```

### MaxHopsExceeded

```solidity
error MaxHopsExceeded()
```

### TooFewHops

```solidity
error TooFewHops()
```

### OracleSanityFailed

```solidity
error OracleSanityFailed()
```

### oracleHub

```solidity
address oracleHub
```

OracleHub consulted by the sanity check.

The `IMetaAGRouter.oracleHub()` getter returns `address`; the sanity check calls it as `IOracleHub`.

### maxOracleSanityDeviation

```solidity
uint256 maxOracleSanityDeviation
```

Max deviation (BPS) before `_oracleSanityCheck` reverts.

### oracleSanityEnabled

```solidity
bool oracleSanityEnabled
```

Master toggle for the oracle sanity check.

### ensure

```solidity
modifier ensure(uint256 deadline)
```

Reverts with `DeadlineExpired` when a nonzero deadline is earlier than the current block timestamp.

### constructor

```solidity
constructor() public
```

### initialize

```solidity
function initialize(address oracleHub_, uint256 maxSanityDeviation, address admin) external
```

Initialize the UUPS proxy

#### Parameters

| Name               | Type    | Description                                |
| ------------------ | ------- | ------------------------------------------ |
| oracleHub\_        | address |                                            |
| maxSanityDeviation | uint256 | Maximum allowed deviation in BPS (0–10000) |
| admin              | address | Account granted DEFAULT_ADMIN_ROLE         |

### \_authorizeUpgrade

```solidity
function _authorizeUpgrade(address) internal
```

### pause

```solidity
function pause() external
```

### unpause

```solidity
function unpause() external
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

### registerAdapter

```solidity
function registerAdapter(address adapter) external
```

Register a new adapter (reads adapter.adapterId() for collision check). Caller must have
DEFAULT_ADMIN_ROLE.

### deactivateAdapter

```solidity
function deactivateAdapter(bytes32 adapterId_) external
```

### activateAdapter

```solidity
function activateAdapter(bytes32 adapterId_) external
```

### getBestQuote

```solidity
function getBestQuote(address tokenIn, address tokenOut, uint256 amountIn) public view returns (struct IMetaAGRouter.BestQuote best)
```

Best quote across all active adapters (no side effects).

### getAllQuotes

```solidity
function getAllQuotes(address tokenIn, address tokenOut, uint256 amountIn) external view returns (struct IProtocolAdapter.QuoteResult[] quotes, bytes32[] adapterIds, string[] names)
```

Quotes and identifiers in registry order as parallel arrays. Inactive adapters have zero quotes;
failed quote reads also return zero quotes.

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
function swapViaAdapter(bytes32 adapterId_, address tokenIn, address tokenOut, uint256 amountIn, uint256 amountOutMin, uint256 deadline) external returns (uint256 amountOut)
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
function getAdapter(bytes32 adapterId_) external view returns (struct IMetaAGRouter.AdapterEntry)
```

### adapterCount

```solidity
function adapterCount() external view returns (uint256)
```

### isAdapterActive

```solidity
function isAdapterActive(bytes32 adapterId_) external view returns (bool)
```

### \_executeAdapterSwap

```solidity
function _executeAdapterSwap(address adapter, address tokenIn, address tokenOut, uint256 amountIn, uint256 amountOutMin, address payer, address recipient, uint256 deadline, bytes adapterData) internal returns (uint256 amountOut)
```

Executes one adapter swap with an exact input transfer and measured recipient output.

_Transfers `amountIn` of `tokenIn` from `payer` into this router, approves the adapter with
zero-first reset, executes, then resets approval back to zero. Slippage is enforced against
`amountOutMin`._

### \_oracleSanityCheck

```solidity
function _oracleSanityCheck(address tokenIn, address tokenOut, uint256 amountIn, uint256 amountOut) internal view
```

Reverts if the realized `amountOut` deviates from oracle-implied expectation by more than
`maxOracleSanityDeviation` bps.

Normalizes both token amounts by their decimals. Reverts for decimals above 36 or deviations over
the configured limit. Returns when either price is unavailable or zero, or expected output is zero.

### \_findAdapterIndex

```solidity
function _findAdapterIndex(bytes32 adapterId_) internal view returns (uint256)
```

_Finds the index of an adapter by ID. Reverts `AdapterNotFound` if no entry matches — cheaper than a
separate existence map because the list is capped at `MAX_ADAPTERS = 20`._

### removeAdapter

```solidity
function removeAdapter(bytes32 id) external
```

Administrator-only removal. The adapter must be deactivated first.

### InvalidPath

```solidity
error InvalidPath()
```

### TransferAmountMismatch

```solidity
error TransferAmountMismatch()
```

### AdapterRemoved

```solidity
event AdapterRemoved(bytes32 indexed adapterId, address adapter)
```

### \_safeQuote

```solidity
function _safeQuote(address adapter, address tokenIn, address tokenOut, uint256 amountIn) internal view returns (struct IProtocolAdapter.QuoteResult result)
```

Uses a 500000 gas limit for the adapter quote and returns an empty quote on failure.
