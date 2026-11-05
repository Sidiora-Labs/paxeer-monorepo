# Programs

Programs runs deterministic WASM inside the Paxeer node's kernel host. Paxeer
is one BFT network; the kernel participates in block proposal and execution.
The EVM chain ID is `125` (`0x7d`); a receipt is final when its block is
committed. Program storage, authenticated `402LXP` transfers and the transaction
result belong to that execution. Persistent storage can accrue future occupancy
charges, but those charges do not defer transaction finality.

Programs is kernel module `9` (`LXP_MODULE_PROGRAMS`), with activity types
`0x0009xxxx`. Its EVM entry point is the `layerxprograms` precompile at
`0x000000000000000000000000000000000000101D`. Programs emit authenticated
transfer sets; guests have no `set_balance` authority.

| Source path | Role |
| --- | --- |
| `src/modules/programs/` | Kernel activity admission, lifecycle, accounts, calls, occupancy and sandbox controllers |
| `modules/lxphost/` | Mounted node host, execution, simulation, persistence and read accessors |
| `precompiles/layerxprograms/` | EVM ABI, payload encoders, handlers, events and response decoder |
| `programs/crates/layerx-programs-runtime/` | Deterministic WASM validation, execution, metering and capability checks |
| `programs/crates/layerx-programs-registry/` | Program interfaces, deployment and account evidence, chain proofs |
| `programs/crates/layerx-programs-sandbox/` | Bounded leases, isolated execution, snapshots and expiry |
| `programs/sdk/` | Rust, C and AssemblyScript guest SDKs |

See [Sandbox](sandbox.md) for lease execution and its integration limits,
[StorageScan](storage-scan.md) for scoped storage scans, and
[Payments](../overview/payments.md) and [Assets](../concepts/assets.md) for
payment integration. Source availability describes implementation in this tree;
it does not establish that a particular node has activated every feature.

## Calling the merged network

Use the public node JSON-RPC at the root HTTPS path, for example
`https://api1.mainnet-beta.paxeer.network` or
`https://api-mainnet-beta.paxeer.network`. Check `eth_chainId` before submitting
transactions. The numbered nodes and connection guidance are listed in
[Public RPC endpoints](../reference/public-rpc.md). Hosted gateway `px_` methods
have their own contract in `services/platform/hosted/gateway/openrpc.json`;
they are not automatically methods of every public node.

The precompile ABI is `precompiles/layerxprograms/abi.json`, with the Solidity
interface in `precompiles/layerxprograms/LayerXPrograms.sol`. Important methods
are:

| Operation | Method |
| --- | --- |
| Publish or replace code | `deploy`, `upgrade` |
| Execute or preview a guest call | `call`, `simulateCall` |
| Inspect code and interface | `getProgram`, `getInterface`, `getProgramOwner`, `getAccountProfile` |
| Register and inspect value accounts | `registerAccount`, `getProgramAccount`, `getProgramAccountById`, `getValueAccount` |
| Read fees and metering | `quoteCallFee`, `getCurrentFeeSchedule`, `getCurrentMeteringSchedule` |
| Wind down and exit value | `deprecate`, `tombstone`, `setExitRoute`, `exitValue`, `exitValueBounded` |
| Lease execution | `sandboxFund`, `sandboxActivate`, `sandboxExecute`, `getSandbox` |
| Retire storage | `retireStorage`, `getRetirementControl` |

Write handlers resolve the EVM caller's bound DID and invoke `Host.Execute`
with protocol 3. An unbound caller receives `LXP_ADDR_EVM_NOT_BOUND`. Methods
reject attached EVM value; kernel fee limits and spending grants are separate
from EVM transaction value and gas. Writes under static call and delegate calls
are refused. Views and `simulateCall` are available under static call.

`call` and `simulateCall` take `programId`, `abiVersion`, `entrypoint`,
`callData`, `capabilities`, `accessDeclaration`, `responseCapacity`, a seven-word
`uint64` budget, and `uint128 feeLimit`. Both return:

```text
(int32 terminalResult, uint8 terminalKind, bytes response)
```

`response` is the complete canonical terminal frame, including execution,
occupancy/authority attachments and applied transfer legs. It is not just guest
output. `responseCapacity` bounds the nested guest bytes. Use
`DecodeProgramExecutionResult` in `precompiles/layerxprograms/response.go` and
the format in `precompiles/layerxprograms/response.md` to separate the guest
response from its evidence. Authenticate the receipt and its transfer root
independently before trusting the legs; a root computed from untrusted response
bytes does not authenticate them.

