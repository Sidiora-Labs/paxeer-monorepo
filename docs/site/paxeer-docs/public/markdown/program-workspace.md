<!-- Source: https://docs.paxeer.app/program-workspace/ -->

# Programs Developer Workspace

Build deterministic Paxeer X programs with runtime crates, language SDKs, fixtures, policies and reproducible development workflows.

## Start with the execution contract

The Programs workspace contains the deterministic WebAssembly execution surface for the LayerX kernel domain of Paxeer X. Applications deploy artifacts and interfaces, execute under explicit capabilities, and express monetary effects as authenticated native transfer sets. Read [Programs](https://docs.paxeer.app/programs) before choosing entrypoints, account seeds, and spending grants.

Rust workspace packages use edition 2021 and declare a minimum Rust version of 1.91.1. Cargo resolves dependencies from the checked-in vendor tree in offline mode, and the interpreter is pinned to `wasmi = 0.31.2`. Run Cargo from inside `programs/` so its local configuration applies. Lockfiles, source checksums, and frozen ABI vectors make builds and byte contracts reproducible.

## Choose the right package

| Workspace area | Responsibility |
| --- | --- |
| `programs/crates/layerx-programs-runtime` | Validation, budgets, metering, host imports, storage, cross-program calls, transfer authorization, occupancy, replay, and kernel FFI. |
| `programs/crates/layerx-programs-registry` | Receipt-bound deployment journals, interfaces, program account bindings, account proofs, and lifecycle exits. Its `chain_proof` module verifies committed chain evidence for program-state reads. |
| `programs/crates/layerx-programs-ai-market` | PAXAI schema-1 common contracts and the direct native Programs entrypoint. |
| `programs/crates/layerx-programs-interpreter` | Bounded deterministic scripts with canonical input validation before effects. |
| `programs/crates/layerx-programs-market` | Compute leases, usage claims, challenge windows, provider stake, attesters, and bisection disputes. |
| `programs/crates/layerx-programs-arbiter` | Sandbox trace replay and disputed single-step adjudication. |
| `programs/crates/layerx-programs-sandbox` | Bounded lease lifecycle, isolated execution, snapshots, and restore. |
| `programs/sdk/` and `programs/porting/` | Rust, C, and AssemblyScript guests; migration guides and reference guests for EVM, Solana, and CosmWasm developers. |
| `programs/fixtures/`, `programs/tests/`, `programs/fuzz/`, `programs/benches/` | Canonical artifacts, cross-implementation vectors, hostile guests, fuzz corpora, and interpreter comparisons. |

## Build a guest application

1. Choose a guest SDK and a reference application close to your use case.
2. Define canonical calldata and responses, entrypoint bounds, storage scopes, and required grants. Select the guest ABI that supplies those host imports.
3. Keep state transitions deterministic. Consume admitted oracle or web evidence through host calls rather than adding a network client to execution.
4. Compile a WASM artifact, generate its canonical interface, and retain the artifact hash and interface together.
5. Deploy with the chosen upgrade policy, verify the receipt, register any program-owned accounts, and fund them through native transfers.
6. Simulate a bounded call, submit the signed activity, and verify committed terminal evidence.

Rust examples cover escrow, naming, LXT-721, merchant payments, constant-product swaps, LXT-20, vaults, and web readers. C and AssemblyScript provide paid-counter guests. A reference application is an executable contract: preserve its documented selectors and authorization requirements when adapting it.

```
# From the repository root
cd programs
cargo build --locked --manifest-path sdk/rust/examples/token-lxt20/Cargo.toml \
  --target wasm32-unknown-unknown --release
```

## Build and validate the workspace

The repository Makefile coordinates Rust and native C checks. Run these commands from the repository root; they enter the Programs directory where required.

```
make programs-build
make programs-lint
make programs-abi-drift
make programs-test
```

| Gate | What it establishes |
| --- | --- |
| `programs-build` | Builds the locked workspace and sandbox host FFI used by native integration. |
| `programs-lint` | Checks runtime module boundaries, Clippy, dependency and vendoring policy, advisories, sources, and dependency bans. |
| `programs-abi-drift` | Checks frozen vectors and the runtime linker against the canonical guest import contract. |
| `programs-core-test` | Exercises native registration, lifecycle, calls, monetary invariants, occupancy, fees, accounts, and wind-down. |
| `programs-adversarial` | Checks guest isolation, composition, and monetary-law refusal paths. |
| `programs-fuzz-smoke` | Replays validation, instantiation, and execution corpora. |
| `programs-differential` | Checks deterministic replay and serial versus parallel execution behavior. |
| `programs-interpreter-conformance` | Builds the interpreter WASM and exercises it through the runtime. |
| `programs-test` | Combines protocol, native, guest SDK, porting, adversarial, fuzz, ABI, and differential checks. |

## Preserve ABI compatibility

Frozen guest ABI versions 1 through 5 have independently pinned vector checksums in `programs/abi-frozen.sha256`. The ABI gate compares generated canonical bytes, manifests, validator allowlists, and SDK declarations with those baselines. An existing frozen import contract is immutable; add a new ABI version for a new surface.

Keep the canonical vectors, interface generators and clients aligned with the selected ABI version. Deployment validation, host imports and response decoding use that version to select the same byte contract.

## Maintain runtime boundaries

Host-function families register through their own modules and access execution state through `RuntimeState`. The boundary gate rejects forbidden sibling dependencies, legacy monolithic modules, and production paths into qualification-only helpers. The workspace denies unsafe code by default; explicitly bounded FFI adapters carry the reviewed boundary work.

Dependency policy excludes entropy, clocks, arbitrary networking, dynamic registration, database engines, and alternate execution engines from the deterministic boundary. It verifies vendored checksums, the exact interpreter revision, approved licenses, and integer-only consensus-adjacent code. Workspace linting also rejects unchecked unwraps, expects, and float arithmetic.

## Canonical fixtures

Lifecycle, capability, executed-receipt, and reference-program fixtures keep native encoders and SDKs aligned. Use the check targets when validating existing output; generation targets intentionally write new fixtures.

```
make programs-check-capability-fixture
make programs-check-native-lifecycle-fixtures
make programs-check-executed-v4-fixture

# Rebuild reference artifacts when intentionally changing their source
make programs-reference-fixtures
```

Retain the artifact hash, interface and chosen ABI with each deployed program. Applications verify deployment and execution receipts, including committed block evidence, before relying on a final program outcome.

Continue with [bounded storage scans](https://docs.paxeer.app/storage-scan) and [sandbox execution](https://docs.paxeer.app/sandbox).

**Technical references:** `programs/Cargo.toml`, `programs/.cargo/config.toml`, `Makefile`, `programs/abi-frozen.sha256`, `scripts/programs/dependency-policy.sh`, and `tests/programs/runtime-module-boundaries.sh`.
