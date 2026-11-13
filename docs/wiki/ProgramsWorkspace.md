# Programs workspace gates

The `programs/` Cargo workspace supplies the deterministic WASM runtime,
receipt-bound registry, guest SDK and reference programs used by the merged
Paxeer network. Programs execution is kernel module `9` (`LXP_MODULE_PROGRAMS`
in `include/layerx/lxp_module.h`). The kernel runs inside the network and
proposes blocks; program effects and EVM transactions belong to the same
chain, and a receipt is final when its block is committed.

The C implementation is `src/modules/programs/`. Rust host entry points live
in the runtime's `src/ffi*.rs` and the sandbox's `src/host_ffi.rs`.
See [Programs](Programs.md) for activity kinds and [Roadmap](Roadmap.md) for
the distinction between implemented guest references and deployment status.

## Workspace and crate layout

`programs/Cargo.toml` is the workspace inventory. It selects resolver `2`,
edition `2021`, Rust `1.91.1` and package version `0.1.0`.
`programs/clippy.toml` uses the same MSRV. Workspace lints deny unsafe code,
Clippy `all` and `pedantic`, unwraps, expects and floating-point arithmetic.
The explicit FFI modules carry local unsafe allowances for C calls.

| Crate under `programs/crates/` | Responsibility |
| --- | --- |
| `layerx-programs-runtime` | WASM validation, metering, capability checks, execution and native host FFI. |
| `layerx-programs-registry` | Deployment provenance, executable admission, interfaces, chain proofs, program accounts and wind-down evidence. |
| `layerx-programs-interpreter` | Bounded deterministic scripting guest. |
| `layerx-programs-market` | Offers, leases, usage claims, attestations and disputes. |
| `layerx-programs-ai-market` | PAXAI market guest, request codecs, aggregation, evaluation, rewards and direct native Programs entry point. |
| `layerx-programs-arbiter` | Authenticated replay, chain-commit authority, proof verification and market sandbox evidence. |
| `layerx-programs-sandbox` | Bounded leases, ephemeral namespaces, usage, escrow, snapshots and expiry; exports host FFI with `host-ffi`. |

The other workspace members are `benches/compiled-equivalent`, `fuzz`,
`sdk/rust`, `sdk/rust/lint`, and the CosmWasm, EVM and Solana porting crates
and their `guest` subdirectories. Examples can have their own Cargo manifests;
use the owning example's build instructions rather than assuming a workspace
build produces every WASM artifact. See [Porting](Porting.md).

## Frozen ABI

`programs/sdk/rust/src/abi_policy.rs` defines admitted guest ABI versions
**1 through 5**. Upgrades can retain or increase the version and refuse a
downgrade. Version 1 uses capability encoding V1; versions 2–5 use V2.
The runtime's crate-root `ABI_VERSION` is still **4**. Admission of v5 does
not change that default: registration must explicitly select the ABI the
guest imports. The C module registration version is a separate version space
(`include/layerx/programs.h`, `src/modules/programs/registration.c`).

`programs/abi-frozen.sha256` independently pins all five immutable vector
files, `programs/tests/vectors/abi-v{version}.hex`:

| ABI | SHA-256 of the vector file |
| --- | --- |
| 1 | `09fcad46aeea9659d7d555a4a09ec151bd36d60b85bfd052d1f7a43cf71d58a3` |
| 2 | `8827869bf1324c3e82c60baf7360b1c3e2baa84d8591e49eb6b73c2cea6c8b69` |
| 3 | `0d1c2505bb16c52cbac38f966359fafd4e930949f968484250ec96520a7de0b2` |
| 4 | `733f27cbf903eaceab532424d13e321f6dd8939afb1f85511c7a5a12c0289aef` |
| 5 | `07d573becee2e920b7bacb3f7d712ff017070681d46337f62e9b6987c58a2e87` |

### `generate-abi-vectors.py`

`scripts/programs/generate-abi-vectors.py` compares the runtime's
`abi/mod.rs`, `abi/manifest.rs` and crate-root manifest with the SDK's
`src/abi.rs` and canonical `src/abi_policy.rs`. It checks composite manifests
and host tables through v5, function signatures and types, the validator's
manifest-derived import allowlist, SDK agreement and the v4 runtime default.

Each ABI vector is the hexadecimal encoding of a big-endian u16 version
followed by manifest bytes, with a trailing newline. Frozen vectors must
exist, match the independent SHA-256 baseline and match the audited source.
The generator refuses to recreate or overwrite a frozen vector. An existing
unfrozen vector needs a reviewed baseline; `--check` also refuses a missing
new vector. Without `--check`, it can write a missing unfrozen vector.
Allocate a new version for a changed frozen surface.

The generator also checks or writes `terminal-v4-schema.json` and
`terminal-v4-outcome.hex` under `programs/tests/vectors/`, deriving the latter
from the retained executed C receipt fixture. Terminal encoding versions and
guest ABI versions are distinct.

