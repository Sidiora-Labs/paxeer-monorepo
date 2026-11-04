# Checkpoints

Paxeer is one BFT network, with EVM chain ID `125` (`0x7d`). The kernel runs
inside `paxd`; its state changes commit with the rest of the node's transaction
state. A receipt is final once its block is committed. There is no later
checkpoint registration or external settlement step required for finality.

## Committed block evidence

The node executes transactions before proposing them in receipt order. The
signed receipt records execution; the committed block establishes finality.
`pax_sequencerReceipt` distinguishes three rungs:

| Rung | Evidence |
| --- | --- |
| `submitted` | A known transaction without a signed execution receipt |
| `receipted` | A signed execution receipt awaiting block commitment |
| `committed` | A receipt covered by the committed block's sequence batch |

These names come from `rpc/sequencer_receipt.go`. A successful submission or
signed receipt alone must not be treated as a committed result. The sequence
batch and its admission rules live in `node/sequencer/` and
`modules/sequencer/`; kernel execution uses `modules/lxphost/`.

For independent verification, authenticate the block header and CometBFT
commit, then verify transaction or state inclusion against the corresponding
authenticated root. State proofs must name the relevant height and key;
an unrelated newer head does not prove an earlier receipt's effects. The
chain proof verifier is in
`programs/crates/layerx-programs-registry/src/chain_proof/`.

Use the node's root endpoint from [Public RPC endpoints](../reference/public-rpc.md).
Hosted payment and authority APIs have their own evidence contracts; their
response labels do not add a second consensus finality step.

## Historical checkpoint encoding

The external checkpoint registry, bonded guarantor attestations and L0–L4
settlement ladder are retired. Archived certificates describe that former
protocol, not the current network's finality.

For interpreting archived headers, the retained fields are `protocol_version`,
`network_id`, `epoch`, `batch_number`, `first_sequence`, `last_sequence`,
`previous_state_root`, `resulting_state_root`, `activity_merkle_root`,
`receipt_merkle_root`, `event_merkle_root`, `data_availability_root`,
`oracle_root`, `timestamp_ms`, and `sequencer_id`. These are commitments to batch
contents; a header alone does not supply the underlying activities or receipts.

In that encoding, an epoch identifies an authority lifecycle. Multiple
consecutive batches can share an epoch. The historical registration rules used
non-decreasing epochs, consecutive batch numbers and activity ranges, and
state-root continuity; they did not require exactly one registration per epoch.
The retained anchor module in `modules/layerxanchor/` refuses new checkpoint
submissions. Archived headers are decoded by `layerxproof/codec/batch.go`.

[Settlement evidence](../operators/settlement-evidence.md) retains archival
witness formats. Its checkpoint publication procedures are historical and
must not be used as current funding or finality instructions. For current
owner enrollment and recovery evidence, see
[Owner registration details](../human/owner-registration-details.md).
