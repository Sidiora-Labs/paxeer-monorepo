# LayerX Programs

**Deterministic guest execution inside Paxeer X Network, with monetary effects applied through `402LXP`.**

This workspace holds the programs surface of the LayerX kernel: a deterministic WASM
runtime, the registry that proves what a program deployed and what it holds, the C↔Rust
bridge into the kernel, reference programs, developer SDKs, porting kits, and the
adversarial test corpus.

Programs are a **first-class execution surface** of the merged network. The kernel
modules live in [`src/modules`](../src/modules) and are numbered in
[`include/layerx/lxp_module.h`](../include/layerx/lxp_module.h). Guest execution is
dispatched under the programs module (`LXP_MODULE_PROGRAMS = 9`), but a program is not a
module of its own and writes no balances. Every monetary effect a program produces
compiles to an authenticated `402LXP` transfer set applied by the kernel - the same
single money doorway every module uses.

Where the rest of the system fits:

- **Identity and authority are kernel**, not a program concern. A program executes inside
  the authority of the activity that invoked it and can never widen it.
- **`402LXP` is the sole balance writer.** Programs emit transfer sets; they do not call
  `set_balance` (no such function exists).
- **Oracle reads** use committed perps observations through the ABI-3 `oracle_read`
  host function ([`host/oracle.rs`](crates/layerx-programs-runtime/src/host/oracle.rs)).
  A guest state transition never dials out to fetch a price.
- **Execution and finality** belong to one BFT network: `paxd`, EVM chain ID `125`
  (`0x7d`). The kernel runs inside `paxd` and participates in block proposal and
  execution. A receipt is final when its block is committed; a signed execution receipt
  before commitment is execution evidence, not finality. Programs do not require
  periodic checkpoint settlement or a separate settlement deployment.
- **Network access** uses public node JSON-RPC at the root URL; see the
  [public endpoint reference](../docs/site/docs/reference/public-rpc.md).
  Hosted gateway `px_` methods have their own
  [OpenRPC contract](../services/platform/hosted/gateway/openrpc.json).
- **Tenant domains** are database namespaces within this network. Launching a
  sub-chain does not require new chain infrastructure.

Normative behavior lives in
[`spec/layerx-merge/spec.kvx`](../spec/layerx-merge/spec.kvx). This document is the
human read of the programs workspace and is not a substitute for the spec.

---

## Workspace layout

The programs workspace is a Cargo workspace (`programs/Cargo.toml`) with a strict lint
profile: `unsafe_code` is denied by default, Clippy `all` and `pedantic` are denied, and
`unwrap_used`, `expect_used`, and float arithmetic are denied across the tree.

| Path | Purpose |
| --- | --- |
| `crates/layerx-programs-runtime` | Deterministic WASM runtime: validation, metering, the ABI/capability boundary, cross-program calls, transfers, occupancy accounting, replay, and the FFI bridge into the C kernel |
| `crates/layerx-programs-registry` | Receipt-bound registry: deployment journal, program interfaces, program value-account bindings, real-balance proofs, wind-down/deprecation, and the reference-program interface generators. Chain-proof read bridge for the mounted `lxphost` store |
| `../precompiles/layerxprograms` | Live Programs deploy, call and query interface at `0x101D`; registry chain-proof reads verify evidence against committed chain state |
| `crates/layerx-programs-interpreter` | A bounded deterministic scripting program: scripts are submitted as canonical bytes and fully validated before any effect is staged |
| `crates/layerx-programs-market` | Compute-lease market program: usage claims with a challenge window, provider stake and slashing, attester policy, and on-chain bisection disputes |
| `crates/layerx-programs-ai-market` | PAXAI guest: market registry, workers and evaluators, commit/reveal epochs, aggregation, reputation, and rewards |
| `crates/layerx-programs-arbiter` | Verified replay of sandbox traces and the market-step verdict that judges the single step a dispute bisection collapses to |
| `crates/layerx-programs-sandbox` | Protocol-state models for bounded, ephemeral program sandboxes |
| `sdk/rust`, `sdk/c`, `sdk/assemblyscript` | Guest program SDKs; Rust ships `escrow`, `naming`, `nft-lxt721`, `payments-merchant`, `swap-cpmm`, `token-lxt20`, `vault`, and `web-reader` examples, while C and AssemblyScript ship `paid-counter` |
| `porting/evm`, `porting/solana`, `porting/cosmwasm` | Migration crates and `MIGRATION.md` guides mapping Solidity / Anchor / CosmWasm vocabulary onto the programs ABI |
| `fuzz` | Structure-aware fuzz target and corpus for the runtime |
| `benches` | Interpreter benchmarks and their compiled ABI-v2 equivalents |
| `fixtures/pay5` | Token, merchant and swap artifacts, interfaces and registry values, account-authorization vectors, and native activity/receipt fixtures |
| `../scripts/programs` | Dependency policy, immutable ABI-vector generation, and reference-fixture build scripts |
| `../tests/programs` | Native C tests, ABI-drift and runtime module-boundary checks |
| `tests` | Cross-implementation vectors, the hostile-program `gauntlet`, conservation and parallel-differential checks, calldata fixtures, and the root package's registered registry-read integration test |
| `vendor` | Vendored, pinned dependencies for a hermetic build |

