# wasm-runtime

This directory holds Paxeer's Go wrapper around the CosmWasm VM, the Rust native
library and their shared contract types. It compiles WebAssembly bytecode,
initializes contracts and executes their entry points for the node's
[`wasm/x/wasm`](../wasm/x/wasm) module. CosmWasm, the EVM and the kernel run within
the same Paxeer network.

## Layout

| Path | Contents |
| --- | --- |
| [`builders/`](builders) | Docker build definitions and guest scripts for native libraries. |
| [`cmd/`](cmd) | Go demo that loads contract bytecode through the wrapper. |
| [`devtools/`](devtools) | Script for updating the native library version. |
| [`internal/`](internal) | Private cgo API, host callbacks, memory and iterator handling, native libraries and linker settings. |
| [`libwasmvm/`](libwasmvm) | Rust FFI library, Cargo manifest and lockfile, generated C header and vendored cryptography crate. |
| [`spec/`](spec) | Architecture and specification of the wrapper's parameters and entry points. |
| [`testdata/`](testdata) | Compiled contracts used by the Go tests and their download script. |
| [`types/`](types) | Shared environment, message, response, query, storage, gas and IBC types. |
| [`Makefile`](Makefile) | Rust and Go builds, tests, binding updates and containerized release targets. |
| [`README.md`](README.md) | Component layout, build, configuration and documentation guide. |
| [`lib.go`](lib.go) | cgo-enabled `VM`, code storage, contract execution and IBC entry points. |
| [`lib_no_cgo.go`](lib_no_cgo.go) | Shared type aliases, `CreateChecksum` and the `LibwasmvmVersion` API. |
| [`version_cgo.go`](version_cgo.go) | Native library version lookup with cgo. |
| [`version_no_cgo.go`](version_no_cgo.go) | Version lookup error when cgo is disabled. |
| [`lib_test.go`](lib_test.go) | VM lifecycle, execution and gas tests. |
| [`lib_no_cgo_test.go`](lib_no_cgo_test.go) | Checksum tests. |
| [`ibc_test.go`](ibc_test.go) | Contract IBC callback tests. |
| [`libwasmvm-linux.sha256`](libwasmvm-linux.sha256) | Checksums for Linux native libraries. |

The wrapper belongs to the [root Go module](../go.mod). Its import path is
`github.com/Sidiora-Labs/Paxeer-X-Network/wasm-runtime` and its package name is
`cosmwasm`. The `types` package can compile without cgo. Both `cosmwasm` and
`internal/api` use `types`; `cosmwasm` also calls `internal/api`. The latter is
private under Go's `internal/` convention and requires cgo.

The Rust crate is version 1.5.4 in
[`libwasmvm/Cargo.toml`](libwasmvm/Cargo.toml). Its `cosmwasm-std` and
`cosmwasm-vm` dependencies use the pinned `Paxeer-Network/pax-cosmwasm` fork.

## Build and test

Run these commands from `wasm-runtime/`. The [Makefile](Makefile) builds Rust
into a native library, links it through cgo and builds the Go packages and demo.
Go builds can use the native libraries already present in `internal/api`.

```sh
# Build the Rust release library, refresh bindings and build Go packages.
make build

# Build only the native release library and refresh internal/api/bindings.h.
make build-rust

# Build the Go packages and demo against the native library.
make build-go

# Run the Go test suite with caching disabled.
make test

# Run the Rust unit tests declared by the Cargo crate.
(cd libwasmvm && cargo test)
```

`make build-rust` runs `cargo build --release` in `libwasmvm`, copies the library
into `internal/api` and refreshes the C header through `make update-bindings`.
It requires Cargo, rustc and a C toolchain; Rust can be installed with rustup.
`make build-rust-debug` selects the Rust debug build. If a matching native
library is already present, `make test` can run without rebuilding Rust.

The public Go package and `types` also support builds without cgo:

```sh
go build ./types
CGO_ENABLED=0 go build ./types
go build .
CGO_ENABLED=0 go build .
```

Without cgo, shared type aliases and `CreateChecksum` remain available, and
`LibwasmvmVersion` returns an error. The `VM` type and contract execution methods
require cgo.

### Native libraries and platforms

The Rust VM is linked into the final Go binary as a shared or static library,
so platform support also depends on the native library. The libraries present
in `internal/api` cover Linux and macOS. The linker files select:

| Platform | Architecture | Library | Go build tag |
| --- | --- | --- | --- |
| Linux glibc | x86_64 | `libwasmvm.x86_64.so` | default |
| Linux glibc | aarch64 | `libwasmvm.aarch64.so` | default |
| Linux musl | x86_64 | `libwasmvm_muslc.a` | `muslc` |
| Linux musl | aarch64 | `libwasmvm_muslc.aarch64.a` | `muslc` |
| macOS | x86_64 or aarch64 | `libwasmvm.dylib` | default |
| macOS | x86_64 | `libwasmvmstatic_darwin.amd64.a` | `static_wasm` |
| macOS | aarch64 | `libwasmvmstatic_darwin.arm64.a` | `static_wasm` |

The Windows linker expects `wasmvm.dll`. The `sys_wasmvm` build tag selects a
system-installed `libwasmvm`. The Makefile provides these containerized release
targets for the native libraries:

```sh
make release-build-alpine
make release-build-linux
make release-build-macos
make release-build-macos-static
make release-build-windows
```

The musl static archives are stored in parts: `libwasmvm_muslc.a` and
`libwasmvm_muslc.aarch64.a` are linker `GROUP` scripts naming their `.partNN.a`
members. The macOS static libraries are stored as one archive per architecture.
[`internal/api/static-archives.json`](internal/api/static-archives.json) records
the size and SHA-256 of every archive, part and gzip copy.
`make release-build-alpine` and `make release-build-macos-static` regenerate them
through [`scripts/package-static-wasm.py`](../scripts/package-static-wasm.py).
[`wasm/staticarchive`](../wasm/staticarchive) checks these artifacts.

## Configuration

The embedding application supplies `NewVM` with `dataDir`,
`supportedCapabilities`, `memoryLimit`, `printDebug` and `cacheSize`. These
control the bytecode and cache directory, supported capabilities, execution
memory, contract debug output and memory cache size. Runtime settings are
passed through this API; the wrapper does not read an application configuration
file or environment variables for them. Node configuration is described in the
[Wasm module guide](../wasm/x/wasm/README.md).

Build and test tooling uses the following names:

| Names | Use |
| --- | --- |
| `CARGO_MANIFEST_DIR` | Read by `libwasmvm/build.rs` to locate the crate for C binding generation. |
| `CARGO_PKG_VERSION` | Read at compile time by `libwasmvm/src/version.rs`. |
| `CGO_ENABLED` | Go build selection for native execution support. |
| `OS` | Platform selection in the Makefile. |
| `RUST_BACKTRACE`, `GODEBUG` | Diagnostics in the Makefile's Go test targets. |
| `CARGO_REGISTRIES_CRATES_IO_PROTOCOL` | Cargo registry setting exported by the guest build scripts. |
| `CC`, `CXX`, `PATH`, `LIBZ_SYS_STATIC` | Compiler, tool lookup and static zlib settings in guest builds. |
| `CC_aarch64_unknown_linux_gnu`, `AR_aarch64_unknown_linux_gnu`, `CFLAGS_aarch64_unknown_linux_gnu` | Compiler, archiver and compiler flags in the GNU aarch64 guest build. |
| `CARGO_TARGET_AARCH64_UNKNOWN_LINUX_GNU_LINKER`, `CARGO_TARGET_AARCH64_UNKNOWN_LINUX_GNU_RUNNER`, `qemu_aarch64` | Linker and emulator selection in the GNU aarch64 guest build. |

[`libwasmvm/Cargo.toml`](libwasmvm/Cargo.toml) declares dependencies, features
and build profiles; [`libwasmvm/Cargo.lock`](libwasmvm/Cargo.lock) pins resolved
dependencies. [`libwasmvm/cbindgen.toml`](libwasmvm/cbindgen.toml) configures C
header generation and [`libwasmvm/clippy.toml`](libwasmvm/clippy.toml) configures
Clippy. The builder images install
[`builders/guest/cargo-config`](builders/guest/cargo-config) as Cargo's
cross-compilation configuration.

## Documentation

- [Integration overview](spec/Index.md), [architecture](spec/Architecture.md) and
  [parameter and entry-point specification](spec/Specification.md).
- [Wasm module](../wasm/x/wasm/README.md): node integration and configuration.
- [Release readiness matrix](../docs/release/7.0.0-readiness-matrix.md): runtime
  build and test entries.
- [Validator runbook](../docs/release/7.0.0-validator-runbook.md): native library
  requirements for node releases.
- [WASM Runtime site page](../docs/site/paxeer-docs/src/app/wasm-runtime/page.tsx)
  and its [Markdown page](../docs/site/paxeer-docs/public/markdown/wasm-runtime.md).
- [WASM module site page](../docs/site/paxeer-docs/public/markdown/wasm.md).

Generate and open the Rust API documentation from this directory with:

```sh
(cd libwasmvm && cargo doc --no-deps --open)
```