### Simulate is not an activity type

`simulateCall` builds the same CALL payload and uses `Host.Simulate`
(`modules/lxphost/simulate.go`). Simulation runs against a private host and a
cached context, leaving committed state, balances, sequences and logs unchanged.
It produces a preview, not a committed transaction or a final payment receipt.
Use the same caller, capabilities, access declaration and budgets intended for
the write; state can change between the preview and the transaction.

Submit writes as EVM transactions to the precompile, retain the transaction
hash, and read `eth_getTransactionReceipt`. Check transaction status, committed
block evidence and the relevant Programs logs. `ProgramCallOutcome` binds its
program and activity to the transfer root and execution digests. A simulation
has no committed outcome log; its evidence must come from the trusted local
simulation receipt. The old standalone activity-submission and LNI simulation
transport are retired.

## Activity kinds

`include/layerx/programs.h` and `src/modules/programs/registration.c` define
the kernel activities behind the EVM methods:

| Type constant | Value | Controller |
| --- | --- | --- |
| `LX_PROGRAMS_DEPLOY` | `0x00090001` | `deploy.c` |
| `LX_PROGRAMS_UPGRADE` | `0x00090002` | `deploy.c` |
| `LX_PROGRAMS_CALL` | `0x00090003` | `call.c` |
| `LX_PROGRAMS_REGISTRY` | `0x00090004` | `registration.c` |
| `LX_PROGRAMS_TRANSFER` | `0x00090005` | `transfer.c` |
| `LX_PROGRAMS_ACCOUNT` | `0x00090006` | `accounts.c` |
| `LX_PROGRAMS_WIND_DOWN` | `0x00090007` | `winddown.c` |
| `LX_PROGRAMS_SANDBOX` | `0x00090009` | `sandbox.c` |
| `LX_PROGRAMS_SANDBOX_DESTROY` | `0x0009000A` | `sandbox_expiry.c` |
| `LX_PROGRAMS_STORAGE_RETIREMENT` | `0x0009000B` | Storage retirement controller |

Ordinal 8 is not an advertised activity. Fee schedules are read through the
current host/precompile surfaces; the former Programs fee-governance activity
is retired.

Module registration controls the activity inventory. v1 advertises ordinals
1–5; v2 adds ACCOUNT and WIND_DOWN; v3 adds SANDBOX; v4 adds SANDBOX_DESTROY.
`programs_module_registration_v4_storage_retirement` also advertises
STORAGE_RETIREMENT. `programs_module_registration_v5` uses that same inventory
and raises the guest ABI ceiling to 5.

### DEPLOY

The native payload is big endian:

```text
program_id32 || abi_version:u16 || policy:u8 || reserved_zero:u8
|| authority32 || code_hash32 || wasm_length:u32
|| [interface_length:u32 || interface_bytes] || wasm_bytes
```

Policy `0` is immutable and requires a zero authority; policy `1` requires a
nonzero authority. Without an interface the fixed header is 104 bytes. With an
interface it is 108 bytes, plus the interface itself (at most 952 bytes).
`EncodeDeployPayload` in `payload_lifecycle.go` matches the kernel decoder.

The kernel checks the WASM version-1 header, SHA-256 code hash, supported guest
ABI, metering admission and runtime validation. A duplicate program is refused
with `LXP_ERR_SEQUENCE_REUSED`; a hash mismatch is
`LXP_ERR_PAYLOAD_HASH_MISMATCH`. Successful execution stores the artifact,
program record and optional interface and emits the deployed code hash.

The deployment payload limit is **524,288 bytes**, including the native header
and any interface. A guest with no interface can therefore occupy at most
524,184 bytes. A WASM build alone does not establish deployment admission.

### UPGRADE

The fixed header is 106 bytes, or 110 with interface framing:

```text
program_id32 || abi_version:u16 || flags:u8 || reserved_zero:u8
|| old_hash32 || new_hash32 || hook_length:u16 || wasm_length:u32
|| [interface_length:u32] || hook_bytes || interface_bytes || wasm_bytes
```

Flag bit 0 declares a migration hook; bit 1 drops the interface. The hook
length is always present and agrees with bit 0. Upgrade requires an active
program with authority policy, an authorized caller and a matching old hash.
ABI transitions are monotonic across supported versions. The version counter
increments and the upgrade event carries both code hashes. Use the current
policy checks in `deploy.c`, including protocol-3 account authorization, rather
than treating a supplied authority field as proof of permission.

