---
order: 2
---

# Block Sync

This page specifies the compatibility block-sync protobuf protocol used by
`consensus/internal/blocksync`. In classic mode it supplies blocks to the sync
controller. In Autobahn mode, node setup retains a reactor for serving block and
status requests without installing the classic sync controller. Autobahn fetches
its own certificates and data through `StreamFullCommitQCs` and `GetBlock`; see
[Messages](./README.md#mode-applicability).

The authoritative wire schema is
[`tendermint/blocksync/types.proto`](../../../proto/tendermint/blocksync/types.proto),
and the serving and receiving behavior is in
[`reactor.go`](../../../internal/blocksync/reactor.go). These messages use the
[compatibility connection](../connection.md#mconnection), not Autobahn stream
framing.

## Channel

Block sync has one channel. Its current Go constant is `BlockSyncChannel`.

| Name | Number |
| --- | --- |
| `BlockSyncChannel` | 64 (`0x40`) |

## Message Types

Field names below use their protobuf spelling. The transport serializes a
`Message` envelope containing one of these five variants.

### BlockRequest

Requests the block at the specified height from the peer's block store.

| Name | Type | Description | Field number |
| --- | --- | --- | --- |
| `height` | `int64` | Requested block height | 1 |

### NoBlockResponse

Reports that the serving node cannot load a block at the requested height. This
can include a pruned block; it is not a certificate about chain history.

| Name | Type | Description | Field number |
| --- | --- | --- | --- |
| `height` | `int64` | Unavailable block height | 1 |

### BlockResponse

Returns the requested stored block after conversion to protobuf. The sync
controller decodes it with `types.BlockFromProto` before passing it to its block
pool. Receiving a response alone does not establish finality.

| Name | Type | Description | Field number |
| --- | --- | --- | --- |
| `block` | [`tendermint.types.Block`](../../../proto/tendermint/types/block.proto) | Requested block | 1 |

Field number 2 and the name `ext_commit` are reserved in `BlockResponse`.

### StatusRequest

An empty message requesting the highest and lowest heights retained by the
peer's block store.

### StatusResponse

The serving reactor sends `store.Height()` as `height` and `store.Base()` as
`base`. These are storage bounds, not a validator vote or a quorum certificate.
Pruning can move `base` above the initial height; an empty store has no retained
block range.

| Name | Type | Description | Field number |
| --- | --- | --- | --- |
| `height` | `int64` | Highest stored block height | 1 |
| `base` | `int64` | Lowest retained block height | 2 |

When a sync controller and block pool are active, the receiver passes both
bounds to `SetPeerRange` so requests can respect the peer's retained range.
A reactor with no sync controller still answers requests, but ignores incoming
responses. A store range does not imply that every node retains all history.

### Message

`Message.sum` is a protobuf `oneof` with these variants:

| Name | Type | Description | Field number |
| --- | --- | --- | --- |
| `block_request` | [BlockRequest](#blockrequest) | Request a block | 1 |
| `no_block_response` | [NoBlockResponse](#noblockresponse) | Requested block is unavailable | 2 |
| `block_response` | [BlockResponse](#blockresponse) | Return the requested block | 3 |
| `status_request` | [StatusRequest](#statusrequest) | Request stored height bounds | 4 |
| `status_response` | [StatusResponse](#statusresponse) | Return stored height bounds | 5 |

## Autobahn Data Fetch

Autobahn's `StreamFullCommitQCsReq.next_block` and `GetBlockReq.global_number`
are `uint64` global block numbers, distinct from the classic messages' `int64`
heights. A `FullCommitQC` contains a commit certificate and lane block headers;
`GetBlockResp.block` is an optional Autobahn block, not a
`tendermint.types.Block`. Data ingestion verifies certificates and block
contents through the Autobahn types and data state. The
[stream registrations](./README.md#autobahn-streaming-rpcs) give the wire kinds
and limits; the [verification applicability map](../../light-client/verification/verification_002_draft.md#current-implementation-map)
identifies the consumers.
