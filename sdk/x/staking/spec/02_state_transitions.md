<!--
order: 2
-->

# State Transitions

These transitions operate on the [staking state](./01_state.md) in the node.
[Messages](./03_messages.md) initiate delegation changes;
[EndBlock](./05_end_block.md#validator-set-changes) selects the bonded validator
set and processes mature queues. The implementation is in
[delegation.go](../keeper/delegation.go),
[val_state_change.go](../keeper/val_state_change.go) and
[slash.go](../keeper/slash.go).

## Validators

A validator can be `Unbonded`, `Unbonding` or `Bonded`. The first two are
collectively not bonded. EndBlock selects up to `MaxValidators` non-jailed
validators with positive consensus power, ordered by descending power. A bonded
validator leaving that set enters `Unbonding` before it can become `Unbonded`.

Changing validator status leaves individual delegation shares in place. It does
not start withdrawals for every delegator.

### Not bonded to Bonded

When an unbonded or unbonding validator is selected:

- Delete its old power-index entry, set `Status = Bonded`, and save the validator
  and updated power-index entry.
- Remove its address from any existing validator-unbonding queue entry.
- Call `AfterValidatorBonded` with the consensus and operator addresses.
- Account for its tokens moving from `not_bonded_tokens_pool` to
  `bonded_tokens_pool`.

EndBlock nets these pool movements against tokens leaving the bonded set,
emits the consensus validator update when its power changes, and records the
new last-validator powers.

### Bonded to Unbonding

When a bonded validator leaves the selected set:

- Delete the old power-index entry and set `Status = Unbonding`.
- Set `UnbondingHeight` to the current height and `UnbondingTime` to current
  block time plus the configured `UnbondingTime` duration.
- Save the validator, reindex it if it is not jailed, and insert its address in
  the [validator queue](./01_state.md#validatorqueue).
- Call `AfterValidatorBeginUnbonding`.
- Account for its remaining tokens moving from the bonded pool to the
  not-bonded pool, remove its last-validator power and emit a zero-power
  consensus update.

### Unbonding to Unbonded

After validator-set selection, EndBlock processes validator-queue keys whose
completion time and recorded unbonding height have both been reached. It sets
the validator's status to `Unbonded`. If no delegation shares remain, it removes
the validator and its indexes. It then deletes the processed queue key. Tokens
backing remaining delegations stay in the not-bonded pool.

### Jail/Unjail

Jailing sets `Jailed = true`, saves the validator and removes it from the power
index. EndBlock subsequently removes it from the active set if necessary.
Unjailing clears the flag and restores the power-index entry; selection still
depends on power and the active-set limit.

The public `MsgUnjail` path is guarded by the slashing keeper: the validator must
be jailed, have sufficient self-delegation, have served its jail deadline when
signing information exists, and not be tombstoned. A
[tombstone](../../slashing/spec/07_tombstone.md) permanently prevents unjailing.

## Delegations

### Delegate

`Delegate` checks the exchange rate and the configured
[voting-power limit](./08_params.md#voting-power-controls) before changing stake.
It then:

- Calls `BeforeDelegationCreated` for a new delegation or
  `BeforeDelegationSharesModified` for an existing one.
- For an account-funded delegation, transfers the coins to the bonded pool if
  the validator is bonded, otherwise to the not-bonded pool.
- Adds validator tokens and issues shares at the current exchange rate, saving
  the validator and its power index as appropriate.
- Adds the shares to the delegation, saves it and calls `AfterDelegationModified`.

For redelegation, coins are already in a staking pool; `Delegate` transfers them
between pools only when the source and destination bond statuses require it.

### Begin Unbonding

`Undelegate` checks the validator and `MaxEntries` limit, then calls `Unbond` to
remove shares and compute the integer token amount. `Unbond`:

- Calls `BeforeDelegationSharesModified` and verifies sufficient shares.
- Reduces the delegation's shares. If the operator's remaining self-delegation
  falls below `MinSelfDelegation`, it jails the validator unless already jailed.
- Removes a zero-share delegation via `BeforeDelegationRemoved`, or saves the
  reduced delegation and calls `AfterDelegationModified`.
- Removes the corresponding shares and tokens from the validator, updating its
  power index. An unbonded validator with no remaining shares is removed.

`Undelegate` moves the removed tokens to the not-bonded pool if the validator was
bonded. For **Bonded, Unbonding and Unbonded** validators alike, it creates an
`UnbondingDelegation` entry with the current height, initial and remaining
balance, and `CompletionTime = current block time + UnbondingTime`. It adds the
pair to the unbonding-delegation queue and returns that completion time.

There is no immediate account payout for an unbonded validator, and no
inheritance of an unbonding validator's earlier deadline. Those source-status
shortcuts belong to redelegation's `getBeginInfo`, not to `Undelegate`.
For example, with a 21-day parameter, an undelegation requested today waits
21 days even if its validator is already unbonded or finishes unbonding in one
hour. A partial redelegation from that unbonding validator instead uses the
one-hour validator deadline; the destination receives the stake immediately.

### Complete Unbonding

When the queue matures, EndBlock calls `CompleteUnbonding` for the pair. In a
cached context, it processes all entries whose completion time is no later than
the current block time. For each nonzero remaining balance it checks
`bankKeeper.CanSendTo` and transfers coins from `not_bonded_tokens_pool` to the
**delegator account**, then removes the entry. Zero-balance mature entries need
no transfer. It removes the pair's record and validator index if no entries
remain, otherwise saves the remaining entries.

If recipient policy refuses a nonzero payout, completion returns
`sdkerrors.ErrInvalidRecipient`. A transfer error also aborts completion. The
cached changes are committed only after the entire completion succeeds, so an
error preserves balances and entries without a partial payment. EndBlock
requeues a failed pair with a remaining record at the current block time,
deduplicating the retry pair in that timeslice. The retry changes the queue
work time, preserving each entry's original `CompletionTime`, height and balance.
The entry record and queue are both committed state. Later blocks retry it,
including after a restart. A successful completion removes the paid entries, preventing
them from being paid again, and emits the completion event.

### Begin Redelegation

`BeginRedelegation` uses a cached context for the entire move. It rejects the
same source and destination, missing validators, a receiving redelegation for
the same delegator/source pair, the per-triple `MaxEntries` limit and a token
return amount that truncates to zero.

It calls `Unbond` on the source and immediately delegates the returned tokens to
the destination. It moves coins between staking pools when their bond statuses
differ. Destination exchange-rate and voting-power checks apply; an error leaves
the source stake unchanged because the cache is not committed.

After the move, `getBeginInfo` looks up the source's remaining validator record:

| Source state at that lookup | Tracking completion |
| --- | --- |
| Bonded, or record absent | Current block time plus `UnbondingTime`, with current creation height |
| Unbonding | Source validator's existing `UnbondingTime` and `UnbondingHeight` |
| Unbonded | Complete immediately; no redelegation entry or queue item |

The absent-record case matters when `Unbond` removes an empty unbonded source.
For immediate completion the returned time is Go's zero `time.Time`; otherwise
an entry records the initial token amount and destination shares and is queued
at the selected deadline. Until maturity, this tracking exposes the stake at
the destination to eligible earlier faults of the source validator.

### Complete Redelegation

EndBlock removes mature redelegation entries and deletes the record and indexes
when none remain. This releases the source-fault tracking and transitive
redelegation restriction associated with those entries. No account payout or
second delegation occurs: the stake was moved when redelegation began.

## Slashing

### Slash Validator

The staking keeper receives an infraction height, historical consensus power
and slash factor from the calling module. It computes:

`slashAmount = truncate(slashFactor * TokensFromConsensusPower(power))`.

After `BeforeValidatorModified`, it scans eligible unbonding delegations and
source redelegations for past-height infractions. For an infraction at the
current height that scan is unnecessary. The calculated slash amounts for those
entries reduce the remaining amount to take from the validator. The remaining
burn is bounded between zero and the validator's current token balance.

For a positive validator balance it calls `BeforeValidatorSlashed` with the
effective fraction, rounded up and capped at one, before reducing the validator's
tokens and burning coins from its pool. This lowers total token supply and the
value of outstanding delegation shares. An unbonded validator must not be passed
to this slash operation.

Delayed evidence affects state when it is processed, rather than rewriting the
infraction block. See [tombstoning](../../slashing/spec/07_tombstone.md) for the
accepted-equivocation path and its repeated-evidence cap.

### Slash Unbonding Delegation

An entry is eligible when `CreationHeight >= infractionHeight` and it is not
mature at the current block time. The calculated slash is
`truncate(slashFactor * InitialBalance)`; the actual burn is capped at the
remaining `Balance`. Coins are burned from the not-bonded pool. Mature entries
are excluded even if payout has been delayed by recipient refusal.

### Slash Redelegation

The same creation-height and maturity checks apply to source redelegations.
For eligible entries the keeper calculates the nominal slash from
`InitialBalance` and removes `slashFactor * SharesDst` from the destination
delegation, capped at the shares that delegation still holds. It burns the
returned tokens from the destination validator's pool. Entries created before
the infraction, mature entries and missing destination delegations do not
produce that share removal.

## How Shares are calculated

At a given time, a validator has tokens `T` and outstanding shares `S`. Delegator
`i` holds `S_i` and is entitled to `T * S_i / S` tokens before integer rounding.
These are staking tokens; distribution rewards have their own accounting.

A new delegation of `T_j` tokens to a validator with an existing valid exchange
rate receives `S_j = S * T_j / T` shares. The resulting totals are `T + T_j` and
`S + S_j`, preserving the proportional claims. The first delegation receives
`S_j = T_j` when no shares exist. Slashing changes `T` without issuing shares,
reducing the tokens represented by each share. See
[delegator shares](./01_state.md#delegator-shares) and the
[hook ordering](./06_hooks.md) used for reward accounting around these changes.
