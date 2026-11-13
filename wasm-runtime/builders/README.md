# Cross Compilation Scripts

The Go wrapper links native Rust libraries from `internal/api`. This directory
contains Dockerfiles and guest scripts for compiling those libraries for other
platforms.

## Builder images

The [Makefile](Makefile) sets `BUILDERS_PREFIX` to
`cosmwasm/go-ext-builder:0017`. Its `docker-images` target builds three images:

| Suffix | Base image | Rust toolchain | Output |
| --- | --- | --- | --- |
| `-alpine` | `golang:1.20.10-alpine` | `1.73.0` | Linux musl static libraries |
| `-centos7` | `centos:centos7` | `1.73.0` | Linux glibc shared libraries |
| `-cross` | `rust:1.73.0-bullseye` | `1.73.0` | macOS libraries and Windows DLL |

The Alpine image includes Go for testing. Each image installs the scripts from
[`guest`](guest) and the Cargo configuration used for cross-compilation.

## Usage

From `wasm-runtime/`, build the images with Docker:

```sh
(cd builders && make docker-images)
```

The release targets in the parent [Makefile](../Makefile) use a separate
`BUILDERS_PREFIX`, `cosmwasm/libwasmvm-builder:0103`, with `-alpine`, `-debian`
and `-cross` suffixes. Those targets copy the generated native libraries into
`internal/api` and refresh `internal/api/bindings.h`. For example, from
`wasm-runtime/`:

```sh
make release-build
```

`release-build` runs the Alpine, Linux, macOS shared and Windows targets in
sequence. `make release-build-macos-static` builds the macOS static library
separately. The Alpine and macOS static targets invoke
[`package-static-wasm.py`](../../scripts/package-static-wasm.py) to package the
archives and update their manifest.
