# Programs contract

Programs execute deterministic WebAssembly inside the Paxeer network. `paxd` hosts the
kernel, which participates in block proposal; a receipt is final when its block is
committed. The EVM chain ID is **125 (`0x7d`)**. Program execution and monetary effects
belong to this same chain.

Use HTTPS JSON-RPC at the configured node origin's root path. Check
`eth_chainId` before signing or submitting a transaction.
Sub-chains are tenant domains in the database; a tenant namespace does not require
another chain process.

This page describes the implemented Programs boundary. The
[EVM ABI](../precompiles/layerxprograms/abi.json),
[kernel declarations](../include/layerx/programs.h), and source links below define
the field layouts. An interface declaration alone does not establish that an RPC
service is registered.

## LayerXPrograms 0x101D

The Programs precompile is **`0x000000000000000000000000000000000000101D`**.
[Construction and registration](../precompiles/layerxprograms/layerxprograms.go)
connect it to `modules/lxphost`, the EVM keeper and the XWeb keeper through
`HostKeepers.LayerXHost()` and `HostKeepers.XWebK()`. The address is the Go variable
`Address` in [events.go](../precompiles/layerxprograms/events.go).

All writes are nonpayable. Write methods refuse static calls, and dispatch refuses
delegate calls. Submissions resolve the caller's bound DID and authority, obtain
its account and activity sequences, and derive an idempotency key from the EVM
transaction hash, call ordinal and selector. See
[submission](../precompiles/layerxprograms/submit.go) and
[idempotency](../precompiles/layerxprograms/idempotency.go).

| Operations | ABI methods |
| --- | --- |
| Deployment and upgrade | `deploy`, `upgrade` |
| Execution | `call`, `callWithReceipts`, `simulateCall` |
| Transfers and account registration | `transfer`, `registerAccount`, `registryRead` |
| Wind-down | `setExitRoute`, `deprecate`, `tombstone`, `exitValue`, `exitValueBounded` |
| Sandbox lifecycle | `sandboxFund`, `sandboxActivate`, `sandboxExecute` |
| Storage retirement | `retireStorage` |
| Program metadata | `getProgram`, `getInterface`, `getProgramOwner`, `getAccountProfile`, `getStateHead` |
| Accounts and wind-down views | `getProgramAccount`, `getProgramAccountById`, `getValueAccount`, `getExitRoute`, `getWindDown` |
| Fees and metering | `getCurrentFeeSchedule`, `getFeeSchedule`, `getPendingFeeSchedule`, `quoteCallFee`, `getCurrentMeteringSchedule`, `getMeteringSchedule` |
| Other state views | `getSandbox`, `getRetirementControl`, `getWebRequest` |

Use the [Solidity interface](../precompiles/layerxprograms/LayerXPrograms.sol) or ABI
for full parameter order and widths. `getValueAccount` returns `(uint128 balance,
bool frozen)`. Fee schedule views include the `bytes32 feeAsset`; consumers must
retain it when interpreting prices and fee ceilings.

### Call results and events

`call` and `simulateCall` take `(bytes32 programId, uint16 abiVersion, string
entrypoint, bytes callData, bytes capabilities, bytes accessDeclaration, uint32
responseCapacity, uint64[7] budget, uint128 feeLimit)` and return
`(int32 terminalResult, uint8 terminalKind, bytes response)`.

`callWithReceipts` takes the same arguments followed by
`ReceiptAuthority[] receiptAuthorities`. Each tuple contains `bytes canonicalReceipt`
and `uint64 blockHeight`; the handler passes those authorities to the host. It
returns the same terminal result tuple. See the
[receipt-aware handler](../precompiles/layerxprograms/method_call_receipts.go).

`response` contains the complete terminal evidence frame. `responseCapacity`
bounds the guest bytes nested inside a successful execution; it does not bound or
truncate the evidence frame. Both the guest capacity and the decoder's independent
terminal-frame bound are at most 1,048,576 bytes. Use
[`DecodeProgramExecutionResult`](../precompiles/layerxprograms/response.go) and the
[response framing contract](../precompiles/layerxprograms/response.md).

The outer frame is `LXP/programs/terminal-applied-legs/v1\0`, followed by a
big-endian `u32` execution length, execution bytes, a `u32` applied-legs length and
the applied legs. Authenticate the expected transfer root independently from the
receipt or a trusted local simulation before decoding. A root calculated only from
the returned bytes cannot authenticate those bytes.

