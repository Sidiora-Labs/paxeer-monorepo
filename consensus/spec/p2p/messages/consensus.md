---
order: 7
---

# Consensus

This page specifies the classic height/round/step gossip protocol implemented by
[`internal/consensus/reactor.go`](../../../internal/consensus/reactor.go).
The wire authority is
[`tendermint/consensus/types.proto`](../../../proto/tendermint/consensus/types.proto).
These messages apply when the classic consensus reactor is active. Autobahn uses
its own signed messages and peer RPCs, defined in
[`internal/autobahn/autobahn.proto`](../../../internal/autobahn/autobahn.proto) and
[`internal/p2p/giga/api.proto`](../../../internal/p2p/giga/api.proto); it does not
use the envelope below as its consensus RPC contract.

Paxeer is one BFT network whose kernel participates in proposing blocks. A
transaction receipt is final when its block is committed. Peer transport and
identity checks are described in [Peers](../peer.md); bootstrapping from snapshots
is described in [State Sync](state-sync.md). The [node RPC reference](../../rpc/README.md)
describes queries and their availability under each consensus mode.

## Channel

The classic reactor opens four channels. Channel numbers are decimal below;
message field numbers are protobuf tags, independent of channel numbers.

| Name | Number | Hex | Accepted messages |
| --- | --- | --- | --- |
| StateChannel | 32 | 0x20 | NewRoundStep, NewValidBlock, HasVote, VoteSetMaj23 |
| DataChannel | 33 | 0x21 | Proposal, ProposalPOL, BlockPart |
| VoteChannel | 34 | 0x22 | Vote |
| VoteSetBitsChannel | 35 | 0x23 | VoteSetBits |

Every channel carries a `tendermint.consensus.Message` envelope. A valid envelope
on the wrong channel is still an error. During synchronization the reactor
ignores data, vote and vote-set-bits messages rather than entering consensus with
them.

## Message Types

The nested block types below are defined in
[`tendermint/types/types.proto`](../../../proto/tendermint/types/types.proto).
See also the [core data structures](../../core/data_structures.md).

### Proposal

Wraps the validator's signed proposal for a block. The nested proposal carries
its height, round, proof-of-lock round, block ID, timestamp and signature; block
contents travel separately as parts.

| Name | Type | Description | Field Number |
| --- | --- | --- | --- |
| proposal | tendermint.types.Proposal | Signed block proposal | 1 |

### Vote

Wraps a signed prevote or precommit. The nested vote identifies the validator by
address and index, the height, round, vote type, block ID and timestamp. A nil
block vote expresses no vote for a particular block. The vote's signature is
verified by consensus; possession advertisements below do not replace it.

| Name | Type | Description | Field Number |
| --- | --- | --- | --- |
| vote | tendermint.types.Vote | Validator's signed vote | 1 |

### BlockPart

Gossips one part of a proposed block, with the part's Merkle proof.

| Name | Type | Description | Field Number |
| --- | --- | --- | --- |
| height | int64 | Block height | 1 |
| round | int32 | Consensus round | 2 |
| part | tendermint.types.Part | Block part, index and proof | 3 |

### NewRoundStep

Announces a height/round/step transition so peers can select useful gossip.

| Name | Type | Description | Field Number |
| --- | --- | --- | --- |
| height | int64 | Current height | 1 |
| round | int32 | Current round | 2 |
| step | uint32 | Classic consensus round-step value | 3 |
| seconds_since_start_time | int64 | Seconds since round-state StartTime; may be negative | 4 |
| last_commit_round | int32 | Round of the previous height's commit; -1 at the initial height | 5 |

### NewValidBlock

Announces a block for which the sender has a proposal and more than two thirds
of prevote voting power in the indicated round. The part-set header identifies
the block's parts; the bit array advertises which parts the sender holds.
`is_commit` indicates that the announced block is also committed.

