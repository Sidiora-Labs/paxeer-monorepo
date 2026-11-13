# wasmvm

This is a wrapper around the
[CosmWasm VM](https://github.com/CosmWasm/cosmwasm/tree/main/packages/vm). It
allows you to compile, initialize and execute CosmWasm smart contracts from Go
applications, in particular from [`wasm/x/wasm`](../wasm/x/wasm).

This directory is the Paxeer X Network copy of wasmvm. It is part of the root Go
module, so its import path is `github.com/Sidiora-Labs/Paxeer-X-Network/wasm-runtime`.
The Rust library (`libwasmvm`, crate version 1.5.4) builds against `cosmwasm-vm` from
[Paxeer-Network/pax-cosmwasm](https://github.com/Paxeer-Network/pax-cosmwasm)
(`libwasmvm/Cargo.toml`). Unless noted otherwise, the commands below run from
`wasm-runtime/`.

## Structure

This repo contains both Rust and Go code. The Rust code is compiled into a
library (shared `.dll`/`.dylib`/`.so` or static `.a`) to be linked via cgo and
wrapped with a Go API. The full build step compiles the Rust library and links
it to the Go code. Precompiled libraries in `internal/api` allow Go applications
to import the wrapper without compiling Rust themselves.

### Rust code

The Rust code lives in a sub-folder `./libwasmvm`. This folder compiles to a
library that can be used via FFI. It is compiled like this:

```sh
# Run unit tests
(cd libwasmvm && cargo test)

# Create release build for your current system. Uses whatever default Rust
# version you have installed.
make build-rust

# Create containerized release builds for other systems
make release-build-alpine
make release-build-linux
make release-build-macos
make release-build-windows
```

### Go code

The wrapper's three main Go packages are:

1. The types (the `github.com/Sidiora-Labs/Paxeer-X-Network/wasm-runtime/types` import), using
   `package types`
2. The internal package `internal/api`, using `package api`
3. This directory (the `github.com/Sidiora-Labs/Paxeer-X-Network/wasm-runtime` import), using `package cosmwasm`

The dependencies between them are as follows:

```mermaid
graph TD;
    api-->types;
    cosmwasm-->types;
    cosmwasm-->api;
```

The Go code is built like this:

```sh
make build-go
make test
```

#### Package github.com/Sidiora-Labs/Paxeer-X-Network/wasm-runtime/types

This package contains types used by the two other packages. It can be compiled
without cgo.

```sh
# Build
go build ./types
# Build without CGO
CGO_ENABLED=0 go build ./types
```

#### Package internal/api

This package contains the code binding the libwasmvm build to the Go code. All
low level FFI handling code belongs there. This package can only be built using
cgo. Using the `internal/` convention makes this package fully private.

#### Package github.com/Sidiora-Labs/Paxeer-X-Network/wasm-runtime

This is the package users import. Without cgo, it provides the shared type
aliases and `CreateChecksum`; `LibwasmvmVersion` returns an error. The `VM`
type and contract execution methods require cgo.

```sh
# Build
go build .
# Build without CGO
CGO_ENABLED=0 go build .
```

## Supported Platforms

The Rust implementation of the VM is compiled to a library called libwasmvm.
This is then linked to the Go code when the final binary is built. For that
reason not all systems supported by Go are supported by this project.

The checked-in native libraries cover Linux and macOS. The Makefile also
contains a Windows cross-compilation target.

### Builds of libwasmvm

The native linker files in `internal/api` select these libraries:

| Platform | Architecture | Library | Go build tag |
| --- | --- | --- | --- |
| Linux glibc | x86_64 | `libwasmvm.x86_64.so` | default |
| Linux glibc | aarch64 | `libwasmvm.aarch64.so` | default |
| Linux musl | x86_64 | `libwasmvm_muslc.a` | `muslc` |
| Linux musl | aarch64 | `libwasmvm_muslc.aarch64.a` | `muslc` |
| macOS | x86_64 or aarch64 | `libwasmvm.dylib` | default |
| macOS | x86_64 | `libwasmvmstatic_darwin.amd64.a` | `static_wasm` |
| macOS | aarch64 | `libwasmvmstatic_darwin.arm64.a` | `static_wasm` |

The Windows linker file expects `wasmvm.dll`, which can be built with
`make release-build-windows`.

The libraries are committed in `internal/api`. The static archives are stored in
parts: `libwasmvm_muslc.a` and `libwasmvm_muslc.aarch64.a` are linker `GROUP`
scripts naming their `.partNN.a` members, the macOS static library is stored as one
archive per architecture, and `internal/api/static-archives.json` records the size
and SHA-256 of every archive, part and gzip copy. `make release-build-alpine` and
`make release-build-macos-static` regenerate them through
[`scripts/package-static-wasm.py`](../scripts/package-static-wasm.py), and
[`wasm/staticarchive`](../wasm/staticarchive) checks them. The `sys_wasmvm` build tag
links a system-installed `libwasmvm` instead.

## Docs

Run `(cd libwasmvm && cargo doc --no-deps --open)`.

## Design

Please read the [Documentation](./spec/Index.md) to understand both the general
[Architecture](./spec/Architecture.md), as well as the more detailed
[Specification](./spec/Specification.md) of the parameters and entry points.

## Development

There are two halves to this code - Go and Rust. The first step is to ensure that
there is a proper library built for your platform in `internal/api`:

- `libwasmvm.x86_64.so` or `libwasmvm.aarch64.so` for Linux (glibc) systems
- `libwasmvm.dylib` for macOS
- `wasmvm.dll` for Windows, built separately

If this is present, then `make test` will run the Go test suite and you can
import this code freely. If it is not present, build it for your system with
`make build-rust`, which runs `cargo build --release` in `libwasmvm`, copies the
library into `internal/api` and refreshes `internal/api/bindings.h`. This depends on
`cargo` and `rustc` being installed; `rustup` is the simplest way to get them.
