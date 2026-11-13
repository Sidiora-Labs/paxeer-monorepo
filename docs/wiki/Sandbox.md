# Sandbox

`programs/crates/layerx-programs-sandbox/` is the bounded-lease state
machine, lease-scoped guest execution, and renter-authorized snapshot
path. This page is those three surfaces.

---

## Lease states and transitions

`LeaseState` tags (`programs/crates/layerx-programs-sandbox/src/lease.rs`):

| State | Tag |
| --- | ---: |
| `Requested` | 0 |
| `Funded` | 1 |
| `Active` | 2 |
| `Settling` | 3 |
| `Expired` | 4 |
| `Destroyed` | 5 |

`LeaseState::is_terminal` is true only for `Destroyed`
(`programs/crates/layerx-programs-sandbox/src/lease.rs`).
`Lease::request_with_schedule` constructs `Requested`
(`programs/crates/layerx-programs-sandbox/src/lease.rs`).

`LeaseActivity` tags (`programs/crates/layerx-programs-sandbox/src/lease.rs`):

| Activity | Tag |
| --- | ---: |
| `Request` | 0 |
| `Fund` | 1 |
| `Activate` | 2 |
| `BeginSettlement` | 3 |
| `Expire` | 4 |
| `Destroy` | 5 |
| `CloseBoundExceeded` | 6 |
| `Snapshot` | 7 |

`declared_edge` admits only these triples
(`programs/crates/layerx-programs-sandbox/src/lease.rs`):

| Activity | From | To | Principal slot |
| --- | --- | --- | --- |
| `Request` | `Requested` | `Requested` | acquire |
| `Fund` | `Requested` | `Funded` | hold |
| `Activate` | `Funded` | `Active` | hold |
| `BeginSettlement` | `Active` | `Settling` | hold |
| `CloseBoundExceeded` | `Active` | `Settling` | hold |
| `Snapshot` | `Active` | `Active` | hold |
| `Expire` | `Requested` | `Expired` | release |
| `Expire` | `Funded` | `Expired` | release |
| `Expire` | `Active` | `Expired` | release |
| `Expire` | `Settling` | `Expired` | release |
| `Destroy` | `Expired` | `Destroyed` | none |

`LeaseBook` stores `leases` and `active_by_principal`
(`programs/crates/layerx-programs-sandbox/src/lease.rs`).
`MAX_CONCURRENT_LEASES_PER_PRINCIPAL` is `32`
(`programs/crates/layerx-programs-sandbox/src/lease.rs`).
`insert_requested` refuses `count >= 32` as
`LeaseRefusal::PrincipalLeaseLimit`, then stores `count + 1`
(`programs/crates/layerx-programs-sandbox/src/lease.rs`).

`LeaseBook::transition` treats a lease as holding a slot when its state
is not `Expired` and not `Destroyed`. After a successful transition, if
that lease held a slot and the new state is `Expired` or `Destroyed`,
it subtracts one from `active_by_principal`
(`programs/crates/layerx-programs-sandbox/src/lease.rs`).
`Lease::transition` refuses `Request` and `CloseBoundExceeded` as
`IntrinsicActivityRequired`, refuses `Destroy` as `StorageRequired`,
and refuses `Snapshot` as `SnapshotRequired`
(`programs/crates/layerx-programs-sandbox/src/lease.rs`). The
only `LeaseBook::transition` edge that therefore releases a slot is
`Expire` into `Expired`.

`LeaseBook::destroy_with_evidence` applies `Destroy` and does not change
`active_by_principal`
(`programs/crates/layerx-programs-sandbox/src/lease.rs`).
`Expire` has already released the slot.

`Expire` from a state other than `Settling` requires
`evidence.batch_sequence >= expiry` or it is `NotExpired`
(`programs/crates/layerx-programs-sandbox/src/lease.rs`).
`Expire` from `Settling` does not use that bound.

