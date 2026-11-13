# Sandbox

Sandbox leases bound guest execution by tenant, image, namespace, resources,
escrow and lifetime. The implementation is in
`programs/crates/layerx-programs-sandbox/`, with native controllers in
`src/modules/programs/sandbox.c` and `sandbox_expiry.c` and EVM handlers in
`precompiles/layerxprograms/`.

Sandbox work executes through the same node host as other
[Programs](index.md). A committed transaction is final with its block.
Lease settlement is resource accounting within that network, not an external
finality step. The Rust state machine, chain-proof consumers and native writer
share canonical lease, escrow and usage encodings.

## Lease states and transitions

`LeaseState` in `src/lease.rs` defines:

| State | Tag |
| --- | ---: |
| `Requested` | 0 |
| `Funded` | 1 |
| `Active` | 2 |
| `Settling` | 3 |
| `Expired` | 4 |
| `Destroyed` | 5 |

Only `Destroyed` is terminal. `Lease::request_with_schedule` constructs a
requested lease. `LeaseActivity` tags are Request 0, Fund 1, Activate 2,
BeginSettlement 3, Expire 4, Destroy 5, CloseBoundExceeded 6 and Snapshot 7.
The declared state edges are:

| Activity | From | To | Principal slot |
| --- | --- | --- | --- |
| Request | Requested | Requested | Acquire |
| Fund | Requested | Funded | Hold |
| Activate | Funded | Active | Hold |
| BeginSettlement | Active | Settling | Hold |
| CloseBoundExceeded | Active | Settling | Hold |
| Snapshot | Active | Active | Hold |
| Expire | Requested, Funded, Active or Settling | Expired | Release |
| Destroy | Expired | Destroyed | None |

`LeaseBook` allows at most `MAX_CONCURRENT_LEASES_PER_PRINCIPAL = 32`.
Requested, funded, active and settling leases hold slots. Expiry releases a
slot; destruction does not release it a second time. The 33rd concurrent
request is `LeaseRefusal::PrincipalLeaseLimit`.

A direct `Lease::transition` of Request or CloseBoundExceeded is
`IntrinsicActivityRequired`; Destroy is `StorageRequired`. Request admission,
usage accounting and storage-aware destruction use their own entry points.
`record_usage` closes an active lease with `UsageOutcome::ClosedByBound` when
resource, lifetime or escrow bounds are exceeded. Expiry from a state other
than Settling requires the observed batch to reach the lease expiry, otherwise
it is `NotExpired`. Replay, stale-state, tenant and history checks apply to
receipt-backed transitions.

## Native funding, activation and execution

The merged EVM entry point is the Programs precompile at
`0x000000000000000000000000000000000000101D`. Use its checked-in ABI rather
than encoding a standalone transport. The handlers submit kernel activity
`LX_PROGRAMS_SANDBOX = 0x00090009`, with version-1 payload operations:

| Method | Operation byte | Native effect |
| --- | ---: | --- |
| `sandboxFund` | 2 | Validate requested terms, transfer escrow funding, write Funded lease, escrow and empty usage ledger |
| `sandboxActivate` | 3 | Validate expected lease digest and history sequence, write Active lease |
| `sandboxExecute` | 1 | Run the bound guest, meter usage and publish lease/escrow/ledger updates and usage events |

Funding binds lease ID, tenant, host program, escrow account, Asset, amount,
expiry, fee schedule, canonical lease and authenticated transfer legs.
Activation takes `(leaseId, expectedLeaseDigest, expectedSequence)`. Its
sequence is the next lease-history entry; execution has its own expected usage
sequence. Keep those two sequence domains distinct.

