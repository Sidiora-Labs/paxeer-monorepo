<!-- Source: https://docs.paxeer.app/wasm-runtime/ -->

# WASM Runtime

Understand Paxeer’s in-tree CosmWasm virtual machine, Go and Rust boundary, native library builds, and gas accounting.

**Source:** `Paxeer-X-Network/wasm-runtime/`, especially `lib.go`, `internal/api/`, and `libwasmvm/`.

## Runtime responsibilities

The in-tree CosmWasm runtime connects the Go node to a Rust virtual machine through a C interface. It stores and compiles WebAssembly modules, caches them, and executes contract entry points with host callbacks for storage, addresses, queries, and gas.

A contract developer compiles Rust into a `.wasm` artifact separately. The runtime's `StoreCode` compiles that WebAssembly into its execution representation; it does not accept Rust source.

## Package layout

| Path | Responsibility |
| --- | --- |
| `lib.go` | CGO-enabled `VM`, cache lifetime, code storage, and contract entry points. |
| `lib_no_cgo.go` | Shared types, Wasm checksum creation, and version API available without a VM execution build. |
| `internal/api/` | C bindings, callbacks, memory ownership, iterators, and platform linker settings. |
| `libwasmvm/` | Rust library and generated C header. |
| `types/` | Environment, messages, query results, gas, storage, and IBC types shared across the boundary. |
| `builders/` | Containerized native-library build definitions. |
| `testdata/` | Compiled contracts used by runtime tests. |

The node imports `github.com/Sidiora-Labs/Paxeer-X-Network/wasm-runtime`. Use the version in the Paxeer tree with its matching native library and generated bindings.

## CGO and native libraries

`lib.go` is guarded by the `cgo` build tag. Without CGO, checksum and shared-type utilities remain available, but the VM execution API is not compiled. `LibwasmvmVersion` explicitly returns an unavailable error when CGO is disabled.

Linker files cover Linux glibc, Linux musl, macOS, Windows, and a system-library mode. Install the native library matching the node's operating system and architecture.

| Build path | Relevant files or targets |
| --- | --- |
| Linux glibc | `link_glibclinux_x86_64.go`, `link_glibclinux_aarch64.go`, `release-build-linux` |
| Linux musl | `link_muslc.go`, `release-build-alpine` |
| macOS | `link_mac.go`, `link_mac_static.go`, macOS release targets |
| Windows | `link_windows.go`, `release-build-windows` |
| System library | `link_system.go` and its `sys_wasmvm` build selection |

## Building the runtime

Use the node's [installation guide](https://docs.paxeer.app/installation) to build Paxeer. When changing the runtime itself, its local Makefile has separate Rust and Go targets. Run these from the repository root:

```
# Build the native release library and refresh the generated C bindings.
make -C wasm-runtime build-rust-release

# Build the Go packages and demo against the native library.
make -C wasm-runtime build-go

# Run the runtime's Go test suite with the actual native library.
make -C wasm-runtime test
```

The Rust target copies the compiled library into `internal/api/` and updates `bindings.h`. It requires a working Rust and C toolchain. Containerized release targets build specific distribution artifacts; choose the target matching the node you will run.

## VM lifecycle and caching

`NewVM` creates a cache using a data directory, supported capabilities, a per-execution memory limit, a debug-output setting, and a cache size. Call `Cleanup` when releasing a VM. `StoreCode` returns the checksum used for later instantiation and execution; `StoreCodeUnchecked` does the same while skipping static validation checks, so prefer `StoreCode` for untrusted bytecode. `Pin` and `Unpin` control retention in the memory cache.

Code upload, contract instantiation, storage updates, and message dispatch are coordinated by the [CosmWasm module](https://docs.paxeer.app/wasm). Upload or instantiation permissions are module parameters, not a statement that the VM implementation is absent.

## Execution and gas

The VM receives an environment, contract message bytes, storage and query callbacks, and gas limits. Its result carries contract data, emitted messages, and execution gas information back to the module. Host work also consumes gas through the supplied meter.

When an EVM contract calls the wasmd precompile, the adapter maps the supplied EVM gas into the native execution meter and reports remaining gas. A read-only call made from another on-chain contract still consumes execution gas. Measure the complete call with its real payload and state instead of assuming a fixed gas cost for all CosmWasm operations.

## Runtime integration

[WASM bindings](https://docs.paxeer.app/wasmbinding) add Paxeer-specific query and message handlers. [EVM precompiles](https://docs.paxeer.app/precompiles) expose selected runtime and native-module operations to Solidity. Their ABI and execution-context checks are part of the integration contract.
