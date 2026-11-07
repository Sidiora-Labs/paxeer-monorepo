<!--
order: 1
-->

# Concepts

The governance module runs on the Paxeer X chain as wired in
[node/app.go](../../../../node/app.go). Paxeer is one BFT network: its kernel is
part of the network and proposes blocks, and a receipt is final when its block
is committed. Governance proposals change chain state through registered content
handlers.

The lifecycle is:

- **Proposal submission:** An account submits content and an initial deposit,
  which may be empty, using `MsgSubmitProposal`.
- **Deposit:** Contributions accumulate until the applicable minimum deposit is
  met, or the deposit deadline expires.
- **Vote:** During the voting period, accounts submit ordinary or weighted votes.
  Bonded staking power determines their effect at tally time.
- **Tally and execution:** At the voting deadline, `EndBlocker` tallies the votes
  and executes the content handler if the proposal passes.

See [State](02_state.md), [Messages](03_messages.md), and [Events](04_events.md)
for the stored fields, transaction interfaces, and emitted results.

## Proposal submission

### Right to submit a proposal

Any account can submit a proposal; the handler does not require the proposer to
be a validator or delegator. The proposer signs `MsgSubmitProposal` and pays the
initial deposit, if supplied. Each accepted submission receives a unique
`proposal_id` from the keeper's next-ID counter.

### Proposal types

Proposal content is packed in protobuf `Any` and must implement the
[Content interface](../types/content.go). Its proposal type must be registered,
and its `ProposalRoute()` must identify a registered content handler.

Common content types include:

- `TextProposal`: records a decision or signal; its handler does not apply
  application changes.
- `SoftwareUpgradeProposal`: schedules a named upgrade plan at a block height.
- `CancelSoftwareUpgradeProposal`: clears the scheduled upgrade plan.
- `CommunityPoolSpendProposal`: requests a community-pool transfer to a recipient.
- `ParameterChangeProposal`: requests updates to existing module parameters.

This is not a closed list. The chain application's handler registrations in
[node/app.go](../../../../node/app.go) include additional module proposal types,
with Wasm proposals conditional on the enabled proposal configuration. A passing
proposal executes the corresponding handler in a cached context: success commits
its changes and records `StatusPassed`; an execution error discards those changes
and records `StatusFailed`.

### Expedited Proposals

The submission message's `is_expedited` field selects the expedited deposit,
voting period, quorum, and approval threshold. The keeper stores that flag on the
proposal. Use the current governance parameters for these values; SDK defaults
are not a statement of deployed network settings.

