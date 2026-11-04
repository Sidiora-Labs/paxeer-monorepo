# Node sequencing contract

Sequencing is an execution role inside `paxd` on the merged Paxeer network.
The kernel is part of that network and proposes blocks. Mainnet-beta uses
EVM chain ID 125 (`0x7d`), and a receipt is final when its block is committed.
The pre-merge separate sequencer service and guarantor finality model are
retired. [Consensus fork](consensus-fork.md) documents the consensus transport,
wire types and RPC behavior.

Sub-chains are tenant domains represented by database namespaces, launchable
without provisioning separate chain infrastructure.

## Node config

[`daemon/paxd/cmd/root.go`](../daemon/paxd/cmd/root.go) composes the node's
configuration, validates sequencer options and constructs the application.
`config/config.toml` holds consensus settings, while `config/app.toml` holds
application settings, including the execution-role `[sequencer]` section.
The declarations in [`app_config.go`](../daemon/paxd/cmd/app_config.go) and
[`consensus/config/config.go`](../consensus/config/config.go) are authoritative.

Public Ethereum JSON-RPC uses HTTPS at the root path of
`api1.mainnet-beta.paxeer.network` through
`api14.mainnet-beta.paxeer.network`, and
`api-mainnet-beta.paxeer.network`. The node namespaces are `debug`, `echo`,
`eth`, `net`, `pax`, `test` and `txpool`. Hosted `px_` methods belong to the
[gateway contract](../services/platform/hosted/gateway/openrpc.json).
`node.hyperpaxeer.com` is the build library mirror, never an RPC endpoint.
See [consensus RPC routes](consensus-fork.md#rpc-routes) for the separate local
consensus listener.

## Sequencer config

[`node/sequencer/config.go`](../node/sequencer/config.go) defines `Config`,
`DefaultConfig`, `ReadConfig`, `Validate`, the application TOML template and
start flags. These are the application keys:

| Key under `sequencer.` | Default | Enabled-role constraints |
| --- | --- | --- |
| `enable` | `false` | Enables the in-process sequencing role. |
| `role` | `primary` | `primary` originates replication; `standby` receives it. |
| `mode` | `sequential` | `sequential` or `microbatch`. |
| `wal-dir` | empty | Required. |
| `receipt-key-file` | empty | Required; see [receipt keys](#sequencer-receipt). |
| `receipt-forward-url` | empty | Must be empty when enabled; used for receipt forwarding by a node with sequencing disabled. |
| `standby-address` | empty | When supplied, must be `host:port`: a primary's destination or a standby's listen address. |
| `replication-timeout` | `0s` | Cannot be negative; must be positive with `standby-address`. |
| `lease-duration` | three times `replication-timeout` when omitted | Must be positive. |
| `wal-retain-blocks` | `2` | Must be positive. |
| `nonce-queue-max-per-sender` | `0` | Must be positive. |
| `nonce-queue-max-total` | `0` | Must be positive and at least the per-sender bound. |
| `microbatch-window` | `0s` | Must be positive in `microbatch` mode. |
| `microbatch-max-count` | `0` | Must be positive in `microbatch` mode. |

The disabled defaults are not a ready-to-activate primary configuration.
Queue limits, timing, keys and replication topology must be chosen for the
activation. `ReadConfig` rejects invalid conversions and combinations.

`[consensus.sequencer]` in `config.toml` is a different section. Its
`enable` defaults to false. Its timeout defaults are `timeout-propose = "1s"`,
`timeout-propose-delta = "500ms"`, `timeout-vote = "50ms"` and
`timeout-commit = "50ms"`; enabled mode requires positive values. The proposal
timeout grows by the delta for each round. Application enablement does not
replace the on-chain activation boundary.

## Sequencer executor

[`NewNode`](../node/sequencer/node.go) returns no sequencing node when disabled.
For a primary it opens the WAL, loads the committed receipt-key pins and
sequence state, loads the distinct signing key, creates the executor and
installs the receipt and consensus adapters. A standby follows
[`StartStandby`](../node/sequencer/standby_role.go).

[`Executor`](../node/sequencer/executor.go) defines:

- `Open(height, timestampMs)`: create the next open block above committed state
  and run BeginBlock once. Height must follow the committed height, and time
  must be positive.
- `Accept(txBytes)`: execute on a transaction branch of the open layer. A
  successful acceptance merges the branch and assigns the next sequence
  position. A refused transaction writes nothing and consumes no position.
- `Seal(height)`: close that block and return its accepted transactions in
  sequence order. Transaction index zero is reserved for the sequence batch.
- `Snapshot()`: obtain a read-only view of the open layer at the last accepted
  transaction; its lifetime is bounded by sealing.
- `PendingTx(hash)`: look up a receipted EVM transaction in the open layer.
- `Fence()`: permanently refuse later Open, Accept and Seal calls on that executor.

The sequential and microbatch implementations share this contract. Block and
tenant-domain limits, fee policy and nonce admission remain part of the real
execution path. [`layers.go`](../node/sequencer/layers.go) owns open/sealed-layer
errors and bounds; [`microbatch.go`](../node/sequencer/microbatch.go) owns group
execution. Sealed state is consumed by the application and consensus source,
not submitted to a separate settlement chain.

## Sequencer receipt

[`receipt.go`](../node/sequencer/receipt.go) encodes canonical signed execution
receipts, using protocol version 2, structure tag `0x5201` and receipt layout
version 1. `node.EVMReceiptModuleID` is 12, above the eleven reserved kernel
module IDs; it is a receipt identifier, not a precompile address. The Cosmos
operation byte is `0xff`.

Receipts bind the activity ID, sequence and sequence hashes, execution result,
height and execution effects. The summary includes gas, transaction index,
created-contract address, sponsor, VM-error hash, return-data length, log
count and fee denomination. Log effects bind address, topics and a hash of
log data. Use [`layerxproof/codec`](../layerxproof/codec) and
[`layerxproof/verify`](../layerxproof/verify) for decoding and verification.

[`LoadReceiptKey`](../node/sequencer/receiptkey.go) requires a regular,
owner-only file containing a hex-encoded 32-byte Ed25519 seed. It rejects the
consensus key file, the consensus public key, malformed pins and a key without
authorization for the next open height. A restart reloads and revalidates the
persisted key before replaying receipts.

[`Accepter.Accept`](../node/sequencer/accepter.go) releases the signed response
only after durable replication acknowledgement. When the primary is configured
without a standby, the replicator uses standalone local durability. An error
after execution or during replication halts further sends until recovery.
This acknowledgement protects the execution record; block commitment supplies
BFT finality.

## Sequencer WAL

[`node/sequencer/wal/wal.go`](../node/sequencer/wal/wal.go) defines entry version
1 and the `OPEN`, `ACCEPT`, `SEAL`, `COMMITTED` and `LEASE` records. A frame is
`u32be crc32c(entry) || u32be len(entry) || entry`, where an entry begins with
one version byte and one kind byte. The maximum entry size is 8 MiB; the default
segment size limit is 64 MiB.

`Append` returns a `Pending` handle. `GroupSync` flushes and calls `fdatasync`
before releasing the covered handles. A write or sync failure closes the WAL
and fails the group. The directory contains `wal.lock`, `head.seg` and sealed
`<index>-<maxHeight>.seg` segments.

`OpenWAL` locks the directory, validates sealed segments and truncates a torn
or corrupt head tail to its good prefix. A corrupt sealed segment is an error.
`Replay` applies entries after the last `COMMITTED` entry in WAL order and
compares re-executed receipt bytes with stored bytes; divergence returns
`ErrReceiptMismatch`. [`AppReplayer`](../node/sequencer/replay.go) connects this
to the real executor. [`Node.Start`](../node/sequencer/node_start.go) checks the
WAL's committed height, sequence tip and application hash against application
state before replay and refuses a mismatched tip. `Prune` removes only sealed
segments below both the retention height and last committed height. Recovery and role changes are
implemented in [`standby.go`](../node/sequencer/standby.go),
[`failover.go`](../node/sequencer/failover.go) and
[`handover.go`](../node/sequencer/handover.go).

## Sequencer module

[`modules/sequencer`](../modules/sequencer) registers module and store name
`sequencer`. Its keeper persists the sequence tip, receipt-key pins, ordered
leader list, activation height and decided votes.

[`GenesisState`](../modules/sequencer/types/genesis.go) carries `Leaders`,
`ReceiptKeys`, `ActivationHeight`, `LastSeq` and `LastSeqHash`. Leaders must be
nonzero and distinct; receipt key IDs must be nonzero and distinct, public
keys canonical and inclusive height ranges valid. A nonzero activation height
must be covered by a receipt key. Sequence zero requires a zero hash, and a
nonzero sequence requires a nonzero hash. Activation height zero lets genesis
initialization choose the next block.

Store keys are declared in [`keys.go`](../modules/sequencer/types/keys.go):
`0x01` sequence tip, `0x02` tip hash, `0x03` receipt keys, `0x04` leaders,
`0x05` decided-vote prefix and `0x07` activation height. The keeper's `0x06`
key records the last batch height. These are KV-store keys, not EVM addresses.

## Sequencer msg

[`MsgSequenceBatch`](../modules/sequencer/types/msgs.go) is transaction zero
of every block at or above activation. It has no Cosmos signer; a pinned
Ed25519 receipt key authenticates it. Its fields are `Height`, `FromSeq`,
`ToSeq`, `PrevSeqHash`, `LastSeqHash`, `TxRoot`, `KeyId` and `Sig`.
Hashes and key IDs are 32 bytes, and signatures are 64 bytes.

The signed digest is SHA-256 of
`"PAXEER/v1/sequence-batch\x00" || chain-id || u64be height || u64be fromSeq ||
u64be toSeq || prevSeqHash || lastSeqHash || txRoot`.
Here `chain-id` is the application chain identifier supplied to `SignBytes`.
An empty batch has `ToSeq = FromSeq - 1`, an unchanged sequence hash and
`TxRoot = SHA256(empty bytes)`.

[`DeliverSequenceBatch`](../modules/sequencer/keeper/keeper.go) checks block-only
delivery, position zero, height, activation, uniqueness within the block,
continuity from stored sequence state and the pinned key's signature before
storing the new tip. CheckTx and ReCheckTx refuse the batch.

## Consensus leader pin

[`LeaderPin`](../consensus/internal/consensus/types/round_state.go) selects
`L[round mod len(L)]` at or above the activation height. Round zero therefore
uses the first leader. Below activation the normal validator-set election
applies. Pin construction rejects a nonpositive activation height, an empty
list, a zero key or duplicate keys.
[`LeaderPinFromSchedule`](../consensus/internal/consensus/types/leader_schedule.go)
converts the persisted schedule; zero activation produces no active pin.

## Errors

Application errors use codespace `sequencer`; these codes are not consensus
JSON-RPC transport codes:

| Code | Error |
| --- | --- |
| 2 | `ErrInvalidSequenceBatch` |
| 3 | `ErrBadSignature` |
| 4 | `ErrInvalidGenesis` |
| 5 | `ErrBatchPosition` |
| 6 | `ErrBatchRepeated` |
| 7 | `ErrSeqDiscontinuity` |
| 8 | `ErrBatchBlockOnly` |
| 9 | `ErrUnknownReceiptKey` |
| 10 | `ErrCorruptStore` |
| 11 | `ErrNotActivated` |

Declarations live in [`types/msgs.go`](../modules/sequencer/types/msgs.go),
[`types/genesis.go`](../modules/sequencer/types/genesis.go) and
[`keeper/keeper.go`](../modules/sequencer/keeper/keeper.go).
Executor, key-loading and WAL errors are Go errors defined in their respective
packages. In particular, `ErrReplicationHalted` prevents additional sends after
an unsafe release, and `ErrPendingServedBySequencer` refuses application pending
reads on a node without an open-block executor.

## Node CLI

[`root.go`](../daemon/paxd/cmd/root.go) registers the `paxd` command tree,
including `start`, `query`, `tx`, `sequencer`,
`add-genesis-receipt-key` and `add-genesis-sequencer-leader`.
`paxd sequencer gen-receipt-key --output FILE` creates a new seed file with
mode `0600`, refuses overwrite and outputs the public key.
[`add-genesis-receipt-key`](../daemon/paxd/cmd/genreceiptkey.go) pins the public
key and inclusive range using `--public-key`, `--first-height` and
`--last-height`; the key ID is SHA-256 of the public key bytes.
[`add-genesis-sequencer-leader`](../daemon/paxd/cmd/genseqleader.go) accepts
`--pubkey HEX`. Genesis-editing commands prepare a chain's genesis file; they
are not a way to change an already-running chain's committed state.

## Node genesis

[`node/genesis.go`](../node/genesis.go) builds module genesis state and reads
`genesis.stream-import` and `genesis.genesis-stream-file`. Stream import is
disabled by default; enabling it requires a nonblank stream file. Sequencer
genesis must meet the [module validation rules](#sequencer-module).

## Node ABCI

[`Application`](../consensus/abci/types/application.go) is the exact Go contract
between the application and consensus. It includes `Info`, `Query`,
`GetValidators`, `CheckTx`, `GetTxPriorityHint`, `EvmNonce`, `EvmBalance`,
`InitChain`, `ProcessProposal`, `FinalizeBlock`, `Commit` and snapshot methods.
This fork's CheckTx uses `RequestCheckTxV2` and `ResponseCheckTxV2`.

[`node/app.go`](../node/app.go) and [`node/abci.go`](../node/abci.go) own the
implementation. Proposal processing checks sequence ordering and proposer
authorization; block delivery applies the sequence batch and execution results.
Finalization and commitment are distinct application calls. The legacy
BeginBlock/DeliverTx/EndBlock helpers remain internal parts of execution.

## Node Go API

`App` exposes `SetOpenBlock`, `SetSequencerSource`, `SequencerSource` and
`SetSequencerRPC` for installing the in-process adapters. `LatestCtx` and pending
read helpers select execution state; the application exposes
`ErrPendingServedBySequencer` when the required executor is absent. The wrappers
in [`root.go`](../daemon/paxd/cmd/root.go) retain the sequencing node's lifecycle.

## Node engine

[`engine/executor/config/config.go`](../engine/executor/config/config.go)
declares `giga_executor.enabled` and `giga_executor.occ_enabled`, both true by
default. These select the Giga execution engine and its optimistic concurrency
control. They are distinct from the sequencing role and its sequential versus
microbatch mode. [`node/paxdb.go`](../node/paxdb.go) owns engine/storage setup.

## Node storage

[`storage/config`](../storage/config) defines state-commit, state-store and
receipt-store configuration. [`node/paxdb.go`](../node/paxdb.go) connects those
stores to the application. Persistent committed state and the speculative
sequencer WAL have separate responsibilities.

## Sequencer receipt store

[`ReceiptStoreConfig`](../storage/config/receipt_config.go) reads
`receipt-store.rs-backend`, defaulting to `pebbledb`; the misspelled
`receipt-store.backend` is rejected. Receipt retention is set by the application
from `min-retain-blocks`; the config default of zero means no pruning.
[`TxHashIndex`](../storage/ledger_db/receipt/tx_hash_index.go) maps a transaction
hash to a block number and supports block indexing and pruning. A missing hash
returns `(0, false, nil)`. This committed receipt index differs from the
in-memory receipt source used for open-block responses.

## Node admin

[`admin/config.go`](../admin/config.go) defines
`admin_server.admin_enabled` (false) and `admin_server.admin_address`
(`127.0.0.1:9095`). When enabled, the address must contain a loopback IP literal.
The gRPC service in [`admin/service.go`](../admin/service.go) provides
`SetLogLevel`, `GetLogLevel` and `ListLoggers`. Invalid inputs return
`InvalidArgument`; unmatched logger lookups or patterns return `NotFound`.

## Node ante

[`node/ante`](../node/ante) and [`node/antedecorators`](../node/antedecorators)
own Cosmos/EVM admission and delivery decorators, including fee and priority
handling. Sequenced execution still passes through the application's real
transaction checks. The block-only sequence batch is refused at mempool
admission, as described under [sequencer msg](#sequencer-msg).

## Node upgrades

[`node/upgrades.go`](../node/upgrades.go) and versioned handlers own application
upgrades. [`HardForkManager`](../node/upgrades/fork_manager.go) registers handlers
and executes them at their target height; handler failure panics. The sequencer
layer code refuses to open across a scheduled upgrade height with
`ErrUpgradeHeight`. Activation is persisted module state and survives restart.

## Node metrics

[`node/sequencer/metrics.go`](../node/sequencer/metrics.go) declares
`sequencer_receipt_latency_seconds`, `sequencer_wal_sync_seconds`,
`sequencer_standby_ack_seconds`, `sequencer_open_seq` and
`sequencer_sealed_height` through OpenTelemetry. These instrument names describe
receipt release, WAL durability, acknowledgement and execution progress;
consensus Prometheus configuration and instruments are covered in
[consensus metrics](consensus-fork.md#metrics).
