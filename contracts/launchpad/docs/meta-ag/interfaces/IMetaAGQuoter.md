# Solidity API

## IMetaAGQuoter

UUPS quote interface for oracle-priced PECORVault swaps. Quote methods read state without changing
it. Cross-adapter route selection is exposed by [IMetaAGRouter](IMetaAGRouter.md).

With a configured PECOR engine, quotes use its detailed exact input or exact output calculations.
When `pecor` is zero, quotes use fee-free oracle conversion. Quote construction catches dependency
errors and returns a partial result with stale flags. Quotes report available output reserves after
reservations; `getLiquidityInfo` and `getAllLiquidityInfo` report total accounted reserves.

With a configured engine, exact input quotes report `feeAmount` in output-token units and
`feeBps` as the engine's tiered fee rate. Exact output quotes report `feeAmount` as the extra input
above fee-free oracle conversion, including fee and impact costs; `feeBps` is that extra input
divided by the fee-free input, scaled by 10,000 and rounded down.

Source: [IMetaAGQuoter.sol](../../../contracts/meta-ag/interfaces/IMetaAGQuoter.sol).

### QuoteResult

```solidity
struct QuoteResult {
  uint256 amountIn;
  uint256 amountOut;
  uint256 grossAmountOut;
  uint256 executionPrice;
  uint256 spotPriceIn;
  uint256 spotPriceOut;
  uint256 feeAmount;
  uint256 feeBps;
  bool sufficientLiquidity;
  uint256 availableLiquidity;
  uint256 priceTimestampIn;
  uint256 priceTimestampOut;
  bool priceStaleIn;
  bool priceStaleOut;
}
```

### QuoteRequest

```solidity
struct QuoteRequest {
  address tokenIn;
  address tokenOut;
  uint256 amount;
  bool isExactIn;
}
```

### initialize

```solidity
function initialize(address priceOracle, address vault, address weth, address pecor, address admin) external
```

Initialize the UUPS proxy

#### Parameters

| Name        | Type    | Description                                                        |
| ----------- | ------- | ------------------------------------------------------------------ |
| priceOracle | address | IPriceOracle address                                               |
| vault       | address | IPECORVault address                                                |
| weth        | address | Wrapped native coin (WPAX on Paxeer)                               |
| pecor       | address | PECOR engine used for exact quote amounts and detailed fee queries |
| admin       | address | Timelock — granted DEFAULT_ADMIN_ROLE                              |

### quoteExactIn

```solidity
function quoteExactIn(address tokenIn, address tokenOut, uint256 amountIn) external view returns (struct IMetaAGQuoter.QuoteResult result)
```

### quoteExactOut

```solidity
function quoteExactOut(address tokenIn, address tokenOut, uint256 amountOut) external view returns (struct IMetaAGQuoter.QuoteResult result)
```

### quoteExactInNative

```solidity
function quoteExactInNative(address tokenOut, uint256 nativeAmountIn) external view returns (struct IMetaAGQuoter.QuoteResult result)
```

### quoteExactInToNative

```solidity
function quoteExactInToNative(address tokenIn, uint256 amountIn) external view returns (struct IMetaAGQuoter.QuoteResult result)
```

### quoteMarketBuy

```solidity
function quoteMarketBuy(address stablecoin, address token, uint256 stablecoinAmount) external view returns (struct IMetaAGQuoter.QuoteResult result)
```

### quoteMarketSell

```solidity
function quoteMarketSell(address token, address stablecoin, uint256 tokenAmount) external view returns (struct IMetaAGQuoter.QuoteResult result)
```

### batchQuote

```solidity
function batchQuote(struct IMetaAGQuoter.QuoteRequest[] requests) external view returns (struct IMetaAGQuoter.QuoteResult[] results)
```

### getLiquidityInfo

```solidity
function getLiquidityInfo(address token) external view returns (uint256 available, uint256 tokenPrice, bool isStale)
```

### getAllLiquidityInfo

```solidity
function getAllLiquidityInfo() external view returns (address[] tokens, uint256[] reserves, uint256[] prices, bool[] stale)
```

### getTokenPrice

```solidity
function getTokenPrice(address token) external view returns (uint256 price, uint256 timestamp, bool isStale)
```

### getTokenPrices

```solidity
function getTokenPrices(address[] tokens) external view returns (uint256[] prices, uint256[] timestamps, bool[] stale)
```

### getTWAP

```solidity
function getTWAP(address token, uint256 period) external view returns (uint256)
```