`make programs-abi-drift` invokes `tests/programs/check-abi-drift.sh`
(`generate-abi-vectors.py --check`), then the runtime's `abi_linker` test.
The check mode leaves the vectors untouched.

## AI-market deployment artifact

Build the AI-market guest for `wasm32-unknown-unknown` in release mode with
optimization level `z`. The workspace release profile defaults to level `3`
and overrides the AI-market package to `z`. The shared guest recipe also sets
LTO, one codegen unit, aborting panics, stripped symbols and its canonical
code-generation flags, then packages the output with Binaryen `wasm-opt`
version 133. Install that optimizer and the WASM target first. From the
repository root:

```sh
PAXAI_CHAIN_DOMAIN=62b6b14734fa769c3e0fbc94de495a3d038be49a2524b1c071d75d0a4eed71f6 \
bash programs/crates/layerx-programs-ai-market/tests/fixtures/guest-recipe.sh \
  build programs /tmp/ai-market.wasm
```

`PAXAI_CHAIN_DOMAIN` is read at compile time with `option_env!` in the guest's
`src/lib.rs`. It must be 64 lowercase hexadecimal digits encoding the domain
derived by SHA-256 over `PAXAI/chain-domain/v1` followed by the big-endian u64
chain ID `125`. The guest and `paxai-host-boundary-build` Makefile recipe both
default to that merged-chain domain. The guest refuses malformed or foreign
overrides with `WRONG_DOMAIN`; set the domain explicitly when packaging.

The command above publishes the packaged artifact at `/tmp/ai-market.wasm`.
`make paxai-host-boundary-build` uses the same guest recipe, checks the packaged
WASM against the 524184-byte ceiling and runs the native host-boundary executable
against that artifact. The intermediate Cargo output is
`wasm32-unknown-unknown/release/layerx_programs_ai_market.wasm` under the selected
target directory (normally `programs/target`).
`LXP_MAX_PAYLOAD_BYTES` in `include/layerx/lxp_protocol.h` is **524,288 bytes**.
The basic deployment encoding in `src/modules/programs/deploy.c` consumes
**104 bytes** before WASM, leaving at most **524,184 bytes** for WASM with
that encoding. An attached interface and its framing consume additional
payload space. Measure the complete encoded payload, not just the WASM file.
A native workspace build alone does not establish that the deployment fits.

Keep the guest, declared deployment ABI and host registration in agreement
on every deployment and call path. In particular, ABI-5 guest execution must
be admitted as ABI 5 even though the runtime default constant is 4.

## Dependency policy

`make programs-lint` runs the module-boundary check, workspace Clippy with
`host-ffi`, `scripts/programs/dependency-policy.sh`, cargo-deny advisories
and sources, then cargo-deny bans with `--exclude-dev`.

The shell policy inspects locked Cargo metadata, including development
dependencies. Its banned package names are `bindgen`, `libsqlite3-sys`,
`rusqlite`, `sqlx-sqlite`, `ctor`, `inventory`, `getrandom`, `rand`, `chrono`,
`time`, `instant`, `tokio`, `mio`, `socket2`, `wasi` and `wasmtime`.
`rand_core` is allowed only without its `getrandom` and `std` features.

Sourced dependencies need accepted SPDX licensing and a vendored manifest
with `.cargo-checksum.json`. The WASM engine is pinned to
`vendor/wasmi-0.31.2` with version `=0.31.2`.
`programs/.cargo/config.toml` replaces crates.io with `vendored-sources` and
sets offline mode. The script checks unsafe-code exceptions and refuses
`f32` or `f64` in `programs/crates/`.

The script and `programs/deny.toml` have separate license checks: the shell
allowlist includes `BSD-1-Clause`, while cargo-deny's does not. Cargo-deny's
bans run without development dependencies; the shell package-name check
includes them. Passing one check does not imply the other passed.
Policy refusals exit nonzero with a diagnostic; the shell script's success
message is `programs dependency, vendoring, unsafe and integer-only policies passed`.

## Module-boundary check

`make programs-module-boundaries` invokes
`tests/programs/runtime-module-boundaries.sh` against the runtime source,
then runs the script's rejection self-tests. It checks required files,
rejects legacy monolithic `abi.rs` and `host.rs`, and rejects production
FFI/lifecycle references to qualification-only APIs.

The checked ABI inventory is `balance`, `capability`, `codec`, `context`,
`event_tests`, `host_state`, `manifest`, `mod`, `response` and `storage_ops`.
The host inventory is `balance`, `calls`, `context`, `crypto`, `events`,
`market`, `memory`, `mod`, `oracle`, `scan`, `scan_tests`, `signature`, `storage`,
`transfer` and `web`.
Host families must reach execution state through `RuntimeState`; the script
rejects sibling-family imports, forbidden parent aliases and direct access
to the ABI, composition, storage or meter state.

