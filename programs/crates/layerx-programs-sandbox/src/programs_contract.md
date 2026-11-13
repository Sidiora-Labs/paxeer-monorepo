# Programs sandbox crate

The Programs module registration ABI is 4 under protocol 3. Guest execution uses
its separately declared runtime ABI (the sandbox supports versions 1 through 4).
A sandbox submission starts with a one-byte version 1, an operation byte
(execute 1, fund 2, activate 3), two zero reserved bytes, then a 32-byte lease
identifier. Operation-specific fields follow this 36-byte header.
Activation carries a 32-byte expected lease digest and a big-endian u64 usage
sequence, for 76 bytes total; both fields must be nonzero. Execution and
funding carry their respective call or funding payloads. Malformed or
truncated submissions return `SandboxActivityRefusal::NonCanonical`.

## Chain transition authority

`TransitionEvidence::verify_call` consumes a `VerifiedChainHead`, the previous
`Lease`, the declared `LeaseTransition`, an `AuthorizationContext`, canonical
resulting lease bytes (`canonical_lease`) and ICS23 `lease_ops`. The producer is
the included Programs call; the transport is committed chain state and its
proof; the consumer is the lease transition verifier. The proof must authenticate
the resulting canonical lease in the `lxphost` store, at
`programs_state_key([b's', 0] + lease_id)` (module 9). Escrow and usage state use
kinds 1 and 2. The permanent native terminal uses
`programs_state_key(b"ts" + lease_id)`.

The verified chain head supplies chain authority and state height. The verifier
also checks the tenant, request binding, declared edge, prior receipt history,
last transition and receipt sequence. `InvalidCanonicalEvidence` refuses an
invalid state proof; tenant, lease and transition mismatches have distinct
`LeaseRefusal` variants. Activity/header kernel proofs are not inputs to this
interface and cannot substitute for chain state proof operations.

## Native destruction result

The C host supplies a `u64` call token to
`layerx_programs_sandbox_destroy_host` along with three pointers to 32-byte
lease/root/activity fields, a `u64` expected sequence and boundary. The Rust
producer reads canonical lease, escrow and ledger state from the host and
archives its resulting canonical state using the same token. It publishes a
144-byte result: receipt digest (32), refund transfer root (32), refunded amount
(16, big endian), destroyed lease digest (32), escrow digest (32).

`layerx_programs_sandbox_destroy_terminal(token: u64, output: *mut u8,
capacity: u32) -> i32` copies this result once. Zero means success, -213 means
token/context mismatch, and -3 means noncanonical input, missing result,
insufficient capacity or a caught panic. A refused read preserves the result
and output bytes. A successful read consumes the result. The host must supply
a writable, nonoverlapping output region of at least the declared capacity.
The C consumer places these 144 bytes at offsets 117 through 260 of its
352-byte `LXSD1` terminal record. Pending results are thread local and transient;
committed state and terminal archives are the recovery authority after restart.

## Expiry and snapshot recovery

The active expiry queue admits at most 4096 entries and one sweep selects at
most 128. Durable terminal history has independent capacity and retains replay
protection across more than 4096 lifetime leases. Sweep reserves terminal space
before invoking destruction. `ExpiryQueue::canonical_state` and `decode_state`
round trip both scheduled work and terminal history; replay, malformed records,
invalid digests and oversized active queues are refused. Allocation refusal is
`QueueFull`; unencodable counts are `AccountingOverflow`.

Snapshot canonical bytes bind the lease, host and namespace. Decoding bounds
memory and namespace bytes by lease limits, collection counts by protocol
maxima and minimum remaining encoded bytes, and uses fallible reservations.
`StateBoundExceeded` refuses protocol/allocation limits,
`TargetBoundExceeded` refuses target lease bounds, and
`NonCanonicalContinuation` refuses truncated or malformed bodies. Restore
requires separately proven renter authority; decoding never grants authority.
