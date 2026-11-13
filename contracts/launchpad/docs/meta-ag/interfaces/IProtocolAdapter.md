# Solidity API

## IProtocolAdapter

Swap adapter interface consumed by MetaAGRouter. The included
[SidioraAdapter](../adapters/SidioraAdapter.md) routes through launchpad pools;
[VaultAdapter](../adapters/VaultAdapter.md) uses oracle-priced vault liquidity.

Adapters report supported pairs, quote amounts and execution data. `getQuote` represents unsupported
pairs or unavailable liquidity with `available == false`. Execution enforces the minimum output and
deadline, pulls from the payer and sends output to the recipient. The included adapters require the
payer to equal the caller and treat a zero deadline as no expiry.

`adapterId` identifies an adapter, and the router rejects ID collisions at registration. During
execution the router passes quote-provided `adapterData` back to the selected adapter.

Source: [IProtocolAdapter.sol](../../../contracts/meta-ag/interfaces/IProtocolAdapter.sol).

### QuoteResult

```solidity
struct QuoteResult {
  uint256 amountOut;
  uint256 priceImpactBps;
  uint256 feeBps;
  uint256 feeAmount;
  bool available;
  bytes adapterData;
}
```

### SwapResult

```solidity
struct SwapResult {
  uint256 amountOut;
  uint256 feeAmount;
  bytes adapterData;
}
```

### getQuote

```solidity
function getQuote(address tokenIn, address tokenOut, uint256 amountIn) external view returns (struct IProtocolAdapter.QuoteResult result)
```

Get a quote for swapping amountIn of tokenIn for tokenOut

_MUST NOT revert — return available=false if unsupported or no liquidity_

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

The included SidioraAdapter and VaultAdapter require `from == msg.sender` and approval of tokenIn to
the adapter. They pull input from `from` and send measured output to `recipient`. Both accept a zero
deadline as no expiry.

#### Parameters

| Name         | Type    | Description                                                 |
| ------------ | ------- | ----------------------------------------------------------- |
| tokenIn      | address | The input token address                                     |
| tokenOut     | address | The output token address                                    |
| amountIn     | uint256 | The exact input amount                                      |
| minAmountOut | uint256 | Minimum acceptable output (slippage guard)                  |
| from         | address | Payer; must equal msg.sender and have approved the adapter  |
| recipient    | address | Address to send tokenOut to                                 |
| deadline     | uint256 | Unix timestamp after which the swap reverts                 |
| adapterData  | bytes   | Encoded execution data from getQuote() (e.g., pool address) |

#### Return Values

| Name   | Type                               | Description                                  |
| ------ | ---------------------------------- | -------------------------------------------- |
| result | struct IProtocolAdapter.SwapResult | The SwapResult with actual amountOut and fee |

### supportsSwap

```solidity
function supportsSwap(address tokenIn, address tokenOut) external view returns (bool supported)
```

Check if this adapter supports a specific token pair

#### Parameters

| Name     | Type    | Description              |
| -------- | ------- | ------------------------ |
| tokenIn  | address | The input token address  |
| tokenOut | address | The output token address |

#### Return Values

| Name      | Type | Description                                         |
| --------- | ---- | --------------------------------------------------- |
| supported | bool | True if the pair can be routed through this adapter |

### getSupportedPairs

```solidity
function getSupportedPairs() external view returns (address[] tokenIns, address[] tokenOuts)
```

Get all token pairs this adapter currently supports

_May return an empty array if pairs are dynamically discovered_

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

_Returns type(uint256).max if no practical limit_

#### Parameters

| Name     | Type    | Description      |
| -------- | ------- | ---------------- |
| tokenIn  | address | The input token  |
| tokenOut | address | The output token |

#### Return Values

| Name  | Type    | Description                             |
| ----- | ------- | --------------------------------------- |
| maxIn | uint256 | Maximum amountIn (limited by liquidity) |

### adapterId

```solidity
function adapterId() external view returns (bytes32 id)
```

Unique identifier for this adapter (e.g., keccak256("Sidiora.v1"))

#### Return Values

| Name | Type    | Description            |
| ---- | ------- | ---------------------- |
| id   | bytes32 | The bytes32 identifier |

### adapterName

```solidity
function adapterName() external view returns (string name)
```

Human-readable name for this adapter

#### Return Values

| Name | Type   | Description      |
| ---- | ------ | ---------------- |
| name | string | The adapter name |

### adapterVersion

```solidity
function adapterVersion() external view returns (string version)
```

Protocol version string

#### Return Values

| Name    | Type   | Description  |
| ------- | ------ | ------------ |
| version | string | e.g. "1.0.0" |

### SwapExecuted

```solidity
event SwapExecuted(bytes32 indexed adapterId, address indexed tokenIn, address indexed tokenOut, uint256 amountIn, uint256 amountOut, address recipient)
```
