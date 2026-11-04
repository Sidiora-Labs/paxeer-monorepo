# Programs workspace gates

The `programs/` Cargo workspace contains the deterministic WASM runtime, receipt-bound registry, guest SDKs, programs, porting kits and qualification corpus. Guest execution is kernel module `0x09` in the merged Paxeer network. The node invokes the C kernel and Rust runtime through `modules/lxphost/`; the Programs EVM precompile lives in `precompiles/layerxprograms/` at `0x000000000000000000000000000000000000101D`. See [Protocol](../protocol/index.md) and [Modules](../protocol/modules.md).

The recipes below describe available repository checks. Select the checks declared by the task being qualified; this catalogue is not a requirement to run every tier for a documentation change.

## Workspace and crate layout

`programs/Cargo.toml` uses resolver 2 and workspace package metadata `edition = "2021"`, `rust-version = "1.91.1"`, `version = "0.1.0"`, license `Apache-2.0`. `programs/clippy.toml` uses the same MSRV. Workspace lints deny unsafe code, Clippy `all` and `pedantic`, `unwrap_used`, `expect_used`, floating-point arithmetic and lossy float literals.

Members under `programs/crates/`:

| Crate | Role |
| --- | --- |
| `layerx-programs-runtime` | WASM validation, execution, metering, host ABI, composition and runtime FFI. |
| `layerx-programs-registry` | Deployment journal, program accounts, verified balance views, lifecycle and wind-down state. |
| `layerx-programs-interpreter` | Bounded deterministic scripting guest. |
| `layerx-programs-market` | Market program types and transitions. |
| `layerx-programs-ai-market` | AI-market guest and its authenticated state transitions, domain binding and host integration. |
| `layerx-programs-arbiter` | Verified replay, chain proofs, boundary and market adjudication evidence. |
| `layerx-programs-sandbox` | Protocol-state models for bounded ephemeral sandboxes and the native host FFI static library. |

Other members are `benches/compiled-equivalent`, `fuzz`, `sdk/rust`, `sdk/rust/lint`, and the host/guest pairs under `porting/cosmwasm`, `porting/evm` and `porting/solana`. Standalone Rust examples and fixtures listed in the workspace's `exclude` array use their own manifests. See [Porting](../operators/porting.md).

