# Oracle module specification

The Paxeer X chain has an `oracle` module that provides asset exchange rates for other modules and contracts. Bonded validators are expected to submit exchange-rate votes.

Voting is a single step. Every vote period, each validator (or its delegated feeder) submits a `MsgAggregateExchangeRateVote` with its proposed rates. On the last block of the period, the module tallies qualifying ballots using voting-power-weighted medians and cross rates. This mid-block hook runs after transaction execution during block completion. Later hooks can read the updated rates; transactions use them from the following block. See [MidBlock design](MidBlock.md#price-visibility).

Each validator has success, abstain, and miss counters. At the end of every slash window, the end blocker evaluates nonempty counters and can slash and jail a bonded, unjailed validator whose valid vote rate is below `MinValidPerWindow`. It then emits the counts and deletes those records. The default minimum valid-vote rate and slash fraction are both zero, so the defaults do not penalize non-participation. See [Parameters](06_params.md).

The current behavior, parameters, CLI, and defaults are summarized in the module [README](../README.md).

## Contents

1. **[Concepts](01_concepts.md)**
2. **[State](02_state.md)**
3. **[End Block](03_end_block.md)**
4. **[Messages](04_messages.md)**
5. **[Events](05_events.md)**
6. **[Parameters](06_params.md)**
7. **[MidBlock design](MidBlock.md)**