### CALL

The native CALL header is 106 bytes:

```text
program_id32 || abi_version:u16 || entrypoint_length:u16
|| calldata_length:u32 || capabilities_length:u16
|| access_declaration_length:u32 || response_capacity:u32
|| budget[7]:u64
|| entrypoint || calldata || capabilities || access_declaration
```

Entrypoints are 1–128 bytes of `[A-Za-z0-9_.]`. The decoder rejects trailing
bytes. Calldata, access declarations and response capacity each have a
1,048,576-byte field ceiling; capabilities have a u16 length ceiling. The outer
activity payload limit still applies, so these field limits cannot all be used
at once.

CALL validation loads the deployed record, requires the exact recorded ABI and
active lifecycle state, and opens the artifact by `(program_id, code_hash)`.
An ABI mismatch returns `LXP_ERR_VERSION_UNSUPPORTED`. Capabilities must admit
the operations, and spending remains bound to the invocation's authority.

## Guest ABI and protocol versions

These version numbers describe different boundaries:

| Number | Meaning | Current source behavior |
| --- | --- | --- |
| Guest ABI | Frozen WASM import surface | Versions 1–5; runtime and Rust SDK crate-root `ABI_VERSION` is 4 |
| Module interface ABI | Registered activities and CALL admission ceiling | Registrations 1–5; v4 with storage retirement and v5 are the node host choices |
| Wire protocol | Activity, occupancy and state-commitment encoding | 1 legacy, 2 occupancy, 3 state commitment; merged precompile writes use 3 |

The canonical guest policy is `programs/sdk/rust/src/abi_policy.rs`, reused by
the runtime. `admit_abi_version` admits 1–5; upgrades preserve or increase the
version. Guest ABI 1 uses the frozen `layerx_v1` imports: storage, events,
program calls, principal-funded transfers and receipt reads. Later versions
extend that surface:

| Guest ABI | Added import module and functions |
| --- | --- |
| 2 | `layerx_v2`: responses/refusals, scoped storage and scans, program-account transfers/funding, context, balance, hashing, signatures and 256-bit integer operations |
| 3 | `layerx_v3.oracle_read` |
| 4 | `layerx_v4.web_read` |
| 5 | `layerx_v5.market_step_adjudicate` |

Guest ABIs 2–5 use capability encoding v2 and require an occupancy-capable
protocol (2 or 3). ABI 1 also supports protocol 1. The C CALL decoder additionally
requires the guest version to be no greater than the registered module ABI.
A frozen ABI-5 vector does not make ABI-5 CALLs valid on a v4 host.

`modules/lxphost/host.go` accepts `Config.ProgramsGuestABI` values 0, 4 or 5;
0 selects v4. The node begins with v4 and the successful merge upgrade persists
`node/programs-guest-abi = 5`, then refreshes the host registration
(`node/app.go`, `node/upgrades.go`). Activation and persisted node state determine
the available ceiling. Check the deployed record and node activation before
selecting an ABI. Legacy C and AssemblyScript `ABI_VERSION` constants still
identify their v1 surface; their newer bindings must be selected explicitly.

### Frozen-ABI drift gates

`programs/abi-frozen.sha256` pins vectors for **all five versions** under
`programs/tests/vectors/abi-v1.hex` through `abi-v5.hex`.
`tests/programs/check-abi-drift.sh` invokes
`scripts/programs/generate-abi-vectors.py --check`, which compares the runtime
host tables, function types and composite manifests with the Rust SDK and
frozen vectors, including ABI 5. Its current crate-root version check is 4.
The source target `make programs-abi-drift` exposes that check; it is also a
dependency of `make programs-test`. These are workspace development gates,
not deployment receipts.

## Examples and builds

The Rust SDK contains `escrow`, `naming`, `nft-lxt721`, `payments-merchant`,
`swap-cpmm`, `token-lxt20`, `vault` and `web-reader` examples. C and AssemblyScript
have `paid-counter` examples. The interpreter, compute market, AI market and
arbiter live under `programs/crates/`. Examples and committed fixtures are
reference implementations; assess their configuration and admission against the
activated node before deploying them.

Run Cargo from inside `programs/` or an example directory so its
`.cargo/config.toml` supplies vendored dependencies, offline mode and WASM flags.
From the repository root, build the token reference with:

