# Data availability

Data availability in the merged Paxeer network means retaining the transactions,
receipts, logs, and state needed to inspect and recover chain execution. The
kernel runs inside `paxd`; sequencing, execution, and BFT block commitment belong
to the same network. A transaction is final once its block is committed.
See [Sequencing and block finality](Sequencing.md) for the proposal and receipt
lifecycle, and [Program porting crates](Porting.md) for program state mappings.

A commitment alone does not supply its underlying data. A signed execution
receipt, durable WAL entry, and committed block establish different facts;
clients must check the evidence and the serving node's retained history.

## Durable execution before commitment

The integrated node implementation is `node/sequencer/`, with its write-ahead
log in `node/sequencer/wal/wal.go`. The log records:

| Entry | Recorded material |
| --- | --- |
| `OPEN` | Height, open-block timestamp, and base fee |
| `ACCEPT` | Sequence, height, transaction bytes, signed receipt, return data, log data, and VM error |
| `SEAL` | Height, sequence range, and last sequence hash |
| `COMMITTED` | Height, last sequence, and application hash |
| `LEASE` | Lease epoch, receipt-key identifier, and expiry |

Frames carry CRC32C checksums and length bounds. `GroupSync` syncs appended
frames before releasing their pending handles. `Accepter.Accept` releases the
signed receipt and records its execution outputs only after the replication
path acknowledges the accepted entry. When configured, that path includes
hot standby acknowledgement. Failure halts further sends rather than issuing
receipts without the required durability.

This establishes recoverable execution before block commitment. It does not
make an uncommitted receipt final. The committed `MsgSequenceBatch` binds the
ordered transactions; see [Sequencing](Sequencing.md#sequencer-and-batches).

## Replay and retention

`AppReplayer` in `node/sequencer/replay.go` reopens the execution layer and
re-executes `ACCEPT` records. It checks sequence, height, and exact signed receipt
bytes; a mismatch returns `wal.ErrReceiptMismatch`. WAL opening rejects corrupt
sealed segments with `ErrCorruptSegment`. Checksums detect storage corruption;
they do not authenticate a remote peer's execution on their own.

Configure the node's WAL with `sequencer.wal-dir` and
`sequencer.wal-retain-blocks` (`node/sequencer/config.go`). Retention defaults to
2 committed blocks and must be positive. WAL segments below the retain height
are pruned by a background worker. This recovery window is not an archive of
all historical chain data.

Committed receipts are stored through `storage/ledger_db/receipt/`. Its
`ReceiptStore` exposes earliest/latest versions, receipt lookup by transaction
hash, and log filtering. `storage/config/receipt_config.go` defines:

| Configuration key | Meaning |
| --- | --- |
| `receipt-store.db-directory` | Receipt database location; empty uses the application home |
| `receipt-store.rs-backend` | `pebbledb` (default), also accepting `pebble` |
| `receipt-store.async-write-buffer` | Commit queue length; nonpositive selects synchronous writes |
| `receipt-store.prune-interval-seconds` | Receipt pruning interval |

Receipt retention is derived at the application layer from `min-retain-blocks`,
not a `receipt-store.keep-recent` option. A zero `KeepRecent` means no pruning.
The parser explicitly rejects the misnamed `receipt-store.backend` key. Receipt
storage, state history, and the sequencing WAL have distinct retention purposes;
keeping one does not guarantee every historical query can be answered.

## Retrieving and checking data

Use the node's Ethereum JSON-RPC surface for blocks, transaction receipts, and
logs. `rpc/block.go` implements block and block-receipt retrieval, and
`rpc/filter.go` implements `eth_getLogs`. Their watermark checks constrain
queries to data the node can serve. An unknown block or a block above the
serving watermark can produce JSON `null`; that response alone does not prove
that the transaction was refused or that committed data was lost.

Public HTTPS JSON-RPC is at the root of the node hosts, for example
`https://api1.mainnet-beta.paxeer.network`, with EVM chain ID 125 (`0x7d`). The
public host set includes `api1` through `api14.mainnet-beta.paxeer.network` and
`api-mainnet-beta.paxeer.network`. Node history and progress can differ, so
compare the requested height with the serving node and use an appropriately
retained node for historical data. See [Public RPC](PublicRpc.md).

For signed early execution evidence, the configured node extension
`pax_sequencerReceipt` returns `submitted`, `receipted`, or `committed` as defined
in `rpc/sequencer_receipt.go`. Authenticate its receipt key against the governed
height-range pins and verify the receipt bytes and output digests. The
`committed` result includes the block number and hash. A successful fetch is
neither independent replay nor proof of finality by itself.

## Retired availability protocol

The pre-merge five-class bundle, 65,536-byte chunk proofs, tag-18/19/20 fetch
stream, sealed-candidate selector, and daemon checkpoint storage are retired
transport instructions. The former `LAYERX_NODE_CHECKPOINT_DIRECTORY` and
`LAYERX_NODE_DA_RETAIN_BATCHES` settings do not configure the integrated node's
WAL or receipt store. Do not apply the old retention minimum or checkpoint
selector rules to current block retrieval.

The guarantor availability-attestation and checkpoint-submission paths are
retired. In `modules/layerxanchor/keeper/checkpoints.go`,
`SubmitAvailabilityAttestation` and `SubmitCheckpoint` return `ErrModuleRetired`.
`modules/layerxanchor/types/errors.go` registers that error in the `layerxanchor`
codespace with code 19 and message `LXP_MODULE_DISABLED: layerxanchor retired`.
Historical records are not a new availability or finality authority. The
network's BFT commit is the finality boundary.

## Related pages

- [Sequencing and block finality](Sequencing.md)
- [Program porting crates](Porting.md)
- [Programs](Programs.md)
- [Home](Home.md)
