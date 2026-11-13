# Oracle

## Abstract

The `oracle` module stores validator-voted asset exchange rates in Paxeer's chain state. Paxeer mainnet-beta is one BFT network, with the kernel participating in block proposals, EVM chain ID 125 (`0x7d`), and finality when a block is committed. Oracle messages and queries use the native module interfaces described below.

The legacy EVM oracle precompile at `0x0000000000000000000000000000000000001008` is retired. Its `getExchangeRates` and `getOracleTwaps` methods reject otherwise valid calls with `ErrOraclePrecompileRetired`: `oracle precompile is retired; oracle data queries are disabled`. See [`precompiles/oracle/oracle.go`](../../precompiles/oracle/oracle.go); it is not a current contract interface for reading this module's prices.

Native Cosmos queries remain available through the oracle query service (CLI, gRPC and REST), and the wasm bindings retain exchange-rate and TWAP queries. Current EVM price consumers use the supported [PriceFeed contract](../../precompiles/pricefeed/PriceFeed.sol) at `0x0000000000000000000000000000000000001020`, backed by `modules/pricefeed`; that interface has its own market identifiers and observation policy and is not the retired oracle ABI.

Voting is a single step: each bonded validator, or its delegated feeder, submits a `MsgAggregateExchangeRateVote` with rates for the current vote targets. On the last block of each vote period, `MidBlocker` tallies ballots using validator voting power. The node calls this hook in `CompleteBlock`, after transaction execution and before EndBlock. See [the block lifecycle](spec/03_end_block.md).

Each validator has success, abstain, and miss counters. EndBlock evaluates them at the end of each slash window and can slash and jail a bonded, unjailed validator whose valid vote rate is below `MinValidPerWindow`. The default minimum and slash fraction are both zero, so the defaults do not penalize non-participation. Query live parameters before assuming a penalty policy.

## Contents

- [Concepts](spec/01_concepts.md)
- [State and freshness](spec/02_state.md)
- [MidBlock and EndBlock](spec/03_end_block.md)
- [Messages](spec/04_messages.md)
- [Events](spec/05_events.md)

## Concepts

### Voting Procedure

Votes are collected per denom into ballots. In the mid-block step of the last block of each vote period (`VotePeriod` blocks):

1. Among the vote targets whose ballots have nonzero power and reach the rounded `VoteThreshold * totalBondedPower`, the one with the highest voting power becomes the reference denom; equal power is resolved by denom name in ascending order. Ballots below the threshold do not update prices; submitting a tuple in such a ballot counts as a win for that target.
2. The reference rate is the weighted median of its ballot. Other ballots are converted to cross rates against the reference denom, tallied, and converted back.
3. Each nonzero tally result is stored with its block height and timestamp and emits an `exchange_rate_update` event. Existing rates are retained when a period supplies no replacement; presence alone does not establish freshness.
4. Ballots are cleared and the vote targets are re-synced with the `Whitelist` param.

### Reward Band

For a passing ballot, a vote counts as a win when it has positive voting power and is within an inclusive spread of `max(median * RewardBand / 2, ballot standard deviation)` around the weighted median, using cross rates for non-reference denoms. The current tally tracks wins but does not distribute token rewards. See [Reward Band](spec/01_concepts.md#reward-band).

### Slashing

On the last block of each slash window (`SlashWindow` blocks), the end blocker computes each nonempty counter's valid vote rate as `success / (success + abstain + miss)`. If it is below `MinValidPerWindow`, a bonded, unjailed validator is slashed by `SlashFraction` and jailed. An `end_slash_window` event reports the counts, then the counter is deleted. EndBlock also removes stored rates whose denoms are no longer vote targets.

### Abstaining from Voting

A stored tuple with a non-positive rate carries zero voting power. This differs from the period's abstain counter: a validator that wins every target receives a success; otherwise, one whose participation was not recorded by the tally receives an abstain; the rest receive a miss. Explicit zero-rate tuples can still record participation. See [Abstaining from Voting](spec/01_concepts.md#abstaining-from-voting).

## Messages

- `MsgAggregateExchangeRateVote` (`exchange_rates`, `feeder`, `validator`) — submits a comma-separated list of decimal rates and target denoms. There is no prevote or salt.
- `MsgDelegateFeedConsent` — lets a validator delegate voting to a separate feeder address.

## Events

`exchange_rate_update`, `aggregate_vote`, `feed_delegate`, and `end_slash_window`; see the [event attributes and emitting hooks](spec/05_events.md).

## Parameters

| Param | Default |
| ----- | ------- |
| `VotePeriod` | 2 blocks |
| `VoteThreshold` | 0.667 |
| `RewardBand` | 0.02 |
| `Whitelist` | `uatom`, `ueth` |
| `SlashFraction` | 0 |
| `SlashWindow` | 216000 blocks (`2 * BlocksPerDay`) |
| `MinValidPerWindow` | 0 |
| `LookbackDuration` | 3600 seconds |

These are [`DefaultParams`](types/params.go), not a statement of deployed parameter values. `BlocksPerDay` is a fixed block-count convention in [`utils/period.go`](utils/period.go), not a wall-clock guarantee.

## Transactions

```bash
paxd tx oracle aggregate-vote '<rate><denom>,<rate><denom>' [validator] --from <feeder-key>
paxd tx oracle set-feeder <feeder-address> --from <validator-operator-key>
```

The validator argument defaults to the signer's operator address; a delegated feeder must specify the validator it represents. Use `pax` account addresses and `paxvaloper` operator addresses. See [message validation](spec/04_messages.md).

## Queries

```bash
paxd q oracle exchange-rates [denom]
paxd q oracle price-snapshot-history
paxd q oracle twaps [lookback-seconds]
paxd q oracle actives
paxd q oracle params
paxd q oracle feeder [validator]
paxd q oracle vote-penalty-counter [validator]
paxd q oracle vote-targets
```

`actives` lists denoms with stored rates; `vote-targets` lists denoms accepted for voting. Exchange-rate responses include observation height and time. Snapshot history and TWAP queries read stored snapshots; the current oracle block hooks do not produce new snapshots. See [state](spec/02_state.md) and the [specification index](spec/README.md).