```sh
(cd programs/sdk/rust/examples/token-lxt20 && \
  cargo build --locked --release --target wasm32-unknown-unknown)
```

Build the merchant separately, using its own manifest:

```sh
(cd programs/sdk/rust/examples/payments-merchant && \
  cargo build --locked --release --target wasm32-unknown-unknown)
```

For the AI market, build release with `opt-level = "z"` and set the deployment
chain domain at build time. `PAXAI_CHAIN_DOMAIN` is a 64-character lowercase hex
encoding of `SHA-256("PAXAI/chain-domain/v1" || chain_id:u64_be)`, with chain ID
125. The guest validates this against its merged-chain identity; a malformed
or foreign setting returns `WRONG_DOMAIN`. An explicit build from the repository
root is:

```sh
(cd programs && \
  PAXAI_CHAIN_DOMAIN=62b6b14734fa769c3e0fbc94de495a3d038be49a2524b1c071d75d0a4eed71f6 \
  CARGO_PROFILE_RELEASE_OPT_LEVEL=z \
  cargo build --locked --release --target wasm32-unknown-unknown \
    -p layerx-programs-ai-market)
```

The workspace release profile otherwise uses optimization level 3. Package the
result under the same 524,288-byte deployment payload bound, including the
104-byte envelope and any interface framing. Host registration, declared guest
ABI and compiled imports must agree on every registration and call path.

## LXT-20

The canonical request codec is `programs/sdk/rust/src/lxt20.rs`; the guest is
`programs/sdk/rust/examples/token-lxt20`. Interfaces and native activity,
receipt and terminal fixtures exist under `programs/fixtures/pay5`, including
`native-token` and `native-merchant`. Generating an interface or registry state
value alone does not execute a deployment.

The token reference is ABI-v2 state backed by one native Asset. It fixes its
issuer, Asset, supply, per-call ceiling and metadata in source. Token balances
and allowances are program storage; backing units move through `402LXP` between
registered program-derived accounts. It has no mint, burn, permit or nested-call
surface.

### Interface and calldata

The reference interface binds eight exports:

| Method | Selector | Payload |
| --- | --- | --- |
| `initialize` | `4c 58 14 00` | empty |
| `transfer` | `4c 58 14 01` | recipient id32, amount u128 |
| `approve` | `4c 58 14 02` | spender id32, amount u128 |
| `transfer_from` | `4c 58 14 03` | owner id32, recipient id32, amount u128 |
| `balance_of` | `4c 58 14 04` | owner id32 |
| `allowance` | `4c 58 14 05` | owner id32, spender id32 |
| `total_supply` | `4c 58 14 06` | empty |
| `metadata` | `4c 58 14 07` | empty |

```text
request  = selector4 || [version=1, width=0x20] || payload_length:u32_be || payload
response = [version=1, width=0x20] || payload_length:u32_be || payload
```

Amounts are big-endian u128. Request payload lengths are 48 for
`transfer`/`approve`, 80 for `transfer_from`, 32 for `balance_of`, 64 for
`allowance`, and zero for the remaining methods. The largest request is 90
bytes. Balance, allowance and supply responses carry one u128; mutations return
an empty payload; metadata returns the configured bytes.

### Account and deployment preparation

A program account is derived as:

```text
SHA-256("LayerX/programs/program-account/v1\0"
        || program_id32 || seed_length:u32_be || seed)
```

For this token the seed is the owner's DID id32. Derivation is not native
registration, funding or debit authority. The deployment authority registers
accounts for the backing Asset through `registerAccount`; each recipient also
calls `approve` at least once (zero is valid) to initialize its token account
storage. Both preparations are needed before it receives backing units.

Deploy with guest ABI 2 and the generated interface. The issuer calls
`initialize` with calldata `4c581400012000000000`, authorizing the exact
principal-funded `Transfer402` grant for the fixed supply. Initialization stages
the funding transfer and initial token state in the same native transaction.

`transfer` and `transfer_from` declare `CallerAuthorizedSpend` descriptors in
the v2 interface encoding. The descriptor fixes the Asset, ceiling and
recipient/amount offsets: `10`/`42` for `transfer`, `42`/`74` for `transfer_from`.
It does not grant debit authority. The caller must supply a matching
`ProgramSpend` grant for the deriving program, seed, source account, Asset,
recipient and sufficient amount. Invalid recipients, zero amounts, excess
ceilings or unmatched grants are refused. A zero approval revokes permission;
`transfer_from` subtracts the exact amount even from a u128-maximum allowance.

