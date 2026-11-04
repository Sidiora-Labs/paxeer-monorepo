<!--
order: 1
-->

# Concepts

The `auth` module defines account identity and transaction authentication for the
SDK application in Paxeer. The `authz` module grants permission to execute
messages on behalf of another account. Authentication and delegated authorization
are separate responsibilities.

Paxeer mainnet-beta is one BFT network. Its kernel participates in the network
and proposes blocks; a receipt is final when its block is committed. The EVM
chain ID is `125` (`0x7d`). SDK transactions use the configured SDK chain ID in
their sign bytes; the numeric EVM chain ID is not a substitute for that value.
See [Client](07_client.md) for the query interfaces.

## Gas & Fees

Gas meters account for work such as state access, signature verification and
transaction bytes. A transaction specifies a gas limit and coin fees. When a
client selects gas prices, it computes each fee as
`ceil(gasLimit * gasPrice)`. The gas limit must also cover execution: meeting a
minimum fee does not prevent an out-of-gas failure.

### Global and local minimum prices

The default SDK fee checker,
[`CheckTxFeeWithValidatorMinGasPrices`](../ante/validator_tx_fee.go), reads both:

- `FeesParams.global_minimum_gas_prices` and `FeesParams.allowed_fee_denoms`,
  obtained through the params keeper's `GetFeesParams`.
- The node's local `minimum-gas-prices`, available as `ctx.MinGasPrices()`.

`GetMinimumGasPricesWantedSorted` takes the union of the global and local price
sets using `UnionMax`, keeping the larger price for each denomination. For each
resulting price, the checker calculates `ceil(price * gasLimit)`. At least one eligible fee coin
must meet its corresponding requirement; it does not require payment in every
listed denomination. The checker retains nonzero fees only in
`sdk.DefaultBondDenom` (`uhpx`) and the configured `allowed_fee_denoms`.

The [default fee parameters](../../params/types/genesis.go) set a global minimum
of `0.01uhpx` per gas and no additional fee denominations. These are code
defaults; operators and clients must read the running chain's parameters rather
than assume those values remain current. For example, a local node setting is:

```bash
paxd start --minimum-gas-prices=0.02uhpx
```

With a global price of `0.01uhpx`, this node requires `4000uhpx` for a
200,000-gas transaction. Reducing the local price to `0.005uhpx` leaves the
global floor of `0.01uhpx` in effect, requiring `2000uhpx` for the same gas limit.
A fee that fails the check returns
`sdkerrors.ErrInsufficientFee`.

The default checker applies this price threshold during `CheckTx` when not
simulating. It does not repeat the threshold check during `DeliverTx`; fee
charging still occurs there. Global configuration therefore influences mempool
admission across nodes, while each node may impose a higher local price. Neither
should be described as a minimum-price execution check on every committed block.

### Transaction priority

The fork computes fee-based priority. For positive gas limits, `GetTxPriority`
iterates the eligible fee coins and calculates an integer fee-per-gas value.
For the registered base denomination (`uhpx` in
[`node/params/config.go`](../../../../node/params/config.go)), it first multiplies
the fee amount by `10^12` to preserve precision. Other denominations use integer
`amount / gas` without that amplification. Values outside `int64` saturate at
`math.MaxInt64`. A zero gas limit produces priority zero.

The loop replaces its current result when that result is zero or the next value
is smaller. With positive values this selects the smallest result; zero values
make mixed-denomination priority sensitive to iteration order. It is not a
sum of fees or a market-price conversion between denominations.
[`DeductFeeDecorator`](../ante/fee.go) attaches the result to the context with
`WithPriority` after charging the effective fees.

### Application wiring

[`node/ante.go`](../../../../node/ante.go) constructs the application's ante
chains. Its SDK chain wraps fee charging in `GaslessDecorator` and uses
[`NewFeeDenomTxFeeChecker`](../../../../node/antedecorators/feedenom.go). When the
EVM keeper's fee-token feature is enabled, that wrapper can convert allowed
fee-token amounts to base-denomination value for minimum-price and priority
checks, while returning the offered coins for deduction. The standalone SDK
checker itself does not do that conversion.

The node also caps ordinary SDK priority and assigns a higher tier to oracle
votes and recognized pricefeed submissions through
[`PriorityDecorator`](../../../../node/antedecorators/priority.go). The specialized
SDK `CheckTx` path in
[`node/ante/cosmos_checktx.go`](../../../../node/ante/cosmos_checktx.go) uses
`CheckAndChargeFees` and `DecoratePriority`; its committed execution counterpart
charges fees through `ChargeFees` in
[`cosmos_delivertx.go`](../../../../node/ante/cosmos_delivertx.go).
EVM transactions have their own ante checks in the same node. The default SDK
fee formula does not describe all EVM fee rules or guarantee transaction order.

See [AnteHandlers](03_antehandlers.md) for decorator order and feegrant handling,
[State](02_state.md) for account and balance ownership, and
[Parameters](07_params.md) for auth-specific gas and sequence settings.
