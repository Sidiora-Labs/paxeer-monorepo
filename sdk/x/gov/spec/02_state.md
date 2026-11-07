<!--
order: 2
-->

# State

The local [governance protobuf](../../../proto/cosmos/gov/v1beta1/gov.proto)
defines the persisted proposal, deposit, vote, and parameter messages. The
[keeper](../keeper/keeper.go) accesses the module's `gov` KVStore and governance
parameter subspace.

## Parameters and base types

The keeper reads and writes three parameter groups in its parameter subspace,
using the keys in [types/params.go](../types/params.go). There is no versioned
`GlobalParams` store in this implementation.

### DepositParams

Stored under `depositparams`:

- `min_deposit`: minimum coins required to activate regular voting.
- `max_deposit_period`: duration from submission to the deposit deadline.
- `min_expedited_deposit`: minimum coins required to activate expedited voting.

### VotingParams

Stored under `votingparams`:

- `voting_period`: regular voting duration.
- `expedited_voting_period`: expedited voting duration.

### TallyParams

Stored under `tallyparams`:

- `quorum` and `expedited_quorum`: minimum participation fractions.
- `threshold` and `expedited_threshold`: approval fractions that must be exceeded.
- `veto_threshold`: veto fraction that must be exceeded to trigger veto.

These fractions are `sdk.Dec` values. The keeper selects regular or expedited
values using the proposal's `IsExpedited` flag. See
[Concepts](01_concepts.md#threshold) for denominators and boundary comparisons.

### VoteOption

| Go constant        | Protobuf enum                 | Value |
| ------------------ | ----------------------------- | ----- |
| OptionEmpty        | VOTE_OPTION_UNSPECIFIED       | 0     |
| OptionYes          | VOTE_OPTION_YES               | 1     |
| OptionAbstain      | VOTE_OPTION_ABSTAIN           | 2     |
| OptionNo           | VOTE_OPTION_NO                | 3     |
| OptionNoWithVeto   | VOTE_OPTION_NO_WITH_VETO      | 4     |

Only values 1 through 4 are valid votes. A `WeightedVoteOption` pairs an option
with an `sdk.Dec` weight, represented by a string in the protobuf schema.

### ProposalStatus

| Go constant         | Value | Meaning                                      |
| ------------------- | ----- | -------------------------------------------- |
| StatusNil           | 0     | Unspecified                                  |
| StatusDepositPeriod | 1     | Waiting for minimum deposit                  |
| StatusVotingPeriod  | 2     | Voting is active                             |
| StatusPassed        | 3     | Tally passed and content handler succeeded   |
| StatusRejected      | 4     | Regular tally did not pass                   |
| StatusFailed        | 5     | Tally passed but content handler failed      |

A proposal dropped for missing the deposit minimum is deleted, rather than
retained with a terminal status.

## Deposit

A `Deposit` stores `proposal_id`, the depositor's account address string, and
`amount` (`sdk.Coins`). Contributions from the same account to the same proposal
accumulate in one record. The coins are held by the `gov` module account.
Refunding transfers the coins back and deletes the record; deleting deposits
without refunding burns the coins. See [keeper/deposit.go](../keeper/deposit.go).

## ValidatorGovInfo

[ValidatorGovInfo](../types/tally.go) is a temporary tally record, not persisted
state. Its fields are `Address` (`sdk.ValAddress`), `BondedTokens` (`sdk.Int`),
`DelegatorShares` and `DelegatorDeductions` (`sdk.Dec`), and `Vote`
(`WeightedVoteOptions`). The deductions remove power that delegators have voted
themselves before the validator's remaining power is tallied.

## Proposals

A `Proposal` contains:

| Protobuf field       | Stored value                                       |
| -------------------- | -------------------------------------------------- |
| proposal_id          | uint64 identifier                                  |
| content              | Any containing a registered Content implementation |
| status               | ProposalStatus                                     |
| final_tally_result   | yes, abstain, no, and no_with_veto totals          |
| submit_time          | Submission block time                              |
| deposit_end_time     | Deposit deadline                                   |
| total_deposit        | Aggregate sdk.Coins                                |
| voting_start_time    | Time voting was activated                          |
| voting_end_time      | Voting deadline                                    |
| is_expedited         | Whether expedited rules currently apply            |

The Go field for the identifier is `ProposalId`. Title, description, and proposal
type belong to the packed content, not to separate fields of `Proposal` or
`MsgSubmitProposal`. A proposal also does not embed depositor lists, voter lists,
or running vote counters.

The [Content interface](../types/content.go) provides `GetTitle() string`,
`GetDescription() string`, `ProposalRoute() string`, `ProposalType() string`,
`ValidateBasic() error`, and `String() string`. A content handler has signature
`func(ctx sdk.Context, content Content) error`. After a passing tally,
`EndBlocker` invokes the handler in a cached context and commits its state only
on success.

`final_tally_result` starts with zero totals and is written after each deadline
tally, including an expedited tally that converts the proposal to regular voting.
The tally uses decimal voting power internally and truncates each option total
to `sdk.Int` for this stored result.

### Votes

A `Vote` stores `proposal_id`, `voter`, and `options`, a list of weighted options.
The keeper stores ordinary votes as one option of weight one. It clears the
deprecated scalar `option` field in storage. Query helpers populate that legacy
field only when there is exactly one option with weight one; otherwise it is
`OptionEmpty`. A new vote overwrites the record at the same proposal/voter key.

## Stores

The key layout in [types/keys.go](../types/keys.go) is:

| Key                                                   | Value                |
| ----------------------------------------------------- | -------------------- |
| 0x00 + proposalID                                     | Proposal protobuf    |
| 0x01 + endTime + proposalID                           | Active proposal ID   |
| 0x02 + endTime + proposalID                           | Inactive proposal ID |
| 0x03                                                  | Next proposal ID     |
| 0x10 + proposalID + length-prefixed depositor address | Deposit protobuf     |
| 0x20 + proposalID + length-prefixed voter address     | Vote protobuf        |

Proposal IDs use eight-byte big-endian encoding. Queue timestamps use
`sdk.FormatTimeBytes`: UTC text in the fixed-width layout
`2006-01-02T15:04:05.000000000`, without a trailing `Z`. For example, the fixture's
expedited deadline is `2023-11-15T22:13:20.000000000`. Each queue value is the same
eight-byte big-endian proposal ID used in its key, not a proposal protobuf.
Address keys contain a one-byte length followed by the raw address bytes. Prefix
iteration retrieves all deposits or votes for a proposal.

## Proposal Processing Queue

Two time-indexed queues drive [EndBlocker](../abci.go):

- The **inactive queue** contains proposals in the deposit period, keyed by
  deposit end time. At expiry the proposal and queue entry are deleted and its
  deposits are burned.
- The **active queue** contains proposals in voting status, keyed by voting end
  time. Activation removes the inactive entry and adds the active entry.

For each due active proposal, the keeper tallies current bonded staking power
and weighted votes. It removes the old active entry, settles deposits for a final
outcome, and either executes passing content or records rejection. Execution
failure records `StatusFailed` while discarding the handler's state changes.

A failed expedited tally instead clears `IsExpedited`, preserves votes and
deposits, and inserts an active entry at the original voting start time plus the
regular voting period. The proposal stays in `StatusVotingPeriod`. Regular
tallies delete vote records; an expedited tally deletes them only if it passes.
See [the lifecycle](01_concepts.md#expedited-voting-period) and
[result events](04_events.md#endblocker).
