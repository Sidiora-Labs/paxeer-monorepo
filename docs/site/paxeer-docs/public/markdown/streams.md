<!-- Source: https://docs.paxeer.app/streams/ -->

# Streaming Payments

Pay for elapsed time or verified usage with funded, capped streams, deterministic accrual, settlement, and cancellation.

## Continuous Commerce on Paxeer X

Streams turn ongoing work into incremental payment obligations. Time streams support service retainers, compute reservations, and recurring agent work. Metered streams pay for signed usage readings such as completed requests or consumed resources. Both use an isolated `agent:<did>:stream:<id>` account and settle through `402LXP` transfer legs.

Accrual and settlement are separate: accrual calculates how much the recipient has earned, while settlement transfers available funds. A stream has a lifetime cap and can be topped up, paused, resumed, or closed by its payer.

## Choose a Payment Mode

| Mode | Accrual input | Typical use |
| --- | --- | --- |
| Time (`1`) | Elapsed batch time in milliseconds, bounded by the stream end time. | Agent availability, access, subscriptions, reserved compute. |
| Metered (`2`) | Increase in a signed cumulative meter reading. | API calls, work units, bandwidth, resource consumption. |

## Opening and Funding a Stream

Set the stream ID, dedicated account, recipient, asset, mode, rate, rate unit, start and end timestamps, total cap, and positive initial funding. Amounts and rates use integer asset base units. The payer supplies authorized funds; the stream account must differ from the recipient account.

For time mode, `rate_unit` is a time interval in milliseconds. For metered mode, it is a quantity of meter units. A metered stream registers between one and eight authority keys in strictly increasing order. A time stream has no meter authorities.

### Accrual Formula

```
new_amount = floor((rate × delta + remainder_carry) / rate_unit)
new_amount = min(new_amount, total_cap - accrued_total)
claimable  = accrued_total - settled_total
payment    = min(claimable, stream_account_balance)
```

The remainder carries across accrual calculations so frequent settlement does not repeatedly discard fractional base units. Batch timestamps provide a shared clock; regressing timestamps fail. Reaching the cap stops additional accrual.

## Time Stream Example

1. Fund a stream with 600 base units and set a lifetime cap of 600.
2. Set `rate = 10` and `rate_unit = 60000`, paying 10 base units per minute.
3. After 15 active minutes, 150 units have accrued. Settle to pay those 150 to the named recipient.
4. Pause for five minutes, then resume. The paused interval earns no additional time payment.
5. Close after another 10 active minutes. With sufficient funding, closing pays the remaining 100 accrued units and refunds the unused 350 to the payer.

## Metered Stream Workflow

A meter authority signs the stream ID, cumulative reading, and authority key under the `LXP:STREAM:METER:v1` message tag. The stream verifies the Ed25519 signature and checks that the key belongs to its registered authority set.

For a rate of 20 base units per 100 usage units, a reading increase from 1,000 to 1,250 accrues 50 base units. Readings cannot move backward. Repeating a reading produces no new accrual; multiple authority keys attest to the same cumulative meter rather than maintaining independent counters.

Submit authenticated readings, then settle the accrued amount. Meter readings accepted while paused or underfunded advance the meter baseline without accruing payment, preventing that recorded usage from being charged retroactively when payment resumes.

## Funding, Claims, and Cancellation

| Action | Behavior |
| --- | --- |
| Top up | Add authorized funds to an existing open stream. Funding does not increase its lifetime cap. |
| Settle / claim | Pay accrued, unsettled value to the fixed recipient, limited to the funded balance. |
| Pause | Payer checkpoints time accrual and stops further earning while paused. |
| Resume | Payer restarts from the current batch timestamp without charging paused time. |
| Rate change | Payer settles accrued value at the existing rate, then replaces the rate for subsequent accrual under a fresh idempotency key. |
| Close / cancel | Payer settles payable accrued value, refunds the excess balance, and permanently closes the stream. |

### Underfunding

If settlement cannot cover all accrued value, it pays the available balance, resets accrued value to the settled total, and marks the stream underfunded. This does not preserve an unpaid debt balance. Accrual stops while underfunded. A top-up clears underfunding and starts the next time-accrual interval at the top-up timestamp.

**Maintain funding for continuous service.** Monitor the stream balance, earned amount, cap, pause state, and underfunding state together. A payment cap limits earned value; a funded balance determines what settlement can actually pay.

### Closing a Stream

Closing a funded active time stream first accounts for eligible elapsed time. It pays the recipient up to the available balance and returns the rest to the payer in the same transfer set. Closing is final: the stream cannot resume or receive further top-ups.

## Protocol Reference

Streams are module `4`, with payload version `1`. The version prefix is two bytes, `00 01`.

| Activity | Type | Payload bytes |
| --- | --- | --- |
| `LX_STREAM_OPEN` | `0x00040001` | 204 + 32 per authority |
| `LX_STREAM_TOP_UP` | `0x00040002` | 50 |
| `LX_STREAM_METER` | `0x00040003` | 138 |
| `LX_STREAM_SETTLE` | `0x00040004` | 66 |
| `LX_STREAM_PAUSE` | `0x00040005` | 34 |
| `LX_STREAM_RESUME` | `0x00040006` | 34 |
| `LX_STREAM_CLOSE` | `0x00040007` | 66 |
| `LX_STREAM_RATE_CHANGE` | `0x00040008` | 82 |

The rate-change payload carries the stream ID, a positive new rate as an unsigned 128-bit integer, and a nonzero idempotency key. The operation settles payable accrued value before storing the new rate. It preserves the stream's cap and rate unit.

### Retries and Validation

Settle and close include a nonzero idempotency key. Retry the same economic operation with its original key and retain the result receipt. A key cannot identify a different stream or operation. Reject malformed versions, trailing payload bytes, meter regressions, unauthorized meters, and operations against a closed stream.

Funding, draws, and refunds use published transfer assets and follow asset pause and asset-match rules. The stream account only permits its protocol-authorized draw and refund paths. Payloads are defined in `include/layerx/lx_stream.h` and `modules/layerxpay/types/payload_stream.go`; execution is implemented in `src/modules/stream/`.

## Related Guides

- [Escrow and milestone payments](https://docs.paxeer.app/escrow)

- [Protocol modules](https://docs.paxeer.app/modules)

- [SDK integration](https://docs.paxeer.app/sdk)