Execution additionally binds the expected lease digest, escrow account, Asset,
fee destination, fee schedule, native CALL payload and fee limit. It returns
the Programs terminal tuple described in [Programs](index.md#calling-the-merged-network).
A guest result and a successful outer transaction are different facts: inspect
both the terminal kind and the committed transaction result.

`host_ffi.rs` owns native lifecycle state staging. Funding writes kinds 0, 1
and 2; activation updates kind 0 without producing a usage receipt. Execution
publishes the canonical lease, escrow and usage ledger together. Capability and
resource checks come from the lease rather than caller-supplied ambient
permissions.

## Mounted state keys and canonical values

Kernel-local sandbox keys are `s || kind:u8 || lease_id32`. A chain proof must
address the **`lxphost`** store, using the module prefix from
`programs/crates/layerx-programs-registry/src/chain_evidence.rs`:

```text
mounted_key = 0x01 || module_id:u16_be || kernel_key
module_id   = 9
```

Thus `sandbox_state_key(kind, lease)` in `src/chain_keys.rs` returns
`01 00 09 73 <kind> <lease_id32>` and admits kinds 0–2. It does not derive the
destruction terminal key.

| Kernel-local key | Native producer value |
| --- | --- |
| `s || 0 || lease_id32` | `Lease::canonical_state_bytes`, domain `LayerX/programs/sandbox/lease-state/v3\0` |
| `s || 1 || lease_id32` | `Escrow::canonical_state`, domain `LayerX/programs/sandbox/escrow-state/v1\0` |
| `s || 2 || lease_id32` | `UsageLedger::canonical_state`, domain `LayerX/programs/sandbox/usage-ledger/v2\0` |
| `ts || lease_id32` | Native destruction terminal, 352 bytes beginning `LXSD1` |
| `xc || lease_id32` | Native resumable cleanup cursor beginning `LXCL1`, removed after cleanup completes |

All integer fields below are big endian. The variable-length lease encoding
contains, in order:

1. The lease-state domain and eight 32-byte identifiers: lease, tenant, host
   program, image hash, namespace, escrow Asset, escrow account and fee destination.
2. u128 escrow amount; eight u64 resource/namespace limits; u64 opened batch
   and expiry; eight u64 usage values; u32 fee-schedule version and seven u64
   prices; u128 consumed escrow.
3. State tag, u8 history count and history records. Each record contains
   activity/from/to tags, activity ID, usage-observation digest, receipt digest
   and u64 batch sequence.
4. u16 snapshot-record count and snapshot bindings, followed by an optional
   restored-from digest.

The escrow encoding contains its domain, lease/account/Asset IDs, u128 funded,
spent and refunded amounts, funding root, optional settlement root and finalized
flag. `Escrow::decode_state` validates it against the lease.

The usage ledger contains its domain, u64 receipt count, u128 spent amount,
accumulator root, and an optional latest receipt framed by a u32 length plus
its digest. `UsageLedger::decode_state` validates it against the lease and
escrow. Individual usage receipts use
`LayerX/programs/sandbox/usage-receipt/v4\0`.

`LXUR` is an **event frame**, not the stored escrow or usage-ledger domain.
`sandbox.c::emit_usage_receipt` emits chunks with a 52-byte header: magic,
activity ID, expected sequence, u16 chunk index/count and u32 total receipt
length, followed by chunk bytes. EVM `SandboxUsageReceipt` logs expose that
chunking. Authenticate and reassemble all chunks for the same activity and
sequence before interpreting the receipt.

`Host.ReadSandbox` and precompile `getSandbox(leaseId, kind)` read the three
live `s` records. They do not expose the `ts` terminal key. A value returned by
a view is not itself an authenticated state proof.

## Chain-proof evidence

`SandboxProtocolContext::new` accepts protocol 3 and active Programs
registration ABI 4. Match that context to the selected host registration;
it is separate from the guest ABI used by the sandbox image.

`AuthenticatedSandboxReceiptEvidence` combines `SandboxReceiptEvidence`
with `SandboxInvocationEvidence`. The chain evidence carries transaction
bytes and inclusion proof, a transaction result and result proof, and
mounted-state proof operations. Invocation evidence carries the signed
Ethereum transaction, native activity bytes, the signer's pre-state DID
binding and, for activation or execution, the pre-state lease proof.
An unbound `SandboxReceiptEvidence` is refused by
`SandboxSubmissionReceipt::verify`.

The verifier authenticates signed chain heads and transaction inclusion at
height H, and proves the successful transaction result through the committed
successor header at H+1. It checks transaction/result indexes, the header
link and next-validator hash. The invocation verifier binds the included
node message to the signed EVM transaction, chain, caller, Programs
precompile, exact selector and canonical calldata. The signer binding and
prior lease establish the tenant and expected native activity.

Funding and activation prove the resulting kind-0 lease. Execution proves
the kind-2 canonical `UsageLedger` and authenticates its latest receipt,
including activity, lease, sequence, prior digest, fee destination and
prices. `verify_native_usage_frames` compares the `LXUR` event chunks with
that authenticated receipt. Kind 1 remains the escrow encoding.

`AuthenticatedTerminalRecord::verify` uses `sandbox_terminal_key` for the
mounted `ts || lease_id32` destruction terminal. It verifies chain trust,
freshness, exact store/key/proof operations and the 352-byte `LXSD1` value.
The `getSandbox` view reads the live kinds 0–2; terminal verification uses
the separate mounted-state proof.

`TransitionEvidence::verify_call` verifies the resulting kind-0 lease,
tenant/request terms, state edge, history extension and sequence. Keep
transaction inclusion and invocation evidence alongside this state proof.

Source contracts are `src/activity.rs`, `src/invocation.rs`,
`src/chain_keys.rs` and `src/expiry.rs` under
`programs/crates/layerx-programs-sandbox/`. Chain-proof verification lives
under `programs/crates/layerx-programs-registry/src/chain_proof/`.

## Expiry, destruction and recovery

`src/expiry.rs` declares `MAX_SWEEP_LEASES_PER_BATCH = 128` and
`MAX_EXPIRY_QUEUE_ENTRIES = 4096`. Native expiry and cleanup are implemented in
`sandbox_expiry.c`. Cleanup can span bounded pages and records its progress in
`xc || lease_id32`. At completion it removes live namespace/snapshot storage,
settles remaining occupancy and refund effects through authenticated transfers,
deletes the three live sandbox records and writes the destruction terminal.

The native terminal starts with `LXSD1`, lease ID, destroy activity ID, expected
sequence, boundary and expected lease root. It also records settlement and
reclamation evidence, including namespace/snapshot roots and reclaimed
cell/byte counts. `SandboxDestroyed` publishes terminal parts as events. For a
remote proof, the exact terminal key is:

```text
01 00 09 || "ts" || lease_id32
```

Persist the actual transaction/result evidence, authenticated lease and usage
sequence, pending cleanup position and terminal evidence needed by the
consumer. After node-store reopen, recover the same mounted records and resume
from their recorded progress. Local Rust models and independently constructed
proof fixtures alone do not establish native reopen recovery. In particular,
retain the source lease's snapshot binding before destruction removes the
live kind-0 record if later restoration will need it.

## Capability refusals

`LeaseCapabilities::derive` in `src/execute.rs` derives the lease namespace's
execution principal and storage namespace. It grants only `StorageRead` and
`StorageWrite`; the canonical encoding is `[0, 2, 1, 2]`. Shared storage,
transfers, balance reads, receipt reads, guest events and callee authority are
absent. Metering and escrow charges are host operations, not spending
capabilities given to the image.

`execute_in_lease` checks Active state, expiry, the leased image and remaining
resource ceilings before committing candidate storage. Refusals include
`LeaseNotActive`, `LeaseExpired`, `ImageMismatch`, `CeilingExhausted` and
`GrowthCeilingExhausted`; runtime program failures remain distinct from those
lease refusals. Denied event emission and cross-program calls are exercised by
the hostile-image tests in `execute.rs`.

## Snapshot persist and restore metering

`src/snapshot.rs` implements renter-authorized capture and restoration.
`SandboxState::canonical_bytes` encodes:

```text
"LayerX/programs/sandbox/snapshot/v1\0"
|| source_lease_id32 || host_program_id32 || namespace32
|| length-prefixed linear memory
|| globals || continuation entrypoint/arguments || ordered namespace cells
```

`Snapshot::commit` requires an Active lease, the leased image, a capture from
that runtime instance and a verified Snapshot transition bound to the snapshot
digest. The digest is SHA-256 of the canonical bytes. The snapshot binding
records owner, source lease/namespace, host, image, byte length and chunk count.

Persistence writes a 44-byte manifest (digest, u64 byte length, u32 chunk count)
and 65,536-byte chunks under the snapshot protocol namespace. Storage metering
counts **keys and values**, including manifest and chunk overhead. Live
namespace bytes plus stored snapshots must remain within the lease's namespace
ceiling; otherwise persistence is `TargetBoundExceeded`.

`restore` requires a snapshot binding in the retained source lease, a Funded
target, the snapshot owner's authorization, matching host/image and a valid
activation transition. It validates the supplied snapshot's source, digest,
length and target bounds. Unless the source is Destroyed, it also checks the
persisted source snapshot bytes.

Restoration charges storage-write bytes for the canonical snapshot plus
rebound live cells before runtime instantiation. Instantiating linear memory
and restoring the continuation incur their own resource charges. Budgeting only
the persisted snapshot size is insufficient. Candidate lease, storage and meter
state are used so a refused restoration does not publish a partially restored
target.

## Source coverage

The crate's lease tests cover state edges, tenant and replay refusals,
resource bounds and principal concurrency. Runtime tests exercise restricted
capabilities and hostile WASM. Snapshot tests cover capture, serialization,
restoration, ownership and metering. Chain-evidence tests cover authenticated
invocations, mounted lease/usage records, native terminal proofs and refusal
of mismatched evidence. The expiry cases retain terminal state across store
reopen.

[Programs](index.md) · [Home](../index.md)
