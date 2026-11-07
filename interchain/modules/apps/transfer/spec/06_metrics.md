<!--
order: 6
-->

# Metrics

The transfer [keeper](../keeper/relay.go) records both legacy SDK telemetry and
OpenTelemetry observations. OpenTelemetry instruments are declared in
[metrics.go](../keeper/metrics.go) under the meter `ibc_transfer_keeper`.
The legacy names below join the SDK telemetry key segments with underscores;
exporter configuration can add prefixes or counter suffixes.

| Legacy telemetry name | OpenTelemetry instrument | Meaning | OpenTelemetry unit | Type |
|-----------------------|--------------------------|---------|--------------------|------|
| `tx_msg_ibc_transfer` | `ibc_transfer_tx_msg` | Last representable successfully sent packet amount for the label set | `{token}` | Gauge (`Int64Gauge` in OpenTelemetry) |
| `ibc_transfer_packet_receive` | `ibc_transfer_packet_receive` | Last representable successfully received packet amount for the label set | `{token}` | Gauge (`Int64Gauge` in OpenTelemetry) |
| `ibc_transfer_send` | `ibc_transfer_send` | Number of successful packet sends | `{count}` | Counter (`Int64Counter` in OpenTelemetry) |
| `ibc_transfer_receive` | `ibc_transfer_receive` | Number of successful packet receives | `{count}` | Counter (`Int64Counter` in OpenTelemetry) |

## Amount gauges

Gauges record individual amounts in denomination base units. They are last-value
observations, not cumulative token volume or balances. They cover both
escrow/burn sends and mint/unescrow receives. Both telemetry implementations
record a gauge only when `Amount.IsInt64()` is true: a valid positive amount
larger than `9223372036854775807` produces no gauge observation, but the transfer
counter still increments. An oversized packet leaves the preceding representable
gauge observation unchanged; it does not reset it to zero. Legacy gauges convert the amount to `float32`, which
can lose integer precision; OpenTelemetry records an `int64`.

Legacy amount gauges use a `denom` label. For sends its value is the expanded
full denomination path. For receives it is the unprefixed denomination on the
unescrow branch and the incoming packet denomination on the mint branch.

OpenTelemetry gauges instead use the bounded `denom_class` attribute computed
by [telemetry.DenomClass](../../../../../sdk/telemetry/utils.go) from those same
strings: `uhpx` for exactly `uhpx`, `ibc` for an `ibc/` prefix, `factory` for a
`factory/` prefix, and `other` otherwise. An expanded path beginning with
`transfer/channel-0/` therefore falls into `other`, even though its bank
representation is an IBC voucher. The last observation is per class, so it
cannot be used to calculate per-denomination totals.

`TestRepairM_1206_Metrics` collects the real legacy and OpenTelemetry series
after sends and receives, including amounts on both sides of the int64 boundary.

## Transfer counters

Each successful send or receive adds one, including transfers whose amount
does not fit in an `int64`. Failed operations do not reach these recording
sites. Acknowledgements, refunds and timeouts do not increment these counters.

| Counter | Labels or attributes |
|---------|----------------------|
| Send | `destination_port`, `destination_channel`, `source` |
| Receive | `source_port`, `source_channel`, `source` |

`source` is a boolean in OpenTelemetry and the string `"true"` or `"false"` in
legacy telemetry. For sends it is true when this chain escrows tokens, false
when it burns vouchers. For receives it is true when this chain unescrows
tokens, false when it mints vouchers. These are the channel-relative roles
explained in [Concepts](01_concepts.md#denomination-trace).

The two telemetry APIs observe the same operations. Do not add their exported
series together as independent transfer counts. Use [Events](05_events.md)
for packet details and acknowledgement or timeout outcomes.
