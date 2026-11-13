# Paxeer command-line tools

`cmd` holds the C genesis and module-registry tools, the Rust `layerxctl` client,
and Python checks for retired command components. The tools prepare kernel
bootstrap artifacts and submit transactions or read state through JSON-RPC for
the Paxeer network. The network node is `paxd`, which embeds the kernel and
proposes blocks; a receipt is final when its block commits.

## Layout

- [layerx-genesis/](layerx-genesis/) — genesis manifest and snapshot construction, bootstrap acceptance, import and reconciliation routines, migration operations, and CLI tests.
- [layerx-module-registry/](layerx-module-registry/) — the C `generate` command, which emits asset metadata and module activity ordinals as JSON from the canonical module table, with Python tests.
- [layerxctl/](layerxctl/) — a standalone Rust crate for the `read-state` and `submit` commands, with its manifest, dependency lockfile, and integration tests.
- [retirement/](retirement/) — Python tests that check retired command sources and their build references remain absent.

## Build and test

Run commands from the repository root. The C tools use the root
[Makefile](../Makefile), the kernel library, and the Programs runtime. Their build
requires a C17 compiler, Make, OpenSSL, and the Rust toolchain declared in
[rust-toolchain.toml](../rust-toolchain.toml). Python suites use `pytest`.

| Command | Purpose |
| --- | --- |
| `make layerx-genesis-build` | Build the genesis tool under the selected build directory. |
| `make layerx-module-registry` | Build the module-registry tool under the selected build directory. |
| `make build` | Build the kernel library and both C tools. |
| `make test-genesis-bootstrap` | Test manifest acceptance, the module table, and the genesis builder CLI. |
| `make test-genesis-import` | Test genesis import routines. |
| `make test-genesis-reconcile` | Test genesis reconciliation routines. |
| `make test-module-registry` | Test registry generation and the hosted deployment's retained material. |
| `make test-python-suites` | Build the C tools and run the Python suite selection, including their CLI tests. |

The Rust client has its own [Cargo manifest](layerxctl/Cargo.toml):

```sh
cargo build --locked --release --manifest-path cmd/layerxctl/Cargo.toml
cargo test --locked --manifest-path cmd/layerxctl/Cargo.toml
```

The client's integration tests start `anvil`, which must be available on `PATH`.
The [retirement tests](retirement/) are Python test sources and need no separate
build; run them with the repository's Python test runner:

```sh
python3 -m pytest -q cmd/retirement
```

## Configuration

The command sources take their runtime settings from arguments. The genesis
builder reads a request file and signer file from positional arguments and writes
`genesis.manifest`, a snapshot, `paxeer-registration-request.lxrr`, and
`paxeer-deployment-descriptor.lxgd`. Its `--migrate-asset-v2` operation reads an
asset record and salt file; `--migrate-snapshot-issuance` reads a snapshot,
genesis manifest, and signer file.

The registry's `generate` command accepts `--asset`, `--symbol`, `--currency`,
`--decimals`, `--network-id`, `--protocol-version`, and `--enable-module`.

`layerxctl` accepts `--rpc-url` and `--ca-cert`; HTTPS requires the supplied DER
root certificate. `submit` reads the file selected by `--transaction`.
`read-state` takes `--to`, `--data`, and optionally `--block-height`; without a
height it reads pending state. Submission reports `receipted` after receipt
verification; establishing block finality requires block inclusion and consensus
evidence.

For builds, the Makefile supports `BUILD_DIR`, `CC`, `OPT_LEVEL`, `EXTRA_CFLAGS`,
and `EXTRA_LDFLAGS`. Python CLI tests read `LAYERX_TEST_BUILD_DIR`, and the module
table conformance tests also read `CC`.

## Documentation

- [Human owner registration](../docs/wiki/HumanOwnerRegistration.md) describes transaction submission and state reads with `layerxctl`.
- [SDK terminal verification](../docs/wiki/SdkTerminalVerification.md) covers block inclusion and consensus evidence.
- [Release build matrix](../docs/release/7.0.0-readiness-matrix.md) lists component build entry points.
- [Installation and build guide](../docs/site/paxeer-docs/public/markdown/installation.md) documents the kernel tools within the network build.
- [Installation site page](../docs/site/paxeer-docs/src/app/installation/page.tsx) presents the build guide on the documentation site.
