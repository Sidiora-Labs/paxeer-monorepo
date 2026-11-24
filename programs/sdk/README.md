# Program SDKs

This directory holds Rust, C, and AssemblyScript guest bindings, deterministic build tools, examples, and shared ABI fixtures for WebAssembly programs. Programs run in the LayerX kernel inside `paxd`, using explicit capabilities for storage, events, program calls, receipt reads, and authenticated 402LXP transfers. The LayerX kernel executes and orders each transaction and builds each block; validators check and commit it. The SDKs provide bounded inputs and exact integer amounts for those host interactions; the kernel applies monetary effects.

## Layout

- [rust/](rust/) — the `layerx-program-sdk` crate, typed binding generation, determinism lint, quickstart, reference programs, and conformance tests.
- [c/](c/) — freestanding C headers and implementation, determinism lint, tests, and a paid-counter example.
- [assemblyscript/](assemblyscript/) — the `@sidiora/layerx-program-sdk` package, compiler configuration, determinism lint, tests, and a paid-counter example.
- [vectors/](vectors/) — shared capability encoding limits and boundary vectors.
- [abi-manifest.tsv](abi-manifest.tsv) — versioned host import names and signatures used by the C determinism lint; the AssemblyScript package carries its own [ABI manifest](assemblyscript/abi-manifest.tsv).

## Build and test

Run these targets from the repository root. They are declared in the root [Makefile](../../Makefile).

| Command | Builds and checks |
| --- | --- |
| `make programs-js-install` | Installs the AssemblyScript SDK and paid-counter compiler dependencies. |
| `make programs-sdk-rust` | Builds and lints the Rust quickstart, response fixture, escrow, and vault; runs the generated binding compiler conformance test. |
| `make programs-sdk-c` | Builds and lints the C paid-counter with strict warnings; builds and runs the capability parity test. |
| `make programs-sdk-assemblyscript` | Checks the shared capability fixture, compiles the SDK safety tests, and builds and lints the AssemblyScript paid-counter. |

Rust guest builds use Cargo and the `wasm32-unknown-unknown` target. C builds use Clang for WebAssembly and a host C compiler for the lint and parity test. AssemblyScript builds use Node.js, npm, and the compiler declared in [package.json](assemblyscript/package.json). The language requirements are recorded in each language directory's `toolchain.json`.

The AssemblyScript package also declares these focused checks, run from the repository root:

```sh
npm --prefix programs/sdk/assemblyscript run lint
npm --prefix programs/sdk/assemblyscript run test:package
```

The first checks SDK sources against the selected ABI import surface. The second tests the packaged SDK's bundled manifest with source and WebAssembly inputs.

## Configuration

The [Rust build script](rust/build.rs) reads `LAYERX_INTERFACE_PATH`, `LAYERX_INTERFACE_DIGEST`, and `LAYERX_PROGRAM_CODE_HASH` together to generate bindings from a canonical interface after checking its digest and program code hash. `LAYERX_BINDINGS_DIR` selects the output directory; otherwise the script uses Cargo's `OUT_DIR`.

Rust example build scripts read `CARGO` and `CARGO_TARGET_DIR`. The [C example build script](c/examples/paid-counter/build.sh) reads `CLANG`, `HOSTCC`, and `STRICT`.

- [rust/Cargo.toml](rust/Cargo.toml) and [rust/lint/Cargo.toml](rust/lint/Cargo.toml) define the SDK and lint crates within the [programs workspace](../Cargo.toml).
- The Rust lint reads the project's `layerx-program.json`, including its `abi_version`; the [quickstart manifest](rust/quickstart/layerx-program.json) provides an example.
- [assemblyscript/asconfig.json](assemblyscript/asconfig.json) configures deterministic compilation; the [paid-counter configuration](assemblyscript/examples/paid-counter/asconfig.json) extends it.
- The C and AssemblyScript lints read their ABI manifests. Their `--abi-version` and `--abi-manifest` options select the ABI surface and manifest file.
- [rust/toolchain.json](rust/toolchain.json), [c/toolchain.json](c/toolchain.json), and [assemblyscript/toolchain.json](assemblyscript/toolchain.json) describe language tools, templates, and determinism checks.

## Documentation

- [Programs guide](../../docs/platform/content/guide/programs.md) — program authority, settlement, SDK references, and deployment commands.
- [Documentation site: deterministic programs](../../docs/site/paxeer-docs/src/app/programs/page.tsx) — program capabilities, lifecycle, and guest interfaces.
- [Documentation site: program workspace](../../docs/site/paxeer-docs/src/app/program-workspace/page.tsx) — building, deploying, and inspecting programs.
