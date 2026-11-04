<!--
order: 5
-->

# Events

The module's event names and attribute keys are declared in [`types/events.go`](../types/events.go). The tables below describe attributes emitted by the oracle keeper and message handlers. Values are strings in the SDK events; framework transaction processing can add its own message metadata.

## MidBlocker

On a vote-period boundary, `MidBlocker` calls `SetBaseExchangeRateWithEvent` for each accepted nonzero tally result. Each call emits:

| Type | Attribute key | Attribute value |
| ---- | ------------- | --------------- |
| `exchange_rate_update` | `denom` | Updated denom |
| `exchange_rate_update` | `exchange_rate` | `sdk.Dec.String()` of the stored rate |

The event carries no observation height or timestamp attributes. Read the rate's [state metadata](02_state.md#exchangerate) for those values. A retained old rate, a below-threshold ballot, or a skipped zero median does not emit a new price update. See [`abci.go`](../abci.go) and [`keeper/keeper.go`](../keeper/keeper.go).

## EndBlocker

At a slash-window boundary, `SlashAndResetCounters` emits one event per nonempty penalty counter, whether or not that validator is penalized:

| Type | Attribute key | Attribute value |
| ---- | ------------- | --------------- |
| `end_slash_window` | `operator` | Validator operator Bech32 address |
| `end_slash_window` | `miss_count` | Unsigned decimal count |
| `end_slash_window` | `abstain_count` | Unsigned decimal count |
| `end_slash_window` | `success_count` | Unsigned decimal count |

These are the counts before the record is deleted. An all-zero counter is skipped without an event or deletion. This event alone does not indicate slashing or jailing. The subsequent `RemoveExcessFeeds` cleanup emits no oracle event. See [`keeper/slash.go`](../keeper/slash.go) and [EndBlock processing](03_end_block.md#slash-window-processing-in-endblock).

## Handlers

The successful message handlers in [`keeper/msg_server.go`](../keeper/msg_server.go) emit the following attributes. They do not emit an `action` attribute themselves; legacy action strings are not part of these tables.

### MsgDelegateFeedConsent

| Type | Attribute key | Attribute value |
| ---- | ------------- | --------------- |
| `feed_delegate` | `feeder` | `msg.Delegate`, the newly authorized feeder account |
| `message` | `module` | `oracle` |
| `message` | `sender` | `msg.Operator`, the validator operator address |

The `feed_delegate` event itself has no `operator` attribute. The associated module `message` event identifies the operator through `sender`. See [MsgDelegateFeedConsent](04_messages.md#msgdelegatefeedconsent).

### MsgAggregateExchangeRateVote

| Type | Attribute key | Attribute value |
| ---- | ------------- | --------------- |
| `aggregate_vote` | `voter` | `msg.Validator`, the validator operator address |
| `aggregate_vote` | `exchange_rates` | Original `msg.ExchangeRates` string |
| `message` | `module` | `oracle` |
| `message` | `sender` | `msg.Feeder`, the signing account address |

The `aggregate_vote` event itself has no `feeder` attribute. Its rate string reports the submission, not a quorum-approved price; only a later tally can emit `exchange_rate_update`. See [MsgAggregateExchangeRateVote](04_messages.md#msgaggregateexchangeratevote) and [the voting procedure](01_concepts.md#voting-procedure).
