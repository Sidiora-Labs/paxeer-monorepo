<!--
order: 4
-->

# Events

The governance [keepers](../keeper/msg_server.go) and [EndBlocker](../abci.go)
emit the event types and values in [types/events.go](../types/events.go).
Transaction message actions are supplied separately by BaseApp.

## EndBlocker

| Type              | Attribute Key   | Attribute Value  |
| ----------------- | --------------- | ---------------- |
| inactive_proposal | proposal_id     | {proposalID}     |
| inactive_proposal | proposal_result | proposal_dropped |
| active_proposal   | proposal_id     | {proposalID}     |
| active_proposal   | proposal_result | {proposalResult} |

An inactive proposal is dropped after its deposit deadline without meeting the
minimum. An active proposal's result is one of:

| Result                       | Meaning                                            |
| ---------------------------- | -------------------------------------------------- |
| proposal_passed              | Tally passed and content execution succeeded       |
| proposal_failed              | Tally passed but content execution failed          |
| proposal_rejected            | Regular proposal did not pass its tally            |
| expedited_proposal_converted | Failed expedited tally converted to regular voting |

Conversion keeps the proposal in voting status. See
[Concepts](01_concepts.md#expedited-voting-period) for retained votes, deposits,
and the extended deadline. Successful content handlers may also emit events;
`EndBlocker` propagates those events from its cached execution context.

## Handlers

### MsgSubmitProposal

| Type             | Attribute Key       | Attribute Value   |
| ---------------- | ------------------- | ----------------- |
| submit_proposal  | proposal_id         | {proposalID}      |
| submit_proposal  | proposal_type       | {contentType}     |
| submit_proposal  | voting_period_start | {proposalID}      |
| proposal_deposit | amount              | {depositAmount}   |
| proposal_deposit | proposal_id         | {proposalID}      |
| message          | module              | governance        |
| message          | sender              | {proposerAddress} |

`voting_period_start` is present only if the initial deposit activates voting.
It contains the proposal ID, not a timestamp. The submission keeper emits
`proposal_id` in one `submit_proposal` event; the message server emits
`proposal_type` and the optional voting-start attribute in another. The type is
`Content.ProposalType()`, for example `Text`.

### MsgVote

| Type          | Attribute Key | Attribute Value       |
| ------------- | ------------- | --------------------- |
| proposal_vote | option        | {weightedVoteOptions} |
| proposal_vote | proposal_id   | {proposalID}          |
| message       | module        | governance            |
| message       | sender        | {voterAddress}        |

Even an ordinary vote is emitted as one weighted option with weight one.

### MsgVoteWeighted

| Type          | Attribute Key | Attribute Value       |
| ------------- | ------------- | --------------------- |
| proposal_vote | option        | {weightedVoteOptions} |
| proposal_vote | proposal_id   | {proposalID}          |
| message       | module        | governance            |
| message       | sender        | {voterAddress}        |

For both vote messages, `option` is the output of
[`WeightedVoteOptions.String()`](../types/vote.go): one JSON object per option,
separated by newlines. See [Messages](03_messages.md#weighted-vote) for the input
validation rules.

### MsgDeposit

| Type             | Attribute Key       | Attribute Value    |
| ---------------- | ------------------- | ------------------ |
| proposal_deposit | amount              | {depositAmount}    |
| proposal_deposit | proposal_id         | {proposalID}       |
| proposal_deposit | voting_period_start | {proposalID}       |
| message          | module              | governance         |
| message          | sender              | {depositorAddress} |

The keeper emits `amount` and `proposal_id` together. The message server emits a
separate `proposal_deposit` event containing `voting_period_start` only if this
deposit starts voting. Amounts use `sdk.Coins.String()`.

## Transaction message actions

[BaseApp](../../../baseapp/baseapp.go) adds `message.action` for every successfully
handled message. Its value depends on the dispatch path: protobuf message service
handling uses the full message type URL, while legacy dispatch uses `Type()`.
The [governance module](../module.go) registers the protobuf message service.

| Message           | Protobuf service action                   | Legacy Type()    |
| ----------------- | ----------------------------------------- | ---------------- |
| MsgSubmitProposal | /cosmos.gov.v1beta1.MsgSubmitProposal     | submit_proposal  |
| MsgVote           | /cosmos.gov.v1beta1.MsgVote               | vote             |
| MsgVoteWeighted   | /cosmos.gov.v1beta1.MsgVoteWeighted       | weighted_vote    |
| MsgDeposit        | /cosmos.gov.v1beta1.MsgDeposit            | deposit          |

The weighted legacy action is `weighted_vote`, as defined by
`TypeMsgVoteWeighted` in [types/msgs.go](../types/msgs.go). BaseApp's action event
and the handler's module/sender event are separate `message` events. Event
consumers should match the value emitted by the dispatch path they use.
