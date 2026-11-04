<!--
order: 3
-->

# Messages

Staking messages update the [state](./01_state.md) through the
[state transitions](./02_state_transitions.md). The message fields and response
types are defined in this fork's
[tx.proto](../../../proto/cosmos/staking/v1beta1/tx.proto); basic validation and
signers are in [types/msg.go](../types/msg.go), and processing is in
[keeper/msg_server.go](../keeper/msg_server.go). These are native staking
transactions in the merged network.

Coin amounts must be valid and positive. Delegation, undelegation and
redelegation use the configured `BondDenom`. The
[parameter reference](./08_params.md) describes current code defaults; deployed
chain values may differ.

## MsgCreateValidator

`MsgCreateValidator` supplies `Description`, `Commission`, `MinSelfDelegation`,
`DelegatorAddress`, `ValidatorAddress`, `Pubkey` and initial self-delegation
`Value`. Basic validation requires the delegator and operator addresses to
represent the same address bytes. The operator funds and signs the initial
self-delegation.

Creation fails for invalid addresses, a missing public key, invalid or nonpositive
coins, empty description or commission settings, or a nonpositive minimum
self-delegation. The initial amount must meet `MinSelfDelegation`. Processing
also rejects:

- An operator address or consensus public key already registered to a validator.
- A public-key type outside the consensus parameters' supported types.
- A coin denomination other than `BondDenom` or insufficient transferable funds.
- Description fields exceeding their length limits.
- Invalid commission rates: `MaxRate` outside `[0, 1]`, `Rate` outside
  `[0, MaxRate]`, or `MaxChangeRate` outside `[0, MaxRate]`.
- An initial `Rate` below the chain's `MinCommissionRate`.
- An initial delegation rejected by the keeper's applicable voting-power check.

The handler creates an `Unbonded` validator, stores its primary and consensus
indexes and initial power-index entry, calls `AfterValidatorCreated`, then makes
the self-delegation through `Delegate`. It returns an empty
`MsgCreateValidatorResponse`. Bonding depends on subsequent EndBlock selection;
creation alone does not guarantee active-set membership.

## MsgEditValidator

`MsgEditValidator` supplies the operator's `ValidatorAddress`, `Description`,
and optional `CommissionRate` and `MinSelfDelegation`. The operator signs it.
Description edits use `UpdateDescription`; the do-not-modify sentinel preserves
individual fields. Length limits still apply, and the validator must exist.

When `CommissionRate` is supplied, the new rate must meet `MinCommissionRate`,
be nonnegative and no greater than the validator's immutable `MaxRate`. At least
24 hours of block time must have elapsed since the previous commission update.
The **increase** `newRate - oldRate` must not exceed `MaxChangeRate`; that check
does not cap the absolute new rate or the size of a decrease. The handler calls
`BeforeValidatorModified` before saving a commission change.

A supplied `MinSelfDelegation` must be positive and strictly greater than the
stored minimum. The current handler compares the requested value with the
validator's **total `Tokens`**, rejecting a larger value; it does not separately
check the operator's own delegation in this edit path. When stake is later
removed, `Unbond` checks the operator's remaining self-delegation and can jail
the validator below that minimum.

The updated validator is stored and an empty `MsgEditValidatorResponse` returned.

## MsgDelegate

`MsgDelegate` contains `DelegatorAddress`, `ValidatorAddress` and `Amount` and is
signed by the delegator. It transfers coins from that account to the pool for
the validator's bond status and creates or increases a `Delegation` at the
current share exchange rate. Validator tokens, shares and the applicable power
index are updated, with the [delegation hooks](./06_hooks.md) around the change.
The response `MsgDelegateResponse` is empty.

