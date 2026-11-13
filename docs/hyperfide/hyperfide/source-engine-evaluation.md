# Source integration engine

The control plane's integration queue uses Git object plumbing. The implementation is [queue.ts](../../../tools/hyperfide/core/control-plane/src/integration/queue.ts), under `tools/hyperfide`. Native source candidates have a separate [qualification and acceptance API](source-control.md#qualification-and-acceptance).

## Queue inputs

`integration.queue.fold` accepts current qualified submission custodies for one repository. Each custody must be the task's current result, bound to its own snapshot, attempt and result tree. The queue verifies base commits and trees and imports a missing result tree from its content-pinned checkpoint capsule.

`planBatches` groups snapshots by their changed paths. An entry joins a batch only when its paths do not overlap entries already in the batch or earlier deferred entries. Overlapping snapshots retain queue order.

## Folding and conflicts

Each snapshot is wrapped as a commit over its recorded base. `git merge-tree --write-tree` applies that change to the internal trunk. Clean fold commits carry `Hyperfide-Snapshot`, `Hyperfide-Task` and `Hyperfide-Custody` trailers. The trunk lives under `refs/hyperfide/trunk/`.

A conflict retains the base, trunk and snapshot sides for each affected path. The queue leaves that snapshot out and raises a decision Ask. It does not select a side on the caller's behalf.

## Requalification

Before advancing the trunk, the queue invokes the module pinned by `HYPERFIDE_QUEUE_REQUALIFIER_MODULE` and `HYPERFIDE_QUEUE_REQUALIFIER_DIGEST`. The module exports `requalify` and returns an outcome and receipt digest. An unsuccessful group is bisected to identify rejected snapshots. Trunk updates use the expected previous Git ID.

A queue result reports folded and skipped snapshots, conflicts, rejected entries, batches and requalification receipts. Folding does not push a remote branch. Publication uses its separate authorized path.

## Measurement

Use the [source traffic measurement guide](source-measurement.md) to measure clone bytes, integration wait and observed conflicts for the actual repository and arrival order. Compare engines on identical source trees, snapshot bases, ordering, conflict policy and requalification work. Throughput depends on the measured workload and execution resources.