The workspace release profile defaults to optimization level 3, LTO, one codegen unit, aborting panics and stripping. AI-market deployment has a stricter size requirement and uses an explicit size-optimization override; see [AI-market deployment qualification](#ai-market-deployment-qualification).

## Frozen ABI

Guest ABI and protocol wire version are separate. Runtime crate-root `ABI_VERSION` selects SDK policy `ABI_V4_VERSION`; ABI 1 through 5 remain allocated and frozen. `programs/sdk/rust/src/abi_policy.rs` owns the version numbers, and the runtime reexports them.

| ABI | Cumulative surface |
| --- | --- |
| 1 | Original `layerx_v1` storage, event, call, transfer and receipt imports. |
| 2 | Adds `layerx_v2` responses, scoped storage and scans, program-account transfers, context, balance, crypto and bigint imports. |
| 3 | Adds `layerx_v3.oracle_read`. |
| 4 | Adds `layerx_v4.web_read`; this is the runtime default. |
| 5 | Adds `layerx_v5.market_step_adjudicate`; AI market requires the v5 host route. |

`programs/abi-frozen.sha256` pins the exact bytes of `programs/tests/vectors/abi-v{version}.hex`:

```text
1 09fcad46aeea9659d7d555a4a09ec151bd36d60b85bfd052d1f7a43cf71d58a3
2 8827869bf1324c3e82c60baf7360b1c3e2baa84d8591e49eb6b73c2cea6c8b69
3 0d1c2505bb16c52cbac38f966359fafd4e930949f968484250ec96520a7de0b2
4 733f27cbf903eaceab532424d13e321f6dd8939afb1f85511c7a5a12c0289aef
5 07d573becee2e920b7bacb3f7d712ff017070681d46337f62e9b6987c58a2e87
```

These are SHA-256 digests of the vector files, including their textual hex encoding and trailing newline. They are independently reviewed baselines, not hashes regenerated to match a changed linker.

### `generate-abi-vectors.py`

`scripts/programs/generate-abi-vectors.py` audits the runtime's `abi/manifest.rs`, `abi/mod.rs` and `lib.rs`, and the SDK's `abi.rs` and `abi_policy.rs`. It checks allocated version numbers, policy reexports, the v4 default, cumulative host tables, function types and signatures, the validator allowlist and SDK agreement for ABI 2 through 5. ABI v4's manifest is owned by crate-root `ABI_MANIFEST`.

For each version the vector is the hex encoding of `u16be(version) || manifest`, followed by a newline. The generator's frozen-vector behavior is fail-closed:

- A missing frozen file is refused and cannot be recreated by the generator.
- A frozen file must match both its independent checksum and the generated surface. Drift requires allocating a new ABI version.
- An existing unfrozen vector requires review and a checksum baseline; it is not silently overwritten.
- `--check` refuses a new version with no vector. Without `--check`, a missing unfrozen vector can be written.

`make programs-abi-drift` runs `tests/programs/check-abi-drift.sh` (the generator with `--check`), then the runtime's `abi_linker` integration test. A frozen surface is not upgraded by changing the default ABI constant.

The merged host selects `programs_module_registration_v5()` when configured for guest ABI 5 and the v4 storage-retirement registration otherwise (`modules/lxphost/host.go`). Registration, deployment metadata, validator and runtime must agree; merely compiling v5 imports does not enable them on a v4 host.

## Dependency policy

`make programs-lint` depends on `programs-module-boundaries`, runs workspace Clippy with sandbox `host-ffi` and `-D warnings`, then runs:

```sh
sh scripts/programs/dependency-policy.sh
cd programs
cargo deny check advisories sources
cargo deny --exclude-dev check bans
```

`programs/deny.toml` supplies cargo-deny policy. The shell policy obtains locked Cargo metadata and checks every package, including development dependencies. Its banned package names are:

```text
bindgen libsqlite3-sys rusqlite sqlx-sqlite ctor inventory getrandom rand
chrono time instant tokio mio socket2 wasi wasmtime
```

`rand_core` itself is permitted, but resolved `getrandom` or `std` features on it are refused. The cargo-deny bans invocation excludes development dependencies; the shell check does not.

### License, vendoring and code boundaries

The shell license check applies to sourced packages and requires an allowed SPDX expression. Its allowed tokens are `Apache-2.0`, `BSD-1-Clause`, `BSD-2-Clause`, `BSD-3-Clause`, `CC0-1.0`, `ISC`, `MIT`, `Unicode-3.0`, `Zlib` and `LLVM-exception`. It checks all operands of an `AND` expression and accepts a fully allowed alternative for `OR` expressions. `programs/deny.toml` has its own allowlist; in particular it does not include the shell policy's `BSD-1-Clause` allowance.

Every sourced package/version must have a vendored `Cargo.toml` and `.cargo-checksum.json`. The engine remains pinned to `vendor/wasmi-0.31.2`, version `=0.31.2`, and `programs/.cargo/config.toml` replaces crates.io with `vendored-sources` and enables offline resolution.

Under `programs/crates/`, unsafe code is restricted to the runtime's `src/ffi*.rs` and sandbox's `src/host_ffi.rs` boundary files. The policy also rejects `f32` and `f64` tokens. These boundaries keep clock, randomness, network and database dependencies out of consensus-adjacent guest execution.

### Failure reporting

The script uses `set -eu`; explicit policy refusals print a diagnostic to stderr and exit 1. Useful prefixes are:

| Diagnostic | Meaning |
| --- | --- |
| `programs dependency policy:` | Forbidden dependency, entropy feature or license. |
| `programs vendoring policy:` | Missing vendored package/checksum, changed engine pin or source replacement. |
| `programs unsafe policy:` | Unsafe code outside permitted FFI boundaries. |
| `programs integer-only policy:` | Floating-point type in the checked crate tree. |

Success prints `programs dependency, vendoring, unsafe and integer-only policies passed`. Consult the script and `deny.toml` for the exact current rule rather than treating one check as a substitute for the other.

## Module-boundary check

`make programs-module-boundaries` runs `sh tests/programs/runtime-module-boundaries.sh`. Its default root is `programs/crates/layerx-programs-runtime/src`; it checks that layout and then runs its own `--self-test` against temporary layouts.

The required split is:

```text
budget.rs
abi/{mod,balance,capability,codec,context,event_tests,host_state,manifest,response,storage_ops}.rs
host/{mod,balance,context,memory,storage,events,calls,transfer,scan,crypto,signature,oracle,web}.rs
```

The script rejects missing files, unexpected ABI/host module inventory, legacy `abi.rs` or `host.rs`, and `_for_qualification` calls in production `ffi*.rs` or `lifecycle.rs`. Host families `storage`, `events`, `calls`, `transfer`, `scan`, `crypto`, `signature`, `oracle` and `web` must not import siblings, reach forbidden parents through aliases, or directly reach the private state types checked by the script. Diagnostics begin `runtime module boundary:`. The self-test exercises accepted layout and refusal cases including missing budget code, qualification API leakage, sibling imports, parent aliases and direct state access.

## Fixture recipes

`PROGRAMS_CARGO` defaults to `cargo`. Capability fixtures are generated by the runtime's `capability_fixture` example. Lifecycle fixtures are encoded by `$(BUILD_DIR)/tests/programs_call_activity`, built from the real C activity test and linked against the C kernel and sandbox static library.

| Recipe | Behavior and output |
| --- | --- |
| `programs-generate-capability-fixture` | Run the locked runtime example and copy its output to `sdk/platform/conformance/fixtures/native-program-capabilities-v2.json`. |
| `programs-check-capability-fixture` | Run the same generator and compare its bytes with the committed JSON using `cmp`. |
| `programs-native-lifecycle-fixtures` | Run `sdk/platform/conformance/fixtures/generate_native_lifecycle_fixtures.py --encoder …`; generate the native deploy, upgrade and wind-down route/deprecate/tombstone/exit v3 JSON files beside that script. |
| `programs-check-native-lifecycle-fixtures` | Run the lifecycle generator with `--check` against those six committed files. |

### Stored receipt and commit verification

Stored vectors in `daemon/committed-fixture/vectors/` remain verification authorities. Preserve their original signed bytes and provenance; regenerating signatures to match a consumer defeats their purpose. The directory name identifies fixture code, not an extra process required to run the merged network.

| Vector | Verification |
| --- | --- |
| `vector-v1-receipted-positive.json` | Successful EIP-1559 contract creation with two logs: key pins, signature, raw transaction and JSON-RPC view. |
| `vector-v2-receipted-revert.json` | Authenticated LayerXLedger revert: status, result code and `LXP_INSUFFICIENT_BALANCE` return data. |
| `vector-v3-receipt-refusals.json` | Refuse the six named mutations of key, height, transaction, sequence, logs and canonical encoding. |
| `vector-v4-committed.json` | Receipt inclusion, both inclusion-envelope refusals and recorded `payExact` proofs; refuse a consensus commit with at most two thirds voting power. |

Reference tests are `TestVectorV1ReceiptedPositiveVerifies`, `TestVectorV2ReceiptedRevertVerifies`, `TestVectorV3EveryRefusalIsRefused` and `TestVectorV4CommittedVerifies` in `daemon/committed-fixture/vector_v*_test.go`.

An execution signature authenticates the signed result. Proving commitment additionally checks transaction, batch and result inclusion and consensus commits against trusted chain authority. These are proof components of the network's committed history, not a second settlement network. A receipt is final once its block is committed. The SDK-adapted vector schemas are documented in `sdk/platform/conformance/README.md`.

## Test tiers

`programs-build` builds the locked workspace with `layerx-programs-sandbox/host-ffi` and fixes `CARGO_TARGET_DIR` to `programs/target`. `PROGRAMS_RUNTIME_LIB` is `programs/target/debug/liblayerx_programs_sandbox.a`. C activity binaries link that library and `$(LIBRARY)` or `$(TEST_LIBRARY)` after the `programs-build` prerequisite.

The following targets are the current Makefile recipes, not additional gates run by this documentation task:

| Recipe | Work performed |
| --- | --- |
| `programs-core-test` | Run ten C binaries: registration, lifecycle, monetary law, transfer principal, call escrow principal, call activity, metered call, metering schedule, accounts and wind-down. |
| `programs-protocol-regression` | Depend on `test-kernel`, `test-module-ctx`, `test-dispatch`, `test-receipts`, `test-state-root`, `test-snapshot`; no separate recipe body. |
| `programs-fuzz-smoke` | Run the locked `programs-fuzz` binary on validation, instantiation and execution corpora at absolute repository paths. |
| `programs-adversarial` | Run runtime integration tests `isolation`, `composition` and `monetary_law`. |
| `programs-conservation` | Run `programs/tests/conservation/run.sh`. |
| `programs-qualify` | Invoke `python3 tools/qualification/release_runner.py programs-qualify`. |
| `programs-differential` | Run runtime `replay` and `determinism` tests, then `$(BUILD_DIR)/tests/programs_parallel_differential`. |
| `programs-interpreter-conformance` | Build the interpreter release WASM for `wasm32-unknown-unknown`, then run `interpreter_program` with `LAYERX_INTERPRETER_WASM` set. |
| `programs-bench` | Depend on `programs-interpreter-bench`, which builds interpreter and compiled-equivalent release WASM, then runs the runtime's `interpreter` benchmark with both artifact paths. |

The differential C binary is compiled from `programs/tests/differential/parallel.c`, which includes `tests/programs/test_call_activity.c`, against the kernel and sandbox libraries. It compares sequential and prepared execution: receipt/state roots, canonical receipt bytes and event bytes, including conflict and planning-refusal workloads. Its current Makefile inputs contain no daemon WAL source.

For interpreter conformance and benchmarks, the recipes set artifact paths under `programs/target`; ensure any Cargo target-directory override agrees with those paths.

## AI-market deployment qualification

AI-market guest packaging must use a release build with size optimization (`opt-level = "z"`), a chain domain bound at build time, and a deployment payload within `LXP_MAX_PAYLOAD_BYTES = 524288` from `include/layerx/lxp_protocol.h`. A deployment with no interface descriptor has a 104-byte envelope, leaving at most **524184 bytes of WASM**. With an interface descriptor the header is 108 bytes plus the descriptor length, so its budget must be reduced accordingly (`precompiles/layerxprograms/payload_lifecycle.go`).

`programs/crates/layerx-programs-ai-market/src/lib.rs` derives the merged chain domain as SHA-256 of `PAXAI/chain-domain/v1` followed by the 8-byte big-endian chain ID `125`. Its value is `62b6b14734fa769c3e0fbc94de495a3d038be49a2524b1c071d75d0a4eed71f6`. `PAXAI_CHAIN_DOMAIN` is read with `option_env!`, so it is a build input, not a runtime setting. An explicit value must be 64 lowercase hex characters and match that domain; malformed or foreign values return `WRONG_DOMAIN`. An unset setting selects the built-in merged-chain domain.

For an explicit deployment build, run from `programs/`:

```sh
PAXAI_CHAIN_DOMAIN=62b6b14734fa769c3e0fbc94de495a3d038be49a2524b1c071d75d0a4eed71f6 \
CARGO_PROFILE_RELEASE_OPT_LEVEL=z \
CARGO_TARGET_DIR=target \
cargo build --locked --release --target wasm32-unknown-unknown \
  -p layerx-programs-ai-market
```

Measure the resulting `target/wasm32-unknown-unknown/release/layerx_programs_ai_market.wasm` and include the actual deployment framing in the payload budget. The workspace's default release optimization level 3 is not this size profile. `make paxai-host-boundary-build` builds and records a guest/host manifest, but its current default `PAXAI_CHAIN_DOMAIN` differs from the guest's chain-125 derivation; supply the domain above and the size-profile override explicitly when using that recipe.

A size check alone does not qualify deployment. The guest must validate as ABI 5 under the selected metering schedule, be admitted by the ABI-v5 Programs registration, and execute through the real host boundary. Relevant authorities are `tests/programs/test_programs_abi5_admission.c`, `tests/programs/test_ai_host_boundary.c`, and the AI-market crate's `chain_domain`, `host_boundary` and `fuel_budget` integration tests. Preserve the artifact's domain, code hash, ABI, size and metering context with deployment evidence.

## Signature and storage test authorities

`tests/vectors/signatures/sources.json` pins the fetched authorities by file, URL and SHA-256:

| File | Source | SHA-256 |
| --- | --- | --- |
| `rfc8032.txt` | `https://www.rfc-editor.org/rfc/rfc8032.txt` | `ed63657ff389301282b169b0abde9b5dd2c7e4d524fdfa5da6ff3094fc93c4c3` |
| `wycheproof.json` | `https://raw.githubusercontent.com/C2SP/wycheproof/main/testvectors_v1/ecdsa_secp256k1_sha256_test.json` | `43db761c0a2eae71fb0755d355d5130e28ce64a5b07846cf27e7072082597a81` |
| `secp256k1.h` | `https://raw.githubusercontent.com/bitcoin-core/secp256k1/v0.6.0/include/secp256k1.h` | `0c3bbf04703f9a240a200a57fb75b3d7ef072464f24369d34bd9fc0848a66f3b` |
| `secp256k1_recovery.h` | `https://raw.githubusercontent.com/bitcoin-core/secp256k1/v0.6.0/include/secp256k1_recovery.h` | `9ef005af267b04b2f4def6e7d3bc7a8ad42dad80063f939aab8eb46317c6212f` |

`tests/vectors/signatures/check.py` independently checks these hashes and uses the OpenSSL cryptography backend to verify the published vectors against the Rust test bytes. The runtime's `signature_vectors.rs` and `signature_authority.rs` tests retain these distinctions:

- Wycheproof `tcId` 3 is a valid SHA-256-prehashed secp256k1 signature, tested with compressed and uncompressed SEC1 keys and compact `r || s` signatures. The independent check recomputes the digest and matches the Rust bytes before verifying the DER signature.
- Wycheproof `tcId` 1 is a genuine high-S signature accepted by the reference verifier. The runtime's low-S policy refuses it with `VerificationFailed`; an all-zero signature is not an equivalent high-S vector.
- The pinned secp256k1 compact parser permits zero scalar encodings but guarantees verification failure for them. The runtime tests classify the zero-signature cases as `MalformedSignature`, including recovery IDs 0 through 3.
- RFC 8032 TEST 1 verifies the empty message. Adding the little-endian group order `L` to `S` must fail. The `[0x02; 32]` public key fails curve-point decoding and is classified `MalformedPublicKey`.

Storage-scan evidence lives in `programs/crates/layerx-programs-runtime/tests/storage_scan.rs`. `expected_page` encodes a big-endian u16 entry count, each entry's u16 key length and u32 value length, then cursor presence, u16 cursor length and cursor bytes. Independent arithmetic in the authority check gives 29 bytes for three one-byte key/value entries without a cursor, 101 bytes for two entries plus an 80-byte cursor, and 13 bytes for one terminal entry.

The scan tests check deterministic pagination across insertion orders and activities, complete-page byte limits independently of entry limits, and cursor binding to scope, prefix and limits. `scan_status_guest` places the output sentinel separately from the cursor: with cursor memory starting at 32, a write to memory `64..68` corrupts cursor bytes `32..36`. That malformed-cursor case must return a negative status without overwriting the sentinel or charging successful read bytes. A subsequent guest trap must roll back same-activity storage writes and discard the response; the original valid cursor must still resume correctly. These checks establish storage and metering behavior, not merely a page's output length.

[Home](../index.md) · [Protocol](../protocol/index.md) · [Modules](../protocol/modules.md)
