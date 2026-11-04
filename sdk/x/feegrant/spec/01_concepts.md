<!--
order: 1
-->

# Concepts

The `feegrant` module lets one account pay the transaction fees of another account
on the Paxeer X network. The chain application wires its keeper into transaction
fee deduction in [node/app.go](../../../../node/app.go).

## Grant

A `Grant` records the `granter`, `grantee`, and an `allowance` packed in a protobuf
`Any`. The granter authorizes use of its balance for the grantee's fees.
Transaction messages still require their normal signers. A grant does not
transfer or reserve coins, so the granter must have sufficient funds when fees
are paid.

There is one grant per granter/grantee pair. `MsgGrantAllowance` rejects an
existing pair, and self-grants are invalid. The granter signs grant and revocation
messages. See [Messages](03_messages.md) and the local
[message validation](../msgs.go) and [message server](../keeper/msg_server.go).

The [protobuf definitions](../../../proto/cosmos/feegrant/v1beta1/feegrant.proto)
and [FeeAllowanceI interface](../fees.go) define the stored types. `Accept` checks
the requested fee, block time, and transaction messages, and returns a removal
flag and an error. The [keeper](../keeper/keeper.go) saves accepted allowance
updates or removes a consumed allowance.

## Fee Allowance types

The [interface registry](../codec.go) registers three implementations:

- `BasicAllowance`: an optional total spending limit and expiration.
- `PeriodicAllowance`: a per-period limit together with basic allowance limits.
- `AllowedMsgAllowance`: a message-type filter around another allowance.

## BasicAllowance

[BasicAllowance](../basic_fee.go) authorizes fees from the granter's account:

- `spend_limit` is the remaining allowance in each denomination. An omitted limit
  is unrestricted by amount. When a limit is present, the requested fee must fit
  within it; an excess returns `ErrFeeLimitExceeded`.
- `expiration` is optional. Use after that timestamp returns `ErrFeeLimitExpired`.
  The check is strictly after the timestamp, using the current block time.
- With both fields omitted, the allowance has no amount or time restriction.
  The granter can revoke it with `MsgRevokeAllowance`.

An accepted fee is subtracted from a configured spending limit. The keeper removes
an allowance when that limit is fully consumed. Expiration is checked on attempted
use; it is not a periodic cleanup process.

## PeriodicAllowance

[PeriodicAllowance](../periodic_fee.go) adds a repeating budget:

- `basic` is an embedded `BasicAllowance` value, containing the optional lifetime
  spending limit and expiration. It is not an optional pointer in the Go type.
- `period` is the duration between budget resets.
- `period_spend_limit` is the maximum budget for a period.
- `period_can_spend` is the remaining budget for the current period.
- `period_reset` is the next reset timestamp.

On use at or after `period_reset`, the budget resets to `period_spend_limit`,
capped by any remaining lifetime limit. The next reset advances by one period;
if that timestamp is already past, it becomes the current block time plus one
period. Unused budget does not accumulate across periods.

Fees reduce both the period budget and any lifetime limit. Exhausting only the
period budget leaves the grant available for its next reset; exhausting the
lifetime limit removes it. The CLI requires a positive `--period` in seconds and
`--period-limit` coins when creating a periodic allowance.

## AllowedMsgAllowance

[AllowedMsgAllowance](../filtered_fee.go) wraps an allowance in `allowance` (`Any`)
and restricts use to the message type URLs in `allowed_messages`. Every message
in the transaction must be allowed. For example, allowing
`/cosmos.gov.v1beta1.MsgVote` does not also allow
`/cosmos.gov.v1beta1.MsgVoteWeighted`.

The wrapper requires a non-empty list and a valid embedded allowance. A message
outside the list returns `ErrMessageNotAllowed`; accepted messages are then
checked against the embedded allowance's amount and time limits. The
[feegrant CLI](../client/cli/tx.go) wraps the selected basic or periodic allowance
when `--allowed-messages` is supplied, for example:

```text
--allowed-messages "/cosmos.gov.v1beta1.MsgSubmitProposal,/cosmos.gov.v1beta1.MsgVote"
```

## FeeAccount flag

This fork's transaction flag is `--fee-account <granter-address>`, defined as
`FlagFeeAccount` in [client flags](../../../client/flags/flags.go). The
[client context](../../../client/cmd.go) reads that address, and the
[transaction builder](../../../client/tx/tx.go) sets the protobuf transaction's
`auth_info.fee.granter`. The grantee signs the transaction and supplies the fee
and gas settings as usual.

## Granted Fee Deductions

The [DeductFeeDecorator](../../auth/ante/fee.go) reads the transaction's fee payer
and fee granter. When a granter is set and differs from the fee payer, it calls
`UseGrantedFees` with the fee payer as the grantee and checks all transaction
messages against the grant. If accepted,
it deducts the fee coins from the granter and transfers them to the fee collector.
A missing grant or insufficient allowance rejects fee authorization.

See the [Auth Module AnteHandlers Guide](../../auth/spec/03_antehandlers.md) for
the transaction ante stage and [Events](04_events.md) for allowance-use events.

## Gas

The message filter charges `gasCostPerIteration` (10 gas) for each configured
allowed type and for each transaction message inspected. It stops at the first
disallowed message. These charges count toward the transaction's gas use. The
allowance's coin budget is reduced by the fee supplied to `UseGrantedFees`. Choose
a filter and transaction gas limit that cover the messages being submitted.
