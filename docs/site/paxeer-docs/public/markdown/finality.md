<!-- Source: https://docs.paxeer.app/finality/ -->

# Finality & Commitment

Follow an activity from execution into a sealed batch, and request the evidence your application needs.

## One history, increasing assurance

Paxeer X combines the fast LayerX activity path with Paxeer custody. Activities are ordered into sealed batches and replicas preserve the history for replay. The layerx-merge upgrade (`v7.0`) retired the separate guarantor and checkpoint settlement plane: checkpoint submission to the Paxeer anchor returns a module-retired error, so applications must not require anchor registration as a live step. See [Checkpoints](https://docs.paxeer.app/checkpoints).

| Stage | Name | Evidence and responsibility |
| --- | --- | --- |
| L0 | Accepted | The activity is ordered into the current batch and issued a state-root-bound receipt. |
| L1 | Sealed | Batch ordering is fixed by the signed canonical header. Replicas retain sealed bytes. |
| L2 | Distributed | The sealed batch and availability bundle are retrievable for independent replay. |

Earlier designs added bonded guarantor attestation and Paxeer checkpoint registration as further stages. That plane is retired. A receipt proves execution at its declared evidence level; it does not by itself prove settlement. Before using evidence to release value, apply the configured settlement and withdrawal rules for the relevant network.

## Public commitment levels

Activity submission and payment middleware expose three exact commitment names. These are evidence requirements rather than interchangeable progress labels. They do not replace the lifecycle stages.

| Commitment | Required evidence | Application decision |
| --- | --- | --- |
| `executed` | A verified canonical receipt for the submitted activity | Inspect the authenticated outcome and its payment or program effects. |
| `batched` | Executed evidence plus an authenticated receipt proof naming the same activity and carrying byte-identical canonical receipt bytes | Require signed-batch inclusion before releasing a resource. |
| `finalised` | Batched evidence plus finalised checkpoint evidence whose canonical header exactly equals the proof's signed batch header | Require the checkpoint evidence bound to that exact batch. |

`batch_evidence` carries the authenticated receipt proof and `checkpoint_evidence` carries the checkpoint. A different checkpoint or a newer head cannot substitute for the exact required header. Admission acknowledgements, queue entries, activity IDs and HTTP 202 responses do not establish execution.

### Use the exact wire vocabulary

The spelling is `finalised`, alongside `executed` and `batched`. Do not substitute other spellings or progress labels.

## Recover a pending commitment

When a bounded wait cannot establish the requested evidence, the gateway returns an error rather than silently returning a weaker success. Recover the existing outcome instead of submitting a duplicate:

1. Keep the original signed activity and activity ID.
2. Read `lx_getActivityStatus` and `lx_getReceipt` to recover the existing outcome.
3. Retrieve the required authenticated receipt and batch evidence.
4. Verify exact activity, canonical receipt and signed-header bindings before satisfying your release policy.

Creating another activity to escape an unknown result can duplicate the economic action.

## Payment release policies

An x402 offer's LayerX terms under `extra.layerx` carry the payer `account` reference and `currency` code; the offer model has no LayerX commitment field. Choose the evidence your release policy requires at the application layer and verify it before releasing the resource.

A successful settlement reference is `lxp:<receipt_digest>`. Validate the receipt, payment facts, configured authority and commitment evidence together. The reference or an HTTP success is insufficient on its own.

## Replay covers the whole economic system

Replay covers payments, program calls, program-owned transfers and storage occupancy settlement from the same ordered history. Program monetary effects are explicit transfer sets applied through the kernel's single module-to-ledger entry, while occupancy charges remain replay-checkable batch evidence. Deterministic execution excludes local clocks, floating point and unstable iteration from consensus decisions.

## Checkpoint finalisation checks (retired submission)

Checkpoint submission is retired and unconditionally returns a module-retired error. For historical checkpoints, the anchor keeper's finalisable check covers checkpoint status, the declared threshold, the bonded guarantor count, open challenges, the challenge window, and that the batch is exactly the latest finalised batch plus one. Do not build new flows on checkpoint registration.

## Liveness and exits

If the ordinary activity path stalls, the last accepted history remains the replay anchor. Custody release on Paxeer remains governed by its applicable evidence, timing, limit, and eligibility rules; see [Custody](https://docs.paxeer.app/custody).

Continue with [Receipts & Proofs](https://docs.paxeer.app/proofs), [Data Availability](https://docs.paxeer.app/data-availability) and [Consensus](https://docs.paxeer.app/consensus).
