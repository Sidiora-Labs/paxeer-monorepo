# Programs porting kits

This directory holds Rust porting kits for selected EVM, Solana/Anchor, and CosmWasm account, storage, message, and value-flow patterns. The kits map those patterns to deterministic LayerX Programs WebAssembly guests and ABI v2 reference programs that execute inside `paxd`. The LayerX kernel inside `paxd` executes and orders each transaction and builds each block; validators check and commit it. A receipt is final when validators commit its block.

## Layout

- [cosmwasm/](cosmwasm/) contains the `layerx-porting-cosmwasm` crate: storage-key framing, JSON message transcoding, monetary mappings, a donation reference, and a shared order book.
- [evm/](evm/) contains the `layerx-porting-evm` crate: storage-slot derivation, selectors and event topics, monetary mappings, a membership-lock reference, and shared token supply.
- [solana/](solana/) contains the `layerx-porting-solana` crate: Anchor account and instruction encoding, seed mappings, monetary mappings, a mint-limit reference, and a shared pool.
- `README.md` is this directory page.

Each kit has a `Cargo.toml`, implementation under `src/`, integration tests under `tests/`, guest context and crypto helpers under `guest/`, a standalone WebAssembly context example under `reference-v2/`, and a `MIGRATION.md` guide.

## Build and test

Run these targets from the repository root; their recipes are in the [root Makefile](../../Makefile).

- `make programs-build` builds the locked Programs workspace, including the porting crates and guest helpers.
- `make programs-porting-v2-references` builds the locked `reference-v2` guests for `wasm32-unknown-unknown` in release mode, checks each guest with `layerx-program-lint`, and runs the `programs_call_activity` reference execution test.
- `make programs-test` runs the Programs workspace test target, which includes `programs-porting-v2-references` and the locked workspace tests.

## Configuration

The porting libraries take terms and authority through their Rust APIs and read no runtime environment variables or configuration files. Cargo uses the [Programs workspace manifest](../Cargo.toml), its [.cargo/config.toml](../.cargo/config.toml), and each kit's `Cargo.toml`. Each `reference-v2/` example has its own `Cargo.toml` and `layerx-program.json` describing the guest ABI and artifact. The Makefile accepts `PROGRAMS_CARGO` and `CARGO_TARGET_DIR` for Cargo selection and build output placement.

## Documentation

- [Porting kits](../../docs/wiki/Porting.md)
- [Programs workspace](../../docs/wiki/ProgramsWorkspace.md)
- [Programs developer workspace](../../docs/site/paxeer-docs/public/markdown/program-workspace.md)
- [Deterministic Programs](../../docs/site/paxeer-docs/public/markdown/programs.md)
- [CosmWasm migration guide](cosmwasm/MIGRATION.md)
- [EVM migration guide](evm/MIGRATION.md)
- [Solana migration guide](solana/MIGRATION.md)
