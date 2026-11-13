# Programs tests

This directory holds shared vectors, test support and runners for the Programs
workspace. They exercise deterministic WebAssembly execution, storage isolation,
monetary conservation, AI market aggregation and committed-state proofs for the
Programs runtime and kernel within the Paxeer network.

## Layout

- [ai-market/](ai-market/) contains `run-aggregation-runtime.sh`, which runs the
  AI market aggregation runtime scenarios and checks each required result.
- [conservation/](conservation/) contains `run.sh`, which runs Go and Rust checks
  for program balances, fees, leases, market escrow and swap reserves.
- [differential/](differential/) contains `parallel.c`, a native kernel harness
  comparing serial and parallel execution receipts, events and state roots,
  including retry and refusal cases.
- [gauntlet/](gauntlet/) contains the hostile-action inventory
  `attack-inventory.tsv` and shared-storage isolation checks in `state.rs`, used
  by the runtime isolation and composition suites.
- [vectors/](vectors/) contains frozen guest ABI bytes, calldata cases, ICS23
  proof vectors, snapshot bindings and terminal outcome encodings.
- [repair_22_33_238.rs](repair_22_33_238.rs) tests registry membership and
  non-membership proofs and the corresponding workspace documentation contract.

## Build and test

The directory uses the workspace manifest [../Cargo.toml](../Cargo.toml) and the
repository [Makefile](../../Makefile). Run these targets from the repository root:

| Command | Coverage |
| --- | --- |
| `make programs-build` | Builds the locked Rust workspace and sandbox host FFI used by native tests. |
| `make programs-test` | Runs the aggregate Programs tests, including workspace integration tests, SDK checks and native tests. |
| `make programs-adversarial` | Runs runtime isolation, composition and monetary-law tests, including gauntlet support. |
| `make programs-conservation` | Runs the Go and Rust conservation suites through `conservation/run.sh`. |
| `make programs-differential` | Runs runtime replay and determinism tests and builds and runs the C parallel differential harness. |
| `make programs-abi-drift` | Checks frozen ABI vectors and the runtime ABI linker. |

Run the aggregation scenarios from the repository root:

```sh
programs/tests/ai-market/run-aggregation-runtime.sh
```

The [calldata vector guide](vectors/calldata/README.md) defines the vector format
and its focused test command. Run that test from `programs/`:

```sh
cargo test --locked -p layerx-programs-runtime --test calldata_golden_vectors
```

The workspace declares Rust `1.91.1` as its minimum version. Run Cargo from inside
`programs/` so its vendored, offline dependency configuration applies. The
aggregate test target also builds WebAssembly guests and runs the C and
AssemblyScript SDK checks; see the [workspace page](../README.md) for their
toolchain requirements.

## Configuration

- `PROGRAMS_CARGO` selects the Cargo executable used by the aggregation and
  conservation scripts and the Programs Makefile targets.
- [../.cargo/config.toml](../.cargo/config.toml) configures vendored dependencies,
  offline dependency resolution and WebAssembly compiler flags.

## Documentation

- [Programs workspace](../README.md)
- [Programs workspace reference](../../docs/wiki/ProgramsWorkspace.md)
- [Programs execution model](../../docs/wiki/Programs.md)
- [Qualification guide](../../docs/QUALIFICATION.md)
- [Programs Developer Workspace on the documentation site](../../docs/site/paxeer-docs/public/markdown/program-workspace.md)
- [Calldata vector format](vectors/calldata/README.md)