Cargo promotes path dependencies below the workspace root into workspace members.
Consequently `cargo test --workspace` includes the vendored `parity-wasm`,
`wasm-instrument`, and `wasmi` unit and documentation tests as well as the programs
packages. A workspace build alone does not execute those tests. The vendored crates
also remain subject to `../scripts/programs/dependency-policy.sh`.

### The core crates

**`layerx-programs-runtime`** is the deterministic WASM foundation for guest programs.
It runs on a pinned `wasmi` interpreter with floating point and other nondeterminism
removed. The module map (see `src/lib.rs`) separates concerns deliberately:

- `validate.rs`, `limits.rs` - static module validation and structural bounds (module
  bytes, function count, stack height, call depth).
- `budget.rs`, `meter.rs` - caller-declared activity ceilings, the admitted budget token,
  and per-execution resource metering (CPU fuel, memory, storage read/write, output).
- `abi/` - the transaction boundary. `abi/capability.rs` owns capability grants, their
  canonical encoding, and downward-only narrowing; `abi/response.rs` owns response and
  refusal transport; `abi/storage_ops.rs` owns namespaced storage; `abi/codec.rs` owns
  the canonical calldata encoding.
- `accounts.rs` - deterministic derivation of program-owned accounts.
- `transfer.rs`, `ffi_transfer.rs` - the sole monetary exit: typed `402LXP` requests
  bound to invocation authority and submitted as one atomic set to the kernel primitive.
- `occupancy.rs` - deterministic, receipt-bound storage-occupancy accounting.
- `calls.rs`, `engine.rs`, `execute.rs`, `entrypoint.rs`, `lifecycle.rs` - cross-program
  calls, execution, and program lifecycle.
- `host/` - linker orchestration; each host-function family is registered in its own
  unit and reaches execution state only through `RuntimeState`.
- `ffi.rs`, `ffi_call.rs`, `ffi_interface.rs` - the FFI bridge the C programs module calls
  into.

**`layerx-programs-registry`** (`#![forbid(unsafe_code)]`) turns protocol receipts into
answers about programs: the `DeploymentJournal`, `ProgramValueAccountBinding` records, and
`VerifiedAccountSnapshot`/`ValueAccount` types that resolve a program's real balance from
account-tree Merkle proofs rather than a declared bookkeeping column. It also owns
deprecation and wind-down (`AuthorizedExit`, `ExitRoute`, `WindDownView`) and the
`lxt20`, `lxt721`, `naming`, and `swap` reference interfaces.

Registry reads use [`protocol_evidence.rs`](crates/layerx-programs-registry/src/protocol_evidence.rs)
and [`chain_proof/`](crates/layerx-programs-registry/src/chain_proof) to verify deployment,
lifecycle and state evidence against chain commitments. `VerifiedProgramCatalog` in
[`resolver.rs`](crates/layerx-programs-registry/src/resolver.rs) admits executable
programs from verified bundles. Integration uses the registry chain-proof APIs below.

