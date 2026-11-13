# Sequencing and block finality

Paxeer mainnet-beta is one BFT network, with EVM chain ID **125** (`0x7d`).
The kernel runs inside `paxd` and participates in block production. The proposer
executes transactions, signs receipts, and proposes those transactions in the
same order. A transaction receipt is final when its block is committed.

Sequencing is an integrated node role implemented in `node/sequencer/`.
If the network cannot commit blocks, new receipts cannot progress to committed
finality.

See [Data availability](DataAvailability.md) for durability and retrieval, and
[Program porting crates](Porting.md) for program execution and state mappings.

## Sequencer and batches

`ConsensusSource` in `node/sequencer/consensus_source.go` supplies sealed
transactions to consensus. At heights where sequencing is activated in keeper
state, transaction zero is a signed `MsgSequenceBatch`; the remaining
transactions follow in sequence order. This is the chain's proposal path.

The batch fields in `modules/sequencer/types/msgs.go` are:

| Field | Meaning |
| --- | --- |
| `Height` | Block height |
| `FromSeq`, `ToSeq` | Inclusive range of sequenced transactions |
| `PrevSeqHash`, `LastSeqHash` | Previous and resulting sequence hashes, each 32 bytes |
| `TxRoot` | Commitment to the ordered activity IDs, 32 bytes |
| `KeyId` | Identifier of the pinned receipt key, 32 bytes |
| `Sig` | Ed25519 signature, 64 bytes |

`batch_tx.go` encodes the domain `PAXEER/v1/sequence-batch\0`, the three
big-endian u64 positions, the hashes, key identifier, and signature in that
order. `MsgSequenceBatch.SignBytes` hashes the domain, consensus chain ID,
positions, and three hashes. The signing context uses the consensus chain ID;
it must not be replaced with the EVM chain ID.

The keeper requires `FromSeq` to equal the stored last sequence plus one and
`PrevSeqHash` to equal the stored last hash. An empty block still carries a
batch: `ToSeq = FromSeq - 1`, `LastSeqHash = PrevSeqHash`, and `TxRoot` is
SHA-256 of the empty input. A nonempty batch must cover exactly its ordered
transactions. `VerifyBatchTxs` recomputes their root and sequence hash chain.

`ProcessProposalHandler` in `node/app.go` rejects a missing or malformed batch,
transaction/root mismatch, discontinuity, invalid receipt-key signature, or
unauthorized proposer. `DeliverSequenceBatch` in
`modules/sequencer/keeper/keeper.go` also requires transaction index zero,
matching height, activation, and no prior batch at that height before storing
the new sequence position. Validators process the proposed transactions and
the application commits their state through the normal block lifecycle.

## Receipts and commitment

The node implementation in `rpc/sequencer_receipt.go` defines
`pax_sequencerReceipt` with three rungs:

| Rung | Evidence |
| --- | --- |
| `submitted` | The transaction is held without a signed receipt |
| `receipted` | Signed execution receipt; its batch is not yet committed |
| `committed` | The receipt's sequence is covered by a committed block's batch; the result includes block number and hash |

Receipt release follows execution and durable WAL acknowledgement through
`Accepter.Accept`. A signed early receipt is useful execution evidence, but the
BFT block commit supplies finality. Pending reads observe the open speculative
block; callers requiring final state must use committed evidence.

Receipt keys are separate from validator consensus keys and are pinned for
height ranges in `modules/sequencer/keeper/`. A verifier must authenticate the
signing key against those pins. `SequencerReceiptResult.Verify` checks the
receipt signature, decoded fields, and execution-output digests; without pins
it establishes consistency only. The result's adjacent `signingKey` field is
not itself a trusted pin.

The code supports local receipt serving and forwarding from other nodes. If a
node has neither a local source nor a forwarding URL, this method returns
JSON-RPC error `-32000`, `pax_sequencerReceipt served by sequencer`. Availability
of this extension depends on the node's configuration and activation state.

## Recovery and handover

The integrated primary records execution in `node/sequencer/wal/`. Its
replication path can require a hot standby acknowledgement before receipt
release. Recovery re-executes logged transactions and compares sequence,
height, and signed receipt bytes; disagreement returns `wal.ErrReceiptMismatch`.
A replication failure halts further sends with `ErrReplicationHalted`.

Standby promotion in `node/sequencer/failover.go` requires its receipt key to be
pinned at the open height and syncs a later lease before returning the recovered
position. Lease fencing prevents an old primary from continuing to release
receipts. Neither promotion nor a durable WAL entry substitutes for BFT commit.

`node/sequencer/config.go` registers these node options:

| Option | Purpose |
| --- | --- |
| `sequencer.enable` | Enable the integrated role on this node; disabled by default |
| `sequencer.role` | `primary` or `standby` |
| `sequencer.mode` | `sequential` or `microbatch` |
| `sequencer.wal-dir` | Required WAL directory when enabled |
| `sequencer.receipt-key-file` | Required receipt signing seed file when enabled |
| `sequencer.standby-address` | Replication destination or listen address, `host:port` |
| `sequencer.replication-timeout` | Positive timeout required when a standby address is configured |
| `sequencer.wal-retain-blocks` | Positive retained committed-block count; default 2 |
| `sequencer.lease-duration` | Positive lease duration; derived from three times the replication timeout when unset |

Microbatch mode additionally requires positive `sequencer.microbatch-window`
and `sequencer.microbatch-max-count`. Enabled roles require positive nonce-queue
bounds (`sequencer.nonce-queue-max-per-sender` and
`sequencer.nonce-queue-max-total`), with the total at least the per-sender bound.
Lease duration must also be positive. These settings configure the integrated
node role. Replication additionally requires mutual TLS credentials and an
authorized peer configured through `sequencer.replication-tls-cert`,
`sequencer.replication-tls-key`, `sequencer.replication-tls-ca`,
`sequencer.replication-peer-name` and `sequencer.replication-peer-spki`.
`sequencer.receipt-forward-url` configures receipt forwarding when sequencing
is disabled and must be empty when it is enabled.

## Data availability and network access

BFT commitment supplies transaction finality. See
[Data availability](DataAvailability.md) for storage and recovery boundaries.
Use the configured public node's HTTPS root for JSON-RPC and require
`eth_chainId` to return `0x7d`. [Public RPC](PublicRpc.md) describes node access.
Hosted `px_` methods have their contract in
`services/platform/hosted/gateway/openrpc.json`; method availability follows the
node or hosted deployment's configuration.

## Start here

- [Data availability](DataAvailability.md)
- [Program porting crates](Porting.md)
- [Public RPC](PublicRpc.md)
- [Programs](Programs.md)
- [Home](Home.md)