The ABI declares 14 events. Lifecycle and transfer logs are projected by
[events.go](../precompiles/layerxprograms/events.go); call logs by
[events_call.go](../precompiles/layerxprograms/events_call.go); sandbox logs by
[events_sandbox.go](../precompiles/layerxprograms/events_sandbox.go). Verify the
emitter is 0x101D, topic 0 matches the ABI event signature, and the indexed identity
matches the operation before using log data as evidence.

In the current ABI, `ProgramGuestEvent` carries indexed `programId`, then
`principal`, `activityId`, `framePath`, `depth`, `eventIndex`, `topicLength`,
`topicDigest`, `dataLength` and `dataDigest`. It carries digest evidence for guest
content. `ProgramCallOutcome` additionally binds runtime, ABI, fee and metering
versions, terminal result, transfer root, call graph, terminal detail and event
envelope digests. Bind these to the same transaction receipt and block.

Simulation uses `Host.Simulate`, returns the same response tuple, and does not
publish committed logs or mutate persistent state, sequences, balances or events.
Its result is a simulation result; it does not establish a committed execution.

## Frozen ABI

The [shared ABI policy](../programs/sdk/rust/src/abi_policy.rs) admits guest
revisions **1 through 5**. An upgrade may retain or increase the revision within
that set; downgrades and unsupported revisions are refused. ABI 1 uses capability
encoding V1; ABI 2 through 5 use V2. The runtime's default `ABI_VERSION` is 4;
select the deployed program's revision explicitly when calling or replaying it.

Guest ABI revision, kernel activity/module version, runtime version and receipt
encoding version are separate fields. In particular, `LX_PROGRAMS_ABI_VERSION = 1`
does not limit guests to ABI 1. Registration and execution must agree on the ABI
in the program record, interface encoding and request. The
[interface validator](../programs/crates/layerx-programs-runtime/src/ffi_interface.rs)
checks this agreement and the code hash; mismatches return context/version refusals.

### Fmt ABI frozen

[`programs/abi-frozen.sha256`](../programs/abi-frozen.sha256) pins the checksums of
the allocated ABI vectors. Existing version bytes remain frozen. The
[ABI manifest](../programs/sdk/abi-manifest.tsv) records revision, import module,
host function and WASM signature. It lists the `layerx_v1` compatibility imports,
`layerx_v2` response, scoped-storage, account-transfer, context and crypto imports,
`layerx_v3.oracle_read`, and `layerx_v4.web_read`.

## Runtime crate

[`layerx-programs-runtime`](../programs/crates/layerx-programs-runtime/src/lib.rs)
validates modules, meters resources, enforces capabilities and access declarations,
and executes through the pinned WASM interpreter. The
[validator](../programs/crates/layerx-programs-runtime/src/validate.rs) refuses
floating-point types/instructions, vector types, undeclared imports, duplicate
imports and mismatched import signatures. Its
[default validation limits](../programs/crates/layerx-programs-runtime/src/limits.rs)
are 1,048,576 module bytes, 4,096 functions, 65,536 value-stack entries and a WASM
call depth of 512. These are validation ceilings; transaction payload limits can
be tighter.

Storage is namespaced and checked against the invocation's authority and resource
budget. Cross-program grants may narrow authority; they cannot widen it. Programs
stage authenticated `402LXP` transfer sets for the kernel's transfer primitive and
have no direct balance-writing authority. Program-account authority also binds
the owning program and seed. See
[capabilities](../programs/crates/layerx-programs-runtime/src/abi/capability.rs),
[transfers](../programs/crates/layerx-programs-runtime/src/transfer.rs) and
[kernel accounts](../src/modules/programs/accounts.c).

### Terminal encoding

After decoding the outer applied-legs frame, use
[`decode_terminal_payload(kind, abi, encoded)`](../programs/crates/layerx-programs-runtime/src/terminal.rs)
for native terminal details. It returns typed execution, failure or resource
details and preserves occupancy and transfer-authority attachments. Kind 1 is
execution success, kind 2 failure and kind 3 resource refusal. Execution version
2/3 is historical ABI-1 evidence, version 4 is ABI 2, and version 5 is ABI 3–5.
The decoder checks the declared kind and ABI against the encoded outcome.

