<!--
order: 3
-->

# Begin Block

The merged node calls [BeginBlocker](../abci.go) through
[node/legacyabci/begin_block.go](../../../../node/legacyabci/begin_block.go).
At heights greater than `1`, it allocates the fee collector's current coin
balance, normally collected during the previous block, and then stores the
current header's proposer address for the next allocation. At height `1`, it
only records that proposer.

[AllocateTokens](../keeper/allocation.go) transfers the collected integer coins
from the fee collector module account to the `distribution` module account.
The subsequent allocation updates decimal accounting claims; it does not
immediately transfer rewards to validators or delegators. Withdrawals later
pay from the distribution account. `FundCommunityPool` adds funds to the pool;
it is not a reward withdrawal.

## The Distribution Scheme

The distinction between consensus inputs and configured parameters matters.
[DefaultParams](../types/params.go) sets all three rates to zero:

| Parameter field | Parameter-store key | Default |
| --- | --- | --- |
| `CommunityTax` | `communitytax` | `0` |
| `BaseProposerReward` | `baseproposerreward` | `0` |
| `BonusProposerReward` | `bonusproposerreward` | `0` |

`WithdrawAddrEnabled` defaults to `true`. Persisted genesis or parameter state
can supply different values; defaults are not evidence of a running node's
parameter values. See [Parameters](07_params.md) for the parameter interface.

The Autobahn adapter in
[consensus/internal/p2p/giga_router.go](../../../../consensus/internal/p2p/giga_router.go)
passes an empty `DecidedLastCommit` in `RequestFinalizeBlock`. It does not
synthesize Tendermint precommit votes. The distribution begin-block loop
therefore computes zero total previous power and zero signed power on this
path. An empty vote list here does not mean validators failed to participate
in consensus.

### Reward to the Community Pool

When `totalPreviousPower == 0`, `AllocateTokens` adds **all collected fees** to
`FeePool.CommunityPool` and returns before calculating proposer rewards, tax or
validator allocations. This is the branch reached by the current Autobahn
adapter, irrespective of the configured proposer rates or community tax.

For nonzero total power, the community pool receives the remainder after
proposer and validator allocations. This includes the tax share, decimal
rounding remainders and any unallocated proposer reward. Allocation uses
`sdk.DecCoins` and decimal truncation; it does not round every validator reward
to an integer coin during begin block.

### Reward To the Validators

The keeper retains a configurable allocation calculation for callers that
provide a nonzero-power vote list. Let `fees` be collected fees, `P` the signed
power divided by total power, and `powFrac` a vote entry's power divided by
total power:

```text
proposerMul = BaseProposerReward + BonusProposerReward * P
voteMul = 1 - CommunityTax - proposerMul
proposerReward = fees * proposerMul
validatorReward = fees * voteMul * powFrac
```

The implementation truncates decimal products and power fractions. It attempts
to allocate the proposer reward to the previous proposer's staking validator.
If that proposer is unknown, it logs the condition and leaves the reward in the
remaining amount that goes to the community pool. It then allocates voting-power
rewards for every supplied vote entry, including entries with
`SignedLastBlock == false`; that flag affects `P`, not eligibility in this loop.

With the default zero proposer rates, `proposerMul` is zero, so there is no
special proposer bonus. On the nonzero-power path, a known proposer still
produces a zero-amount `proposer_reward` event and a call to validator allocation;
its empty decimal coin list serializes as an empty `amount` string. Zero rates
do not suppress events on that path. These formulas do not establish a 1% to 5% proposer
payment or a precommit-inclusion incentive for the current Autobahn path,
which returns through the zero-power branch before using them.

### Rewards to Delegators

When a validator allocation occurs, `AllocateTokensToValidator` splits it using
the validator's staking commission rate. Commission accrues in
`ValidatorAccumulatedCommission`; the shared portion accrues in
`ValidatorCurrentRewards`. The full amount also increases
`ValidatorOutstandingRewards`. A validator's self-delegation participates in
the same reward calculation as other delegations.

Individual delegation rewards are calculated later using the
[F1 fee distribution scheme](01_concepts.md), through historical reward ratios,
starting stake and slash events. They are not calculated per delegator in
begin block. See [State](02_state.md) for the stored records and withdrawal
rounding behavior. The current empty-vote branch adds no validator commission
or shared rewards from these collected fees.

### Example Distribution

Suppose the fee collector holds `100uhpx` at the start of a block above height
`1`, the community pool is initially empty, and the node uses the current
Autobahn adapter with default parameters:

1. The bank transfers `100uhpx` from the fee collector to `distribution`.
2. The empty vote list gives `totalPreviousPower = 0`.
3. The community pool claim increases by `100uhpx` in decimal accounting.
4. No proposer, validator or delegator reward is allocated by this call.

For comparison, if a caller supplies a valid nonzero-power list with validators
at 60% and 40% of total power, the same fees and zero rates allocate `60uhpx`
and `40uhpx` to those validators before commission. At a 10% commission rate,
the first allocation adds `6uhpx` to its commission and `54uhpx` to its current
shared rewards, with `60uhpx` added to its outstanding rewards. This illustrates
the retained nonzero-power calculation, not the inputs supplied by Autobahn.