Callers obtain program balances through node-root JSON-RPC `eth_call` to the Programs
precompile at `0x101D` (`0x000000000000000000000000000000000000101D`). ABI-encode
`getValueAccount(bytes32,bytes32)` with the program id and registered account id, and
select an explicit committed block height so related reads share one state snapshot.
The [published ABI](../precompiles/layerxprograms/abi.json) returns `uint128 balance`
and `bool frozen`; the boolean is a freeze flag, not an existence flag. An absent
account returns `(0, false)`, so that pair alone does not prove registration. The
precompile reads the kernel account state; registry chain-proof verification supplies
authenticated evidence rather than trusting an adapter's bookkeeping balance.

### Registry reads from mounted chain state

The read bridge is **`layerx-programs-registry`**, within the same chain-125 network
that executes programs inside `paxd`. The node mounts the kernel state in the
`lxphost` store ([store keys](../storage/common/keys/store_keys.go),
[node wiring](../node/app.go)); there is no separate settlement chain.

Use the registry's [`chain_evidence`](crates/layerx-programs-registry/src/chain_evidence.rs)
and [`chain_proof`](crates/layerx-programs-registry/src/chain_proof.rs) APIs for reads:

1. Decode the canonical `LXCP` commitment bundle with `decode_commitment_proof`.
2. Authenticate its chain header and CometBFT commit with `ChainProofVerifier`
   against trusted chain identity and validator information. A validator set supplied
   in an untrusted bundle alone does not establish trust.
3. Verify the `ics23:iavl` key/value proof and the `ics23:simple` multistore proof
   for `lxphost` against the authenticated header's `app_hash`. State at height `H`
   is authenticated by the header at `H + 1`; `verify_state` returns `H`.
   `verify_absent` authenticates non-membership through the same store root.
4. Let `protocol_evidence` bind the verified state, deployment or lifecycle evidence
   before `VerifiedProgramCatalog` admits executable programs. A decoded bundle or
   an execution receipt alone is insufficient evidence of committed state.

---

## Kernel surfaces

The surfaces below are described as implemented; the authoritative sources are the code
and [`spec/layerx-merge/spec.kvx`](../spec/layerx-merge/spec.kvx).

### Program-owned accounts

A program can own accounts that no principal can claim. An account id is derived
deterministically from the program and a seed - a pure function of public inputs, with no
host state, clock, or entropy involved:

```text
account_id = SHA-256(
    "LayerX/programs/program-account/v1\0"   // domain tag, distinct from principal ids
    || program_id                            // 32 bytes, bound before the seed
    || u32_be(seed_len)                      // length prefix removes concatenation ambiguity
    || seed                                  // up to 128 bytes
)
```

The construction is domain-separated: the tag is disjoint from the `LX:ACCOUNT:v1` domain
used for principal/named account ids, so a principal cannot present a public key whose
identifier collides with a derived program account without breaking SHA-256 preimage
resistance. The C kernel (`src/modules/programs/accounts.c`) and the Rust runtime
(`accounts.rs`) agree byte-for-byte, and frozen golden vectors pin the digests.

Deriving an account conveys **no authority**. Program value accounts are registered as
`LX_ACCOUNT_MODULE_VALUE` accounts that carry no authority key, so:

- the ordinary account-open path refuses the `module:programs:value:…` namespace;
- registration is gated on the program owner, not first-caller squatting;
- debits require program authority bound to the deriving program's own frame and exact
  seed (re-derived and checked at transfer time), never a principal signature.

A principal can **fund** a program account (an ordinary `402LXP` credit leg) but can never
**authorize debits** as if it owned the account.

### Downward-only spending grants

Spending authority narrows, never widens. The `ProgramSpend` capability
(`abi/capability.rs`) conveys a bounded grant over accounts the granting program itself
derives - distinct from the `Transfer402` grant over the invoking principal's balance. It
binds `owner_program`, `seed`, `source_account`, `asset`, `to`, and a `maximum_amount`.

Across a program-to-program call edge the grant may only be narrowed:

- any change to identity fields (`owner_program`, `seed`, `source_account`, `asset`, `to`)
  produces a different capability key, so the parent lookup fails and the edge is refused
  with `AbiError::CapabilityEscalation`;
