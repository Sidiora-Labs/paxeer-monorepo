<!-- Source: https://docs.paxeer.app/sandbox/ -->

# Bounded Sandboxes

Lease isolated deterministic computation with explicit resource ceilings, receipt-backed lifecycle transitions, metered snapshots, and resumable execution.

## A lease is the execution boundary

Paxeer X sandboxes run validated guest images inside a bounded lease. Each lease binds a renter principal, host program, image, private storage namespace, resource limits, lifetime, escrow, and fee schedule. Execution derives its authority from this lease rather than inheriting unrestricted host-program access.

Use a sandbox for isolated computation whose state and execution must remain replayable and whose cost and duration must remain bounded. Native financial application logic uses the broader [Programs execution surface](https://docs.paxeer.app/programs); a sandbox guest receives only lease-scoped storage authority.

## Receipt-backed lease lifecycle

| State | Meaning and next step |
| --- | --- |
| Requested | The lease holds a principal concurrency slot. Fund it or let it expire. |
| Funded | Escrow is established. Activate execution or restore an eligible snapshot into this target. |
| Active | Execute within the lease bounds, persist an authorized snapshot, begin settlement, or expire. |
| Settling | Finalize usage and the lease's economic disposition; proceed to expiry. |
| Expired | The concurrency slot is released. Destroy storage with the required evidence. |
| Destroyed | Terminal state. The same lease cannot be funded, activated, or revived. |

The ordinary lifecycle is Requested → Funded → Active → Settling → Expired → Destroyed. Snapshot preserves the Active state. Expiry can also close a Requested, Funded, or Active lease once its batch-sequence deadline is reached. A Settling lease can expire through its settlement path.

Transitions consume verified receipt evidence and reject replayed evidence or undeclared state edges. A principal has at most 32 concurrent leases. Requested, Funded, Active, and Settling all count toward this limit. Expiry releases the slot; destroying an already expired lease does not release it a second time.

## Resource limits and closure

Declare execution ceilings, namespace capacity, lifetime in protocol batches, and escrow before activation. Usage observations are monotonic: a later observation cannot reset consumed resources. Execution refuses an exhausted resource budget or a request at or after expiry. Recording usage beyond a resource bound, at expiry, or beyond escrow requires verified `CloseBoundExceeded` transition evidence tied to that observation; the lease then moves from Active to Settling. Without that evidence, the usage update is refused.

| Dimension | Protocol declaration ceiling |
| --- | --- |
| CPU fuel | 1,000,000,000 |
| Linear memory | 1 GiB |
| Storage reads / writes | 1 GiB each |
| Output bytes | 1 GiB |
| Output values | 65,536 |
| Table elements | 1,048,576 |
| Namespace storage | 1 GiB, including persisted snapshot records |
| Lifetime | 1,000,000 protocol batches |

Escrow is also capped: a lease escrow must be nonzero and at most 1,000,000,000,000,000,000,000,000. Each lease records at most 64 snapshots and 70 lifecycle transitions. These are maximum declarations, not allocations promised to every workload. Each lease selects its own admitted limits and fee coverage. Meter memory, state, outputs, snapshot persistence, and restoration together when planning a resumable workload.

## Authority that cannot escape the lease

The execution principal and storage namespace derive from the host program and lease identity. The capability set contains only `StorageRead` and `StorageWrite`. Shared storage, native transfers, balance reads, receipt reads, event emission, and calls to other programs are absent.

A guest that imports event or call functions does not gain their authority. Attempted event emission refuses; a call to an unleased program fails the composition capability check. The refused operation leaves sandbox storage unchanged. The boundary is enforced by capability derivation rather than by trusting the guest to follow a convention.

## Persist execution state and resume it

### Canonical snapshots

A renter-authorized snapshot captures the source lease, host program, namespace prefix, linear memory, globals, continuation, and ordered namespace cells. Canonical encoding begins with `LayerX/programs/sandbox/snapshot/v1` followed by a NUL byte. SHA-256 commits the exact encoded state.

Persistence stores a 44-byte manifest containing the digest, byte length, and chunk count, followed by chunks of up to 65,536 bytes. Metered storage includes keys and values. Snapshot records share the lease's namespace capacity with live state, so repeated snapshots cannot bypass the storage ceiling.

### Restore into a funded target

Restore requires the canonical snapshot state, a recognized snapshot digest, a Funded target lease, the same owner principal, the same host program, the same image hash and a guest that supports continuation. Live cells are rebound to the target namespace. Restoration charges the canonical snapshot bytes and reconstructed cells before instantiation; reconstructing memory and the continuation also consumes runtime resources.

A successful restore preserves the captured memory, globals, continuation entry point and arguments, and namespace cells. Retain the canonical snapshot bytes before destroying the source storage. The target is a new bounded lease. Destroying the source lease does not make its identity reusable.

## End-to-end workflow

1. Validate and identify the guest image. Choose the host program and renter principal.
2. Request a lease with explicit resource, namespace, lifetime, escrow, and fee-schedule bounds. Retain the verified request evidence.
3. Fund and activate through the admitted lifecycle transitions.
4. Run under derived lease capabilities and record monotonic usage evidence.
5. If resumption is required, persist a renter-authorized snapshot while Active and retain its canonical bytes and digest.
6. Settle, expire, and destroy the source lease with the required evidence.
7. Fund a compatible target lease, restore the snapshot with sufficient reconstruction budget, and continue under the new lease's bounds.

## Results and verification

Retain the lease ID, image hash, transition receipt digests, batch sequences, usage observations, and snapshot digest where applicable. These identify what ran, under which bounds, which state was captured, and how the lease reached its final state. A guest return value alone does not prove funding, settlement, or destruction.

Sandbox trace replay also supports deterministic compute-market dispute adjudication: the arbiter checks the step to which a bisection dispute resolves. The sandbox remains storage-only even when a surrounding market program manages stake, claims, or settlement.

Read [Programs](https://docs.paxeer.app/programs) for deployment, broader capability grants, native value accounts, and atomic transfer settlement.

**Technical references:** `programs/crates/layerx-programs-sandbox/src/` contains `lease.rs`, `execute.rs`, `snapshot.rs` and `expiry.rs`; market replay verification in `programs/crates/layerx-programs-arbiter/src/market.rs`.
