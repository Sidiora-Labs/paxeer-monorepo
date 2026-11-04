<!--
order: 1
-->

# State

The staking module maintains validator and delegation state inside the merged
Paxeer network. The layouts below follow this fork's
[staking key constructors](../types/keys.go) and
[protobuf definitions](../../../proto/cosmos/staking/v1beta1/staking.proto).
`LP(address)` denotes the raw address prefixed by its length in one byte; `|`
denotes byte concatenation. Protobuf values are encoded with the module codec.

## Pool

The bank module holds the bond denomination in two module accounts:

- `bonded_tokens_pool` (`BondedPoolName`) holds bonded validator tokens.
- `not_bonded_tokens_pool` (`NotBondedPoolName`) holds unbonded and unbonding
  validator tokens and pending unbonding-delegation balances.

The queried `Pool` reports these balances; it is not a separate store of coins.
See the [pool keeper](../keeper/pool.go) and the
[transitions](./02_state_transitions.md) that move tokens between the accounts.

## LastTotalPower

`LastTotalPower` records the sum of bonded validators' consensus voting powers
from the preceding validator-set update, rather than their raw token balances.
The `Last` indexes remain unchanged during block processing until the EndBlock
validator-set update.

- LastTotalPower: `0x12 -> protobuf(sdk.IntProto)`.

## Params

Parameters are stored in the `staking` parameter subspace. The keeper's
`GetParams` assembles the registered parameter values into `Params`; it does not
load a single staking-store `Params` record. See the
[parameter reference](./08_params.md) for fields, store keys, types and defaults.

## Validator

A validator has one of three bond statuses:

- `Unbonded`: outside the active set, with its tokens in the not-bonded pool.
  It can receive delegations and become bonded when selected at EndBlock.
- `Bonded`: selected for the active set. Its tokens are in the bonded pool and
  its consensus power contributes to block signing and reward accounting.
- `Unbonding`: it has left the active set and is waiting until its recorded
  `UnbondingTime` to become unbonded. Its remaining tokens are already in the
  not-bonded pool and may still be slashable for earlier faults.

