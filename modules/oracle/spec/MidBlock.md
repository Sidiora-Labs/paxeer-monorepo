# Oracle MidBlock Spec

## Context

The `oracle` module is part of the Paxeer network's node application. Validators,
or their authorized feeders, submit `MsgAggregateExchangeRateVote` transactions.
The [message server](../keeper/msg_server.go) validates the feeder and vote
targets and stores each validator's aggregate vote. Price calculation is
implemented by [`MidBlocker`](../abci.go), exposed through
[`AppModule.MidBlock`](../module.go).

The default `VotePeriod` is **2 blocks**. MidBlock runs at every block's
completion, but the oracle tallies only when
`(height + 1) % VotePeriod == 0`, as defined by
[`IsPeriodLastBlock`](../utils/period.go). Setting `VotePeriod` to 1 makes every
block a tally block. See [Parameters](06_params.md) for current defaults.

## Design

The implemented order in [`App.ProcessBlock` and `App.CompleteBlock`](../../../node/app.go)
is:

1. Run `BeginBlock`.
2. Execute transactions at their block positions. For a sequenced block,
   validate and apply the sequence batch at transaction position zero before
   executing the remaining transactions. Oracle votes use this transaction
   execution path along with other transactions.
3. Run `CompleteBlock`, which calls `MidBlock`. The module manager invokes
   the oracle hook first and the pricefeed hook second.
4. Complete deferred balance writes and run `EndBlock`.

The proposer also calls `CompleteBlock` when sealing an already executed block
through [`node/sequencer/seal.go`](../../../node/sequencer/seal.go).
There is no oracle-only transaction phase before ordinary transactions in this
path. The earlier proposal to partition votes, tally them, and then execute
other transactions does not describe the current implementation.

### Tally and cleanup

On the last block of each vote period, [`MidBlocker`](../abci.go):

1. Builds claims for the active bonded validator set and organizes stored votes
   into sorted ballots per denom. Non-positive exchange-rate votes have zero
   voting power.
2. Checks ballots for current vote targets against `VoteThreshold` of total
   bonded voting power. The passing ballot with the greatest power supplies
   the reference denom, with alphabetical order breaking ties.
3. Computes the reference ballot's weighted median, converts other passing
   ballots to cross rates, tallies them, and converts them back to base rates.
   Each nonzero result is stored by `SetBaseExchangeRateWithEvent`, emitting
   `exchange_rate_update`.
4. Updates validator success, abstain, and miss counters. Votes in ballots below
   the threshold count toward participation without producing a new price.
5. Clears aggregate ballots and synchronizes vote targets with `Whitelist`.

[`EndBlocker`](../abci.go) performs the separate slash-window work: on the last
block of `SlashWindow`, it calls `SlashAndResetCounters` and `RemoveExcessFeeds`.
The [node end-block wiring](../../../node/legacyabci/end_block.go) invokes this
hook; EndBlock does not perform the oracle price tally.

### Price visibility

For a tally block N, the new rates are stored during completion of N, after its
ordinary transactions have executed. Later hooks can read those updates, and
transactions in the following block can use them. An ordinary transaction in
N cannot rely on prices derived from N's votes, even with `VotePeriod = 1`.
A denom whose ballot fails the voting-power threshold receives no new price
from that tally; an existing rate can therefore remain older than the latest
vote period.