### Calls and receipt reads

The merchant example uses explicitly granted, verified payment facts. Guest
`receipt_read` exposes this exact 116-byte view, not raw kernel state:

```text
receipt_digest32 || result_code:i32_be || asset_id32
|| amount:u128_be || state_root32
```

The Rust SDK refuses a malformed length or a digest different from the request.
Keep that guest view distinct from the full precompile terminal response and
from a chain proof of transaction inclusion.

For named principal/funding accounts, the runtime uses the
`LayerX/programs/402LXP/account-bound-set/v1\0` wrapper to bind original transfer
authorization to canonical account names. Program-spend legs carry empty names;
principal and funding names are re-derived to check their source accounts.
The committed 115-byte transfer legs bind actual account endpoints.
`programs/fixtures/pay5/account-authorization-vectors.json` contains the native
account-authorization vectors; it is an existing source asset.

## Program records and lifecycle states

The kernel-local key `"program\0" || program_id32` holds a 71-byte record:

| Offset | Bytes | Field |
| ---: | ---: | --- |
| 0 | 1 | Upgrade policy: immutable 0, authority 1 |
| 1 | 32 | Upgrade authority |
| 33 | 32 | Current code hash |
| 65 | 2 | Guest ABI version |
| 67 | 4 | Version counter, initially 1 |

Interface records use `"interface\0" || program_id32`. WASM blobs are addressed
by `(program_id, code_hash)`. The node host persists module values and blobs;
restored execution must resolve the same deployed artifact. These are
kernel-local keys; mounted chain proofs add the host's module-key prefix.

Wind-down states are `ACTIVE = 1`, `DEPRECATED = 2`, `TOMBSTONED = 3`. Absence
of a wind-down record means active. CALL and UPGRADE refuse inactive programs
with `LXP_ERR_PROGRAM_REFUSED`. Deprecation and value exit use the registered
account bindings and authorized routes; a registry balance counter is not
proof of a real value-account balance. Use `getWindDown`, `getExitRoute` and
`getValueAccount` against the committed state and authenticate remote evidence.

## Occupancy and committed execution

Occupancy meters namespace bytes held across protocol batch intervals. In the
node host the batch number is block height (`modules/lxphost/host.go`), not
wall-clock time. CALL admission carves an occupancy budget out of the signed
fee limit after pricing execution resources. Occupancy and execution evidence
are preserved in terminal results; bounded transfers remain authenticated
`402LXP` effects.

The rent model charges persistent bytes over later intervals. Continued
storage can therefore incur further charges or hit a responsibility ceiling.
This accounting does not require a separate checkpoint or settlement service:
the transaction result is final at committed block inclusion. Simulation does
not commit the storage or its charges.

## Receipts, terminals and events

Terminal kinds are success `1`, failure `2` and resource refusal `3`. Success
has `LXP_OK`; non-success carries a typed refusal or resource result. Preserve
the full terminal, call graph, event digests and transfer evidence when decoding.
The response contract is in `precompiles/layerxprograms/response.md`.

Guest ABI 1 transfers are principal-funded. ABIs 2–5 support registered
program-account spend and funding under the shared capability policy. Program
spend requires the deriving program's frame and exact seed; principal funding
conveys no authority to debit that account later. Source-authority attachments
allow authorized program debits while keeping `402LXP` as the balance writer.

The kernel publishes guest envelopes as `LX_PROGRAMS_EVENT_GUEST_ENVELOPE = 6`
and successful CALL outcomes as `LX_PROGRAMS_EVENT_CALL_OUTCOME = 7`.
`src/modules/programs/event.c` projects canonical committed events. The EVM
handlers publish `ProgramGuestEvent` and `ProgramCallOutcome` logs at `0x101D`;
deploy/upgrade publish their corresponding code-hash events. Bind logs to the
correct emitter, program, activity and committed transaction rather than trusting
an event-shaped byte string.

## Custody credit retirement and program funding

The kernel custody credit activity and its light client are retired. The former
bridge-profile genesis and deposit-backed issuance procedures are no longer
supported funding paths. See [Custody](../human/custody.md) for the retirement
notice and [Payments](../overview/payments.md) for ledger payments.

Program-account funding is a separate operation. A CALL needs a funded
fee-paying principal. ABIs 2–5 can fund registered program value accounts
through bounded funding legs; subsequent program debits still require the
program's own frame, seed and spend authority.

[Sandbox](sandbox.md) · [Home](../index.md)