- an attempt to raise `maximum_amount` above the parent's is refused; a child amount must
  be less than or equal to the parent's;
- only the **owner program** may originate a fresh `ProgramSpend` over its own account
  on an edge, and only when the parent set has no `ProgramSpend` grant. An inherited
  grant remains subject to narrowing; a callee cannot mint another program's authority.

At transfer time the grant is checked cumulatively against the actual legs, and the
deriving program's frame is the only frame that may stage a program-account debit. There
is no silent widen and no partial transfer set survives a refused escalation.

### Occupancy settlement

State that persists is paid for as long as it persists. Occupancy is deterministic,
batch-indexed rent for persistent program storage - it meters **namespace bytes held
across protocol batches**, priced by the fee schedule. There is no wall-clock component:
"time" here is the protocol batch sequence.

For each occupied storage namespace over a batch interval:

```text
byte_batches = recorded_bytes × (to_batch − from_batch)
accrued_fee  = byte_batches × occupancy_byte_batch_price
amount_due   = prior_arrears + accrued_fee
```

Before writing storage a call must establish a signed responsibility mandate that names
the payer and caps both the bytes and the lifetime charge; the occupancy fee budget is
carved out of the activity's signed fee limit, separate from the execution meter. Each
charge resolves to a disposition - `Paid`, `ChargeCeilingExceeded` (namespace frozen),
`ScheduleCeilingExceeded`, `InsufficientFunds`, or `MigrationRequired` (pre-upgrade legacy
bytes, frozen at price zero until the owner migrates). Settlement is charged to the
declared responsible account through ordinary `402LXP` transfer legs and bound into the
batch receipt as canonical, replay-checkable evidence. New responsibility is bound to
the admitted payer and root program; a principal-scoped namespace must match that
payer. During legacy-storage activation, principal-scoped bytes are assigned to their
principal and shared bytes to the program owner pending migration.

### Protocol-backed program balances

A program's balance is real kernel state, not a registry counter. Registry value
accounts are the same `LX_ACCOUNT_MODULE_VALUE` accounts that live in the kernel account
tree; the registry reads a balance by verifying the account leaf through the account root,
the universal subtree root, and the receipt state root before surfacing it. Wind-down
refuses to strand value: a deprecation cannot complete while a bound account still carries
a non-zero balance without an authorized exit route.

The invariant that ties all of this together: **`402LXP` remains the sole balance
writer.** Programs emit authenticated transfer sets; the kernel validates and applies
them atomically with the program state transition.

---

## SDKs and porting kits

The runtime admits guest ABI versions 1 through 5, and the checksums of their frozen
vectors (`tests/vectors/abi-v1.hex` to `abi-v5.hex`) are pinned in `abi-frozen.sha256`.
The host imports of ABI 1 to 4 are listed in
[`sdk/abi-manifest.tsv`](sdk/abi-manifest.tsv): ABI 2 adds scoped storage,
responses and refusals, program-account transfers, context, balance, hash,
signature and 256-bit integer host calls; ABI 3 adds `oracle_read`; ABI 4 adds
`web_read`. ABI 5 adds `market_step_adjudicate` in the `layerx_v5` import module;
its manifest is in [`sdk/rust/src/abi.rs`](sdk/rust/src/abi.rs) and its host registration
is in [`host/market.rs`](crates/layerx-programs-runtime/src/host/market.rs).
Deployment, upgrade, call and host registration must agree on the guest ABI. The
runtime's default `ABI_VERSION` remains 4; admitting ABI 5 does not automatically
select it. The node host selects its programs guest ABI through
`lxphost.Config.ProgramsGuestABI` in [`modules/lxphost`](../modules/lxphost).
Native activity envelope versions in [`programs.h`](../include/layerx/programs.h)
are distinct from the guest ABI versions. Guest program SDKs include:

- **`sdk/rust`** (`layerx-program-sdk`) - the Rust guest SDK.
- **`sdk/c`** - a C guest SDK with headers, sources, a determinism lint, and a toolchain
  manifest.
