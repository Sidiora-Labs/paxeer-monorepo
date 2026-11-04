<!--
order: 2
-->

# State

The persistent KV store is named `oracle`; the spam-prevention memory store is `oracle_mem`. Key constructors are in [`types/keys.go`](../types/keys.go), record schemas in [`api/oracle/oracle.proto`](../../../api/oracle/oracle.proto), and encoding and access behavior in [`keeper/keeper.go`](../keeper/keeper.go).

## Store Layout

In this table, `denom` means the raw denom string bytes, and `validator` means the raw operator address bytes prefixed with their length using `address.MustLengthPrefix`. Addresses in keys are not Bech32 text. Records marked protobuf are serialized through the keeper's binary codec, not Amino.

| Store | Key | Value |
| ----- | --- | ----- |
| `oracle` | `0x01 + denom` | protobuf `OracleExchangeRate` |
| `oracle` | `0x02 + validator` | raw feeder account address bytes |
| `oracle` | `0x03 + validator` | protobuf `VotePenaltyCounter` |
| `oracle` | `0x05 + validator` | protobuf `AggregateExchangeRateVote` |
| `oracle` | `0x06 + denom` | protobuf `Denom` with `name` |
| `oracle` | `0x07 + uint64 timestamp (8-byte big-endian)` | protobuf `PriceSnapshot` |
| `oracle_mem` | `0x08 + validator` | last admitted block height as 8-byte big-endian uint64 |

Prefix `0x04` belonged to the removed aggregate prevote store and has no current writer. There is no persistent scalar `ExchangeRateVote` record. `VoteForTally` and `ExchangeRateBallot` are temporary Go structures assembled from aggregate votes during [MidBlock](03_end_block.md#tally-exchange-rate-votes-in-midblock).

## ExchangeRate

Each denom's `OracleExchangeRate` contains:

| Field | Go type | Meaning |
| ----- | ------- | ------- |
| `exchange_rate` | `sdk.Dec` | Last stored base exchange rate |
| `last_update` | `sdk.Int` | Observation block height |
| `last_update_timestamp` | `int64` | Observation block time, Unix milliseconds |

`SetBaseExchangeRate` records the current block height and `ctx.BlockTime().UnixMilli()`. `SetBaseExchangeRateWithEvent` also emits an [exchange_rate_update](05_events.md#midblocker) event. `GetBaseExchangeRate` returns the rate, height, timestamp, and error; a missing denom returns `ErrUnknownDenom` (oracle code 14).

Rates are not purged at every vote-period boundary. A ballot that fails to produce a replacement leaves the previous rate and its timestamp unchanged. `Actives` enumerates denoms with stored rates; it does not check freshness or guarantee current-period quorum. `VoteTargets` enumerates accepted voting denoms instead. On a slash-window boundary, `RemoveExcessFeeds` deletes rates for denoms outside the current vote targets. Consumers must assess the metadata according to their own freshness policy; this getter does not impose an expiry.

### Genesis and Observation Freshness

[`api/oracle/genesis.proto`](../../../api/oracle/genesis.proto) defines `exchange_rate_version`. Version 1 uses `oracle_exchange_rates`, whose entries contain a denom and the complete `OracleExchangeRate`. Export writes this version and preserves observation metadata; import uses `RestoreBaseExchangeRate`, without refreshing the observation time.

Version 0 is the legacy format with scalar `exchange_rates`. These values lack observation metadata, so import deletes any stored rate for each listed denom and leaves it unavailable until a new observation. Validation rejects unknown versions, mixed legacy/versioned rate lists, and invalid versioned entries. See [`genesis.go`](../genesis.go) and [`types/genesis.go`](../types/genesis.go).

## FeederDelegation

The value is the raw `sdk.AccAddress` authorized to feed for an operator. When no record exists, `GetFeederDelegation` returns the operator's address bytes interpreted as an account address. The operator account retains permission even after setting a delegate. Message fields and queries use Paxeer Bech32 strings (`pax` accounts and `paxvaloper` operators), as configured in [`node/params/config.go`](../../../node/params/config.go).

## VotePenaltyCounter

`VotePenaltyCounter` contains `uint64` fields `miss_count`, `abstain_count`, and `success_count`. A missing record reads as all zero. [MidBlock accounting](01_concepts.md#slashing) increments one counter per selected validator per vote period. [EndBlock](03_end_block.md#slash-window-processing-in-endblock) evaluates nonempty counters, emits their values, and deletes them.

## AggregateExchangeRateVote

The protobuf record has `exchange_rate_tuples` (repeated `ExchangeRateTuple`) and `voter` (the validator operator's Bech32 string). Each tuple has `denom` and an `sdk.Dec` `exchange_rate`; the proto represents the decimal as a string custom type.

The record is keyed by validator, so `SetAggregateExchangeRateVote` replaces any existing vote for that validator. Votes remain until `ClearBallots` deletes them at the end of the period's MidBlock tally. They contain no salt, hash, or submission height. See [Messages](04_messages.md#msgaggregateexchangeratevote).

## Vote Targets and Parameters

Each vote target is a `Denom{Name: denom}` record. After tallying, `ApplyWhitelist` compares targets with `Params.Whitelist`, recreates the target set when needed, and registers missing bank denom metadata. Target membership and the stored-rate set are distinct.

Parameters use the `oracle` parameter subspace and the keys declared in [`types/params.go`](../types/params.go): `VotePeriod`, `VoteThreshold`, `RewardBand`, `Whitelist`, `SlashFraction`, `SlashWindow`, `MinValidPerWindow`, and `LookbackDuration`. Defaults are listed in the [README](../README.md#parameters).

## Price Snapshots and TWAPs

`PriceSnapshot` contains `snapshot_timestamp` in Unix **seconds** and repeated items pairing a denom with a complete `OracleExchangeRate`. The nested observation timestamp remains in **milliseconds**. `AddPriceSnapshot` retains snapshots in `LookbackDuration` plus the most recent older snapshot, which can supply a price at the lookback boundary.

`CalculateTwaps` reads stored snapshots for current vote targets. The requested lookback must be positive and no greater than `LookbackDuration`, otherwise it returns `ErrInvalidTwapLookback` (oracle code 16). With no qualifying data it returns `ErrNoTwapData` (code 17). Each result reports the duration actually covered in `lookback_seconds`.

The current oracle MidBlock and EndBlock hooks do not add snapshots. Genesis can restore snapshot history; querying TWAPs does not create it. A successful price tally alone does not establish that TWAP history exists.

## Spam-Prevention Counter

The memory-store entry records the last height admitted by `CheckAndSetSpamPreventionCounter` for a validator. A repeated admission at that height returns the SDK `ErrAlreadyExists`. The [ante decorator](../ante.go) calls it only for non-simulated CheckTx, excluding ReCheckTx. It is an admission guard, not a persistent vote-period record or an additional commit/reveal phase.
