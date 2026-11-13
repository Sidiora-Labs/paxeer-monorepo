# Oracle

This directory holds the native `oracle` module, including its keeper, message and query services, block hooks, and tests. It stores validator-voted asset exchange rates in Paxeer's chain state. Paxeer mainnet-beta is one BFT network, with the kernel participating in block proposals, EVM chain ID 125 (`0x7d`), and finality when a block is committed. Oracle messages and queries use the native module interfaces described below.

## Layout

- [`client/`](client/) — transaction and query CLI commands, REST routes, and CosmWasm query bindings.
- [`keeper/`](keeper/) — persistent rates, ballots, feeder delegations, vote targets, snapshots, parameters, query and message servers, migrations, and keeper tests.
- [`simulation/`](simulation/) — randomized genesis, parameter changes, store decoding, and simulated transactions.
- [`spec/`](spec/) — module concepts, state, block lifecycle, messages, events, parameters, and MidBlock design.
- [`types/`](types/) — message and query types, generated protobuf bindings, genesis and parameter validation, ballots, snapshots, codecs, and type tests.
- [`utils/`](utils/) — denomination constants and block-period helpers.
- [`module.go`](module.go) — module registration, service wiring, genesis, migrations, and simulation integration.
- [`abci.go`](abci.go) — vote-period aggregation and slash-window processing.
- [`tally.go`](tally.go) — reference-denomination selection, weighted-median tallying, and vote participation accounting.
- [`ante.go`](ante.go) — vote spam prevention and transaction message restrictions.
- [`handler.go`](handler.go) — routing for aggregate votes and feeder delegation messages.
- [`genesis.go`](genesis.go) — module state initialization and export.
- [`metrics.go`](metrics.go) — block-hook duration and price-update instruments.
- [`common_test.go`](common_test.go) — shared module test setup; the other Go test files exercise block hooks, tallying, messages, genesis, ante handlers, and events.

### Native interfaces and block lifecycle

The legacy EVM oracle precompile at `0x0000000000000000000000000000000000001008` is retired. Its `getExchangeRates` and `getOracleTwaps` methods reject otherwise valid calls with `ErrOraclePrecompileRetired`: `oracle precompile is retired; oracle data queries are disabled`. See [`precompiles/oracle/oracle.go`](../../precompiles/oracle/oracle.go); it is not a current contract interface for reading this module's prices.

Native Cosmos queries remain available through the oracle query service (CLI, gRPC and REST), and the wasm bindings retain exchange-rate and TWAP queries. Current EVM price consumers use the supported [PriceFeed contract](../../precompiles/pricefeed/PriceFeed.sol) at `0x0000000000000000000000000000000000001020`, backed by `modules/pricefeed`; that interface has its own market identifiers and observation policy and is not the retired oracle ABI.

Voting is a single step: each bonded validator, or its delegated feeder, submits a `MsgAggregateExchangeRateVote` with rates for the current vote targets. On the last block of each vote period, `MidBlocker` tallies ballots using validator voting power. The node calls this hook in `CompleteBlock`, after transaction execution and before EndBlock. See [the block lifecycle](spec/03_end_block.md).

Each validator has success, abstain, and miss counters. EndBlock evaluates them at the end of each slash window and can slash and jail a bonded, unjailed validator whose valid vote rate is below `MinValidPerWindow`. The default minimum and slash fraction are both zero, so the defaults do not penalize non-participation. Query live parameters before assuming a penalty policy.

### Voting procedure

Votes are collected per denom into ballots. In the mid-block step of the last block of each vote period (`VotePeriod` blocks):

1. Among the vote targets whose ballots have nonzero power and reach the rounded `VoteThreshold * totalBondedPower`, the one with the highest voting power becomes the reference denom; equal power is resolved by denom name in ascending order. Ballots below the threshold do not update prices; submitting a tuple in such a ballot counts as a win for that target.
2. The reference rate is the weighted median of its ballot. Other ballots are converted to cross rates against the reference denom, tallied, and converted back.
3. Each nonzero tally result is stored with its block height and timestamp and emits an `exchange_rate_update` event. Existing rates are retained when a period supplies no replacement; presence alone does not establish freshness.
4. Ballots are cleared and the vote targets are re-synced with the `Whitelist` param.