`CloseBoundExceeded` is not a `Lease::transition` activity. `record_usage`
applies it when resource usage first exceeds a declared limit, consumed
escrow exceeds the escrow amount, or `observed_batch >= expiry`. It returns
`UsageOutcome::ClosedByBound`
(`programs/crates/layerx-programs-sandbox/src/lease.rs`). Direct
`LeaseBook::transition` of that activity is `IntrinsicActivityRequired`.

Tests:

- `transition_matrix_refuses_every_undeclared_edge` asserts
  `declared_edge` on every activity/from/to triple, including that no
  activity leaves `Destroyed` for `Funded`
  (`programs/crates/layerx-programs-sandbox/src/lease.rs`).
- `real_receipts_drive_every_lifecycle_edge_and_destroyed_never_revives`
  drives receipt-backed `Request` → `Fund` → `Activate` →
  `BeginSettlement` → `Expire` → `Destroy`, refuses replayed evidence,
  and refuses `Fund` from `Destroyed` as `InvalidTransition`
  (`programs/crates/layerx-programs-sandbox/tests/lease.rs`).
- `real_bound_receipt_closes_intrinsically_and_refuses_mismatch_and_regression`
  records in-limit usage, refuses a regression, refuses
  `CloseBoundExceeded` through `transition` as
  `IntrinsicActivityRequired`, and closes through `record_usage` as
  `ClosedByBound`
  (`programs/crates/layerx-programs-sandbox/tests/lease.rs`).
- `real_request_evidence_enforces_principal_concurrency_and_expiry`
  admits 32 leases for one principal, refuses the 33rd as
  `PrincipalLeaseLimit`, `Expire`s one `Requested` lease, admits a
  replacement, then `Destroy`s the expired lease and still refuses a
  further insert as `PrincipalLeaseLimit`. After every remaining lease
  is expired and destroyed, 32 new leases admit and the next is again
  `PrincipalLeaseLimit`
  (`programs/crates/layerx-programs-sandbox/tests/lease.rs`).
- `principal_concurrency_and_declaration_bounds_are_enforced` asserts
  `ensure_principal_capacity(31) == Ok(())` and
  `ensure_principal_capacity(32) == Err(PrincipalLeaseLimit)`
  (`programs/crates/layerx-programs-sandbox/src/lease.rs`).

---

## Capability refusals

`LeaseCapabilities::derive` builds the only authority a sandbox image
receives: the lease-namespace execution principal, that principal's
storage namespace, and `CapabilitySet::new([StorageRead, StorageWrite])`
(`programs/crates/layerx-programs-sandbox/src/execute.rs`). Shared
storage, transfer, balance, receipt, event, and callee authority are not
admitted (`programs/crates/layerx-programs-sandbox/src/execute.rs`).

`capabilities_are_derived_and_contain_no_escape_authority` asserts the
grant encoding `vec![0, 2, 1, 2]`
(`programs/crates/layerx-programs-sandbox/src/execute.rs`).
`hostile_authority_families_are_absent_by_construction` asserts that
tags `3`, `4`, `5`, `6`, `7`, `8`, `9`, and `10` are absent from that
encoding (`programs/crates/layerx-programs-sandbox/src/execute.rs`).

`hostile_images_cannot_emit_or_call_an_unleased_program` instantiates a
validated guest image that imports one host function and calls it from
`CALL_ENTRY_EXPORT` under `LeaseCapabilities` authorization
(`programs/crates/layerx-programs-sandbox/src/execute.rs`).
Storage is cloned before the call and compared after; both refusals
leave storage equal to that clone
(`programs/crates/layerx-programs-sandbox/src/execute.rs`).

| Host import | Input | Typed error |
| --- | --- | --- |
| `event_emit` | arity 4, `i32` zeros, `ABI_MODULE` import | `ExecutionError::Entrypoint(EntrypointRefusal::GuestRefused { code: -1 })` |
| `program_call` | arity 6, callee id `[8; 32]` in data, `ABI_MODULE` import | `ExecutionError::Composition(CompositionRefusal::Authority(AbiError::CapabilityDenied))` |

