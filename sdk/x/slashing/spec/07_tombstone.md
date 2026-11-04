<!--
order: 7
-->

# Staking Tombstone

## Abstract

Tombstoning is the implemented, permanent response to accepted equivocation
(double-signing) evidence. It is distinct from the recoverable jail imposed for
downtime. These rules run in the staking, slashing and evidence modules of the
merged Paxeer network; the penalty is applied to chain state when the evidence
is processed.

Evidence may arrive after the infraction. Until the applicable unbonding or
redelegation entry matures, stake that contributed to the fault can still be
slashed. The [staking transitions](../../staking/spec/02_state_transitions.md#slashing)
describe how those balances are handled.

### Infinite jail

The evidence keeper's
[`HandleEquivocationEvidence`](../../evidence/keeper/infraction.go) handles valid
equivocation reported by consensus. Before applying a penalty, it checks the
consensus address/public-key relation, evidence age, validator state and signing
information. Evidence is ignored when the public key is unknown, the validator
is absent or already `Unbonded`, or the validator is already tombstoned. Missing
signing information for an otherwise eligible validator is an invariant failure.
When consensus evidence limits are available, evidence is too old only when
**both** `MaxAgeDuration` and `MaxAgeNumBlocks` have been exceeded.

For accepted evidence, the handler:

1. Calls slashing with the current `SlashFractionDoubleSign`, the validator power
   carried by the evidence and distribution height
   `evidence.Height - sdk.ValidatorUpdateDelay`.
2. Jails the validator if it is not already jailed. Staking removes jailed
   validators from its power index; the active-set change is handled at EndBlock.
3. Sets `JailedUntil` to `DoubleSignJailEndTime`, defined in
   [evidence parameters](../../evidence/types/params.go) as
   `time.Unix(253402300799, 0)` (9999-12-31 23:59:59 UTC).
4. Sets `ValidatorSigningInfo.Tombstoned` to `true` and stores the evidence.

The [signing-info keeper](../keeper/signing_info.go) persists the tombstone under
the consensus address. The independent tombstone flag makes the exclusion
permanent: waiting for a jail timestamp does not make the validator eligible
again.

[`MsgUnjail`](./03_messages.md) calls the
[slashing unjail keeper](../keeper/unjail.go). After checking validator existence,
self-delegation, the minimum self-delegation and the jailed flag, that keeper
rejects a tombstoned validator with `ErrValidatorJailed` (slashing codespace,
code `4`: `validator still jailed; cannot be unjailed`). Restoring self-delegation
or using the operator key cannot clear a tombstone.

### Single slashing amount

The tombstone caps repeated equivocation punishment for a consensus address.
After the first accepted evidence tombstones the validator, subsequent evidence
for that tombstoned address is ignored. The keeper does not track a maximum
penalty across multiple finite slashing periods, compare several fault
percentages, or start a new period after an unjail transaction.

The slash uses the configured `SlashFractionDoubleSign`; it is not a hard-coded
percentage. In this fork's [slashing defaults](../types/params.go), both
`SlashFractionDoubleSign` and `SlashFractionDowntime` are zero. A zero monetary
penalty still leaves the equivocation handler's jail and tombstone operations in
effect. Query chain parameters for the values in force on a particular network.

## Downtime jail

Downtime follows [liveness processing](../keeper/infractions.go), which monitors
the signed-block window. When its threshold is exceeded for a validator that is
not already jailed, it applies `SlashFractionDowntime`, jails the validator and
sets `JailedUntil` to the current block time plus `DowntimeJailDuration`. It resets
the missed-block counter and offset. This path does not set a tombstone.

A jailed validator can submit `MsgUnjail` after that deadline if it has sufficient
self-delegation and is not tombstoned. Unjailing restores eligibility in the
staking power index; [active-set selection](../../staking/spec/02_state_transitions.md#validators)
still determines whether it becomes bonded. The code default for
`DowntimeJailDuration` is ten minutes; use the chain's current parameter value
rather than historical upstream defaults.

## Delegations after tombstoning

Jailing and the subsequent validator bond-status transition do not automatically
withdraw individual delegations. Their shares remain recorded. Delegators,
including the operator, can request
[undelegation or redelegation](../../staking/spec/03_messages.md#msgundelegate).
An undelegation always starts a fresh full unbonding period, even if the validator
is already unbonding or unbonded. Redelegation has a separate completion rule.
Existing eligible unbonding and redelegation entries may be slashed for the
source validator's earlier infraction.

A new validator registration requires a new, available operator address and
consensus public key, an initial self-delegation and the other
[`MsgCreateValidator` checks](../../staking/spec/03_messages.md#msgcreatevalidator).
Delegations are not transferred to it automatically.

## Historical motivation

Earlier designs considered finite consensus-fault jail periods and tracking the
worst delayed infraction in each period. That design would require retaining
several periods while older evidence could still arrive. Permanent tombstoning
replaces that bookkeeping and prevents a compromised consensus key from causing
repeated equivocation slashes after the first accepted evidence. Finite jail for
consensus safety faults and infinite jail as a future proposal are historical
design discussions, not the current lifecycle.