Delegation fails if the validator is missing, the denomination is wrong, funds
cannot be delegated, or the exchange rate is invalid (zero validator tokens with
positive outstanding shares). At positive block heights the shared keeper also
checks `MaxVotingPowerRatio` once the projected selected-set power reaches
`MaxVotingPowerEnforcementThreshold`, returning `ErrExceedMaxVotingPowerRatio`
when the proposed destination power exceeds the limit. See
[voting-power controls](./08_params.md#voting-power-controls) for units and defaults.
There is no separate module-wide minimum-delegation parameter for an ordinary
positive delegation.

A jailed validator can receive a delegation subject to the same checks, but
remains excluded from the power index until it is unjailed. Delegation cannot
clear a tombstone.

## MsgUndelegate

`MsgUndelegate` contains `DelegatorAddress`, `ValidatorAddress` and token `Amount`
and is signed by the delegator. `ValidateUnbondAmount` converts the requested
tokens into shares at the validator's current rate, checks the available
shares and caps a rounding excess at the delegation's full share balance.
The message fails for a missing validator or delegation, insufficient shares,
a wrong denomination or an existing pair already at `MaxEntries`.

The handler calls `Undelegate`, which removes shares and validator tokens,
updates or removes the delegation, and moves tokens from the bonded pool to the
not-bonded pool when necessary. If removal leaves the operator's self-delegation
below `MinSelfDelegation`, the validator is jailed unless already jailed.

Every successful request creates a pending unbonding entry and queue item:

| Validator status at the request | `MsgUndelegateResponse.CompletionTime` |
| --- | --- |
| Bonded | Current block time plus `UnbondingTime` |
| Unbonding | Current block time plus `UnbondingTime` |
| Unbonded | Current block time plus `UnbondingTime` |

The response is an earliest maturity time, not confirmation of payment. It does
not reuse the validator's own unbonding deadline or pay an unbonded validator's
delegator immediately. At maturity EndBlock transfers the entry's remaining
balance, potentially reduced by eligible slashing, to the delegator account.

`CompleteUnbonding` checks the bank recipient policy before each nonzero payout.
A refusal returns `sdkerrors.ErrInvalidRecipient`; any completion error rolls
back that completion's cached balance and entry changes. EndBlock requeues the
pair when its record remains. A later successful retry removes paid entries,
so it cannot pay them twice. See
[complete unbonding](./02_state_transitions.md#complete-unbonding) for the durable
retry lifecycle.

## MsgBeginRedelegate

`MsgBeginRedelegate` contains `DelegatorAddress`, `ValidatorSrcAddress`,
`ValidatorDstAddress` and token `Amount` and is signed by the delegator. It
converts the amount to source shares using the same validation as undelegation.
It immediately removes source shares and delegates the corresponding tokens to
the destination, moving coins between pools if required.

The message fails for:

- A missing source delegation or validator, missing destination validator,
  insufficient shares, or the wrong denomination.
- Identical source and destination validators.
- A receiving redelegation record for this delegator whose destination is the
  proposed source (`ErrTransitiveRedelegation`).
- An existing delegator/source/destination triple already at `MaxEntries`.
- A returned integer token amount of zero (`ErrTinyRedelegationAmount`).
- A destination exchange-rate, voting-power or other keeper error.

The move uses a cached context, so a destination rejection does not leave the
source shares removed. The source operator's self-delegation minimum is checked
during removal, just as for undelegation.

`MsgBeginRedelegateResponse.CompletionTime` concerns source-fault tracking,
not when the destination receives the stake. After the move, the keeper looks
up the source validator to choose the tracking deadline:

| Source at that lookup | Completion time and tracking |
| --- | --- |
| Bonded | Full `UnbondingTime` from current block time; create entry and queue item |
| Unbonding | Existing validator `UnbondingTime` and `UnbondingHeight`; create entry and queue item |
| Unbonded | Immediate completion with zero `time.Time`; no tracking entry |
| Record absent after removing its last shares | Full `UnbondingTime` from current block time; create entry and queue item |

Immature entries expose destination shares to eligible earlier infractions of
the source. EndBlock removes mature tracking entries without a second transfer
or payout. See [begin redelegation](./02_state_transitions.md#begin-redelegation)
for the complete transition and [state indexes](./01_state.md#redelegation) for
the transitive-redelegation check.