The NUL-terminated domains `LXP/program-execution-with-transfer-authority/v2\0`
and `LXP/program-execution-with-occupancy/v1\0` identify wrappers around execution
bytes. Preserve their authorization, transfer root and occupancy evidence.
Malformed encodings, inconsistent kind/ABI, unknown tags and trailing bytes are
refused. The Go execution-result binding handles execution frames; non-execution
failure details require their native typed decoder.

Failures retain their typed fields: resource, limit and attempted usage; program
identity and refusal; ABI/entrypoint/composition errors; or callback stage and
status. Preserve those details alongside the outer result code. A program refusal
and an exhausted resource budget are different outcomes.

### Replay and recovery

[`ReplayHostWitnessV1`](../programs/crates/layerx-programs-runtime/src/replay.rs)
binds code hash, ABI state, runtime state, meter and storage baseline under
`LayerX/programs/replay-host-witness/v1\0`. Trace and replay records must retain
their code, runtime, ABI, schedule and authority bindings. Reconstructing an
execution with different host state is not evidence for the recorded transition.
Persistent state is loaded through the chain host; an interpreter or compiled
module cache does not establish durable state or receipt authority.

## Runtime FFI

The chain calls the C Programs module through
[`modules/lxphost`](../modules/lxphost/host.go), linked to the Rust runtime and
sandbox exports. Programs activity codes are defined in
[`programs.h`](../include/layerx/programs.h): deploy `0x00090001`, upgrade
`0x00090002`, call `0x00090003`, registry `0x00090004`, transfer `0x00090005`,
account `0x00090006`, wind-down `0x00090007`, sandbox `0x00090009`, sandbox
destroy `0x0009000A`, and storage retirement `0x0009000B`.

The [call bridge](../programs/crates/layerx-programs-runtime/src/ffi_call.rs) and
[interface bridge](../programs/crates/layerx-programs-runtime/src/ffi_interface.rs)
read activity-owned bytes using bounded callbacks and tokens. Tokens do not grant
authority outside their invocation. Preserve signed status widths and exact
version fields across C and Rust. A negative kernel status must not be cast into
an unsigned success value.

## Host interface

[`Host.Execute`](../modules/lxphost/host.go),
[`Host.Simulate`](../modules/lxphost/simulate.go) and the
[program reads](../modules/lxphost/read_program.go) are the node-facing boundary.
Execution binds the current SDK context, caller authority, sequence, fee ceiling
and canonical activity payload. Read APIs expose program metadata, interfaces,
accounts, lifecycle, sandbox and retirement records from host state.
The [receipt-authority code](../modules/lxphost/host.go) supplies
height-bound native authority; arbitrary receipt bytes do not authorize a call.

## Registry crate

[`layerx-programs-registry`](../programs/crates/layerx-programs-registry/src/lib.rs)
tracks verified deployments, interfaces, value-account bindings and lifecycle.
Balance reads use authenticated account snapshots and registered bindings rather
than a balance declared in an application record. Preserve the asset identifier,
frozen status, proof head, state root and freshness information with each read.

[`DeploymentEnvelope`](../programs/crates/layerx-programs-registry/src/deployment_envelope.rs)
pairs a deployment record with its admission proof under
`LayerX/deployment-envelope/v1\0`. Its encoding frames the record and proof and
seals both together. Durable publication stages, syncs and renames this unit so
recovery observes both parts or neither. Decoding checks framing, seal and
provenance; replay must still verify the evidence before admitting a deployment.

## Registry evidence

The chain store is **`lxphost`**. A Programs kernel key becomes a chain-store key as
`0x01 || u16_be(9) || kernel_key`, hence prefix **`01 00 09`**. This layout is
implemented by
[`programs_state_key`](../programs/crates/layerx-programs-registry/src/chain_evidence.rs).
For example, a program record uses `program\0 || program_id` after that prefix.

[`ChainProgramStateProof`](../programs/crates/layerx-programs-registry/src/chain_evidence/program_state.rs)
checks the expected store, complete key, value, program identity and proof against
a verified chain head. Retain the resulting state height and app hash. A bare EVM
view value is useful for an ordinary read but does not contain this proof bundle.

## Chain proof

