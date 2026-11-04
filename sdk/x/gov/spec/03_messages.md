<!--
order: 3
-->

# Messages

The local [transaction protobuf](../../../proto/cosmos/gov/v1beta1/tx.proto)
defines four governance messages. The [message validation](../types/msgs.go),
[message server](../keeper/msg_server.go), and keepers implement their behavior.
Transactions also pass normal signature, fee, and ante checks. Governance
contributions and voting power are expressed in this chain's coins and bonded
staking state.

## Proposal Submission

`MsgSubmitProposal` has these fields:

| Field           | Type   | Purpose                                         |
| --------------- | ------ | ----------------------------------------------- |
| content         | Any    | Registered proposal content                     |
| initial_deposit | Coins  | Proposer's initial contribution, possibly empty |
| proposer        | string | Signing account address                         |
| is_expedited    | bool   | Select expedited governance rules               |

Basic validation requires a proposer, valid non-negative coins, unpackable
content, a registered proposal type, and successful `Content.ValidateBasic()`.
An empty initial coin list is valid. Title and description are fields of the
content, not fields of this message.

`SubmitProposalWithExpedite` checks for a handler for `Content.ProposalRoute()`.
For `ParameterChangeProposal`, it also checks that the named parameter subspaces
and keys exist. The handler then:

1. Reads the next proposal ID and creates a `Proposal` with content packed in
   `Any`, deposit status, zero final tally totals, submission time, deposit
   deadline, and the message's expedited flag.
2. Stores the proposal, inserts its inactive queue entry, advances the ID counter,
   and invokes the submission hook.
3. Calls `AddDeposit` for the initial contribution, transferring the supplied
   coins to the `gov` module account and updating the proposal and deposit record.
4. Activates voting if the aggregate deposit meets the minimum selected by the
   proposal's expedited flag. This sets voting times and moves its queue entry.
5. Returns `MsgSubmitProposalResponse` containing `proposal_id`.

Content is executed only after a passing deadline tally. A text proposal's
handler records no application changes; other proposal handlers define their
own effects. See [Concepts](01_concepts.md#proposal-types).

## Deposit

`MsgDeposit` contains `proposal_id` (`uint64`), `depositor` (account address
string), and `amount` (`sdk.Coins`). The depositor signs. Basic validation checks
the depositor and coin validity and rejects negative amounts; an empty coin list
is not rejected by this validation.

[AddDeposit](../keeper/deposit.go) requires an existing proposal in either
`StatusDepositPeriod` or `StatusVotingPeriod`. It transfers the contribution from
the depositor to the `gov` module account, increases `TotalDeposit`, and creates
or increases that account's deposit record. A transfer failure, including
insufficient funds, returns an error.

While still in deposit status, meeting `MinDeposit` (or `MinExpeditedDeposit`)
activates voting. Further deposits during voting do not restart the voting
period. Deposit expiry is processed by `EndBlocker`; `AddDeposit` checks status
rather than performing its own wall-clock deadline check. Final refund and burn
rules are described in [Concepts](01_concepts.md#deposit-refund-and-burn).

## Vote

`MsgVote` contains `proposal_id`, `voter`, and `option`. The voter signs. Basic
validation requires a voter and one of `OptionYes` (1), `OptionAbstain` (2),
`OptionNo` (3), or `OptionNoWithVeto` (4).

The message server converts the option into a weighted option with weight one,
then calls [AddVote](../keeper/vote.go). The proposal must exist and have
`StatusVotingPeriod`. The keeper writes the vote at the proposal/voter key,
replacing any prior ordinary or weighted vote, and invokes the vote hook.
It checks proposal status rather than a separate timestamp deadline.

The vote is recorded without requiring the account to have bonded stake. Its
effective power is calculated from current bonded staking state at tally time;
the handler does not increment counters on the proposal. See
[State](02_state.md#votes) and [voting power](01_concepts.md#participants).

## Weighted Vote

`MsgVoteWeighted` contains `proposal_id`, `voter`, and `options`, a non-empty list
of `WeightedVoteOption` records. The voter signs. Each record pairs a valid vote
option with a decimal weight greater than zero and at most one. Options must be
unique and their weights must sum exactly to one.

The message server passes the distribution to the same `AddVote` path. It uses
the same proposal status checks and replaces any existing vote by that voter.
Both vote messages emit weighted options in `proposal_vote.option`; their
transaction actions differ as described in [Events](04_events.md).

## Errors and responses

The keeper uses the `gov` codespace errors in [types/errors.go](../types/errors.go):

| Error                      | Code | Trigger                                 |
| -------------------------- | ---- | --------------------------------------- |
| ErrUnknownProposal         | 2    | Proposal ID not found                   |
| ErrInactiveProposal        | 3    | Proposal status disallows the operation |
| ErrInvalidProposalContent  | 5    | Invalid or missing content              |
| ErrInvalidProposalType     | 6    | Unregistered content type               |
| ErrInvalidVote             | 7    | Invalid option, weight, or distribution |
| ErrNoProposalHandlerExists | 9    | Content route has no handler            |

Address and coin validation, an empty weighted-option list, and bank transfers
can return SDK or bank errors instead. Except for submission's `proposal_id`,
the successful response messages are empty. See [Events](04_events.md) for the
observable deposit, voting-start, and vote attributes.
