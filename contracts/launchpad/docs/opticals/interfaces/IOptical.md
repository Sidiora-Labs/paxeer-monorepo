# Solidity API

Source: [IOptical.sol](../../../contracts/opticals/interfaces/IOptical.sol).

## IOptical

Interface for optical hook plugins that inject custom logic into pool lifecycle.

_Inspired by Uniswap V4's hook system, adapted for Sidiora's launchpad model. Each optical
implements a subset of hooks indicated by getFlags(). Bit 0: beforeSwap, Bit 1: afterSwap, Bit 2:
beforeFeeDistribution, Bit 3: afterFeeDistribution_

`SidioraERC20` also recognizes bit 4 (16) for the optional
`beforeTokenTransfer(address pool, address from, address to, uint256 amount)` extension. That
callback is declared separately from this interface.

### beforeSwap

```solidity
function beforeSwap(address pool, address sender, bool isBuy, uint256 amountIn) external returns (bool proceed, int256 amountDelta)
```

Called by `SidioraPool` before a swap executes. A negative input adjustment must be smaller than
the input amount. The pool calculates its swap fee after applying the adjustment.

#### Parameters

| Name     | Type    | Description                                                       |
| -------- | ------- | ----------------------------------------------------------------- |
| pool     | address | The pool address executing the swap                               |
| sender   | address | The address initiating the swap                                   |
| isBuy    | bool    | True if buying tokens with USDL, false if selling tokens for USDL |
| amountIn | uint256 | The input amount for the swap                                     |

#### Return Values

| Name        | Type   | Description                                                                                      |
| ----------- | ------ | ------------------------------------------------------------------------------------------------ |
| proceed     | bool   | Whether the swap should continue (false = reject)                                                |
| amountDelta | int256 | Signed input adjustment; SidioraPool accepts zero or negative values and rejects positive values |

### afterSwap

```solidity
function afterSwap(address pool, address sender, bool isBuy, uint256 amountIn, uint256 amountOut) external returns (bytes4)
```

Called by `SidioraPool` after swap accounting and transfers. `amountIn` is the input after any
optical adjustment and before the swap fee. The pool requires this hook's selector as the response.

#### Parameters

| Name      | Type    | Description                             |
| --------- | ------- | --------------------------------------- |
| pool      | address | The pool address that executed the swap |
| sender    | address | The address that initiated the swap     |
| isBuy     | bool    | True if it was a buy, false if sell     |
| amountIn  | uint256 | The input amount used                   |
| amountOut | uint256 | The output amount received              |

#### Return Values

| Name | Type   | Description                            |
| ---- | ------ | -------------------------------------- |
| [0]  | bytes4 | Function selector confirming execution |

### beforeFeeDistribution

```solidity
function beforeFeeDistribution(address pool, uint256 feeAmount) external returns (uint256 adjustedFee)
```

Called by `FeeAccumulator` while recording a fee, after separating the protocol share. `feeAmount`
is the pool's share. The returned amount must be at most that share; the accumulator records any
difference as optical surplus.

#### Parameters

| Name      | Type    | Description                            |
| --------- | ------- | -------------------------------------- |
| pool      | address | The pool address                       |
| feeAmount | uint256 | The fee amount about to be distributed |

#### Return Values

| Name        | Type    | Description                                      |
| ----------- | ------- | ------------------------------------------------ |
| adjustedFee | uint256 | The adjusted fee amount after optical processing |

### afterFeeDistribution

```solidity
function afterFeeDistribution(address pool, uint256 feeAmount) external returns (bytes4)
```

Called by `FeeAccumulator` after recording pool fees and transferring the protocol share to the
treasury. `feeAmount` is the pool's share after optical diversion. The accumulator requires this
hook's selector as the response.

#### Parameters

| Name      | Type    | Description                         |
| --------- | ------- | ----------------------------------- |
| pool      | address | The pool address                    |
| feeAmount | uint256 | The fee amount that was distributed |

#### Return Values

| Name | Type   | Description                            |
| ---- | ------ | -------------------------------------- |
| [0]  | bytes4 | Function selector confirming execution |

### getFlags

```solidity
function getFlags() external view returns (uint8 flags)
```

Returns a bitmap of active hooks for this optical.

Bit 0 enables `beforeSwap`, bit 1 enables `afterSwap`, bit 2 enables `beforeFeeDistribution`, and
bit 3 enables `afterFeeDistribution`. `SidioraPool` checks swap flags; `FeeAccumulator` checks fee
flags. `SidioraERC20` checks bit 4 for the separate token-transfer extension.

#### Return Values

| Name  | Type  | Description                  |
| ----- | ----- | ---------------------------- |
| flags | uint8 | The active hook flags bitmap |