[`ChainProofVerifier`](../programs/crates/layerx-programs-registry/src/chain_proof/head_verify.rs)
produces `VerifiedChainHead` after checking the chain header and commit against its
trust state. The [proof implementation](../programs/crates/layerx-programs-registry/src/chain_proof.rs)
defines header, validator, vote, transaction/result and IAVL/multistore verification.
State membership and absence must bind the exact store and key to the authenticated
app hash. Trust roots and freshness limits are verifier inputs; a self-hashed record
or a signed receipt without the required chain binding is insufficient.

## Protocol adapter

The maintained proof-backed read type is
[`ProtocolProgramStateRead`](../programs/crates/layerx-programs-registry/src/state_record.rs)
in the registry crate. Its canonical encoding is fallible. `restore_verified`
requires independently verified receipt and current account-state heads plus
time and staleness bounds before publishing restored balances. Profile-2 reads
use the explicit profile-2 restoration path and evidence. Stored bytes cannot
authorize themselves. Use this type for proof-backed reads rather than assuming
an independently deployed adapter service.

## Sandbox crate

[`layerx-programs-sandbox`](../programs/crates/layerx-programs-sandbox/src/lib.rs)
models tenant-bound ephemeral execution. Lease states are `Requested = 0`,
`Funded = 1`, `Active = 2`, `Settling = 3`, `Expired = 4`, `Destroyed = 5`.
Only declared transitions are accepted; destruction is terminal.

The [lease contract](../programs/crates/layerx-programs-sandbox/src/lease.rs)
binds tenant, host program, image hash, namespace, escrow account/asset,
fee destination, limits, schedule, expiry and history. There are at most 32
concurrent leases per principal. Caller-selected budgets remain subject to both
lease and runtime limits.

### State keys and transition evidence

[`sandbox_state_key(kind, lease_id)`](../programs/crates/layerx-programs-sandbox/src/chain_keys.rs)
returns `01 00 09 || 's' || kind_u8 || lease_id_32`. `getSandbox` accepts kinds
0 through 2: the C host stores lease, escrow and usage-ledger state respectively.
Use their canonical decoders; these are different record types. Missing records
are returned as `LXP_NOT_FOUND`, and invalid kinds as `LXP_NON_CANONICAL`.

`TransitionEvidence::verify_call` takes `&VerifiedChainHead`, `&Lease`, the
declared `LeaseTransition`, `&AuthorizationContext`, canonical resulting lease
bytes and lease `ProofOp`s. It verifies the kind-0 `lxphost` state proof, tenant
and request binding, allowed edge, resulting state and exact history extension.
It refuses a stale starting state or a history/receipt mismatch. Sandbox
submission verification and funding use `ChainProofVerifier` in
[activity.rs](../programs/crates/layerx-programs-sandbox/src/activity.rs).

### Settlement, expiry and restart

[`UsageReceipt`](../programs/crates/layerx-programs-sandbox/src/usage.rs) binds
the activity, expected/resulting lease digests, sequence, previous receipt and
accumulator, observed usage, fee schedule, cumulative spend and transfer root.
Its current domain is `LayerX/programs/sandbox/usage-receipt/v4\0`.
`DurableUsageState` retains the receipt chain and accumulator for recovery;
an independently hashed usage value is not an `AuthenticatedUsageReceipt`.

Expiry reclamation is bounded: `MAX_SWEEP_LEASES_PER_BATCH = 128` in
[expiry.rs](../programs/crates/layerx-programs-sandbox/src/expiry.rs). A caller may
choose a smaller sweep page. The
[C expiry path](../src/modules/programs/sandbox_expiry.c) persists terminal and
cleanup state so bounded reclamation can continue across blocks.

[`restore`](../programs/crates/layerx-programs-sandbox/src/snapshot.rs) validates
snapshot digest, source/target lease bindings, authorization, module and activation
evidence before replacing the target namespace. It checks namespace and resource
bounds and prepares candidate storage/meter state before publication. Restart
must restore canonical lease, escrow and usage state together; it must not reset
usage, spend, sequence or duplicate-submission protection.

## Arbiter crate

[`layerx-programs-arbiter`](../programs/crates/layerx-programs-arbiter/src/lib.rs)
verifies replay evidence and market-step verdicts. `ReplayMetadata` is proven
under the `lxphost` key `01 00 09 || "progreplay/v1/" || activity_id` against an
authenticated app hash. The
[authority path](../programs/crates/layerx-programs-arbiter/src/authority.rs)
authenticates receipt keys and admission facts; the
[step verifier](../programs/crates/layerx-programs-arbiter/src/step.rs) checks code,
catalogue, lifecycle, interface, boundary adjacency and execution bindings.
An untrusted decoded replay record does not establish an authenticated verdict.