Leaving the active set changes the **validator's** bond status. It does not
create an `UnbondingDelegation` for every delegator or send their tokens back to
accounts. Delegation shares remain recorded across these status changes. A
delegator
requests withdrawal through [MsgUndelegate](./03_messages.md#msgundelegate);
only the requested shares are removed, and the separate
[request/completion lifecycle](./02_state_transitions.md#begin-unbonding)
determines when the resulting tokens can be paid.

The `Jailed` flag is independent of bond status. Jailed validators are excluded
from the power index. A validator's operator address identifies its primary
record; the consensus-address index supports lookups from consensus evidence.

- Validators: `0x21 | LP(OperatorAddr) -> protobuf(Validator)`.
- ValidatorsByConsAddr: `0x22 | LP(ConsAddr) -> OperatorAddr` (raw bytes).
- ValidatorsByPower: `0x23 | BE64(PotentialConsensusPower) | LP(inverted OperatorAddr) -> OperatorAddr`.
- LastValidatorPower: `0x11 | LP(OperatorAddr) -> protobuf(google.protobuf.Int64Value)`.

The power key contains the bitwise complement of each operator-address byte;
the address-length byte is unchanged. Reverse iteration ranks power descending
and breaks equal-power ties by operator address ascending. Potential consensus
power is integer token balance divided by `PowerReduction`, whose application
default is `10^6`. Jailed validators have no power-index entry.

`LastValidatorPower` contains the last selected bonded set and each validator's
power. It is updated during
[EndBlock validator-set changes](./05_end_block.md#validator-set-changes), unlike
the current validator and power records that can change within a block.

## Delegation

A `Delegation` identifies one delegator/validator pair and records its shares in
the validator's tokens. The coins themselves are held in the staking pools.

- Delegation: `0x31 | LP(DelegatorAddr) | LP(ValidatorAddr) -> protobuf(Delegation)`.

### Delegator Shares

Let `T` be the validator's `Tokens` and `S` its `DelegatorShares`. For an existing
nonzero exchange rate:

- Shares issued per token: `S / T`.
- Tokens represented by one share: `T / S`.
- A delegator holding `s` shares has a token claim of `T * s / S`, subject to the
  keeper's decimal arithmetic and integer rounding on removal.

The first delegation, when no shares exist, issues one share per token. Shares
are accounting units, not transferable fungible coins. Slashing a validator's
tokens reduces the value of its outstanding shares without rewriting every
delegation. A validator with zero tokens and positive outstanding shares has an
invalid exchange rate and cannot accept further delegations.

## UnbondingDelegation

An `UnbondingDelegation` groups pending withdrawals for a delegator/validator
pair. Entries contain `CreationHeight`, `CompletionTime`, `InitialBalance` and
remaining `Balance`; they no longer contain delegation shares. Eligible entries
can lose balance through slashing before maturity.

- UnbondingDelegation: `0x32 | LP(DelegatorAddr) | LP(ValidatorAddr) -> protobuf(UnbondingDelegation)`.
- UnbondingDelegationsFromValidator: `0x33 | LP(ValidatorAddr) | LP(DelegatorAddr) -> empty bytes`.

The primary key supports delegator queries. The validator index locates entries
for slashing. A successful `Undelegate` adds an entry, creating the pair's record
if necessary, and schedules it at current block time plus `UnbondingTime` for
**all** validator statuses. Completion transfers the remaining balance from the
not-bonded pool to the delegator account. See
[begin and complete unbonding](./02_state_transitions.md#begin-unbonding), including
recipient-refusal retries.

## Redelegation

Redelegation immediately moves stake from a source validator to a destination
validator. A `Redelegation` record tracks exposure to earlier source-validator
faults while its entries are immature. Each entry records `CreationHeight`,
`CompletionTime`, `InitialBalance` and destination shares `SharesDst`.

- Redelegations: `0x34 | LP(DelegatorAddr) | LP(ValidatorSrcAddr) | LP(ValidatorDstAddr) -> protobuf(Redelegation)`.
- RedelegationsBySrc: `0x35 | LP(ValidatorSrcAddr) | LP(DelegatorAddr) | LP(ValidatorDstAddr) -> empty bytes`.
- RedelegationsByDst: `0x36 | LP(ValidatorDstAddr) | LP(DelegatorAddr) | LP(ValidatorSrcAddr) -> empty bytes`.

The source index locates redelegations for source-validator slashing. The
destination index detects receiving redelegations for a particular delegator,
preventing that delegator from immediately moving the received stake onward
through the same validator (transitive redelegation).

A source that is already unbonded normally needs no tracking record; an
unbonding source uses its existing validator deadline, and a bonded source uses
a full period from the current block time. See
[begin redelegation](./02_state_transitions.md#begin-redelegation) for the keeper's
completion-time selection.

## Queues

Queue entries schedule completion work. `sdk.FormatTimeBytes` encodes UTC time
using `2006-01-02T15:04:05.000000000`, with nine fixed fractional digits and no
time-zone suffix. It preserves nanosecond precision and strips monotonic clock
metadata; it does not round to a coarser interval. The fixed format makes
completion timestamps sortable as bytes.

### UnbondingDelegationQueue

- UnbondingQueue: `0x41 | format(CompletionTime) -> protobuf(DVPairs)`.

`DVPairs.Pairs` contains delegator/validator address pairs. EndBlock dequeues
mature timeslices and completes each pair's mature entries. A failed completion
with a remaining unbonding record schedules a retry at the current block time.
Thus a retry's queue time can be later than the entry's original maturity time.

### RedelegationQueue

- RedelegationQueue: `0x42 | format(CompletionTime) -> protobuf(DVVTriplets)`.

`DVVTriplets.Triplets` contains delegator/source/destination address triples.
Mature entries are removed from redelegation tracking; this completion does not
pay tokens to the delegator because they are already delegated at the destination.

### ValidatorQueue

- ValidatorQueue: `0x43 | BE64(len(timeBytes)) | timeBytes | BE64(UnbondingHeight) -> protobuf(ValAddresses)`.

Here `timeBytes` is the formatted validator `UnbondingTime`, both integer fields
are eight-byte unsigned big-endian encodings, and `UnbondingHeight` is the height
at which the validator began unbonding. `ValAddresses.Addresses` holds Bech32
operator-address strings. Multiple validators can share a queue key. With the
29-byte time encoding, the key is 46 bytes: one prefix byte, eight length
bytes, 29 timestamp bytes and eight height bytes. The length field is the
big-endian integer `29`, not an address length or a single-byte time prefix.

The keeper checks **both** that the encoded time is no later than the current
block time and that the encoded height is no greater than the current height
before completing validator unbonding. Rebonding deletes the validator's queued
address. Validator completion changes bond status; it does not withdraw the
remaining delegations.

## HistoricalInfo

- HistoricalInfo: `0x50 | ASCII(decimal block height) -> protobuf(HistoricalInfo)`.

At BeginBlock, `TrackHistoricalInfo` combines the current block header with
`GetLastValidators`, the set recorded by the preceding validator-set update.
`NewHistoricalInfo` sorts that set by consensus voting power **descending**, then
consensus address **ascending** for equal power, using the keeper's power
reduction. This matches consensus ordering; it is not operator-address ordering.
For
powers `3, 5, 5`, the stored order is `5, 5, 3`; the two power-5 validators
are ordered by their consensus addresses. The encoded height in the historical
key is decimal ASCII (`0x50 | "11"` at height 11), unlike the binary height
suffix in a validator queue key.

The keeper retains the most recent `HistoricalEntries` heights and prunes older
records. Setting that parameter to zero disables new historical records and
prunes the existing contiguous history. See the
[historical-info keeper](../keeper/historical_info.go),
[ordering implementation](../types/historical_info.go) and
[parameters](./08_params.md).
