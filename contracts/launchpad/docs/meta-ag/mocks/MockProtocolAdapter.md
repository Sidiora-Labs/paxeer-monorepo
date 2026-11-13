# Solidity API

Source: [MockProtocolAdapter.sol](../../../contracts/meta-ag/mocks/MockProtocolAdapter.sol).

## IMockMintableERC20

ERC20 transfer and balance methods used by `MockProtocolAdapter`.

### transfer

```solidity
function transfer(address to, uint256 amount) external returns (bool)
```

### transferFrom

```solidity
function transferFrom(address from, address to, uint256 amount) external returns (bool)
```

### balanceOf

```solidity
function balanceOf(address who) external view returns (uint256)
```

## MockProtocolAdapter

Configurable `IProtocolAdapter` implementation for router tests. `getQuote` returns the configured
quote; its initial `available` value is true and its initial amounts are zero. `executeSwap` pulls
`amountIn` of `tokenIn` from `from` and transfers the configured `swapAmountOut` of `tokenOut` to
`recipient`. Fund the adapter with output tokens before swapping.

`lastSwap` and `swapCallCount` record successful swaps. `expectAdapterData(bytes)` configures the
expected swap payload; `clearExpectedAdapterData()` removes that check. Quote and swap calls can be
configured to revert independently.

### MockForcedRevert

```solidity
error MockForcedRevert()
```

### MockUnexpectedAdapterData

```solidity
error MockUnexpectedAdapterData()
```

### quoteAvailable

```solidity
bool quoteAvailable
```

### quoteAmountOut

```solidity
uint256 quoteAmountOut
```

### quotePriceImpactBps

```solidity
uint256 quotePriceImpactBps
```

### quoteFeeBps

```solidity
uint256 quoteFeeBps
```

### quoteFeeAmount

```solidity
uint256 quoteFeeAmount
```

### quoteAdapterData

```solidity
bytes quoteAdapterData
```

### revertOnQuote

```solidity
bool revertOnQuote
```

### swapAmountOut

```solidity
uint256 swapAmountOut
```

### swapFeeAmount

```solidity
uint256 swapFeeAmount
```

### swapResultAdapterData

```solidity
bytes swapResultAdapterData
```

### revertOnSwap

```solidity
bool revertOnSwap
```

### expectedAdapterData

```solidity
bytes expectedAdapterData
```

### expectedAdapterDataSet

```solidity
bool expectedAdapterDataSet
```

### LastSwap

```solidity
struct LastSwap {
  address tokenIn;
  address tokenOut;
  uint256 amountIn;
  uint256 amountOutMin;
  address from;
  address recipient;
  uint256 deadline;
  bytes adapterData;
}
```

### lastSwap

```solidity
struct MockProtocolAdapter.LastSwap lastSwap
```

### swapCallCount

```solidity
uint256 swapCallCount
```

### \_supportsSwap

```solidity
bool _supportsSwap
```

### constructor

```solidity
constructor(bytes32 id, string name, string version) public
```

### setQuoteResult

```solidity
function setQuoteResult(bool available, uint256 amountOut, uint256 priceImpactBps, uint256 feeBps, uint256 feeAmount, bytes adapterData) external
```

### setSwapResult

```solidity
function setSwapResult(uint256 amountOut, uint256 feeAmount, bytes adapterData) external
```

### setRevertOnQuote

```solidity
function setRevertOnQuote(bool flag) external
```

### setRevertOnSwap

```solidity
function setRevertOnSwap(bool flag) external
```

### setSupportsSwap

```solidity
function setSupportsSwap(bool flag) external
```

### expectAdapterData

```solidity
function expectAdapterData(bytes data) external
```

### clearExpectedAdapterData

```solidity
function clearExpectedAdapterData() external
```

### getQuote

```solidity
function getQuote(address, address, uint256) external view returns (struct IProtocolAdapter.QuoteResult result)
```

### executeSwap

```solidity
function executeSwap(address tokenIn, address tokenOut, uint256 amountIn, uint256 amountOutMin, address from, address recipient, uint256 deadline, bytes adapterData) external returns (struct IProtocolAdapter.SwapResult result)
```

### supportsSwap

```solidity
function supportsSwap(address, address) external view returns (bool)
```

### getSupportedPairs

```solidity
function getSupportedPairs() external pure returns (address[] tokenIns, address[] tokenOuts)
```

Get all token pairs this adapter currently supports

Returns two empty arrays. `supportsSwap` returns the configured boolean for every pair.

#### Return Values

| Name      | Type      | Description                                   |
| --------- | --------- | --------------------------------------------- |
| tokenIns  | address[] | Array of input token addresses                |
| tokenOuts | address[] | Array of corresponding output token addresses |

### getMaxInput

```solidity
function getMaxInput(address, address) external pure returns (uint256)
```

### adapterId

```solidity
function adapterId() external view returns (bytes32)
```

Adapter identifier supplied to the constructor.

#### Return Values

| Name | Type    | Description |
| ---- | ------- | ----------- |
| [0]  | bytes32 |             |

### adapterName

```solidity
function adapterName() external view returns (string)
```

Human-readable name for this adapter

#### Return Values

| Name | Type   | Description |
| ---- | ------ | ----------- |
| [0]  | string |             |

### adapterVersion

```solidity
function adapterVersion() external view returns (string)
```

Protocol version string

#### Return Values

| Name | Type   | Description |
| ---- | ------ | ----------- |
| [0]  | string |             |
