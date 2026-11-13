<!-- Source: https://docs.paxeer.app/sequencing/ -->

# Sequencing & Finality

Follow an activity from deterministic ordering into a sealed, signed batch, and request the evidence your application needs.

Paxeer X combines the LayerX activity execution plane with Paxeer custody. Activities are ordered into signed batches and replicas preserve the sealed history for replay. A normal payment or agent action does not require a separate settlement-chain transaction.

The layerx-merge upgrade (`v7.0`) retired the separate sequencer, checkpoint, and guarantor settlement plane. The native sequencer recovery, equivocation, loss, and handover interfaces no longer exist in the kernel, and checkpoint submission to the Paxeer anchor returns a module-retired error. Sections below describe the live ordering and batch-evidence path; see [Checkpoints](https://docs.paxeer.app/checkpoints) for the retired anchor surface.

## Global ordering and signed batches

Admitted activities receive contiguous global sequence numbers. A batch begins immediately after the previous batch’s last sequence and extends the state-root chain. A sequence gap, a reversed range, or a mismatched previous root refuses the batch. Account sequences enforce each actor’s own ordering alongside the network-wide sequence.

| Header commitment | Fields |
| --- | --- |
| Protocol and network | `protocol_version`, `network_id`, `epoch` |
| Ordered range | `batch_number`, `first_sequence`, `last_sequence` |
| State transition | `previous_state_root`, `resulting_state_root` |
| Execution evidence | `activity_merkle_root`, `receipt_merkle_root`, `event_merkle_root` |
| Reconstruction inputs | `data_availability_root`, `oracle_root` |
| Time and signer | Batch timestamp and `sequencer_id`; the native header stores time in `timestamp_ms`. |

The canonical header contains exactly these 15 fields, encodes to 354 bytes, and is signed by the key bound to the authorized sequencer identity. Batch time is the only time consulted during execution. Protocol-3 batches append an `LXP/batch-maintenance/v1` receipt and an `LXP/batch-maintenance-effects/v1` event leaf after activity leaves, including an empty effects frame when no maintenance module is enabled.

## Evidence stages

| Level | Evidence | Responsibility |
| --- | --- | --- |
| L0 · Accepted | The ordered activity has a receipt chained to its state root. | Inclusion and ordering. |
| L1 · Sealed | The signed batch fixes ordering and committed roots. | Replica preservation of sealed bytes. |
| L2 · Distributed | The batch and availability bundle are retrievable for independent replay. | Availability of the material needed for replay. |

Earlier designs extended this ladder with bonded guarantor attestation (L3) and checkpoint registration on Paxeer (L4). That settlement plane is retired: a receipt’s existence does not imply Paxeer checkpoint settlement, and applications must not wait for or require anchor registration. Certificates and attestations in historical data used the distinct domains `LXP/v2/checkpoint-certificate` and `LXP/v2/guarantor-attestation`, each with its terminating zero byte.

## Replay

1. Retrieve the complete batch body: activities, receipts, events, oracle inputs, state-diff material, and recovery metadata.
2. Verify actor, grant, oracle, and sequencer signatures.
3. Replay from the previous state root using the committed protocol and parameters.
4. Recompute every committed root and compare the canonical results byte for byte.

Any disagreement stops acceptance of that replay. Deterministic execution excludes local clocks, floating point, and unstable iteration, so identical ordered input reproduces identical receipts and roots.

## Public commitment levels

Submission uses the exact names `executed`, `batched`, and `finalised`. These are evidence requirements alongside the lifecycle stages.

| Requested level | Required evidence |
| --- | --- |
| `executed` | A verified canonical receipt for the submitted activity. |
| `batched` | Execution evidence plus an authenticated receipt proof whose canonical value exactly matches the receipt. |
| `finalised` | Batch evidence plus finalised checkpoint evidence whose canonical header exactly matches the proof’s signed batch header. |

The gateway never downgrades the requested assurance. If its bounded wait cannot establish the requested commitment, it returns an error rather than a weaker success. Preserve the original signed activity and recover through `lx_getActivityStatus`, `lx_getReceipt`, and the required proof reads. An acknowledgement, HTTP 202, or activity identifier alone is not an executed receipt. The accepted spelling is `finalised`.

## Data availability and recovery

A state root without its reconstruction material is insufficient. Retrieval must re-hash to the committed availability roots. Missing data prevents independent replay from accepting the batch.

After a crash, recovery reads the durable log head, loads a usable snapshot, replays committed records, compares roots, and rebuilds disposable projections.

### Equivocation

Two different signed batches at one batch number, or two activities at one global sequence, are conflicting histories. The later conflicting batch is refused. There is no native sequencer equivocation, loss, recovery, or handover interface in the current kernel; those entry points were retired with the separate sequencer plane.

## The Paxeer custody boundary

Economic rules for orders, service delivery, escrow, and ordinary transfers execute in LayerX. Custody operations on Paxeer consume proven facts from the executed history. Deposits credit against proven custody facts. Withdrawals debit LayerX first and pay on Paxeer at most once.

If Paxeer is unreachable, ordinary LayerX execution continues. Paxeer payouts wait for settlement connectivity. Applications should choose their commitment requirement according to the value and reversibility of the action they release.

## Continue reading

- [Deterministic kernel execution](https://docs.paxeer.app/layerx-kernel)

- [Checkpoints (retired anchor)](https://docs.paxeer.app/checkpoints)

- [Data availability and reconstruction](https://docs.paxeer.app/data-availability)

- [Deposits, withdrawals, and exits](https://docs.paxeer.app/bridge)

- [Receipt and proof verification](https://docs.paxeer.app/receipts)

- [Paxeer consensus](https://docs.paxeer.app/consensus)
