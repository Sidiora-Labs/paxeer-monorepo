# Solidity API

## SidioraAdapter

Connects MetaAGRouter to the Sidiora Router, Quoter and PoolRegistry for USDL-to-token,
token-to-USDL and token-to-token swaps.

The caller approves this adapter and must equal `from`. The adapter pulls the exact input, approves
the Sidiora Router, executes `buy`, `sell` or `swapTokenForToken`, resets the Router allowance to
zero, and forwards the received output to `recipient`. Input transfers that change the expected
amount revert with `TransferAmountMismatch`. Pools with a nonzero optical are excluded from quotes
and execution. A zero deadline means no expiry.

`DEFAULT_ADMIN_ROLE` controls configuration. The adapter ID is `keccak256("SidioraAMM.v1")`, its
name is `"SidioraAMM.v1"`, and its version is `"1.0.0"`.

Source: [SidioraAdapter.sol](../../../contracts/meta-ag/adapters/SidioraAdapter.sol).

### ZeroAddress

```solidity
error ZeroAddress()
```

### ZeroAmount

```solidity
error ZeroAmount()
```

### Expired

```solidity
error Expired()
```

### PairNotSupported

```solidity
error PairNotSupported()
```

### PoolNotFound

```solidity
error PoolNotFound()
```

### SlippageExceeded

```solidity
error SlippageExceeded()
```

### poolRegistry

```solidity
contract IPoolRegistry poolRegistry
```

### quoter

```solidity
contract IQuoter quoter
```

### sidioraRouter

```solidity
contract IRouter sidioraRouter
```

### usdl

```solidity
address usdl
```

### constructor

```solidity
constructor(address poolRegistry_, address quoter_, address sidioraRouter_, address usdl_, address admin_) public
```

### setPoolRegistry

```solidity
function setPoolRegistry(address registry) external
```

### setQuoter

```solidity
function setQuoter(address quoter_) external
```

### setSidioraRouter

```solidity
function setSidioraRouter(address router) external
```

### setUsdl

```solidity
function setUsdl(address usdl_) external
```

### adapterId

```solidity
function adapterId() external pure returns (bytes32)
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

### adapterVersion

```solidity
function adapterVersion() external pure returns (string)
```

Protocol version string

#### Return Values

| Name | Type   | Description |
| ---- | ------ | ----------- |
| [0]  | string |             |

### supportsSwap

```solidity
function supportsSwap(address tokenIn, address tokenOut) public view returns (bool)
```

Check if this adapter supports a specific token pair

#### Parameters

| Name     | Type    | Description              |
| -------- | ------- | ------------------------ |
| tokenIn  | address | The input token address  |
| tokenOut | address | The output token address |

#### Return Values

| Name | Type | Description |
| ---- | ---- | ----------- |
| [0]  | bool |             |

### getSupportedPairs

```solidity
function getSupportedPairs() external pure returns (address[] tokenIns, address[] tokenOuts)
```

Get all token pairs this adapter currently supports

_Sidiora pools are dynamically discovered via the registry; returning an empty list is the cheapest
and forwards discovery to supportsSwap._

#### Return Values

| Name      | Type      | Description                                   |
| --------- | --------- | --------------------------------------------- |
| tokenIns  | address[] | Array of input token addresses                |
| tokenOuts | address[] | Array of corresponding output token addresses |

### getMaxInput

```solidity
function getMaxInput(address tokenIn, address tokenOut) external view returns (uint256)
```

Get the maximum input amount this adapter can handle for a pair

For a supported USDL input, returns `type(uint256).max`. For a token input, returns the conservative
sell bound `realUsdl * tokenReserve / virtualUsdl`. Returns zero when the pair is unsupported,
reserves cannot be read, or real USDL liquidity is zero; a zero virtual reserve returns
`type(uint256).max`.

#### Parameters

| Name     | Type    | Description      |
| -------- | ------- | ---------------- |
| tokenIn  | address | The input token  |
| tokenOut | address | The output token |

#### Return Values

| Name | Type    | Description |
| ---- | ------- | ----------- |
| [0]  | uint256 |             |

### getQuote

```solidity
function getQuote(address tokenIn, address tokenOut, uint256 amountIn) external view returns (struct IProtocolAdapter.QuoteResult result)
```

Get a quote for swapping amountIn of tokenIn for tokenOut

_Invariant I1 — never reverts. Returns `available=false` on any issue._

#### Parameters

| Name     | Type    | Description              |
| -------- | ------- | ------------------------ |
| tokenIn  | address | The input token address  |
| tokenOut | address | The output token address |
| amountIn | uint256 | The exact input amount   |

#### Return Values

| Name   | Type                                | Description                                                     |
| ------ | ----------------------------------- | --------------------------------------------------------------- |
| result | struct IProtocolAdapter.QuoteResult | The QuoteResult with amountOut, impact, fee, and execution data |

### executeSwap

```solidity
function executeSwap(address tokenIn, address tokenOut, uint256 amountIn, uint256 minAmountOut, address from, address recipient, uint256 deadline, bytes adapterData) external returns (struct IProtocolAdapter.SwapResult result)
```

Execute a swap through this adapter

`from` must equal the caller and approve this adapter for at least `amountIn`. `adapterData` encodes
`(uint8 mode, address poolA, address poolB)` from the quote; execution validates the mode and
resolves pools through the registry. Input receipt must match `amountIn`, the input balance must
return to its starting value, and the recipient's received output must meet `minAmountOut`.

#### Parameters

| Name         | Type    | Description                                                 |
| ------------ | ------- | ----------------------------------------------------------- |
| tokenIn      | address | The input token address                                     |
| tokenOut     | address | The output token address                                    |
| amountIn     | uint256 | The exact input amount                                      |
| minAmountOut | uint256 | Minimum acceptable output (slippage guard)                  |
| from         | address | Payer; must equal msg.sender and have approved this adapter |
| recipient    | address | Address to send tokenOut to                                 |
| deadline     | uint256 | Unix timestamp after which the swap reverts                 |
| adapterData  | bytes   | Encoded execution data from getQuote() (e.g., pool address) |

#### Return Values

| Name   | Type                               | Description                                  |
| ------ | ---------------------------------- | -------------------------------------------- |
| result | struct IProtocolAdapter.SwapResult | The SwapResult with actual amountOut and fee |

### \_hasPool

```solidity
function _hasPool(address token) internal view returns (bool)
```

### \_getPool

```solidity
function _getPool(address token) internal view returns (address pool)
```

### UnauthorizedPayer

```solidity
error UnauthorizedPayer()
```

### TransferAmountMismatch

```solidity
error TransferAmountMismatch()
```

### ConfigurationUpdated

```solidity
event ConfigurationUpdated(bytes32 indexed field, address oldValue, address newValue)
```

### quoteChecked

```solidity
function quoteChecked(address tokenIn, address tokenOut, uint256 amountIn) external view returns (QuoteResult memory result)
```

Builds a quote directly and may revert on dependency errors. getQuote catches such errors and
returns an unavailable result.

### \_hasRealLiquidity

```solidity
function _hasRealLiquidity(address pool, uint256 output) internal view returns (bool)
```
