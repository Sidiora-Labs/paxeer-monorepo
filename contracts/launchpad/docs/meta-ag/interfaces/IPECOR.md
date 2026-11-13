# Solidity API

## IPECOR

Interface for the UUPS oracle-priced PECOR swap engine using PECORVault. It exposes token swaps,
market buys and sells, native coin swaps, fee collection and quotes. Users approve the vault for
token inputs. A zero deadline means no expiry. [IPECOROrders](IPECOROrders.md) describes limit,
stop-loss and stop-limit orders.

The effective fee is `swapFeeBps`, plus `tier1FeeBps` at 10,000 USD and `tier2FeeBps` at 100,000
USD. Volume uses 18 decimal scaling and the sum of all three rates is capped at 200 basis points.
Price impact is a separate deduction capped at 500 basis points. Retained output is reserved in the
vault as accrued fees. See [PECOR](../engine/PECOR.md) for role requirements and reservation state.

Source: [IPECOR.sol](../../../contracts/meta-ag/interfaces/IPECOR.sol).

### initialize

```solidity
function initialize(address priceOracle, address vault, address weth, address tracker, address admin) external
```

Initialize the UUPS proxy

#### Parameters

| Name        | Type    | Description                                                  |
| ----------- | ------- | ------------------------------------------------------------ |
| priceOracle | address | IPriceOracle address (source of truth for vault-side prices) |
| vault       | address | IPECORVault v2 address                                       |
| weth        | address | Wrapped native coin (WPAX on Paxeer)                         |
| tracker     | address | ITransactionTracker address (may be zero at bootstrap)       |
| admin       | address | Timelock — granted DEFAULT_ADMIN_ROLE                        |

### setPriceOracle

```solidity
function setPriceOracle(address oracle) external
```

### setTransactionTracker

```solidity
function setTransactionTracker(address tracker) external
```

### setSwapFee

```solidity
function setSwapFee(uint256 feeBps) external
```

### setTieredFees

```solidity
function setTieredFees(uint256 tier1FeeBps, uint256 tier2FeeBps) external
```

### setPriceImpact

```solidity
function setPriceImpact(bool enabled, uint256 scalarBps) external
```

### setFeeCollector

```solidity
function setFeeCollector(address collector) external
```

### pause

```solidity
function pause() external
```

### unpause

```solidity
function unpause() external
```

### collectFees

```solidity
function collectFees(address token) external
```

### swapExactIn

```solidity
function swapExactIn(address tokenIn, address tokenOut, uint256 amountIn, uint256 amountOutMin, uint256 deadline) external returns (uint256 amountOut)
```

### swapExactOut

```solidity
function swapExactOut(address tokenIn, address tokenOut, uint256 amountOut, uint256 amountInMax, uint256 deadline) external returns (uint256 amountIn)
```

### marketBuy

```solidity
function marketBuy(address stablecoin, address token, uint256 stablecoinAmount, uint256 minTokenAmount, uint256 deadline) external returns (uint256 tokenAmount)
```

### marketSell

```solidity
function marketSell(address token, address stablecoin, uint256 tokenAmount, uint256 minStablecoinAmount, uint256 deadline) external returns (uint256 stablecoinAmount)
```

### swapExactInNative

```solidity
function swapExactInNative(address tokenOut, uint256 amountOutMin, uint256 deadline) external payable returns (uint256 amountOut)
```

### swapExactInToNative

```solidity
function swapExactInToNative(address tokenIn, uint256 amountIn, uint256 amountOutMin, uint256 deadline) external returns (uint256 amountOut)
```

### getQuoteExactIn

```solidity
function getQuoteExactIn(address tokenIn, address tokenOut, uint256 amountIn) external view returns (uint256 amountOut)
```

### getQuoteExactOut

```solidity
function getQuoteExactOut(address tokenIn, address tokenOut, uint256 amountOut) external view returns (uint256 amountIn)
```

### getDetailedQuote

```solidity
function getDetailedQuote(address tokenIn, address tokenOut, uint256 amountIn) external view returns (uint256 grossOut, uint256 netOut, uint256 priceImpactBps, uint256 feeBps, uint256 feeAmount)
```

Full oracle-priced quote with price impact and fee breakdown.

#### Return Values

| Name           | Type    | Description                                     |
| -------------- | ------- | ----------------------------------------------- |
| grossOut       | uint256 | Raw amountOut before fee / impact.              |
| netOut         | uint256 | amountOut user actually receives.               |
| priceImpactBps | uint256 | Price impact applied (BPS).                     |
| feeBps         | uint256 | Effective fee rate after tiered stacking (BPS). |
| feeAmount      | uint256 | Fee deducted from grossOut.                     |

### multicall

```solidity
function multicall(bytes[] data) external returns (bytes[] results)
```

Batch multiple calls atomically via delegatecall.

### SimpleSwap

```solidity
event SimpleSwap(address indexed user, address indexed tokenIn, address indexed tokenOut, uint256 amountIn, uint256 amountOut, uint256 executionPrice)
```

### MarketOrderExecuted

```solidity
event MarketOrderExecuted(address indexed user, address indexed stablecoin, address indexed token, uint256 stablecoinAmount, uint256 tokenAmount, bool isBuy)
```

### NativeSwap

```solidity
event NativeSwap(address indexed user, address indexed tokenOther, uint256 nativeAmount, uint256 tokenAmount, bool nativeIsInput)
```

### PriceImpactApplied

```solidity
event PriceImpactApplied(address indexed user, address indexed tokenOut, uint256 impactBps, uint256 deductedAmount)
```

### TieredFeeApplied

```solidity
event TieredFeeApplied(address indexed user, address indexed tokenOut, uint256 volumeUSD, uint256 feeBps, uint256 feeAmount)
```

### SwapFeeUpdated

```solidity
event SwapFeeUpdated(uint256 feeBps)
```

### TieredFeesUpdated

```solidity
event TieredFeesUpdated(uint256 tier1FeeBps, uint256 tier2FeeBps)
```

### PriceImpactConfigUpdated

```solidity
event PriceImpactConfigUpdated(bool enabled, uint256 scalarBps)
```

### FeeCollectorUpdated

```solidity
event FeeCollectorUpdated(address indexed collector)
```

### PriceOracleUpdated

```solidity
event PriceOracleUpdated(address indexed oracle)
```

### TransactionTrackerUpdated

```solidity
event TransactionTrackerUpdated(address indexed tracker)
```

### FeesCollected

```solidity
event FeesCollected(address indexed token, address indexed collector, uint256 amount)
```