### Reward band

For a passing ballot, a vote counts as a win when it has positive voting power and is within an inclusive spread of `max(median * RewardBand / 2, ballot standard deviation)` around the weighted median, using cross rates for non-reference denoms. The current tally tracks wins but does not distribute token rewards. See [Reward Band](spec/01_concepts.md#reward-band).

### Slashing

On the last block of each slash window (`SlashWindow` blocks), the end blocker computes each nonempty counter's valid vote rate as `success / (success + abstain + miss)`. If it is below `MinValidPerWindow`, a bonded, unjailed validator is slashed by `SlashFraction` and jailed. An `end_slash_window` event reports the counts, then the counter is deleted. EndBlock also removes stored rates whose denoms are no longer vote targets.

### Abstaining from voting

A stored tuple with a non-positive rate carries zero voting power. This differs from the period's abstain counter: a validator that wins every target receives a success; otherwise, one whose participation was not recorded by the tally receives an abstain; the rest receive a miss. Explicit zero-rate tuples can still record participation. See [Abstaining from Voting](spec/01_concepts.md#abstaining-from-voting).

### Messages

- `MsgAggregateExchangeRateVote` (`exchange_rates`, `feeder`, `validator`) — submits a comma-separated list of decimal rates and target denoms. There is no prevote or salt.
- `MsgDelegateFeedConsent` — lets a validator delegate voting to a separate feeder address.

### Events

`exchange_rate_update`, `aggregate_vote`, `feed_delegate`, and `end_slash_window`; see the [event attributes and emitting hooks](spec/05_events.md).

### Default parameters

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

### Transactions

```bash
paxd tx oracle aggregate-vote '<rate><denom>,<rate><denom>' [validator] --from <feeder-key>
paxd tx oracle set-feeder <feeder-address> --from <validator-operator-key>
```

The validator argument defaults to the signer's operator address; a delegated feeder must specify the validator it represents. Use `pax` account addresses and `paxvaloper` operator addresses. See [message validation](spec/04_messages.md).

### Queries

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

## Build and test

The module belongs to the repository's root [`go.mod`](../../go.mod) and has no separate manifest or Makefile. From the repository root, use the targets in [`Makefile`](../../Makefile):

```sh
make paxeer-build
make paxeer-test
```

`paxeer-build` delegates to [`chain.mk`](../../chain.mk) to build `paxd`, including the Oracle module and its native library prerequisites. `paxeer-test` delegates to the chain test target, which discovers Go packages with tests and runs them in groups with the race detector and coverage, including the Oracle packages. Both targets set `GOPROXY=off`, so Go dependencies must already be available locally.

## Configuration

The keeper reads the on-chain Oracle parameter subspace, initialized from the module genesis state by [`InitGenesis`](genesis.go). The parameter keys declared in [`types/params.go`](types/params.go) and read in [`keeper/params.go`](keeper/params.go) are `VotePeriod`, `VoteThreshold`, `RewardBand`, `Whitelist`, `SlashFraction`, `SlashWindow`, `MinValidPerWindow`, and `LookbackDuration`.

The module reads no module-specific environment variables or configuration files directly. Query the active settings with `paxd q oracle params` and the accepted denominations with `paxd q oracle vote-targets`.

## Documentation

- [Oracle specification index](spec/README.md)
- [Concepts](spec/01_concepts.md)
- [State and freshness](spec/02_state.md)
- [MidBlock and EndBlock](spec/03_end_block.md)
- [Messages](spec/04_messages.md)
- [Events](spec/05_events.md)
- [Parameters](spec/06_params.md)
- [MidBlock design](spec/MidBlock.md)
- [Chain module parameters and messages](../../docs/system/chain-modules.md#oracle)
- [Oracle documentation site page](../../docs/site/paxeer-docs/src/app/modules/oracle/page.tsx)
