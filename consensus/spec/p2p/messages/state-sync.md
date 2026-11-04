---
order: 5
---

# State Sync

State sync restores application state from a snapshot rather than replaying all
historical blocks. This page describes the snapshot protocol in
[`internal/statesync/reactor.go`](../../../internal/statesync/reactor.go) and
[`tendermint/statesync/types.proto`](../../../proto/tendermint/statesync/types.proto).
It is separate from [classic consensus gossip](consensus.md) and Autobahn's peer
RPCs in [`internal/p2p/giga/api.proto`](../../../internal/p2p/giga/api.proto).
Peers first complete the [authenticated peer handshake](../peer.md).

The `[statesync]` configuration in
[`config/config.go`](../../../config/config.go) uses `enable`, `use-p2p`,
`rpc-servers`, `trust-height`, `trust-hash` and `trust-period`. In this snapshot
service, state sync is attempted only without existing local application state.
With `use-p2p = true`, light-client state is obtained through the channels below;
otherwise the light client requires at least two compatible consensus RPC
servers. See the [RPC reference](../../rpc/README.md) for the local consensus
listener and consensus-mode restrictions before selecting servers.

## Channels

Each channel carries a `tendermint.statesync.Message` protobuf envelope. The
receive capacities below limit encoded messages, so payloads must leave room
for the envelope and other fields.

| Name | Number | Hex | Receive message capacity (bytes) | Messages |
| --- | --- | --- | --- | --- |
| SnapshotChannel | 96 | 0x60 | 4,000,000 | SnapshotsRequest, SnapshotsResponse |
| ChunkChannel | 97 | 0x61 | 16,000,000 | ChunkRequest, ChunkResponse |
| LightBlockChannel | 98 | 0x62 | 10,000,000 | LightBlockRequest, LightBlockResponse |
| ParamsChannel | 99 | 0x63 | 100,000 | ParamsRequest, ParamsResponse |

## Message Types

### SnapshotsRequest

An empty message asking a peer to advertise recent snapshots. The receiver calls
ABCI `ListSnapshots`, sorts the available snapshots, and advertises up to the ten
most recent snapshots, one `SnapshotsResponse` per snapshot.

### SnapshotsResponse

Snapshot metadata supplied by the application. Height, hash and chunk count
must be nonzero/nonempty. The snapshot hash is application-defined; it is not a
substitute for a light-client-verified application hash.

| Name | Type | Description | Field Number |
| --- | --- | --- | --- |
| height | uint64 | Height at which the snapshot was taken | 1 |
| format | uint32 | Application-defined snapshot format | 2 |
| chunks | uint32 | Number of chunks in the snapshot | 3 |
| hash | bytes | Application-defined snapshot hash | 4 |
| metadata | bytes | Application metadata; may be nondeterministic | 5 |

The syncer tracks providers for each snapshot. It obtains a trusted application
hash from its state provider and passes the snapshot and that hash to ABCI
`OfferSnapshot`. It downloads chunks after the application accepts the offer.

### ChunkRequest

Requests one chunk of the accepted snapshot. The chunk index is zero-based and
must be less than the advertised chunk count.

| Name | Type | Description | Field Number |
| --- | --- | --- | --- |
| height | uint64 | Snapshot height | 1 |
| format | uint32 | Snapshot format | 2 |
| index | uint32 | Chunk index within the snapshot | 3 |

### ChunkResponse

The receiver calls ABCI `LoadSnapshotChunk` and sends the returned bytes. Field
4 is **chunk contents**, not a digest. The height, format and index identify the
request being answered.

| Name | Type | Description | Field Number |
| --- | --- | --- | --- |
| height | uint64 | Snapshot height | 1 |
| format | uint32 | Snapshot format | 2 |
| index | uint32 | Chunk index within the snapshot | 3 |
| chunk | bytes | Actual snapshot chunk bytes | 4 |
| missing | bool | True when the application has no such chunk | 5 |

The server sets `missing` when `LoadSnapshotChunk` returns a nil chunk. An empty
but present chunk is valid and uses `missing = false`. The explicit flag
preserves the distinction between absence and zero-length contents. Message
validation rejects a missing response with nonempty contents and a present
response with nil contents; see
[`message.go`](../../../proto/tendermint/statesync/message.go). The chunk queue
also rejects nil contents rather than applying them as snapshot data.

The syncer queues received bytes, checks snapshot height/format/index, and passes
the bytes to ABCI `ApplySnapshotChunk` with their index and sender. Unanswered
requests are retried, potentially with another provider. The application can
request chunk refetches, reject senders, retry or reject a snapshot, or abort.
Rejecting a sender removes its snapshots and chunks from this sync attempt;
this is not a promise of a permanent network ban.

The application must validate the actual chunk contents and reconstruct its
state. After restoration, the syncer checks ABCI `Info` against the snapshot
height and trusted application hash. Receiving all chunks alone does not verify
the restored state.

### LightBlockRequest

Requests a light block for light-client verification or backfill.

| Name | Type | Description | Field Number |
| --- | --- | --- | --- |
| height | uint64 | Requested light-block height | 1 |

### LightBlockResponse

The serving peer constructs a light block from its block and state stores.
The requester verifies light-client data before using it to derive trusted
state; an untrusted peer's response alone does not establish the application
hash.

| Name | Type | Description | Field Number |
| --- | --- | --- | --- |
| light_block | tendermint.types.LightBlock | Signed header and validator set | 1 |

See the [core data structures](../../core/data_structures.md) for the nested
block types. The snapshot and chunk handlers discard responses when no syncer
is active; light-block responses are also used by the backfill process.

### ParamsRequest

Requests consensus parameters at a specified height. The state provider verifies
the parameters against the corresponding trusted header.

| Name | Type | Description | Field Number |
| --- | --- | --- | --- |
| height | uint64 | Requested parameter height | 1 |

### ParamsResponse

The serving peer loads consensus parameters from its state store.

| Name | Type | Description | Field Number |
| --- | --- | --- | --- |
| height | uint64 | Parameter height | 1 |
| consensus_params | tendermint.types.ConsensusParams | Parameters at that height | 2 |

### Message

`Message.sum` is a protobuf `oneof` with eight variants. Snapshot names are
plural in both the variant and the message type.

| Name | Type | Field Number |
| --- | --- | --- |
| snapshots_request | [SnapshotsRequest](#snapshotsrequest) | 1 |
| snapshots_response | [SnapshotsResponse](#snapshotsresponse) | 2 |
| chunk_request | [ChunkRequest](#chunkrequest) | 3 |
| chunk_response | [ChunkResponse](#chunkresponse) | 4 |
| light_block_request | [LightBlockRequest](#lightblockrequest) | 5 |
| light_block_response | [LightBlockResponse](#lightblockresponse) | 6 |
| params_request | [ParamsRequest](#paramsrequest) | 7 |
| params_response | [ParamsResponse](#paramsresponse) | 8 |