If an expedited tally does not pass, `EndBlocker` converts the proposal to regular
voting. This applies to every failed tally, including failure from insufficient
quorum or veto. The conversion behavior is described under
[Expedited Voting period](#expedited-voting-period).

## Deposit

The [deposit keeper](../keeper/deposit.go) holds contributions in the `gov` module
account. `MsgSubmitProposal` accepts an empty initial coin list. Any non-empty
coin list must be valid SDK coins, and the contributor must have sufficient
funds. Other accounts can contribute with `MsgDeposit`.

Voting starts when total deposits are greater than or equal to every coin amount
in `MinDeposit`, or `MinExpeditedDeposit` for an expedited proposal. Contributions
may continue while the proposal is in either deposit or voting status. Activation
sets the voting times and moves the proposal from the inactive queue to the
active queue.

If the minimum is not met, `EndBlocker` deletes the proposal and burns its deposits
when the deposit deadline is reached. The deadline is based on submission time
plus `MaxDepositPeriod`.

### Deposit refund and burn

At the final tally, deposits are normally refunded to their contributors. The
[tally implementation](../keeper/tally.go) requests burning when quorum is not met
or the veto fraction exceeds `VetoThreshold`. With no bonded stake it does not
request burning. Once quorum is met, an all-abstain result also does not request
burning. A passing proposal's deposits are refunded even if its content handler
subsequently fails.

Failed expedited proposals retain their deposits during conversion to regular
voting; the later regular tally determines refund or burn.

## Vote

### Participants

Accounts may record votes while the proposal is in voting status. The vote
handler does not require a staking balance. At tally time, voting power comes
from delegations to the current bonded validators; a vote without such power has
no weight. The tally reads current staking state, rather than a snapshot of the
validator set or delegations at the start of voting.

### Voting period

When deposits activate voting, `VotingStartTime` becomes the current block time
and `VotingEndTime` becomes that time plus the applicable voting period. The
regular duration is `VotingParams.VotingPeriod`. Proposals are tallied by
`EndBlocker` at their deadline; this module does not close voting early when a
particular level of support is reached.

### Expedited Voting period

An expedited proposal starts with `VotingParams.ExpeditedVotingPeriod`. When its
tally fails, conversion clears the proposal's `IsExpedited` flag, keeps its voting
status, votes, and deposits, and sets its end time to the original
`VotingStartTime + VotingParams.VotingPeriod`. It is requeued for a regular tally;
the extension is not a fresh full voting period from the conversion time.

For example, a delegator holding 6 of 16 bonded tokens can cast a minority Yes
vote before conversion. Its 37.5% participation misses the default expedited
quorum (66.7%) but meets the regular quorum (33.4%). If no one changes that vote,
the regular tally passes. Votes and deposits survive a committed-store restart
between those tallies; conversion does not require accounts to vote again.

### Option set

The protobuf `VoteOption` values are:

| Option       | Value | Meaning                               |
| ------------ | ----- | ------------------------------------- |
| Yes          | 1     | Support the proposal                  |
| Abstain      | 2     | Participate without support or veto   |
| No           | 3     | Oppose the proposal                   |
| NoWithVeto   | 4     | Oppose and count toward the veto test |

Value 0 is unspecified and is not a valid vote. Abstentions count toward quorum
but are excluded from the approval denominator. Veto uses total participating
power, including abstentions, as its denominator.

### Weighted Votes

[Weighted voting](../types/vote.go) splits voting power across options, for
example 70% `Yes` and 30% `No`. An account can
submit this distribution with `MsgVoteWeighted`. Each option must be valid and
unique, each decimal weight must be greater than zero and at most one, and the
weights must sum exactly to one. `MsgVote` is stored as a single option with
weight one. A later vote replaces the account's earlier vote on that proposal.

### Quorum

Quorum compares total participating voting power, including abstentions, with
total bonded tokens. Participation must be greater than or equal to `Quorum`, or
`ExpeditedQuorum` for an expedited proposal. With no bonded tokens the proposal
cannot pass.

### Threshold

After quorum is met, the tally rejects an all-abstain result. It then rejects a
veto fraction **strictly greater than** `VetoThreshold`, before checking approval.
The `Yes` share of non-abstaining power must be **strictly greater than**
`Threshold`, or `ExpeditedThreshold` for an expedited proposal. Equality with the
approval threshold does not pass; equality with the veto threshold does not veto.

### Inheritance

The tally first applies each voting delegator's weighted options to that
account's delegated power and deducts its shares from the corresponding
validator's inherited power. A validator's vote then applies to the remaining
power of its delegators. Thus an explicit delegator vote overrides inheritance
regardless of whether it was submitted before or after the validator's vote.

### Validator's punishment for non-voting

The governance tally does not slash validators for failing to vote.

### Governance address

Governance messages are signed by account addresses: the proposer, depositor, or
voter. The tally recognizes a validator vote by converting the voter's account
address bytes to a validator operator address. Governance transactions do not
use the consensus signing key.

## Software Upgrade

### Schedule

The [software-upgrade proposal handler](../../upgrade/handler.go) calls
`ScheduleUpgrade` with the proposal's `Plan`. A plan has a non-empty `name`, a
positive `height`, and optional `info`. Time-based plans are rejected, as are
plans scheduled below the current height or with an already completed name.
Scheduling stores one plan; a new plan replaces the pending plan. A passing
`CancelSoftwareUpgradeProposal` clears it.

### Apply

The [upgrade BeginBlocker](../../upgrade/abci.go) checks the plan at its target
height. A binary with the matching named handler applies the upgrade. Without
that handler, the node writes upgrade information to disk and halts so the
appropriate binary can be installed. The pending plan remains available after
that halt; the installed handler applies it on restart at the scheduled height,
records completion and clears the plan. A completed name cannot be rescheduled.
A configured skip height instead clears the pending plan without applying it.
For a non-minor upgrade, starting a binary with the handler before the scheduled
height aborts. A plan whose `info` has `"upgradeType":"minor"` permits the new
binary to start early; application still occurs at the scheduled height.
Operators must coordinate binary deployment
for the scheduled height; proposal approval does not download or switch binaries
through proposal-ID signals in precommits. See the
[upgrade module](../../upgrade/spec/01_concepts.md) for upgrade operation details.
