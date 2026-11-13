# Solidity API

Source: [MetaAGQuoter.sol](../../../contracts/meta-ag/quoter/MetaAGQuoter.sol).

## MetaAGQuoter

UUPS-upgradeable quoter for oracle-priced vault swaps. Quote methods return amounts, fees, liquidity
and price freshness. `batchQuote` handles multiple requests and maps zero token addresses to the
configured wrapped-native token.

With a PECOR engine configured, exact-input quotes use `getDetailedQuote` and exact-output quotes
use `getQuoteExactOut`. Otherwise, amounts are computed from oracle prices and token decimals. Quote
liquidity uses the vault's available reserves after reservations and quarantine checks.
Cross-adapter quote selection is provided by `MetaAGRouter.getBestQuote` and `getAllQuotes`.

Initialization sets the oracle, vault, wrapped-native token, optional engine and administrator.
Upgrades require `DEFAULT_ADMIN_ROLE`.

### BPS_DENOMINATOR

```solidity
uint256 BPS_DENOMINATOR
```

### ZeroAddress

```solidity
error ZeroAddress()
```

### priceOracle

```solidity
contract IPriceOracle priceOracle
```

### vault

```solidity
contract IPECORVault vault
```

### weth

```solidity
address weth
```

Canonical wrapped-native token. Semantically immutable — assigned exactly once in `initialize` and
never mutated afterwards.

### pecor

```solidity
address pecor
```

Optional PECOR engine used for detailed exact-input and exact-output quotes.

### constructor

```solidity
constructor() public
```

### initialize

```solidity
function initialize(address priceOracle_, address vault_, address weth_, address pecor_, address admin) external
```

Initialize the UUPS proxy

#### Parameters

| Name          | Type    | Description                        |
| ------------- | ------- | ---------------------------------- |
| priceOracle\_ | address |                                    |
| vault\_       | address |                                    |
| weth\_        | address |                                    |
| pecor\_       | address |                                    |
| admin         | address | Account granted DEFAULT_ADMIN_ROLE |

### \_authorizeUpgrade

```solidity
function _authorizeUpgrade(address) internal
```

### quoteExactIn

```solidity
function quoteExactIn(address tokenIn, address tokenOut, uint256 amountIn) external view returns (struct IMetaAGQuoter.QuoteResult)
```

### quoteExactOut

```solidity
function quoteExactOut(address tokenIn, address tokenOut, uint256 amountOut) external view returns (struct IMetaAGQuoter.QuoteResult)
```

### quoteExactInNative

```solidity
function quoteExactInNative(address tokenOut, uint256 nativeAmountIn) external view returns (struct IMetaAGQuoter.QuoteResult)
```

### quoteExactInToNative

```solidity
function quoteExactInToNative(address tokenIn, uint256 amountIn) external view returns (struct IMetaAGQuoter.QuoteResult)
```

### quoteMarketBuy

```solidity
function quoteMarketBuy(address stablecoin, address token, uint256 stablecoinAmount) external view returns (struct IMetaAGQuoter.QuoteResult)
```

### quoteMarketSell

```solidity
function quoteMarketSell(address token, address stablecoin, uint256 tokenAmount) external view returns (struct IMetaAGQuoter.QuoteResult)
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

### \_buildQuote

```solidity
function _buildQuote(address tokenIn, address tokenOut, uint256 amount, bool isExactIn) internal view returns (struct IMetaAGQuoter.QuoteResult result)
```

Builds a full QuoteResult for a vault-side swap.

Calls `quoteChecked` and catches failures. Missing prices return partial metadata; caught failures
return a zero quote with both stale flags set. `sufficientLiquidity` uses available output reserves.

### \_safeGetPrice

```solidity
function _safeGetPrice(address token) internal view returns (uint256 price, uint256 timestamp, bool stale)
```

_Non-reverting price fetch. Returns (0, 0, true) if the oracle call reverts (missing token or stale
price)._

### \_calculateOutput

```solidity
function _calculateOutput(uint256 amountIn, uint256 priceIn, uint256 priceOut, uint256 decimalsIn, uint256 decimalsOut) internal pure returns (uint256)
```

Converts input to output using oracle prices and token decimals.

### \_calculateInput

```solidity
function _calculateInput(uint256 amountOut, uint256 priceIn, uint256 priceOut, uint256 decimalsIn, uint256 decimalsOut) internal pure returns (uint256)
```

Computes the input required at the given prices and token decimals, rounding up.

### quoteChecked

```solidity
function quoteChecked(address tokenIn, address tokenOut, uint256 amount, bool isExactIn) external view returns (struct IMetaAGQuoter.QuoteResult result)
```

Builds a quote from oracle, vault and optional engine reads. Unlike the public quote wrappers, this
method propagates failures from those reads and calculations.
