# Perps precompile ABI

The perps precompile ABI is defined in
[`precompiles/perps/abi/abi.json`](../precompiles/perps/abi/abi.json).
Callers must use its selectors, argument types and argument order.

## Opening a position

`openPosition(uint256,address,uint256,uint256,bool)` takes the market ID,
collateral token, collateral amount, leverage and direction, in that order.
It returns a `uint256` position ID.

The TypeScript SDK's
[`perpsOpenPositionCall`](../services/agent/sdk/typescript/src/perps.ts)
encodes these arguments as a bigint market ID, token address, bigint collateral
amount, bigint leverage and boolean direction.

Collateral uses the token's raw units; USDL has six decimals. Leverage uses
18-decimal fixed point, so `10000000000000000000n` represents 10× leverage.
`isLong: true` opens long and `false` opens short. The implementation is in
[`positions.go`](../precompiles/perps/positions.go).

## Cancelling an order

`cancelOrder(uint256)` takes one order ID. The SDK's `perpsCancelOrderCall`
encodes that ID as a bigint.

The SDK's [perps tests](../services/agent/sdk/typescript/test/perps-v2.test.ts)
derive selectors from the embedded ABI, check encoded argument words and
reject invalid inputs.

The encoders validate ABI argument widths and address encoding. Market
availability, collateral support, balances and trading limits are checked when
the node executes the call.
