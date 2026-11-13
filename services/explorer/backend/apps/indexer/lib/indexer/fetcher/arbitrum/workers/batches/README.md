# Arbitrum batch processing

These modules discover parent-chain batch commitments for an Arbitrum rollup
and associate them with indexed rollup blocks, transactions and L2-to-L1
messages. They run for the Arbitrum configuration. Native Paxeer receipts
are final when their BFT block commits.

## Modules

| Source | Responsibility |
| --- | --- |
| [tasks.ex](tasks.ex) | Forward discovery, historical discovery and missing-batch inspection |
| [events.ex](events.ex) | Retrieve and decode `SequencerBatchDelivered` events |
| [discovery.ex](discovery.ex) | Validate commitment transactions, decode calldata and prepare imports |
| [discovery_utils.ex](discovery_utils.ex) | Batch boundaries and discovery helpers |
| [rollup_entities.ex](rollup_entities.ex) | Prepare rollup block/transaction associations and fetch missing entities |
| [recovery.ex](recovery.ex) | Persist DA outcomes and reconcile canonical batch associations |

`Tasks.check_new/1`, `Tasks.check_historical/1` and
`Tasks.inspect_for_missing/1` schedule the three discovery directions.
Discovery checks the commitment transaction's hash and block against the
canonical event before importing the batch. Batch number zero is skipped.
For older calldata without message counts, neighboring batches and rollup
block searches determine the batch boundaries.

## Data availability

Batches can refer to calldata, EIP-4844 blobs, AnyTrust, Celestia or EigenDA.
The [DA modules](../../da/README.md) decode and prepare the associated
metadata. The indexer stores batch boundaries, parent-chain lifecycle
transactions and associations to rollup entities. Included L2-to-L1 messages
advance to the stored status `:sent`.

A DA decoding failure leaves `batch_container` unset and records a DA outcome
with failed status and retry enabled while retaining the other batch facts.
`Recovery.da_outcome_record/1` prepares these `data_type: 2` records.
`Recovery.unresolved_l1_blocks/1` selects parent-chain blocks with an unset
container or an outcome that requests retry, so DA metadata can be revisited.

## Canonical associations

`RollupEntities` fetches missing rollup block and transaction data through RPC
when preparing associations. `Recovery.reconcile/1` reconciles batch data in
a database transaction with an advisory lock. It rebuilds associations for
the affected batches, retains confirmations when the before/after
accumulators agree, and revokes sent messages that no longer belong to a
canonical batch. `Recovery.remove_orphaned_batches/3` removes associations
for orphaned batches during canonical reconciliation.

Forward, backward and gap discovery use bounded parent-chain block ranges.
Existing commitment transactions have their recorded block numbers and
timestamps updated when their parent-chain placement changes.
