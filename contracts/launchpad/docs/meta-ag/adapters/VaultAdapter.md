# Solidity API

## VaultAdapter

Exposes PECORVault inventory to MetaAGRouter using PriceOracle prices. The vault must grant this
adapter `OPERATOR_ROLE` before it can send output.

The caller approves this adapter and must equal `from`. The adapter pulls the exact input, approves
the vault, deposits it, resets the allowance, and sends output through `vault.pushTokens`. Both
input receipt amounts must equal `amountIn`; the recipient's received output must meet
`minAmountOut`. A zero deadline means no expiry.

`DEFAULT_ADMIN_ROLE` controls the oracle, fee rate and fee collector. Fees are capped at 200 basis
points and paid in the output token. The adapter ID is `keccak256("PECORVault.v1")`, its name is
`"PECORVault.v1"`, and its version is `"1.0.0"`.

Source: [VaultAdapter.sol](../../../contracts/meta-ag/adapters/VaultAdapter.sol).

### MAX_FEE_BPS

```solidity
uint256 MAX_FEE_BPS
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

### SameToken

```solidity
error SameToken()
```

### Expired

```solidity
error Expired()
```

### FeeTooHigh

```solidity
error FeeTooHigh()
```

### SlippageExceeded

```solidity
error SlippageExceeded()
```

### InsufficientLiquidity

```solidity
error InsufficientLiquidity()
```

### vault

```solidity
contract IPECORVault vault
```

### priceOracle

```solidity
contract IPriceOracle priceOracle
```

### feeBps

```solidity
uint256 feeBps
```

### feeCollector

```solidity
address feeCollector
```

### constructor

```solidity
constructor(address vault_, address priceOracle_, uint256 feeBps_, address feeCollector_, address admin_) public
```

### setFee

```solidity
function setFee(uint256 feeBps_) external
```

### setFeeCollector

```solidity
function setFeeCollector(address collector) external
```

### setPriceOracle

```solidity
function setPriceOracle(address oracle) external
```

### adapterId

```solidity
function adapterId() external pure returns (bytes32)
```

Returns `keccak256("PECORVault.v1")`.

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
function supportsSwap(address tokenIn, address tokenOut) external view returns (bool)
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
function getSupportedPairs() external view returns (address[] tokenIns, address[] tokenOuts)
```

Get all token pairs this adapter currently supports

Returns every ordered pair of distinct registered vault tokens. Fewer than two tokens returns empty
arrays.

#### Return Values

| Name      | Type      | Description                                   |
| --------- | --------- | --------------------------------------------- |
| tokenIns  | address[] | Array of input token addresses                |
| tokenOuts | address[] | Array of corresponding output token addresses |

### getMaxInput

```solidity
function getMaxInput(address tokenIn, address tokenOut) external view returns (uint256 maxIn)
```

Get the maximum input amount this adapter can handle for a pair

Converts available output reserves to an input amount using oracle prices and token decimals,
rounding up. Returns zero for identical tokens, zero available reserves or unavailable prices.

#### Parameters

| Name     | Type    | Description      |
| -------- | ------- | ---------------- |
| tokenIn  | address | The input token  |
| tokenOut | address | The output token |

#### Return Values

| Name  | Type    | Description                             |
| ----- | ------- | --------------------------------------- |
| maxIn | uint256 | Maximum amountIn (limited by liquidity) |

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
function executeSwap(address tokenIn, address tokenOut, uint256 amountIn, uint256 minAmountOut, address from, address recipient, uint256 deadline, bytes) external returns (struct IProtocolAdapter.SwapResult result)
```

Execute a swap through this adapter

`from` must equal the caller and approve this adapter for at least `amountIn`. The adapter approves
the vault for its subsequent deposit. The final bytes argument is unused.

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
| (unnamed)    | bytes   | Unused adapter data                                         |

#### Return Values

| Name   | Type                               | Description                                  |
| ------ | ---------------------------------- | -------------------------------------------- |
| result | struct IProtocolAdapter.SwapResult | The SwapResult with actual amountOut and fee |

### \_safeGetPrice

```solidity
function _safeGetPrice(address token) internal view returns (uint256)
```

### \_calcAmountOut

```solidity
function _calcAmountOut(uint256 amountIn, uint256 priceIn, uint256 priceOut, uint8 dIn, uint8 dOut) internal pure returns (uint256)
```

### \_calcAmountIn

```solidity
function _calcAmountIn(uint256 amountOut, uint256 priceIn, uint256 priceOut, uint8 dIn, uint8 dOut) internal pure returns (uint256)
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

### FeeUpdated

```solidity
event FeeUpdated(uint256 oldValue, uint256 newValue)
```

### quoteChecked

```solidity
function quoteChecked(address tokenIn, address tokenOut, uint256 amountIn) external view returns (QuoteResult memory result)
```

Builds a quote directly and may revert on dependency errors. getQuote catches such errors and
returns an unavailable result.