- **`sdk/assemblyscript`** - an AssemblyScript SDK (`abi`, `capability`, `transfer`,
  `storage`, `event`, `call`, `receipt` bindings) with a determinism lint.

The Rust SDK ships `escrow`, `naming`, `nft-lxt721`, `payments-merchant`, `swap-cpmm`,
`token-lxt20`, `vault`, and `web-reader` examples; the C and AssemblyScript SDKs ship
`paid-counter` examples. ProgramSpend (tag 9) and BalanceView (tag 10) are admitted from
ABI 2 on; the runtime crate defines their canonical encoding. Spending ceilings may
only narrow, and a BalanceView's receipt binding must remain identical when passed
to a child. ABI 1 does not admit these grants.

`sdk/rust` also ships the `lxt20` and `lxt721` request codecs (`src/lxt20.rs`,
`src/lxt721.rs`) and program-account preparation and merchant-split helpers
(`src/payments.rs`). See
[`docs/wiki/PaymentsQuickstart.md`](../docs/wiki/PaymentsQuickstart.md)
and [`docs/wiki/Assets.md`](../docs/wiki/Assets.md).

LXT20 is ABI-v2 guest state backed by one native Asset. Its interface has
`initialize`, `transfer`, `approve`, `transfer_from`, `balance_of`,
`allowance`, `total_supply`, and `metadata`. Request bytes are
`[0x4c,0x58,0x14,method] || [1,0x20] || payload_len:u32_be || payload`.
The issuer calls `initialize` once with `4c581400012000000000`; initialization
funds the complete fixed supply from the issuer into its registered program account
under an exact `Transfer402` grant, atomically with supply and balance storage.
The seven other methods use the Rust SDK's `lxt20::Request` codec. Recipients call
`approve` once, including zero, to register their derived account in token storage.
The deployment authority must also register each account with the native Programs
account module before it receives backing units; an approval alone is insufficient.
Transfers require the exact source program-account spending authority, and
`transfer_from` also checks and consumes the allowance. The reference program has
fixed supply and no mint, burn, permit, or nested-call surface. See
[`sdk/rust/examples/token-lxt20`](sdk/rust/examples/token-lxt20/README.md).

The porting kits map familiar contract vocabularies onto the programs ABI and are explicit
about what does not carry over:

- **`porting/evm`** - porting a Solidity contract.
- **`porting/solana`** - porting a Solana / Anchor program.
- **`porting/cosmwasm`** - porting a CosmWasm contract.

Each has a `MIGRATION.md` written for a developer who already knows the source chain.

---

## Fuzzing, tools, and tests

- **`fuzz/`** - a structure-aware fuzz target (`src/main.rs`) with `validation`,
  `instantiation`, and `execution` corpora.
- **`../scripts/programs/dependency-policy.sh`** - enforces the vendored-dependency policy (`deny.toml`).
- **`../tests/programs/runtime-module-boundaries.sh`** - enforces the runtime's module-boundary rules.
- **`tests/gauntlet/`** - the hostile-program gauntlet (cross-program derivation, callee
  spend attempts, escalation across depth/fan-out/repeated visits) with an
  `attack-inventory.tsv`.
- **`tests/vectors/`** - cross-implementation ABI vectors and
  [calldata fixtures](tests/vectors/calldata/README.md) that keep the C and Rust surfaces
  byte-identical.
- **`fixtures/pay5/native-token` and `native-merchant`** - canonical native activities,
  receipts and terminal attachments for token and merchant flows. The
  [`run_native_roundtrip.py`](fixtures/pay5/run_native_roundtrip.py) driver builds
  the real C kernel/runtime harness and replays into a temporary directory, comparing
  the output bytes with the committed fixtures. Its explicit `--write-fixtures` mode
  regenerates those native records.
- **`fixtures/pay5/account-authorization-vectors.json`,
  `destination-authorization-vectors.json` and
  `receipt-account-bound-v4.json`** - account-bound authority and receipt fixtures.
  The destination vectors prove version 2 evidence and roots for native identity
  and mixed identity/account destinations, and refusal of noncanonical encodings,
  mismatched identity or asset names, and empty destination-name lists.
  Program-account derivation, owner-authorized native registration, transfer-source
  binding and receipt verification are separate checks; a guest codec or derived id
  alone proves none of the others.

