# Programs

Programs is kernel module ID `9` (`LXP_MODULE_PROGRAMS`). The module ID occupies
the high 16 bits of `activity_type`, so Programs activity types are `0x0009xxxx`
with the ordinal in the low 16 bits.
Guest WASM runs through this module. Monetary effects still leave as `402LXP`
transfer sets; the module does not call `set_balance`.

The C kernel lives under `src/modules/programs/`. The deterministic WASM
runtime is `programs/crates/layerx-programs-runtime/`. Receipt-bound registry
reads live in `programs/crates/layerx-programs-registry/`. Bounded leases live
in `programs/crates/layerx-programs-sandbox/`; see [Sandbox](Sandbox.md). Guest
SDKs live under `programs/sdk/`. `sdk/platform/` holds client SDKs (dotnet, go, jvm,
swift, conformance, generators). CALL receipt terminal verification for
those clients is on [SdkTerminalVerification](SdkTerminalVerification.md).

Programs executes inside `paxd` on the merged BFT network, chain ID **125**
(`0x7d`). `modules/lxphost/` hosts the kernel and Rust runtime in the node's
transaction context. Program storage, transfers, and receipt events become
final with their block's commit. See [Home](Home.md) for the network overview.

The Rust guest SDK includes LXT-20 codecs at `programs/sdk/rust/src/lxt20.rs`
and the examples `token-lxt20` and `payments-merchant` under
`programs/sdk/rust/examples/`. See [LXT-20](#lxt-20),
[Payments developer path](PaymentsQuickstart.md), and [Assets](Assets.md).

Sources: `include/layerx/lxp_module.h`, `include/layerx/programs.h`,
`src/modules/programs/registration.c`, `modules/lxphost/host.go`, and
`precompiles/layerxprograms/LayerXPrograms.sol`.

---

## Activity kinds

Dispatch is in `programs_decode` / `programs_validate` / `programs_execute`
(`src/modules/programs/registration.c`). Ordinals 1 and 2 are lifecycle
(`lxp_programs_lifecycle_*` in `deploy.c`). Ordinal 3 is CALL
(`lxp_programs_call_*` in `call.c`). Remaining ordinals have dedicated
controllers.

| Type constant | Value | Ordinal | Controller |
| --- | --- | ---: | --- |
| `LX_PROGRAMS_DEPLOY` | `0x00090001` | 1 | `deploy.c` lifecycle |
| `LX_PROGRAMS_UPGRADE` | `0x00090002` | 2 | `deploy.c` lifecycle |
| `LX_PROGRAMS_CALL` | `0x00090003` | 3 | `call.c` |
| `LX_PROGRAMS_REGISTRY` | `0x00090004` | 4 | generic KV record in `registration.c` |
| `LX_PROGRAMS_TRANSFER` | `0x00090005` | 5 | `transfer.c` |
| `LX_PROGRAMS_ACCOUNT` | `0x00090006` | 6 | `accounts.c` (module ABI ≥ 2) |
| `LX_PROGRAMS_WIND_DOWN` | `0x00090007` | 7 | `winddown.c` (module ABI ≥ 2) |
| `LX_PROGRAMS_SANDBOX` | `0x00090009` | 9 | `sandbox.c` (module ABI ≥ 3) |
| `LX_PROGRAMS_SANDBOX_DESTROY` | `0x0009000A` | 10 | `sandbox_expiry.c` (module ABI ≥ 4) |
| `LX_PROGRAMS_STORAGE_RETIREMENT` | `0x0009000B` | 11 | `blob_lifecycle.c` (retirement-enabled registration) |

Activity availability follows the registered module interface:

- v1: DEPLOY, UPGRADE, CALL, REGISTRY, TRANSFER.
- v2: adds ACCOUNT and WIND_DOWN.
- v3: adds SANDBOX.
- v4: adds SANDBOX_DESTROY. The
  `programs_module_registration_v4_storage_retirement` variant also advertises
  STORAGE_RETIREMENT.
- v5: `programs_module_registration_v5` advertises the same activity set as
  the retirement-enabled v4 interface and permits guest ABI 5.

Ordinal 8, the former fee-governance relay, is retired and is not advertised.
Fee and metering schedules are governed state, with native read/quote methods
listed below. Module registration and guest import versions are distinct;
see [Guest ABI versions and protocol 3](#guest-abi-versions-and-protocol-3).

The native host registers retirement-enabled v4 by default; selection of
`ProgramsGuestABI = 5` registers v5 (`modules/lxphost/host.go`). The node
reconstructs that selection from committed `node/programs-guest-abi` state:
absence means v4, and the two bytes `00 05` select v5 (`node/app.go`). A
runtime that supports ABI 5 is insufficient unless the host registration also
admits it. Kernel genesis registration has its own feature flags in
`src/protocol/lxp_genesis.c`.

### DEPLOY

`lxp_programs_lifecycle_decode` ordinal 1. Payload starts with 32-byte
`program_id`, then `abi_version` (u16), policy byte (`0` immutable / `1`
authority), reserved zero, 32-byte upgrade authority (all-zero iff immutable),
32-byte code hash, u32 wasm length, optional framed interface, then WASM.

`execute_deploy` refuses an existing program key (`LXP_ERR_SEQUENCE_REUSED`),
stores the artifact, writes a 71-byte program record, optionally stores the
interface, and emits event type 1 (`PROGRAM_EVENT_DEPLOYED` / `LX_PROGRAMS_EVENT_DEPLOYED`)
with the new code hash. Registrations after v1 bind the deployer as owner
through `lxp_programs_account_owner_bind`. The account-module predicate admits
module v2/v3, or module v4/v5 on protocol 3. The native host uses protocol 3.

WASM must start with `\0asm` version 1 and SHA-256 to the declared hash;
lifecycle validation also invokes the Rust artifact validator. A deployment
payload must fit `LXP_MAX_PAYLOAD_BYTES = 524288` including its **104-byte**
fixed header. With an interface, add its four-byte length prefix and interface
bytes as well. A WASM file that fits 524288 bytes by itself can still be too
large to deploy. The interface encoding is bounded at 952 bytes.

### UPGRADE

Ordinal 2. Payload: `program_id` (0), `abi_version` u16 (32), flags byte (34;
bit0 = hook present, bit1 = interface drop), reserved zero byte (35), old
hash (36), new hash (68), `hook_length` u16 (100, always present, non-zero
iff bit0), `wasm_length` u32 (102), optional `interface_length` u32 (106),
then hook bytes, interface bytes, WASM. The length field is fixed; the hook
bytes are optional. `execute_upgrade` requires the stored
policy to be authority (`PROGRAM_POLICY_AUTHORITY`), authorization through
`upgrade_policy_authorized`, and `old_hash` to match the record. On protocol 3,
the authority check accepts the principal or its canonical main/named account;
legacy protocols require the principal itself. ABI transitions
must be monotonic (`lxp_programs_abi_transition_validate`). Version counter at
record offset 67 increments. Event type 2 carries old hash then new hash.

A CALL against a deprecated or tombstoned program is refused (see lifecycle).

### CALL

`lxp_programs_call_decode` requires at least `PROGRAM_CALL_FIXED_BYTES`
(32 + 2 + 2 + 4 + 2 + 4 + 4 + 7×8). Layout: `program_id`, guest `abi_version`,
entrypoint length, calldata length, capabilities length, access-declaration
length, response capacity, then seven u64 budget fields, then the four
variable spans. Trailing bytes are an error. Entrypoint bytes are
`[A-Za-z0-9_.]+`.

Validate loads the 71-byte program record, requires guest ABI to match the
record, requires `lxp_programs_program_active`, and opens the artifact by
`(program_id, code_hash)`. Execute binds occupancy (when the protocol uses
occupancy) and hands the arena-owned activity to the Rust CALL FFI
(`layerx_programs_call_begin` via `call_scalar_begin`).

Guest ABI 0 is noncanonical. A guest ABI above the registered module ABI
or above 5 is `LXP_ERR_VERSION_UNSUPPORTED`; ABIs 2–5 also require occupancy
protocol 2 or 3. The guest ABI must equal the deployed record's version.

### Native submission, reads, and simulation

The EVM Programs surface is `layerxprograms` at
`0x000000000000000000000000000000000000101D`. Its complete ABI is
`precompiles/layerxprograms/LayerXPrograms.sol` and `abi.json`. The node imports
the constructor in `node/precompiles.go`; the merge activation policy is in
`node/upgrades.go` and `precompiles/setup.go`. Source registration alone does
not prove activation on a particular endpoint.

| Operation | Native methods |
| --- | --- |
| Deploy or upgrade | `deploy`, `upgrade` |
| Execute a guest | `call` |
| Simulate a guest | `simulateCall` (view) |
| Inspect deployment and interface | `getProgram`, `getInterface`, `getProgramOwner`, `getStateHead` |
| Register and inspect accounts | `registerAccount`, `getAccountProfile`, `getProgramAccount`, `getProgramAccountById`, `getValueAccount` |
| Inspect costs | `quoteCallFee`, `getCurrentFeeSchedule`, `getFeeSchedule`, `getCurrentMeteringSchedule`, `getMeteringSchedule` |
| Wind down | `setExitRoute`, `deprecate`, `exitValue`, `exitValueBounded`, `tombstone` |
| Manage bounded leases and storage | `sandboxFund`, `sandboxActivate`, `sandboxExecute`, `getSandbox`, `retireStorage` |

ABI-encode writes to this address, sign a chain-125 EVM transaction, and
submit with `eth_sendRawTransaction`. Read views with `eth_call` at an explicit
committed block. Retain the transaction hash, receipt, block identity, and
relevant Programs logs. A successful submission is not proof of execution;
check transaction status and the CALL terminal result, then bind the evidence
to the committed block.

The caller must have an EVM-to-DID binding. An unbound caller is refused with
`LXP_ADDR_EVM_NOT_BOUND`. Delegatecall is refused, methods are nonpayable,
and writes under staticcall are refused with `LXP_PROGRAM_REFUSED`.
`submit` constructs a protocol-3 activity from the verified caller and invokes
`Host.Execute`; a program cannot select a different principal by changing
calldata. Sources: `principal.go`, `submit.go`, and `errors.go` in
`precompiles/layerxprograms/`.

Simulation is not an activity ordinal. Use `eth_call` to invoke
`simulateCall` with the same program ID, ABI, entrypoint, calldata,
capabilities, access declaration, response capacity, seven-field budget, and
fee limit as `call`. `method_simulate.go` invokes `Host.Simulate`, which uses
a cached chain context and a private host. It returns a hypothetical terminal
without publishing storage, balances, sequence, or committed logs. Simulation
is evidence about the selected state; it does not reserve a later execution
or establish finality.

Public JSON-RPC uses the root of `https://api-mainnet-beta.paxeer.network`
and `https://api1.mainnet-beta.paxeer.network` through
`https://api14.mainnet-beta.paxeer.network`, chain ID `125` (`0x7d`). See
[Public JSON-RPC](PublicRpc.md).

The node serves the `px` namespace at its root and answers from node state.
`px_getBalances` and `px_getCapabilities` are node reads and do not require a
hosted gateway.
Only `px_register` and `px_requestFunds` are hosted operations.
After parameter validation, node calls to either operation return JSON-RPC
`-32004` with `error.data.code: "hosted_gateway_service_required"`.

The gateway's method schemas are defined in
`services/platform/hosted/gateway/openrpc.json`. Both the node's and gateway's
`px_getCapabilities` probe `getCurrentFeeSchedule()` at `0x101D` for Programs
availability. When Programs is unavailable the gateway refuses direct Programs
transaction submissions with JSON-RPC `-32003` and data code
`surface_unavailable`. Obtain gateway availability from its operator.

Sources: `precompiles/layerxprograms/method_call.go`, `method_simulate.go`,
`submit.go`, `modules/lxphost/host.go`, and `modules/lxphost/simulate.go`.

---

## LXT-20

The Rust guest SDK LXT-20 codecs live at `programs/sdk/rust/src/lxt20.rs`
and `programs/sdk/rust/examples/token-lxt20`. The contract below is that
reference. Deployment, account-registration, CALL receipt, terminal, and
call-graph fixtures are in `programs/fixtures/pay5/native-token/`; the merchant
counterparts are in `programs/fixtures/pay5/native-merchant/`.

The Rust guest SDK's LXT-20 example is an ABI-v2 state machine backed by one
native Asset. Token balances and allowances are program storage; the backing
units move only through `402LXP`. The reference program fixes its Asset,
supply, and ceiling at build time. It has no mint, burn, permit, or nested-call
surface.

### Interface and calldata

The native Programs interface publishes eight entries:

| Method | Selector | Payload |
| --- | --- | --- |
| `initialize` | `4c 58 14 00` | empty |
| `transfer` | `4c 58 14 01` | recipient id32, amount `u128` |
| `approve` | `4c 58 14 02` | spender id32, amount `u128` |
| `transfer_from` | `4c 58 14 03` | owner id32, recipient id32, amount `u128` |
| `balance_of` | `4c 58 14 04` | owner id32 |
| `allowance` | `4c 58 14 05` | owner id32, spender id32 |
| `total_supply` | `4c 58 14 06` | empty |
| `metadata` | `4c 58 14 07` | empty |

Canonical requests are:

```text
selector4 || [version=1, width=0x20] || payload_length:u32_be || payload
```

The exact payload lengths are 48 for `transfer`/`approve`, 80 for
`transfer_from`, 32 for `balance_of`, 64 for `allowance`, and zero for
`initialize`, `total_supply`, and `metadata`. Transfer amounts are positive;
approval of zero is valid and revokes the allowance. The total request is at
most 90 bytes.

Responses are `[1,0x20] || payload_length:u32_be || payload`. Balance,
allowance, and total supply return one `u128`; mutation calls return an empty
payload; metadata is the fixed configured value. `transfer_from` always
subtracts the exact amount, including when the allowance is the maximum
`u128`.

### Account and deployment preparation

`PreparedProgramAccount` derives an account from:

```text
SHA-256("LayerX/programs/program-account/v1\0"
       || program_id32 || seed_length:u32_be || seed)
```

Its native account-registration payload is:

```text
program_id32 || "LXPA1" || asset_id32 || seed_length:u32_be || seed
```

Register the program account for the chosen Asset before funding it. Funding
uses an ordinary principal-funded `Transfer402` grant; spending uses the
separate bounded `ProgramSpend` capability. Derivation is not registration,
funding, or debit authority.

Deploy with ABI version 2 and include the generated interface that binds all
eight selectors, input/output bounds, and transfer capability offsets. Because
`transfer` and `transfer_from` carry `CallerAuthorizedSpend`, the canonical
interface uses `LayerX/program-interface/v2\0`; the v1 interface encoding and
guest ABI 1 refuse that descriptor
(`programs/crates/layerx-programs-registry/src/interface.rs`).

The reference descriptor binds the backing Asset and per-call ceiling, with
recipient/amount offsets `10`/`42` for `transfer` and `42`/`74` for
`transfer_from`
(`programs/crates/layerx-programs-registry/src/lxt20.rs`). It
does not grant spending authority. Admission extracts the recipient and u128
amount from canonical calldata, refuses zero or over-ceiling values, and
requires a matching caller `ProgramSpend` grant bound to this program, its
derived source account, Asset, recipient, and amount
(`programs/crates/layerx-programs-runtime/src/dynamic_spend.rs`). A
registry source/state record alone is not proof that the native deploy
activity executed; require its verified receipt.

The initialization request is:

```text
4c581400012000000000
```

Initialization atomically stages the full fixed supply from the configured
issuer to the registered program account. A recipient must call `approve`,
including zero, at least once so its derived program-account storage exists
before a transfer credits it. The deployment authority must also register each
recipient account with the native account module; guest storage registration
alone does not create the native account.

### Calls and receipt reads

Build the token reference guest from its directory so Cargo discovers the
vendored-source and WASM settings in `programs/.cargo/config.toml`:

```sh
cd programs/sdk/rust/examples/token-lxt20
cargo build --locked --target wasm32-unknown-unknown --release
```

For the merchant example, run the same Cargo command from
`programs/sdk/rust/examples/payments-merchant/`. Both example manifests use
release `opt-level = "z"`. Include the matching generated interface in
`deploy` and confirm the committed deployment and interface with `getProgram`
and `getInterface`. Use the native methods above for submission and reads.
A call's transfer grants must match its interface and registered accounts;
capability widening or unmatched dynamic spend is refused atomically.

Guest `receipt_read` does not expose raw kernel state. The host writes exactly
116 bytes for an explicitly granted digest:

```text
receipt_digest32
|| result_code:i32_be
|| asset_id32
|| amount:u128_be
|| state_root32
```

The SDK refuses any other length, malformed field, or digest different from
the requested receipt. The merchant example therefore consumes verified,
explicitly granted payment evidence rather than trusting a caller-supplied
receipt description.

When a principal or program-funding leg resolves a named native account, the
runtime wraps the original transfer authorization with
`LayerX/programs/402LXP/account-bound-set/v1\0`, the u32 length and bytes of
that authorization, then one u16-length-prefixed account name per leg. Program
spend legs require an empty name; principal and funding legs recompute the
source account from the supplied canonical name. The resulting 115-byte kernel
legs and transfer root therefore commit the actual account endpoints while the
original authorization retains the signer principal and invocation authority
(`programs/crates/layerx-programs-runtime/src/transfer.rs`).
Nested wrappers, trailing data, forged names, or a recomputed root mismatch are
refused. The positive native per-Asset evidence and negative vectors for that
refusal live in `programs/fixtures/pay5/account-authorization-vectors.json`,
alongside native-token and native-merchant fixtures.

---

## Guest ABI versions and protocol 3

These version numbers describe different compatibility boundaries.

| Number | What it versions | Values in tree |
| --- | --- | --- |
| Guest ABI | WASM import surface a program is compiled against | Frozen 1–5 (`programs/abi-frozen.sha256`) |
| Module iface ABI | Which Programs activity types the kernel registers | Registrations v1–v5 (`registration.c`) |
| Protocol version | Envelope / occupancy / state-commitment | 1 legacy, 2 occupancy, 3 state commitment |

### Guest ABI

The canonical policy is `programs/sdk/rust/src/abi_policy.rs`, re-exported by
the runtime. It admits guest ABIs **1–5** and only monotonic upgrades among
those versions. The runtime crate-root `ABI_VERSION` is **4** and its
`ABI_MANIFEST` is the v1–v4 composite. The version-addressed `manifest(5)`
adds the v5 namespace; the crate-root constant is not the admission ceiling.

Imports are cumulative: each admitted version retains the older namespaces
unchanged and adds its own namespace. Exact import names and WASM types are
validated against `programs/crates/layerx-programs-runtime/src/abi/manifest.rs`.

**`layerx_v1` (ABI 1 and later):**

| Host function | Signature |
| --- | --- |
| `storage_read` | `(i32,i32,i32,i32)->i32` |
| `storage_write` | `(i32,i32,i32,i32)->i32` |
| `storage_delete` | `(i32,i32)->i32` |
| `event_emit` | `(i32,i32,i32,i32)->i32` |
| `program_call` | `(i32,i32,i32,i32,i32,i32)->i32` |
| `transfer_402` | `(i64,i64,i32,i32,i32,i32)->i32` |
| `receipt_read` | `(i32,i32,i32,i32)->i32` |

**`layerx_v2` (ABI 2 and later):**

| Host function | Signature |
| --- | --- |
| `response_write` | `(i32,i32,i32)->i32` |
| `program_call_response` | `(i32,i32,i32,i32,i32,i32,i32,i32)->i64` |
| `refusal_write` | `(i32,i32,i32)->i32` |
| `storage_read_scoped` | `(i32,i32,i32,i32,i32)->i32` |
| `storage_write_scoped` | `(i32,i32,i32,i32,i32)->i32` |
| `storage_delete_scoped` | `(i32,i32,i32)->i32` |
| `storage_drop_scoped` | `(i32)->i32` |
| `storage_scan_scoped` | `(i32,i32,i32,i32,i32,i32,i32,i32,i32)->i32` |
| `transfer_program_402` | `(i64,i64,i32,i32,i32,i32,i32,i32,i32,i32)->i32` |
| `fund_program_402` | `(i64,i64,i32,i32,i32,i32,i32,i32)->i32` |
| `context_read` | `(i32,i32,i32)->i32` |
| `balance_read` | `(i32,i32,i32,i32,i32,i32)->i32` |
| `hash` | `(i32,i32,i32,i32)->i32` |
| `signature_verify` | `(i32,i32,i32,i32,i32,i32,i32)->i32` |
| `signature_recover` | `(i32,i32,i32,i32,i32,i32,i32)->i32` |
| `bigint_mul_256` | `(i32,i32,i32,i32,i32,i32)->i32` |
| `bigint_div_256` | `(i32,i32,i32,i32,i32,i32)->i32` |
| `bigint_rem_256` | `(i32,i32,i32,i32,i32,i32)->i32` |
| `bigint_modexp_256` | `(i32,i32,i32,i32,i32,i32,i32,i32)->i32` |

**Later namespaces (in addition to all earlier imports):**

| Guest ABI | Import module and function | Signature |
| --- | --- | --- |
| 3 | `layerx_v3.oracle_read` | `(i32,i32,i32,i32)->i32` |
| 4 | `layerx_v4.web_read` | `(i32,i32,i32,i32)->i32` |
| 5 | `layerx_v5.market_step_adjudicate` | `(i32,i32,i32,i32,i32,i32)->i32` |

Guest capability tags 1–6 are StorageRead, StorageWrite, EmitEvent, Call,
Transfer402, and ReceiptRead. ABI 2 adds SharedStorageRead/Write (7–8),
ProgramSpend (9), and BalanceView (10); all later ABIs retain that capability
encoding. See `abi/capability.rs` and [Storage scan](StorageScan.md) for the
bounded scan surface. Host imports do not grant authority by themselves.

### Host and guest agreement

`admit_abi_version`, `admit_abi_upgrade`, and `capability_encoding` are shared
by the SDK, runtime, and registry. The C lifecycle validator delegates to the
Rust `layerx_programs_abi_transition_admit` FFI. CALL also enforces the
registered module's ceiling and an exact match to the deployed guest ABI.
Thus v4 host registration admits guests through v4; ABI 5 needs v5 host
registration on every deployment, execution, and recovery path.

ABI 1 CALL retains protocol-1/2/3 compatibility. ABIs 2–5 use capability
encoding v2 and require occupancy protocol 2 or 3. Program-account operations
have an additional predicate: ABI 2 has the original account support; ABI
3–5 require protocol 3 and a stored account profile. Use `registerAccount`
with profile 2 to opt in and inspect `getAccountProfile` rather than assuming
that a later guest version automatically grants program spending.
Sources: `src/modules/programs/accounts.c`, `call.c`, `deploy.c`,
`programs/sdk/rust/src/abi_policy.rs`, and runtime `ffi_call.rs`.

The C SDK's legacy `LXP_PROGRAM_ABI_VERSION` remains 1; the same header also
publishes explicit v1–v4 constants. Choose the declared guest version and
matching import surface explicitly; do not infer it from a module-interface
version or the runtime crate-root constant.

### Protocol 3

`LXP_PROTOCOL_VERSION_STATE_COMMITMENT = 3`; protocol 2 is occupancy and
protocol 1 is legacy. The C header default `LXP_PROTOCOL_VERSION` remains 2,
but the native Programs submission and simulation paths explicitly construct
protocol-3 activities. Old standalone hosted deployment pins do not select
the native host's protocol.

On protocol 3, `layerx_programs_call_terminal_publish` writes program-outcome
encoding version **4**, including the applied-legs digest. Other retained
protocols use outcome encoding 3. Guest ABI, protocol version, program-outcome
encoding, and terminal execution encoding are separate version spaces.

### AI market guest packaging

The AI market is `programs/crates/layerx-programs-ai-market/`. Build its WASM
in release mode with size optimization and an explicit chain domain:

```sh
cd programs
PAXAI_CHAIN_DOMAIN=62b6b14734fa769c3e0fbc94de495a3d038be49a2524b1c071d75d0a4eed71f6 \
CARGO_PROFILE_RELEASE_OPT_LEVEL=z \
cargo build --locked --release --target wasm32-unknown-unknown \
  -p layerx-programs-ai-market
```

That 32-byte lowercase hex domain is `MERGED_CHAIN_DOMAIN` for chain 125 in
`src/lib.rs` of the crate; malformed or foreign overrides are refused. Keep
the configured build domain consistent with the deployed network. Package
the resulting artifact within the **524288-byte** deployment payload ceiling,
including the **104-byte** fixed header and any framed interface; without an
interface the WASM ceiling is 524184 bytes. `programs/Cargo.toml` defaults to
release optimization level 3, so the size override above is intentional.
The guest's recorded ABI must also be admitted by the actual host registration.

---

## Program records and lifecycle states

### Deployed program record

Key prefix `"program\0"` plus 32-byte program id (40 bytes). Value is 71 bytes:

| Offset | Bytes | Field |
| ---: | ---: | --- |
| 0 | 1 | policy (`0` immutable, `1` authority) |
| 1 | 32 | upgrade authority |
| 33 | 32 | current code hash |
| 65 | 2 | guest ABI version |
| 67 | 4 | version counter (DEPLOY writes `1`) |

Interface records, when present, use prefix `"interface\0"` plus program id.

WASM bytes are blobs addressed by `(program_id, code_hash)`
(`lxp_programs_artifact_store` / `lxp_programs_artifact_open`). The native host
loads and publishes blobs through its chain store (`modules/lxphost/host.go`);
reopening the host must resolve the same code hash and artifact. A generated
registry record alone does not establish a committed deployment.

### Wind-down states

`lx_programs_lifecycle_status`: `ACTIVE = 1`, `DEPRECATED = 2`,
`TOMBSTONED = 3`. Absence of a wind-down status record means active.
`lxp_programs_program_active` returns `LXP_OK` on `LXP_ERR_UNKNOWN_FIELD` and
`LXP_ERR_PROGRAM_REFUSED` when a deprecation or tombstone record exists.
CALL validate and UPGRADE execute both call it.

WIND_DOWN is a separate activity (ordinal 7). The registry crate models the
same states as `Deprecation`, `AuthorizedExit`, `ExitRoute`, `WindDownView`
over receipt-verified account proofs. A deprecation cannot complete while a
bound value account still carries a non-zero balance without an authorized
exit.

Sources:

- `src/modules/programs/deploy.c`
- `include/layerx/programs.h`
- `src/modules/programs/winddown.c`
- `src/modules/programs/call.c`
- `programs/crates/layerx-programs-registry/src/lib.rs` (exports)
- `programs/crates/layerx-programs-runtime/src/lifecycle.rs` (`UpgradePolicy`, `Deploy`, `Upgrade`)

---

## Occupancy and committed state

Occupancy meters namespace bytes held across protocol batches. Its clock is
a deterministic batch index; the native host uses the block height as its
batch number and the block timestamp as kernel time. The accounting keys are
`progocc/head/v3`, `progocc/final/v3`, and ledger chunks `progocc/l/v3/`;
occupancy receipts use `LXP/programs/occupancy-receipt/v2`.

`lxp_programs_occupancy_bind_call` reserves occupancy budget from the signed
fee limit after pricing the execution budget. A successful occupancy-bearing
CALL requires its occupancy effects to be applied; charges emit authenticated
maintenance transfer sets. The CALL outcome carries byte-batches, fee units,
Asset ID, evidence digest, and the occupancy transfer root.

The kernel's ordered maintenance path uses
`lxp_kernel_prepare_batch_maintenance` and
`lxp_programs_finalize_occupancy_batch_with_maintenance` (or the selected
occupancy-only variant). Finalization checks contiguous batch numbers,
journals changes, processes sandbox expiry, and produces canonical occupancy
evidence. This is deterministic accounting, not another network finality
stage. The old replica replay finalizer is retired.

For native execution, `HostExecute` loads module state and blobs from the
chain store and publishes the resulting state through the transaction's
context. The transaction and its receipt are final at block commit. Later
occupancy charges for storage that remains allocated are later state
transitions; they do not delay finality of the original CALL. Simulation
returns hypothetical accounting evidence and publishes no durable state.

Sources: `src/modules/programs/occupancy.c`, `src/protocol/lxp_kernel.c`,
`src/protocol/lxp_maintenance.c`, `modules/lxphost/host.go`, and
`modules/lxphost/simulate.go`.

---

## Receipts and terminals

A CALL terminal is begun with kind `SUCCESS` (1), `FAILURE` (2), or `RESOURCE`
(3) (`lxp_program_terminal_kind`). Success requires `result_code == LXP_OK`;
non-success forbids `LXP_OK` and forbids fatals. Three reserved buffers -
call graph, terminal payload, events - are hashed on publish.

`layerx_programs_call_terminal_publish` binds `lxp_program_outcome` onto the
activity context. Only success emits `CALL_OUTCOME` and copies the transfer
root. Failure/resource bind the outcome without that event.

### Decode the terminal before consuming the guest response

`call` and `simulateCall` return
`(int32 terminalResult, uint8 terminalKind, bytes response)`. The `response`
contains the complete canonical `LXP/programs/terminal-applied-legs/v1\0`
frame: execution evidence, authority/occupancy attachments, and applied
transfer legs. It is not raw guest return data. `responseCapacity` bounds
only the nested guest response, up to 1048576 bytes; preserve the full evidence
frame for verification.

`DecodeProgramExecutionResult` in `precompiles/layerxprograms/response.go`
validates framing and binds the applied legs to an independently verified
transfer root. For an executed CALL, authenticate the `ProgramCallOutcome`
log's address, program, activity, receipt, and committed block before using
its root. A root computed only from the untrusted frame cannot authenticate
that frame. Simulation has no committed log; its root must come from the
trusted simulation context. See the exact formats and separate frame bound
in `precompiles/layerxprograms/response.md` and
[SDK terminal verification](SdkTerminalVerification.md).

### Guest ABI 1: principal-only transfer sources

`layerx_programs_call_transfer_leg` source kinds:

- `PROGRAM_TRANSFER_SOURCE_PRINCIPAL = 1`
- `PROGRAM_TRANSFER_SOURCE_PROGRAM = 2`
- `PROGRAM_TRANSFER_SOURCE_PROGRAM_FUNDING = 3`

Kind 1 requires the principal's resolved payment account, zero `owner_program`,
and no seed. Kinds 2 and 3 require an account-capable guest (ABI 2–5) and the
account-profile checks described above. ABI 1 cannot debit a program-owned
account or fund one through this CALL transfer path.

### Guest ABI 2 and later: authority attachment

`layerx_programs_call_transfer_apply` validates every leg, then attaches a
`lxp_transfer_source_authority` per distinct `from` (`authorized_from`,
`debit_authority_kind = LXP_AUTH_OWNER`) onto `set->context.source_authorities`
before `lxp_ctx_emit_transfer_set`.

Kind 2 (program spend): `from` must be the derived module-value account for
`(owner_program, seed)`; that account has no authority key; owner must equal
staging program. Kind 3 (program funding): `from` is the principal's resolved
payment account; `to` is the derived program value account.

The 402 context `authorized_from` remains the activity principal. Extra
source authorities allow account-capable guests to debit program-owned
accounts without giving the guest `set_balance`.

Sources:

- `include/layerx/lxp_receipt.h`
- `src/modules/programs/call.c`
- `programs/crates/layerx-programs-runtime/src/ffi_call.rs` (CALL FFI)

---

## Events (`CALL_OUTCOME`)

Programs publication events used by CALL:

| Constant | Value | Emitter |
| --- | ---: | --- |
| `LX_PROGRAMS_EVENT_GUEST_ENVELOPE` | 6 | `lxp_programs_emit_guest_event` |
| `LX_PROGRAMS_EVENT_CALL_OUTCOME` | 7 | `lxp_programs_emit_call_outcome` |

Guest envelopes: domain `LXGE`, envelope version 1, kind 1, 185-byte body.
Topic/data are stored as domain-separated SHA-256
(`LayerX/programs/event-topic/v1`, `LayerX/programs/event-data/v1`). Max 64
events per CALL (`LXP_PROGRAMS_EVENT_MAX_COUNT`). Principal in the envelope
must equal the admission payer.

`CALL_OUTCOME` uses domain `LXMO`, outcome envelope version 2, kind 2,
255-byte body (metering schedule version included). Legacy 251-byte `LXCO`
bodies are still accepted by `envelope_effect_valid`. Emit requires
`terminal_result == LXP_OK`. Body binds program id, principal, activity id,
frame, runtime/ABI/fee/metering versions, transfer-set root, call-graph
digest, terminal-payload digest, event-envelope digest.

`lxp_programs_project_committed_events` (`event.c`) filters those two
Programs event types and orders them by increasing ordinal.
`lxp_programs_project_receipt_events` (`event.c`) treats guest-envelope
or `CALL_OUTCOME` events without `program_outcome.present` as non-canonical.

DEPLOY/UPGRADE still emit event types 1 and 2 from `deploy.c` (code hashes),
which are not in this publication seam.

Sources:

- `include/layerx/programs.h`
- `src/modules/programs/event.h`
- `src/modules/programs/event.c` (`lxp_programs_project_committed_events`)
- `src/modules/programs/event.c` (`lxp_programs_project_receipt_events`)
- `src/modules/programs/call.c`

---

## Custody credit retirement and program funding

The kernel custody credit activity and its light client are retired.
The former bridge-profile genesis and deposit-backed issuance procedures
are no longer supported funding paths. See
[Custody](Custody.md) for the retirement notice and the
[Human custody](HumanCustody.md) and [Payments developer path](PaymentsQuickstart.md)
for the current ledger and payment paths.

Program-account funding is a separate operation. A CALL needs a funded
fee-paying principal. An account-capable guest (ABI 2–5) can stage
principal-authorized `PROGRAM_FUNDING` transfer legs to fund `module:programs:value:<account-id>` accounts. It cannot
authorize debits from a program account: ABI-1 has no program-source kind,
and account-capable program debits require the deriving program's frame,
seed, registered Asset, and bounded grant.

Sources:

- `src/modules/programs/call.c` (`layerx_programs_call_transfer_leg`)
- `src/modules/programs/call.c` (`transfer_source_validate`, including `PROGRAM_TRANSFER_SOURCE_PROGRAM_FUNDING`)
- `src/modules/programs/accounts.c` (`lxp_programs_account_derive`)
- `src/modules/programs/accounts.c` (`lxp_programs_account_lookup`)

---

## Frozen-ABI drift gates

`programs/abi-frozen.sha256` pins five immutable vectors:

```text
1 09fcad46aeea9659d7d555a4a09ec151bd36d60b85bfd052d1f7a43cf71d58a3
2 8827869bf1324c3e82c60baf7360b1c3e2baa84d8591e49eb6b73c2cea6c8b69
3 0d1c2505bb16c52cbac38f966359fafd4e930949f968484250ec96520a7de0b2
4 733f27cbf903eaceab532424d13e321f6dd8939afb1f85511c7a5a12c0289aef
5 07d573becee2e920b7bacb3f7d712ff017070681d46337f62e9b6987c58a2e87
```

`make programs-abi-drift` runs `tests/programs/check-abi-drift.sh`, which
invokes `scripts/programs/generate-abi-vectors.py --check`. The generator
checks all five manifests and vectors (`programs/tests/vectors/abi-v1.hex`
through `abi-v5.hex`), their import tables and function types, canonical SDK
version policy, and frozen checksums. It requires the crate-root manifest
to identify ABI 4 while retaining the separately frozen v5 extension.
`make programs-test` depends on the drift gate. Do not edit frozen older
vectors to make a changed host surface appear compatible.

---

## Start here

- [Home](Home.md)
- [Modules](Modules.md)
- [Asset](Asset.md)
- [Bridge](Bridge.md)
- [Fees](Fees.md)
- [Public JSON-RPC](PublicRpc.md)
- [Sandbox](Sandbox.md)
- [Storage scan](StorageScan.md)
- [Roadmap](Roadmap.md)
