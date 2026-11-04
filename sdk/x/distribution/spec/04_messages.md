<!--
order: 4
-->

# Messages

The distribution messages execute in `paxd` on the merged network. The local
[transaction definitions](../../../proto/cosmos/distribution/v1beta1/tx.proto)
and [message server](../keeper/msg_server.go) define the four operations below.
Each successful operation returns its corresponding empty protobuf response;
withdrawal amounts are reported in [events](06_events.md).

## MsgSetWithdrawAddress

By default, rewards are paid to the delegator's account address. The delegator
signs `MsgSetWithdrawAddress` with `delegator_address` and `withdraw_address` to
change that destination. The `WithdrawAddrEnabled` [parameter](07_params.md)
must be `true`.

[SetWithdrawAddr](../keeper/keeper.go) checks the following conditions in order:

1. An address in the keeper's `blockedAddrs` map is rejected with
   `sdkerrors.ErrUnauthorized`.
2. If changing withdrawal addresses is disabled, it returns
   `ErrSetWithdrawAddrDisabled` (distribution code `8`).
3. If `bankKeeper.CanSendTo` rejects the recipient, it returns
   `sdkerrors.ErrInvalidRecipient`.

On success, the keeper emits `set_withdraw_address` and stores the destination.
When resolving a stored destination, `GetDelegatorWithdrawAddr` checks the
blocked-address map and bank recipient policy again. If the stored address is no
longer allowed, it returns the delegator's own address.

Response: `MsgSetWithdrawAddressResponse {}`.

## MsgWithdrawDelegatorReward

The delegator signs `MsgWithdrawDelegatorReward` with `delegator_address` and
`validator_address` to withdraw rewards from one validator. The keeper requires
both the validator and delegation to exist, returning `ErrNoValidatorDistInfo`
(distribution code `6`) or `ErrEmptyDelegationDistInfo` (code `5`) otherwise.
Missing delegation starting information also returns code `5`.

The keeper ends the validator's current distribution period and calculates the
delegation's rewards. It caps the calculated decimal rewards at the validator's
outstanding rewards, truncates them to integer coins, and transfers nonzero coins
from the distribution module account to the resolved withdrawal address. Decimal
remainders go to the community pool. The full decimal amount, including that
remainder, is deducted from `ValidatorOutstandingRewards`.

The old starting period's reference count is decremented and its delegation
starting information is deleted. The keeper then reinitializes the starting
information for the existing staking delegation; staking shares are unchanged.
The new `PreviousPeriod` is the period just ended, while `Height` is the current
block height. See [Initialize delegation](#initialize-delegation) and the
[staking hooks](05_hooks.md).

### Rewards across slash periods

F1 accounting stores a cumulative reward ratio `R(P)` for each historical
validator period `P`: the accumulated rewards per staked token, separately for
each denomination. For stake held constant from period `A` through `B`, rewards
are `(R(B) - R(A)) * stake`, with the keeper's decimal truncation.

[CalculateDelegationRewards](../keeper/delegation.go) replays slash events between
the delegation's starting block height and the current block height. A slash
fraction `F(P)` is the fraction removed from stake, so the surviving stake is
multiplied by `1 - F(P)`. The following pseudocode shows the accumulation by
validator period; `mul_truncate` denotes the keeper's truncating multiplication:

```text
stake := startingInfo.Stake
previousPeriod := startingInfo.PreviousPeriod
rewards := zero decimal coins
if startingInfo.Height == currentBlockHeight:
    return rewards

for event in slash events between startingInfo.Height and currentBlockHeight:
    P := event.ValidatorPeriod
    if P > previousPeriod:
        rewards += mul_truncate(R(P) - R(previousPeriod), stake)
        stake = mul_truncate(stake, 1 - event.Fraction)
        previousPeriod = P

currentStake := validator.TokensFromShares(delegation.Shares)
if stake > currentStake:
    if stake <= currentStake + 3 * smallestDecimal:
        stake = currentStake
    else:
        panic
rewards += mul_truncate(R(B) - R(previousPeriod), stake)
return rewards
```

Here `B` is the period ended for this withdrawal. Rewards from every interval
are added, including the final interval after the last slash. With no slash
events the final interval covers the entire range. Delegations initialized in
the current block receive no rewards yet. The stake check tolerates at most
three units of the smallest SDK decimal to account for rounding.

Response: `MsgWithdrawDelegatorRewardResponse {}`.

## WithdrawValidatorCommission

The message is named `MsgWithdrawValidatorCommission`. The validator's account
signs it with `validator_address` set to its operator address.

Commission accumulates during allocation in `BeginBlock`. The keeper returns
`ErrNoValidatorCommission` (distribution code `7`) when accumulated commission is
zero. Otherwise, it truncates commission to integer coins, keeps the decimal
remainder in `ValidatorAccumulatedCommission`, and subtracts only the integer
coins from outstanding rewards. Nonzero coins are sent to the withdrawal
address resolved for the validator's account address. A positive commission
smaller than one coin can therefore leave a remainder without sending coins.

Response: `MsgWithdrawValidatorCommissionResponse {}`.

## FundCommunityPool

The message is named `MsgFundCommunityPool`. The depositor signs it with
`depositor` and an `amount` coin list. The message validates the coin list and
requires a depositor; the message server parses the account address.

[FundCommunityPool](../keeper/keeper.go) first transfers the coins from the
depositor to the distribution module account. If that transfer fails, the error
is returned. On success, it adds the same amount as decimal coins to
`FeePool.CommunityPool`.

Response: `MsgFundCommunityPoolResponse {}`.

## Common distribution operations

### Initialize delegation

[initializeDelegation](../keeper/delegation.go) runs after the validator period
has already been incremented by a withdrawal or a before-delegation hook. It
records the period just ended and increments that historical period's reference
count. It stores:

| Starting-info field | Value | Purpose |
| --- | --- | --- |
| `PreviousPeriod` | `GetValidatorCurrentRewards(ctx, val).Period - 1` | Baseline for historical reward ratios |
| `Stake` | `validator.TokensFromSharesTruncated(delegation.GetShares())` | Initial token stake for reward and slash calculations |
| `Height` | `uint64(ctx.BlockHeight())` | Slash-event height range and same-block reward check |

Initialization does not itself increment the validator period. Validator periods
and block heights are distinct counters.