---

## Building and testing

The programs workspace declares Rust `1.91.1` as its minimum supported version in
[`Cargo.toml`](Cargo.toml) and builds against vendored dependencies.
`programs/.cargo/config.toml` points Cargo at `vendor/`, keeps it offline, and sets the
`wasm32-unknown-unknown` code-generation flags; Cargo reads it only when it runs from
inside `programs/` or one of its descendants. Install the
`wasm32-unknown-unknown` target for guest builds. SDK checks also use Node/npm;
the C and AssemblyScript examples declare their own toolchain requirements.
From the repository root, these targets provide different levels of coverage:

```sh
make programs-build              # locked workspace build, including sandbox host-ffi
make programs-lint               # module boundaries, Clippy, dependency policy, cargo deny
make programs-abi-drift           # ABI manifest drift and runtime linker test
make programs-test               # aggregate native, protocol, runtime, SDK and porting tests
make programs-fuzz-smoke          # replay validation, instantiation and execution corpora
make programs-reference-fixtures  # rebuild naming, LXT721 and swap artifact/interface sets
```

The [`Makefile`](../Makefile) defines these targets. `programs-build` is a native
workspace build; it does not produce deployable WASM for every guest. The reference
fixture target runs [`program-fixtures.sh`](../scripts/programs/program-fixtures.sh),
which remaps source paths and regenerates interfaces and registry values alongside
the artifacts. Native token and merchant fixture generation uses the separate driver
above.

### AI market guest packaging

Build the AI market guest in release mode with `opt-level = "z"` and the merged
chain domain fixed at build time. The workspace release default is `opt-level = 3`;
the AI package's release override in [`Cargo.toml`](Cargo.toml) selects `"z"`
automatically. For chain 125, `merged_chain_domain(125)` in
[`lib.rs`](crates/layerx-programs-ai-market/src/lib.rs) is the SHA-256 of
`b"PAXAI/chain-domain/v1" || u64_be(125)`:

```sh
cd programs
PAXAI_CHAIN_DOMAIN=62b6b14734fa769c3e0fbc94de495a3d038be49a2524b1c071d75d0a4eed71f6 \
  cargo build --locked --release --target wasm32-unknown-unknown \
  -p layerx-programs-ai-market
```

`PAXAI_CHAIN_DOMAIN` is a compile-time setting; malformed or foreign domains are
refused by the guest's deployment-domain validation. The deployment payload limit is
524288 bytes (`LXP_MAX_PAYLOAD_BYTES` in
[`lxp_protocol.h`](../include/layerx/lxp_protocol.h)), including the 104-byte
deployment envelope. The guest must fit within the remaining 524184 bytes. When an
interface is included, the decoder in [`deploy.c`](../src/modules/programs/deploy.c)
uses 108 fixed bytes plus the interface bytes, reducing the space for WASM further.
The production `make paxai-host-boundary-build` target builds and packages through
[`guest-recipe.sh`](crates/layerx-programs-ai-market/tests/fixtures/guest-recipe.sh),
the same recipe used by `build-guest.sh` and the RPC guests.
It requires Binaryen 133, release `opt-level = "z"`, LTO and `cdylib`, with the
compiler arguments and size pass list defined in that file. It checks that the
resulting WASM is at most 524184 bytes before passing that same file to the native
host-boundary harness. That bound reserves the 104-byte envelope.
A successful Cargo build alone
does not establish that the final payload fits or that the registering host selected
the matching guest ABI.

Runtime and kernel share golden vectors so the C and Rust implementations stay in
lockstep. Deterministic cross-architecture replay and fault injection are described in
[`docs/QUALIFICATION.md`](../docs/QUALIFICATION.md).

## Status

This workspace targets Paxeer mainnet-beta, chain 125. Public node access and hosted
gateway method contracts are documented separately in the references above; a method's
presence in a hosted contract does not imply availability on every public node.
Source is licensed under the Apache License, Version 2.0. The documentation site
sources are in [`docs/site`](../docs/site).
