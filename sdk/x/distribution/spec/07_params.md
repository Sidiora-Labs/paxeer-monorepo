<!--
order: 7
-->

# Parameters

[types/params.go](../types/params.go) defines the distribution parameter-store
keys, validators and `DefaultParams`. The
[protobuf Params fields](../../../proto/cosmos/distribution/v1beta1/distribution.proto)
use separate names for JSON and genesis state:

| Parameter-store key | JSON / protobuf field | Type | Fork default |
| --- | --- | --- | --- |
| `communitytax` | `community_tax` | string (SDK decimal) | `"0.000000000000000000"` |
| `baseproposerreward` | `base_proposer_reward` | string (SDK decimal) | `"0.000000000000000000"` |
| `bonusproposerreward` | `bonus_proposer_reward` | string (SDK decimal) | `"0.000000000000000000"` |
| `withdrawaddrenabled` | `withdraw_addr_enabled` | bool | `true` |

These are constructor defaults in this tree. Genesis state and subsequent
parameter changes determine the values on a particular chain. The upstream
examples of 2% community tax, 1% base proposer reward and 4% bonus proposer reward
are not this fork's defaults.

Each rate validator requires a non-nil SDK decimal in the inclusive range
`[0, 1]`. Zero is valid for all three rates. `Params.ValidateBasic` additionally
requires their sum to be at most `1`; the individual parameter-store validators
check each value separately. The combined bound is
`community_tax + base_proposer_reward + bonus_proposer_reward <= 1`, which
keeps the validator multiplier non-negative even when all voting power signs.
The sum may equal `1`. [Keeper.SetParams](../keeper/params.go) calls full
validation before writing. Its `ParamChangeProposalHandler` wrapper validates
the final prospective parameter set in a cache before publishing governance
changes. The individual validators alone cannot enforce this combined bound.
The withdrawal-address validator requires a boolean.

## Non-default example

The upstream example `community_tax = 0.02`, `base_proposer_reward = 0.01`,
`bonus_proposer_reward = 0.04`, `withdraw_addr_enabled = true` is valid (the
rates sum to `0.07`), but must be configured explicitly. It is not the network
default. See the [event example](06_events.md#non-default-allocation-example)
for its actual allocations with nonempty votes.

During [fee allocation](../keeper/allocation.go), the proposer multiplier is
`base_proposer_reward + bonus_proposer_reward * previousFractionVotes`, using
truncating decimal multiplication. The validator voting-power allocation uses
`1 - proposerMultiplier - community_tax`; remaining fees go to the community
pool. Zero proposer rates remove the additional proposer allocation, while
validators still receive voting-power allocations when total previous power is
nonzero. With the current Autobahn adapter's empty votes, the zero-power branch
instead places every collected fee in the community pool, regardless of rates;
see [Begin Block](03_begin_block.md).

`withdraw_addr_enabled` controls changes made by
[MsgSetWithdrawAddress](04_messages.md#msgsetwithdrawaddress). Setting it to
`false` rejects changes; stored addresses are still resolved subject to the
blocked-address and bank recipient checks. See [Events](06_events.md) for the
allocation and withdrawal attributes.
