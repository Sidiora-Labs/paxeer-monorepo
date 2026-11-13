# Solidity API

Source: [Quoter.sol](../../contracts/periphery/Quoter.sol).

## Quoter

UUPS-upgradeable contract for read-only pool quotes and statistics. Quote calls compute output, fees
and price impact from pool reserves and the pool's current dynamic fee. Read calls can be made
through `eth_call` without submitting a transaction.

`quoteExactInput` and `quoteMultihop` require registered, unpaused pools without opticals. They
reject zero-output quotes and sells exceeding real USDL reserves. Initialization grants
`DEFAULT_ADMIN_ROLE` to the supplied administrator; upgrades require that role.

### ZeroAddress

```solidity
error ZeroAddress()
```

### poolRegistry

```solidity
address poolRegistry
```

### protocolConfig

```solidity
address protocolConfig
```

### constructor

```solidity
constructor() public
```

### initialize

```solidity
function initialize(address _poolRegistry, address _protocolConfig, address _admin) external
```

### quoteExactInput

```solidity
function quoteExactInput(address pool, uint256 amountIn, bool isBuy) external view returns (struct IQuoter.QuoteResult result)
```

Quote exact input amount for a swap

#### Parameters

| Name     | Type    | Description                           |
| -------- | ------- | ------------------------------------- |
| pool     | address | The pool address                      |
| amountIn | uint256 | Exact input amount                    |
| isBuy    | bool    | True = USDL→Token, False = Token→USDL |

#### Return Values

| Name   | Type                       | Description                                           |
| ------ | -------------------------- | ----------------------------------------------------- |
| result | struct IQuoter.QuoteResult | QuoteResult with amountOut, feeAmount, priceImpactBps |

### getPoolPrice

```solidity
function getPoolPrice(address pool) external view returns (uint256)
```

Get current pool price in USDL

### getPoolStats

```solidity
function getPoolStats(address pool) external view returns (struct IQuoter.PoolStats stats)
```

Get comprehensive pool statistics

### getMarketCap

```solidity
function getMarketCap(address pool) external view returns (uint256)
```

Get market capitalization in USDL

### getAllPools

```solidity
function getAllPools(uint256 offset, uint256 limit) external view returns (address[])
```

Get all pools (paginated) from PoolRegistry

### getPoolsByCreator

```solidity
function getPoolsByCreator(address creator) external view returns (address[])
```

Get pools by creator from PoolRegistry

### quoteMultihop

```solidity
function quoteMultihop(address tokenIn, address tokenOut, uint256 amountIn) external view returns (struct IQuoter.MultihopQuoteResult result)
```

Simulate a multihop swap: Token A → USDL → Token B

#### Parameters

| Name     | Type    | Description                  |
| -------- | ------- | ---------------------------- |
| tokenIn  | address | Address of the token to sell |
| tokenOut | address | Address of the token to buy  |
| amountIn | uint256 | Amount of tokenIn to sell    |

#### Return Values

| Name   | Type                               | Description                                 |
| ------ | ---------------------------------- | ------------------------------------------- |
| result | struct IQuoter.MultihopQuoteResult | MultihopQuoteResult with both legs' details |

### \_calculateFee

```solidity
function _calculateFee(address pool) internal view returns (uint256)
```

### \_getTokenTotalSupply

```solidity
function _getTokenTotalSupply(address token) internal view returns (uint256)
```

### \_authorizeUpgrade

```solidity
function _authorizeUpgrade(address) internal
```

### UnexecutableQuote

```solidity
error UnexecutableQuote()
```

### OpticalQuoteUnsupported

```solidity
error OpticalQuoteUnsupported()
```

### \_validatePool

```solidity
function _validatePool(address pool) internal view
```

Requires a registered, unpaused pool without an optical.
