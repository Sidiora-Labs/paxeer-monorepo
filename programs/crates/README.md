# Programs crates

This directory holds the Rust runtime, registry, guest programs, replay arbiter,
and sandbox support in the [Programs Cargo workspace](../Cargo.toml). They provide
deterministic WebAssembly execution and authenticated program state for the
Programs kernel module inside `paxd`, on the merged Paxeer network. Program
monetary effects use `402LXP` transfer sets, and receipts become final when their
block is committed.

## Layout

- [layerx-programs-ai-market/](layerx-programs-ai-market/) — PAXAI guest with market registration, workers, evaluators, commit/reveal epochs, aggregation, reputation, and rewards.
- [layerx-programs-arbiter/](layerx-programs-arbiter/) — Authenticated sandbox replay, chain-commit proof verification, and disputed market-step adjudication.
- [layerx-programs-interpreter/](layerx-programs-interpreter/) — Bounded scripting guest that validates canonical scripts before staging storage or transfer effects.
- [layerx-programs-market/](layerx-programs-market/) — Compute offers and leases, usage claims, challenge windows, provider stake, attestations, and disputes.
- [layerx-programs-registry/](layerx-programs-registry/) — Deployment provenance, executable admission, interfaces, chain proofs, program value accounts, and wind-down evidence.
- [layerx-programs-runtime/](layerx-programs-runtime/) — WASM validation, metering, capability checks, storage, cross-program calls, transfer authorization, occupancy accounting, and native kernel FFI.
- [layerx-programs-sandbox/](layerx-programs-sandbox/) — Bounded ephemeral leases, isolated namespaces, execution, escrow, usage, snapshots, restoration, and expiry, with native host FFI.

Each crate has a `Cargo.toml` manifest, `src/` sources, and `tests/` integration
tests. The workspace manifest and lockfile live in the parent directory.

## Build and test

Run these targets from the repository root; the [root Makefile](../../Makefile)
enters `programs/` for Cargo commands:

```sh
make programs-build
make programs-test
```

`programs-build` builds the locked workspace with the sandbox's `host-ffi`
feature. `programs-test` runs native and protocol tests, ABI checks, adversarial
and differential checks, fuzz corpus replay, interpreter conformance, SDK and
porting tests, and Cargo workspace tests.

The same Makefile provides focused checks and AI guest packaging:

```sh
make programs-lint
make programs-abi-drift
make programs-interpreter-conformance
make paxai-host-boundary-build
```

The workspace declares Rust `1.91.1`. Guest builds require the
`wasm32-unknown-unknown` target. AI guest packaging uses
[guest-recipe.sh](layerx-programs-ai-market/tests/fixtures/guest-recipe.sh),
release optimization level `z`, and Binaryen `133`. The host-boundary target
checks the packaged guest size and executes it through the native harness.

## Configuration

Cargo reads [../Cargo.toml](../Cargo.toml), [../Cargo.lock](../Cargo.lock),
the crate manifests, and [../.cargo/config.toml](../.cargo/config.toml).
Run Cargo inside `programs/` or its descendants so the offline vendored-source
configuration and WASM compiler flags apply.

- `PAXAI_CHAIN_DOMAIN` — Compile-time AI guest chain-domain setting, validated against the merged chain identity.
- `PAXAI_AGGREGATION_PROBE` — Compile-time instrumentation used by the AI aggregation probe guest.
- `PROGRAMS_CARGO`, `CARGO_TARGET_DIR` — Cargo executable and build-output selection used by the AI guest recipe.
- `PAXAI_HOST_BOUNDARY_MANIFEST` — Native AI host-boundary artifact manifest read by integration tests.
- `LAYERX_INTERPRETER_WASM` — Compiled interpreter guest used by runtime conformance tests.
- `PAXEER_X_DETERMINISM_VECTOR_DIR` — Runtime determinism vector input directory.
- `PAXEER_X_SPEND_GUESTS`, `PAXEER_X_SPEND_RESULTS` — Program-spending composition artifacts and results.
- `PAXEER_X_INTERFACE_INPUTS` — Registry interface test input directory.
- `LAYERX_AUTHENTICATED_REPLAY_INPUTS`, `LAYERX_ARBITER_ADMISSION_INPUTS`, `LAYERX_MARKET_SANDBOX_INPUTS` — Arbiter replay, admission, and market-sandbox test inputs.

## Documentation

- [Programs workspace overview](../README.md)
- [Programs execution and activity reference](../../docs/wiki/Programs.md)
- [Workspace builds, ABI contracts, and crate guide](../../docs/wiki/ProgramsWorkspace.md)
- [Sandbox lifecycle and accounting](../../docs/wiki/Sandbox.md)
- [Documentation site: Programs](../../docs/site/paxeer-docs/src/app/programs/page.tsx)
- [Documentation site: Programs Developer Workspace](../../docs/site/paxeer-docs/src/app/program-workspace/page.tsx)
- [Documentation site: Sandbox](../../docs/site/paxeer-docs/src/app/sandbox/page.tsx)