## Interpreter guest

[`layerx-programs-interpreter`](../programs/crates/layerx-programs-interpreter/src/lib.rs)
is an ordinary bounded scripting guest. Scripts use magic `LXSI`, version 1, a
10-byte header, at most 4,096 bytes, 16 registers, 4,096 expanded steps and repeat
nesting depth 4. The complete script is validated before staging storage or
transfer effects. It has no independent clock, entropy or authority source.

## Market guest

[`layerx-programs-market`](../programs/crates/layerx-programs-market/src/lib.rs)
implements offers, leases, external-input commitments, attestations, usage claims,
challenges and finalization. Persistent keys use the `lx.market.*` prefixes in
that source. Financial effects use authenticated program transfers and the
kernel's balance writer. Replay and dispute evidence use the
[arbiter contract](#arbiter-crate).

## AI market guest

[`layerx-programs-ai-market`](../programs/crates/layerx-programs-ai-market/src/lib.rs)
implements PAXAI codecs, admission, registry, tasks, workers, aggregation,
reputation and rewards. Codecs do not establish host authority or finality.
`PAXAI_CHAIN_DOMAIN` is read at compile time with `option_env!`; changing a process
environment after compilation does not change the guest domain.

The merged-chain domain is SHA-256 of `PAXAI/chain-domain/v1 || u64_be(125)`:
`62b6b14734fa769c3e0fbc94de495a3d038be49a2524b1c071d75d0a4eed71f6`.
The build setting is exactly 64 lowercase hex digits without `0x`; malformed or
foreign domains return `WRONG_DOMAIN`. The source uses this merged domain when
the setting is absent. Set the expected domain explicitly when packaging.

Deployment packaging requires a release guest with `opt-level = "z"`, and a
payload no larger than **524,288 bytes including its 104-byte deployment
envelope** (at most 524,184 guest bytes for that envelope). The protocol payload
ceiling is declared in [lxp_protocol.h](../include/layerx/lxp_protocol.h).
The workspace's general release profile uses optimization level 3, so packaging
must select the required size optimization explicitly. Preserve host/guest ABI
agreement in every registration path as described under [Frozen ABI](#frozen-abi).

## Guest SDK Rust

[`layerx-program-sdk`](../programs/sdk/rust/src/lib.rs) provides `no_std` guest
bindings, checked `Amount` (`u128`), bounded identifiers/buffers, canonical
capabilities, storage, transfers, response/refusal, context, oracle and web reads.
Host imports are enabled on `wasm32`; native builds expose the pure types and
codecs. Use explicit buffers and retain typed errors.

## Guest SDK C

[`layerx/program.h`](../programs/sdk/c/include/layerx/program.h) declares C guest
types and imports. Build a WASM guest against the chosen revision's signatures
and bounds. Integer widths, amount encoding and capability bytes must match the
[shared ABI](#frozen-abi).

## Guest SDK AssemblyScript

[`assembly/index.ts`](../programs/sdk/assemblyscript/assembly/index.ts) exports
the AssemblyScript SDK. Its host declarations and checked helpers implement the
same guest boundary. Determinism and numeric restrictions apply to guest source
and dependencies as well as entrypoints.

## SDK examples

Reference guests live in [Rust examples](../programs/sdk/rust/examples),
[C paid-counter](../programs/sdk/c/examples/paid-counter), and
[AssemblyScript paid-counter](../programs/sdk/assemblyscript/examples/paid-counter).
Use their interface and capability declarations with the corresponding compiled
artifact; source code alone does not prove deployment or current state.

## SDK lint

Determinism checks are implemented in the
[Rust lint crate](../programs/sdk/rust/lint),
[C linter](../programs/sdk/c/tools/determinism_lint.c), and
[AssemblyScript linter](../programs/sdk/assemblyscript/tools/determinism-lint.mjs).
Linting supplements runtime validation; it does not grant capabilities or relax
deployment limits.

## Test vectors

The [vector corpus](../programs/tests/vectors) includes ABI 1–5, terminal,
snapshot, calldata and chain-proof encodings. Existing ABI checksums remain pinned
by [abi-frozen.sha256](../programs/abi-frozen.sha256). Compare the exact allocated
encoding, including framing and byte order, when implementing another language.

## Fixtures

[`programs/fixtures`](../programs/fixtures) contains committed artifacts and
activity/receipt data. The
[fixture builder](../scripts/programs/program-fixtures.sh) records how reference
artifacts are produced. Fixtures are compatibility inputs, not current chain
authority or proof that a deployment is live.

## Fuzz harness

[`programs/fuzz`](../programs/fuzz/src/main.rs) exercises structured runtime and
proof inputs using the committed corpus. Malformed framing, imports, bounds and
proofs must produce the declared refusal; decoding success alone cannot establish
authority.

## Error codes

Kernel numeric results are defined in
[`lxp_result.h`](../include/layerx/lxp_result.h). Precompile error text uses the
`LXP_` names from [errors.go](../precompiles/layerxprograms/errors.go) and its
handlers. Keep those namespaces distinct from guest-defined refusal codes.

| Result or error | Meaning |
| --- | --- |
| `LXP_ERR_NON_CANONICAL` (`-3`) | Invalid canonical input or record encoding |
| `LXP_ERR_VERSION_UNSUPPORTED` (`-101`) | Unsupported version or required versioned state unavailable |
| `LXP_ERR_MODULE_DISABLED` (`-103`), `LXP_MODULE_DISABLED` | Module/activation unavailable |
| `LXP_ERR_PROGRAM_REFUSED` (`-736`) | Program call refused; inspect terminal details |
| `LXP_ADDR_EVM_NOT_BOUND` | Caller EVM address lacks the required DID binding |
| `LXP_PROGRAM_REFUSED` | Precompile refusal, including write under static call or delegate call |
| `LXP_PRECOMPILE_NO_CONTEXT` | Required host/keeper/EVM context unavailable |
| `LXP_PRECOMPILE_UNKNOWN_METHOD` | Missing or unregistered method selector |
| `LXP_NOT_FOUND` | Requested state absent in the relevant view |

`TerminalDecodeError::{Malformed,MismatchedKind,MismatchedAbi}` reports terminal
codec errors. `ChainProofError` distinguishes truncated/noncanonical/unknown-field
data, mismatches, insufficient voting power, expiration and absence. Preserve
these distinctions and the original evidence. Fatal kernel results are not
ordinary EVM reverts: `kernelError` panics for unmapped fatal codes.

## Config keys

Runtime behavior depends on authenticated chain parameters, schedules and the
declared invocation budget. The host's
[`Config`](../modules/lxphost/host.go) fields are `Workers`, `StoreKey`,
`NetworkID`, `Epoch`, `ParameterVersion`, `ProgramsGuestABI`, `TimestampWindowMs`
and `GasLimit`. They configure the embedded host; they are not independent guest
authority. The seven budget dimensions are CPU, memory bytes,
storage-read bytes, storage-write bytes, output values, output bytes and table
elements. Fee prices are CPU, memory, read, write, output values, output bytes and
occupancy byte-batches; do not interpret the seventh fee price as a table price.

Fee asset, schedule versions and activation fields must come from current chain
records or the specified historical record. Metering schedules carry nine
coefficients. The canonical fee record and pending record are decoded by
[view_fees.go](../precompiles/layerxprograms/view_fees.go); metering views by
[view_metering.go](../precompiles/layerxprograms/view_metering.go).
`PAXAI_CHAIN_DOMAIN` is a guest build setting described [above](#ai-market-guest).

### Protobuf declarations

The schema package is `paxprotocol.paxchain.layerxprograms`, with generated Go
types in `precompiles/layerxprograms/types`. The source schemas below declare
messages and services. This tree's precompile handlers execute through `lxphost`;
the schema descriptors by themselves do not register a separate Programs keeper,
Msg server or Query server. Use the implemented [precompile](#layerxprograms-0x101d)
and [proof contracts](#registry-evidence) for integration.

#### Proto layerxprograms FeeSchedule

[`FeeSchedule`](../api/layerxprograms/tx.proto) declares `version`, `cpu`,
`memory_byte`, `storage_read_byte`, `storage_write_byte`, `output_value`,
`output_byte` and `occupancy_byte_batch`.

#### Proto layerxprograms MeteringAuthorityKind

[`MeteringAuthorityKind`](../api/layerxprograms/tx.proto) allocates unspecified 0,
genesis 1 and governance 2.

#### Proto layerxprograms MeteringSchedule

[`MeteringSchedule`](../api/layerxprograms/tx.proto) carries version, coefficients,
activation height, authority kind and authority digest.

#### Proto layerxprograms RetirementProfile

[`RetirementProfile`](../api/layerxprograms/tx.proto) carries version, network ID,
controller, two archive keys, PAXAI program ID and hot horizon.

#### Proto layerxprograms MigrationProfile

[`MigrationProfile`](../api/layerxprograms/tx.proto) carries version, module and
parameter versions, activation epoch, authority digest and limits.

#### Proto layerxprograms Params

[`Params`](../api/layerxprograms/tx.proto) carries pause state, fee/metering
schedules, occupancy asset and demand-response bounds, retirement profile and
migration profile. These schema field names are not process environment keys.

#### Msg layerxprograms UpdateFeeSchedule

#### Proto layerxprograms MsgUpdateFeeSchedule

[`MsgUpdateFeeSchedule`](../api/layerxprograms/tx.proto) declares authority,
schedule, occupancy asset, demand-response bounds and activation height.

#### Proto layerxprograms MsgUpdateFeeScheduleResponse

The declared response is empty.

#### Msg layerxprograms UpdateMeteringSchedule

#### Proto layerxprograms MsgUpdateMeteringSchedule

[`MsgUpdateMeteringSchedule`](../api/layerxprograms/tx.proto) declares authority
and metering schedule.

#### Proto layerxprograms MsgUpdateMeteringScheduleResponse

The declared response is empty.

#### Msg layerxprograms StageRetirementProfile

#### Proto layerxprograms MsgStageRetirementProfile

[`MsgStageRetirementProfile`](../api/layerxprograms/tx.proto) declares authority
and retirement profile.

#### Proto layerxprograms MsgStageRetirementProfileResponse

The declared response is empty.

#### Msg layerxprograms UpdateParams

#### Proto layerxprograms MsgUpdateParams

[`MsgUpdateParams`](../api/layerxprograms/tx.proto) declares authority and params.

#### Proto layerxprograms MsgUpdateParamsResponse

The declared response is empty.

#### Proto layerxprograms GenesisState

[`GenesisState`](../api/layerxprograms/genesis.proto) declares params, repeated
program-store entries and repeated program blobs.

#### Proto layerxprograms ProgramStoreEntry

[`ProgramStoreEntry`](../api/layerxprograms/genesis.proto) is a byte key/value pair.

#### Proto layerxprograms ProgramBlob

[`ProgramBlob`](../api/layerxprograms/genesis.proto) is a byte key/value pair for
blob data.

#### Proto layerxprograms QueryProgramStateRequest

[`QueryProgramStateRequest`](../api/layerxprograms/query.proto) declares program
ID, key, height and proof selection.

#### Proto layerxprograms QueryProgramStateResponse

The declared response carries program ID, key, value, state record, height, app
hash, value proof operations and state-record proof operations.

#### Proto layerxprograms QueryProgramBalancesRequest

[`QueryProgramBalancesRequest`](../api/layerxprograms/query.proto) declares program
ID, height, proof selection and pagination.

#### Proto layerxprograms ProgramBalance

[`ProgramBalance`](../api/layerxprograms/query.proto) carries program/account/asset
IDs, seed, record version, registration sequence/digest, big-endian balance,
frozen flag, account/binding records and their proof operations.

#### Proto layerxprograms QueryProgramBalancesResponse

The declared response carries program ID, balances, height, app hash, pagination,
state record and state-record proof operations.

#### Proto layerxprograms QueryProgramEventsRequest

[`QueryProgramEventsRequest`](../api/layerxprograms/query.proto) declares program
ID, height range, pagination and an `after_sequence` cursor.

#### Proto layerxprograms ProgramEvent

[`ProgramEvent`](../api/layerxprograms/query.proto) carries program ID, height,
log index, transaction hash, event type, topics/data, global sequence, ordinal,
activity type and receipt digest.

#### Proto layerxprograms QueryProgramEventsResponse

The declared response carries events, pagination, `complete_through` and
`scanned_through`. Preserve those cursors separately when interpreting progress.