## Fixture recipes

The following are Makefile recipes for their respective fixtures. Generation
writes artifacts; a `check` recipe compares against the committed artifacts.

| Recipe pair | Producer and evidence |
| --- | --- |
| `programs-generate-capability-fixture` / `programs-check-capability-fixture` | Runtime example `capability_fixture`; `sdk/platform/conformance/fixtures/native-program-capabilities-v2.json`. |
| `programs-native-lifecycle-fixtures` / `programs-check-native-lifecycle-fixtures` | C `programs_call_activity` encoder and `generate_native_lifecycle_fixtures.py`; historical v3 lifecycle inputs are retained, current call/deploy/upgrade/wind-down outputs use v4 names, with EVM wrappers for supported lifecycle operations. |
| `programs-executed-v4-fixture` / `programs-check-executed-v4-fixture` | `generate_executed_program_v4_fixture.py` consumes the C encoder's executed receipt evidence. |

### Committed receipt verification

Use the merged chain's transaction, result and state proofs with trusted
chain authority. The registry's `src/chain_proof/`, `src/chain_evidence/` and
`src/protocol_evidence.rs` implement the proof and evidence bindings.
An execution result or a guest response by itself does not establish block
commitment. Client-side check order is described in
[SDK terminal verification](SdkTerminalVerification.md), with adapted
schemas in `sdk/platform/conformance/README.md`.

The chain Programs precompile is `0x000000000000000000000000000000000000101D`
(`precompiles/layerxprograms/events.go`). Its contract is
`precompiles/layerxprograms/LayerXPrograms.sol`; it exposes program calls,
lifecycle operations and views on the same network. `api/layerxprograms/query.proto`
defines `ProgramState`, `ProgramBalances` and `ProgramEvents` query evidence.

## Test tiers

These recipes describe the current Makefile and their respective build and
test coverage.

`programs-build` builds the locked workspace with
`layerx-programs-sandbox/host-ffi`, using `programs/target` for the native
output. `PROGRAMS_RUNTIME_LIB` is
`programs/target/debug/liblayerx_programs_sandbox.a`.

| Recipe | Coverage |
| --- | --- |
| `programs-core-test` | C registration, lifecycle, monetary law, transfer principal, call escrow principal, call activity, metered call, metering schedule, accounts and wind-down binaries. |
| `programs-protocol-regression` | `test-kernel`, `test-module-ctx`, `test-dispatch`, `test-receipts`, `test-state-root`, `test-snapshot`. |
| `programs-fuzz-smoke` | `layerx-programs-fuzz` validation, instantiation and execution corpora. |
| `programs-adversarial` | Runtime `isolation`, `composition` and `monetary_law` integration tests. |
| `programs-conservation` | `programs/tests/conservation/run.sh`. |
| `programs-differential` | Runtime `replay` and `determinism` tests, then the C parallel differential binary. |
| `programs-interpreter-conformance` | Release interpreter WASM build, then runtime `interpreter_program` with `LAYERX_INTERPRETER_WASM`. |
| `programs-bench` | `programs-interpreter-bench`: release interpreter and compiled-equivalent guests, then the runtime interpreter benchmark. |
| `programs-qualify` | `tools/qualification/release_runner.py programs-qualify`. |

The differential binary is compiled from
`programs/tests/differential/parallel.c`, which includes
`tests/programs/test_call_activity.c`, and links the kernel and Programs
runtime libraries. Its Makefile prerequisites contain those two sources and
libraries; compiler-generated dependency files also track included headers
and sources. It uses the merged kernel execution path.

## Signature and storage test authorities

`tests/vectors/signatures/sources.json` pins the RFC 8032 text, Wycheproof
secp256k1/SHA-256 cases and libsecp256k1 v0.6.0 headers by URL and SHA-256.
`tests/vectors/signatures/check.py` checks those source hashes and independently
compares the Rust signature-vector bytes with the published authorities
using the Python cryptography backend. The runtime tests are
`tests/signature_vectors.rs` and `tests/signature_authority.rs` under
`programs/crates/layerx-programs-runtime/`.

The authority checks cover valid compressed and uncompressed secp256k1
keys, zero scalar refusal, the deliberate low-S rule, RFC 8032 TEST 1 and
the noncanonical Ed25519 `S + L` mutation. Keep the independent authority
pins separate from generated fixtures.

`programs/crates/layerx-programs-runtime/tests/storage_scan.rs` covers
canonical page encoding, cursor binding, deterministic pagination, metered
read bytes and rollback after a guest fault. The independent size arithmetic
in `tests/vectors/signatures/check.py` records 29 bytes for three terminal
one-byte key/value entries, 101 bytes for two such entries plus an 80-byte
cursor, and 13 bytes for one terminal entry. See [Storage scan](StorageScan.md).

[Programs](Programs.md) · [Roadmap](Roadmap.md) · [Home](Home.md)