| Name | Type | Description | Field Number |
| --- | --- | --- | --- |
| height | int64 | Block height | 1 |
| round | int32 | Round in which the block became valid | 2 |
| block_part_set_header | tendermint.types.PartSetHeader | Total parts and their Merkle root | 3 |
| block_parts | tendermint.libs.bits.BitArray | Parts held by the sender, indexed by part number | 4 |
| is_commit | bool | Whether this is a committed block | 5 |

### ProposalPOL

Accompanies a re-proposal with the earlier proof-of-lock round and an
advertisement of the prevotes held for that round. The bit array identifies
validators, not block parts.

| Name | Type | Description | Field Number |
| --- | --- | --- | --- |
| height | int64 | Proposal height | 1 |
| proposal_pol_round | int32 | Earlier round with the proof of lock | 2 |
| proposal_pol | tendermint.libs.bits.BitArray | Prevotes held, by validator index | 3 |

### HasVote

Advertises receipt of a particular validator's vote. This message carries no
signature or vote contents; peers still exchange the actual `Vote` message.

| Name | Type | Description | Field Number |
| --- | --- | --- | --- |
| height | int64 | Vote height | 1 |
| round | int32 | Vote round | 2 |
| type | tendermint.types.SignedMsgType | Prevote (1) or precommit (2) | 3 |
| index | int32 | Index in the validator set | 4 |

### VoteSetMaj23

Claims that the sender has seen more than two thirds of voting power for the
specified `block_id` at a height, round and vote type. The receiver records that
peer's claim and replies with `VoteSetBits` for the same block ID. This
advertisement is not itself a quorum certificate: the receiver needs the signed
votes to verify the claimed majority.

| Name | Type | Description | Field Number |
| --- | --- | --- | --- |
| height | int64 | Vote-set height | 1 |
| round | int32 | Vote-set round | 2 |
| type | tendermint.types.SignedMsgType | Prevote (1) or precommit (2) | 3 |
| block_id | tendermint.types.BlockID | Block for which the majority is claimed | 4 |

### VoteSetBits

Advertises the votes held for one block ID, enabling peers to send missing votes.

| Name | Type | Description | Field Number |
| --- | --- | --- | --- |
| height | int64 | Vote-set height | 1 |
| round | int32 | Vote-set round | 2 |
| type | tendermint.types.SignedMsgType | Prevote (1) or precommit (2) | 3 |
| block_id | tendermint.types.BlockID | Block to which these votes apply | 4 |
| votes | tendermint.libs.bits.BitArray | Votes held, by validator index | 5 |

### BitArray

[`tendermint/libs/bits/types.proto`](../../../proto/tendermint/libs/bits/types.proto)
defines the shared bit-array encoding. Bit `i` is the low-to-high bit
`i % 64` of word `elems[i / 64]`. A set bit means the corresponding part or vote
is present; `bits` is the logical length, including unset trailing bits.

| Name | Type | Description | Field Number |
| --- | --- | --- | --- |
| bits | int64 | Logical number of bits | 1 |
| elems | repeated uint64 | Packed words of bit values | 2 |

### Message

`Message.sum` is a protobuf `oneof`; select one of these variants. Use the exact
wire name `has_vote` for tag 7.

| Name | Type | Field Number |
| --- | --- | --- |
| new_round_step | [NewRoundStep](#newroundstep) | 1 |
| new_valid_block | [NewValidBlock](#newvalidblock) | 2 |
| proposal | [Proposal](#proposal) | 3 |
| proposal_pol | [ProposalPOL](#proposalpol) | 4 |
| block_part | [BlockPart](#blockpart) | 5 |
| vote | [Vote](#vote) | 6 |
| has_vote | [HasVote](#hasvote) | 7 |
| vote_set_maj23 | [VoteSetMaj23](#votesetmaj23) | 8 |
| vote_set_bits | [VoteSetBits](#votesetbits) | 9 |