(`programs/crates/layerx-programs-sandbox/src/execute.rs`).

---

## Snapshot persist and restore metering

`SNAPSHOT_CHUNK_BYTES` is `65_536`
(`programs/crates/layerx-programs-sandbox/src/snapshot.rs`).
`SandboxState::canonical_bytes` encodes domain
`LayerX/programs/sandbox/snapshot/v1\0`, source lease id, host program
id, namespace prefix, length-prefixed linear memory, globals,
continuation, and ordered namespace cells
(`programs/crates/layerx-programs-sandbox/src/snapshot.rs`).

`Snapshot::capture` reads execution state from the runtime instance.
`Snapshot::commit` requires matching validated code hashes and receipt-backed
`Snapshot` transition evidence. It sets `byte_length` to the canonical
encoding's length and SHA-256s it as the snapshot digest
(`programs/crates/layerx-programs-sandbox/src/snapshot.rs`).
`snapshot_entries` persists a 44-byte manifest (digest, `u64` length,
`u32` chunk count) plus `bytes.chunks(65_536)`
(`programs/crates/layerx-programs-sandbox/src/snapshot.rs`).
`storage_bytes` is `entries_metered_bytes` of those entries: each key
and value summed through `storage::metered_bytes`
(`programs/crates/layerx-programs-sandbox/src/snapshot.rs`).
`commit_snapshot_storage` is charged `storage_bytes`
(`programs/crates/layerx-programs-sandbox/src/snapshot.rs`).
Namespace occupancy after persist is live-namespace persistent bytes
plus protocol prefix bytes under `b"snapshot"`; exceeding
`lease.limits().namespace_bytes` is `TargetBoundExceeded`
(`programs/crates/layerx-programs-sandbox/src/snapshot.rs`).

`restore` requires a matching `snapshot_records` digest, a `Funded`
target, owner principal, host program, and image hash
(`programs/crates/layerx-programs-sandbox/src/snapshot.rs`).
It charges
`charge_storage_write(byte_length + entries_metered_bytes(rebound live cells))`
before instantiate
(`programs/crates/layerx-programs-sandbox/src/snapshot.rs`).
`instantiate_sandbox` and `restore_continuation` then run against that
meter (`programs/crates/layerx-programs-sandbox/src/snapshot.rs`).

The restore fixture sets `reconstructed_bytes = snapshot.byte_length() + 15`
and `total_restored_bytes = reconstructed_bytes + 65_536 + 16`
(`programs/crates/layerx-programs-sandbox/tests/snapshot.rs`).
Persist `storage_write_bytes >= snapshot.storage_bytes()`; restore
`storage_write_bytes > snapshot.storage_bytes()` and
`storage_write_bytes == snapshot.byte_length() + 65_536 + 16 + 15`
(`programs/crates/layerx-programs-sandbox/tests/snapshot.rs`).

Tests:

- `snapshot_destroy_restore_and_continue_preserves_exact_execution_state`
  asserts persist `storage_write_bytes >= snapshot.storage_bytes()`,
  then with write budget `byte_length + 15 - 1` restore is
  `SnapshotRefusal::Meter(BudgetExceeded { StorageWrite, ... })`, and
  with write budget `byte_length + 15 + 65_536 + 16 - 1` it is
  `SnapshotRefusal::Runtime(ExecutionFault::Resource { ... StorageWrite })`.
  Successful restore asserts
  `restore_usage.storage_write_bytes > snapshot.storage_bytes()` and
  `restore_usage.storage_write_bytes == snapshot.byte_length() + 65_536 + 16 + 15`
  (`programs/crates/layerx-programs-sandbox/tests/snapshot.rs`).
- `aggregate_snapshot_chunks_never_cross_the_lease_namespace_ceiling`
  commits until `TargetBoundExceeded` and asserts live-namespace
  persistent bytes plus snapshot prefix bytes are `<= namespace_bytes`
  (`programs/crates/layerx-programs-sandbox/tests/snapshot.rs`).

[Home](Home.md)
