<!--
order: 6
-->

# Events

Distribution event names and module-specific attribute keys are declared in
[types/events.go](../types/events.go). Amount attributes contain SDK coin strings;
allocation events use decimal coins and withdrawals use integer coins. Validator
attributes identify operator addresses.

## BeginBlocker

[BeginBlocker](../abci.go) allocates the previous block's collected fees after
height `1`. [AllocateTokens](../keeper/allocation.go) emits:

| Type | Attribute Key | Attribute Value |
| --- | --- | --- |
| proposer_reward | amount | {proposerReward} |
| proposer_reward | validator | {validatorOperatorAddress} |
| commission | amount | {commissionAmount} |
| commission | validator | {validatorOperatorAddress} |
| rewards | amount | {allocatedTokens} |
| rewards | validator | {validatorOperatorAddress} |

The proposer reward key is `amount` (`sdk.AttributeKeyAmount`). The
`proposer_reward` event is emitted only when the previous proposer's validator is
found. Each call to `AllocateTokensToValidator` emits `commission` and `rewards`;
`rewards.amount` is the full allocation before commission, while the validator's
current shared rewards exclude commission. If previous total voting power is
zero, allocation sends all collected fees to the community pool and returns
without these allocation events.

## Handlers

The following events come from the keeper and
[message server](../keeper/msg_server.go). Each successful message also emits a
`message` event with `module=distribution` and `sender` as listed below.

For messages dispatched through the registered protobuf Msg service,
[BaseApp](../../../baseapp/baseapp.go) adds a separate `message` event with an
`action` attribute containing the full message type URL:

| Message | `message.sender` | `message.action` |
| --- | --- | --- |
| `MsgSetWithdrawAddress` | {delegatorAddress} | `/cosmos.distribution.v1beta1.MsgSetWithdrawAddress` |
| `MsgWithdrawDelegatorReward` | {delegatorAddress} | `/cosmos.distribution.v1beta1.MsgWithdrawDelegatorReward` |
| `MsgWithdrawValidatorCommission` | {validatorOperatorAddress} | `/cosmos.distribution.v1beta1.MsgWithdrawValidatorCommission` |
| `MsgFundCommunityPool` | {depositorAddress} | `/cosmos.distribution.v1beta1.MsgFundCommunityPool` |

The short operation names used by legacy message dispatch are not the action
values of the protobuf Msg-service path.

### MsgSetWithdrawAddress

| Type | Attribute Key | Attribute Value |
| --- | --- | --- |
| set_withdraw_address | withdraw_address | {withdrawAddress} |

### MsgWithdrawDelegatorReward

| Type | Attribute Key | Attribute Value |
| --- | --- | --- |
| withdraw_rewards | amount | {integerRewardCoins} |
| withdraw_rewards | validator | {validatorOperatorAddress} |

A withdrawal of zero rewards reports a zero coin in the configured base
denomination, falling back to the SDK default bond denomination when no base
denomination is registered. The staking hooks can also emit this keeper event
when withdrawing rewards before shares change.

### MsgWithdrawValidatorCommission

| Type | Attribute Key | Attribute Value |
| --- | --- | --- |
| withdraw_commission | amount | {integerCommissionCoins} |

### MsgFundCommunityPool

The message server emits the common `message` attributes; the keeper emits no
additional distribution-specific funding event.

See [Messages](04_messages.md) for withdrawal accounting and
[Parameters](07_params.md) for the allocation rates.
