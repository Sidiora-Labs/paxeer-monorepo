---
order: 2
---
# Creating a proposal

Proposal construction depends on the [consensus mode](consensus.md#consensus-modes).
Classic mode builds a protobuf block with a header, transactions, previous
commit, and evidence. Autobahn produces lane payloads and proposes certified
lane ranges for global ordering. Both execute decided transactions through the
[in-process application](../abci/apps.md); neither proposal admission nor data
availability alone is a committed transaction receipt.

## Reaping transactions from the mempool

The classic
[`BlockExecutor.CreateProposalBlock`](../../internal/state/execution.go) obtains
pending evidence up to `ConsensusParams.Evidence.MaxBytes` and uses its actual
encoded byte size to budget the block. Evidence is not automatically limited to
one-tenth of `Block.MaxBytes`.

[`types.MaxDataBytes`](../../types/block.go) has the signature
`MaxDataBytes(maxBytes, evidenceBytes int64, valsCount int) int64` and computes:

```text
maxDataBytes = maxBytes
             - MaxOverheadForBlock
             - MaxHeaderBytes
             - MaxCommitBytes(valsCount)
             - evidenceBytes
```

It panics if the remaining budget is negative. Overhead and transaction sizes
are protobuf sizes, not Amino sizes or a fixed byte cost
per evidence item. `MaxCommitBytes` accounts for the validator-dependent previous
commit bound. `ComputeProtoSizeForTxs` includes protobuf transaction framing.

The executor reaps transactions with separate `ReapLimits` for this data budget,
`Block.MaxGasWanted`, and estimated gas (`Block.MaxGas`). In the ordinary path it
passes those transactions to `State.MakeBlock` with the commit, evidence, and
proposer address.

When the embedded application supplies an active `SequencerSource`, the executor
admits reaped transactions through that source before sealing the pending block.
`InjectSequenceBatch` places the encoded batch first, followed by the sealed
transactions, while enforcing the data budget. Failed admission or sealing
fails proposal construction. The header uses the sealed block's `OpenTime`,
which must exceed the previous block time. These are internal interfaces of the
merged node, not instructions to start an additional service.

A proposal must still pass block/evidence validation and the application's
`ProcessProposal` decision in classic mode. Local mempool admission does not
make a block valid for other validators.

## Validating transactions in the mempool

[`TxMempool.CheckTx`](../../internal/mempool/mempool.go) checks the raw transaction
length against its configured `MaxTxBytes`. It also compares the transaction's
protobuf size with the current `TxConstraints.MaxDataBytes`, then invokes
`CheckTxSafe` after its other admission checks.

The classic state-derived constraint uses
`MaxDataBytesNoEvidence(maxBytes, valsCount)` from
[`types/block.go`](../../types/block.go). It reserves block overhead, the header,
and the maximum commit size, without reserving evidence bytes. This replaces the
old `MaxDataBytesUnknownEvidence` evidence-count recipe. Passing this check does
not guarantee that a transaction fits a particular proposal once actual evidence
and the other selected transactions consume its budget.

## Autobahn proposals

### Lane payloads

[`PayloadBuilder.Build`](../../autobahn/types/block.go) enforces two independent
bounds:

| Bound | Current value |
| --- | --- |
| `MaxTxsPerBlock` | 2000 transactions |
| `MaxTxsBytesPerBlock` | 2000 × 1024 = 2048000 raw transaction bytes |

The byte allowance may be distributed unevenly across transactions; it is not
a 1024-byte limit per transaction. Payload metadata includes `CreatedAt`,
`TotalGasWanted`, and `TotalGasEstimated`. A lane header commits to the payload
hash and identifies its lane, lane-local number, and parent header hash.

The [producer](../../internal/autobahn/producer/state.go) applies the smaller of
the configured JSON `max_txs_per_block` and the protocol count limit. It also
uses its wanted/estimated gas budgets, `allow_empty_blocks`, and `block_interval`.
These settings do not override the hard payload bounds.

### Ordering proposals and certificates

[`NewProposal`](../../autobahn/types/proposal.go) selects lane ranges justified
by lane certificates and the prior commit certificate. The proposal may finalize
at most `MaxFullCommitQCHeaders = 100` lane blocks in total. Its encoded
`Proposal` must fit `MaxProposalProtoBytes = 2500`; its full `ConsensusReq`,
including certificates, must fit `MaxConsensusReqBytes = 10 * 1024`.

Selection rotates lane priority by index and view. A range is omitted if adding
it would exceed the remaining header or encoded byte budget. These are budgets
for ordering already-produced lane blocks, not limits of 100 transactions or
2500 bytes per payload. A prepared reproposal must preserve its certified value.
See [BFT Time](bft-time.md#autobahn-timestamps) for assigned timestamps and fresh
proposal timeliness.

### Peer transport budgets

The internal [giga API](../../internal/p2p/giga/api.go) independently caps encoded
peer messages:

| Message | Maximum encoded size |
| --- | --- |
| `ConsensusReq`, streamed `CommitQC`, app-vote and app-QC messages | 10 KiB |
| Streamed `FullCommitQC` | 100 KiB |
| Lane-proposal and fetched-block responses | `MaxBlockProtoSize + 10 KiB` |

`MaxBlockProtoSize` in [`block.go`](../../autobahn/types/block.go) budgets the
maximum protobuf transaction encoding plus 100 KiB for other fields. It is not
the raw payload byte limit. Protobuf repeated-field bounds also apply, including
100 headers in `FullCommitQC`. Committee construction currently limits the
committee to `MaxCommitteeSize = 16` so certificate justifications fit transport.
The producer, proposal verifier, decoder, and transport limits must all be
satisfied; satisfying just one budget is insufficient.
