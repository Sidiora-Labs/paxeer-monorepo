<!--
order: 4
-->

# Messages

The current protobuf service `paxprotocol.paxchain.oracle.Msg` exposes `AggregateExchangeRateVote` and `DelegateFeedConsent`. Schemas live in [`api/oracle/tx.proto`](../../../api/oracle/tx.proto), basic validation and signers in [`types/msgs.go`](../types/msgs.go), and stateful handling in [`keeper/msg_server.go`](../keeper/msg_server.go).

Account fields use `pax` Bech32 addresses; validator operator fields use `paxvaloper`, as configured in [`node/params/config.go`](../../../node/params/config.go). An operator account uses the same address bytes as its validator operator address, with the account prefix.

## MsgAggregateExchangeRateVote

This is a single-step submission of decimal rates for current vote targets.

| Proto field | Type | Meaning |
| ----------- | ---- | ------- |
| `exchange_rates` (2) | string | Comma-separated decimal coin expressions, such as `1.25uatom,2.5ueth` |
| `feeder` (3) | string | Signing account address |
| `validator` (4) | string | Operator address of the validator represented by the vote |

The proto reserves field 1 and the name `salt`; there is no current `Salt` field or required preceding prevote. `MsgExchangeRatePrevote`, `MsgExchangeRateVote`, and `MsgAggregateExchangeRatePrevote` are removed interfaces, not alternative submission methods.

`GetSigners` selects the feeder account. `ValidateBasic` checks both address formats, rejects a zero-length exchange-rate string, limits it to 4096 bytes, parses its decimal coin tuples, and rejects rates whose internal integer bit length exceeds `255 + sdk.DecimalPrecisionBits`. [`ParseExchangeRateTuples`](../types/vote.go) trims the input and rejects malformed expressions and duplicate denoms. Its whitespace-only input produces an empty tuple list, so clients should submit explicit rate/denom pairs.

The message server then:

1. Authorizes the feeder: it must be the operator account or the stored delegate. The represented validator must exist and be bonded. Delegation does not remove the operator's own permission.
2. Parses the tuples and rejects any denom that is not a current vote target. Query `paxd q oracle vote-targets` before submitting; a stored price does not imply target membership.
3. Writes one `AggregateExchangeRateVote` under the validator's key, replacing any previous value at that key. The server does not require a tuple for every target, but missing targets can prevent period success.
4. Emits `aggregate_vote` and a `message` event. See the [exact attributes](05_events.md#msgaggregateexchangeratevote).

The saved vote is consumed and cleared by [MidBlock](03_end_block.md#tally-exchange-rate-votes-in-midblock), which performs quorum and reward-band checks. Accepting a transaction does not itself update an exchange rate or guarantee a successful period.

```bash
paxd tx oracle aggregate-vote '1.25uatom,2.5ueth' <validator-address> --from <feeder-key>
```

The example values illustrate encoding, not recommended prices. The CLI allows the validator argument to be omitted when voting as the operator account; a separate feeder must supply the validator it represents.

## MsgDelegateFeedConsent

This message authorizes a feeder account to vote on behalf of an existing validator.

| Proto field | Type | Meaning |
| ----------- | ---- | ------- |
| `operator` (1) | string | Validator operator address |
| `delegate` (2) | string | Account address receiving oracle voting permission |

`GetSigners` selects the operator's account address. `ValidateBasic` checks both address formats. The message server requires the validator to exist, writes the feeder delegation, and emits `feed_delegate` and `message`. This operation does not transfer funds or consensus signing authority. It does not require the validator to be bonded; submitting votes does.

```bash
paxd tx oracle set-feeder <feeder-account-address> --from <operator-key>
paxd q oracle feeder <validator-address>
```

Without an explicit delegation, the keeper returns the operator account as its own feeder. A later consent message replaces the stored delegate. See [FeederDelegation](02_state.md#feederdelegation) and [delegation events](05_events.md#msgdelegatefeedconsent).

## Transaction Restrictions

[`node/ante.go`](../../../node/ante.go) installs the oracle decorators from [`ante.go`](../ante.go). `VoteAloneDecorator` rejects a transaction that mixes either oracle message type with unrelated message types, using SDK `ErrInvalidRequest`.

`SpammingPreventionDecorator` validates vote feeders and uses a per-validator memory counter during non-simulated CheckTx, excluding ReCheckTx. A second admitted vote at the same height returns SDK `ErrAlreadyExists`. This is an admission check, not a consensus rule limiting a validator to one vote per period; the message server's setter replaces the stored vote.

## Errors

Oracle codes below are registered in [`types/errors.go`](../types/errors.go). SDK and staking errors retain their own codespaces.

| Condition | Error |
| --------- | ----- |
| Invalid account or operator format in basic validation | SDK `ErrInvalidAddress` |
| Zero-length rate string | SDK `ErrUnknownRequest` |
| Rate string longer than 4096 bytes | SDK `ErrInvalidRequest` |
| Malformed or duplicate rate/denom tuples | SDK `ErrInvalidCoins` |
| Rate integer exceeds the basic-validation bit limit | Oracle `ErrInvalidExchangeRate` (2) |
| Feeder is neither operator nor delegate | Oracle `ErrNoVotingPermission` (5) |
| Validator missing, or not bonded when voting | Staking `ErrNoValidatorFound` |
| Submitted denom is not a vote target | Oracle `ErrUnknownDenom` (14) |

Successful responses are empty `MsgAggregateExchangeRateVoteResponse` and `MsgDelegateFeedConsentResponse` records. Their effects are visible in [state](02_state.md) and [events](05_events.md).
