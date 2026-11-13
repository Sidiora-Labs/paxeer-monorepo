# Solidity API

Source: [MockSidioraRouter.sol](../../../contracts/meta-ag/mocks/MockSidioraRouter.sol).

## MockSidioraRouter

Configurable swap responses for `SidioraAdapter` tests. Implements `buy`, `sell` and
`swapTokenForToken`, with the latter returning `(amountOut, intermediateUsdl)`.

Records call arguments, call counts and observed allowances. When `skipTransfers` is false, buys
pull `usdl`, sells pull `sellTokenIn` when it is nonzero, and multihop swaps pull `tokenIn` from the
caller. Each path transfers a positive configured output amount back to the caller. Set `usdl`,
`buyTokenOut` and `sellTokenIn` as required and fund the output balance before swapping.
When `skipTransfers` is true, both inbound and outbound token transfers are skipped. Sell allowance
recording occurs only when `skipTransfers` is false and `sellTokenIn` is nonzero; otherwise it is zero.

### LastBuy

```solidity
struct LastBuy {
  address pool;
  uint256 usdlAmountIn;
  uint256 minTokensOut;
  uint256 deadline;
  address caller;
  uint256 observedAllowance;
}
```

### LastSell

```solidity
struct LastSell {
  address pool;
  uint256 tokenAmountIn;
  uint256 minUsdlOut;
  uint256 deadline;
  address caller;
  uint256 observedAllowance;
}
```

### LastMultihop

```solidity
struct LastMultihop {
  address tokenIn;
  address tokenOut;
  uint256 amountIn;
  uint256 minAmountOut;
  uint256 deadline;
  address caller;
  uint256 observedAllowance;
}
```

### lastBuy

```solidity
struct MockSidioraRouter.LastBuy lastBuy
```

### lastSell

```solidity
struct MockSidioraRouter.LastSell lastSell
```

### lastMultihop

```solidity
struct MockSidioraRouter.LastMultihop lastMultihop
```

### buyCallCount

```solidity
uint256 buyCallCount
```

### sellCallCount

```solidity
uint256 sellCallCount
```

### multihopCallCount

```solidity
uint256 multihopCallCount
```

### buyReturn

```solidity
uint256 buyReturn
```

### sellReturn

```solidity
uint256 sellReturn
```

### multihopAmountOut

```solidity
uint256 multihopAmountOut
```

### multihopIntermediateUsdl

```solidity
uint256 multihopIntermediateUsdl
```

### revertOnBuy

```solidity
bool revertOnBuy
```

### revertOnSell

```solidity
bool revertOnSell
```

### revertOnMultihop

```solidity
bool revertOnMultihop
```

### revertReason

```solidity
string revertReason
```

### skipTransfers

```solidity
bool skipTransfers
```

If true, skip both inbound and outbound token transfers.

### usdl

```solidity
address usdl
```

### buyTokenOut

```solidity
address buyTokenOut
```

_For `buy(pool, ...)`: which token to send back. Set once per test._

### setUsdl

```solidity
function setUsdl(address usdl_) external
```

Sets the USDL token pulled on buys and returned on sells.

### setBuyTokenOut

```solidity
function setBuyTokenOut(address token) external
```

### setBuyReturn

```solidity
function setBuyReturn(uint256 v) external
```

### setSellReturn

```solidity
function setSellReturn(uint256 v) external
```

### setMultihopReturn

```solidity
function setMultihopReturn(uint256 amountOut_, uint256 intermediateUsdl_) external
```

### setRevertOnBuy

```solidity
function setRevertOnBuy(bool v, string reason) external
```

### setRevertOnSell

```solidity
function setRevertOnSell(bool v, string reason) external
```

### setRevertOnMultihop

```solidity
function setRevertOnMultihop(bool v, string reason) external
```

### setSkipTransfers

```solidity
function setSkipTransfers(bool v) external
```

### resetCounts

```solidity
function resetCounts() external
```

### buy

```solidity
function buy(address pool, uint256 usdlAmountIn, uint256 minTokensOut, uint256 deadline) external returns (uint256 amountOut)
```

### sell

```solidity
function sell(address pool, uint256 tokenAmountIn, uint256 minUsdlOut, uint256 deadline) external returns (uint256 amountOut)
```

### sellTokenIn

```solidity
address sellTokenIn
```

_Test sets this BEFORE calling the adapter's executeSwap for a sell, so the mock knows which ERC20
to pull from the adapter._

### setSellTokenIn

```solidity
function setSellTokenIn(address token) external
```

### swapTokenForToken

```solidity
function swapTokenForToken(address tokenIn, address tokenOut, uint256 amountIn, uint256 minAmountOut, uint256 deadline) external returns (uint256 amountOut, uint256 intermediateUsdl)
```
