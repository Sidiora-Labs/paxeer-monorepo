# Platform tools

This Cargo workspace holds the `layerx` developer CLI, its local protocol emulator
library, and release tooling for Paxeer X Network. The CLI provides wallet, token,
program, tenant-domain and agent integration commands. The emulator wraps the
native core for local state inspection, account prefunding, time controls and
snapshots. Network execution runs through the merged `paxd` node; a receipt is
final once its block is committed. The release tooling checks registry metadata,
release workflows and published artifact manifests.

## Layout

- [cli/](cli/) — the `layerx-platform-cli` package, `layerx` executable, examples and integration tests.
- [emulator/](emulator/) — the `layerx-platform-emulator` library, native core bridge, build script and tests.
- [release/](release/) — the `layerx-platform-release` package, registry catalogue, artifact verification and tests.
- [Cargo.toml](Cargo.toml) — workspace members, shared dependencies, Rust requirements and lint settings.
- [Cargo.lock](Cargo.lock) — pinned workspace dependency resolution.
- [clippy.toml](clippy.toml) — Clippy configuration.
- [deny.toml](deny.toml) — dependency advisory, license, source and package policies.
- [.gitignore](.gitignore) — workspace output exclusions.

## Build and test

Run these commands from the repository root. The recipes are declared in
[services/platform/Makefile.inc](../../services/platform/Makefile.inc), included
by the root [Makefile](../../Makefile).

Build just this workspace with the Cargo invocation used by `platform-build`:

```sh
cargo build --offline --manifest-path tools/platform/Cargo.toml --locked --workspace
```

The workspace declares Rust `1.91.1` as its minimum version. Offline builds need
the locked dependencies available locally. The emulator build script invokes
`make build/liblayerx.a`, compiles its C bridge and links the native core with
OpenSSL, SQLite and pthreads. The CLI executable is written under
`tools/platform/target/debug/` unless Cargo's target directory is overridden.

The root targets provide the build and test entry points:

```sh
make platform-build
make platform-test-tooling
make platform-release-test
make platform-release-check
```

`platform-build` builds the platform services, this workspace and SDK generators,
then checks generated SDKs and the release registry catalogue.
`platform-test-tooling` tests the CLI and emulator along with the hosted faucet
and testnet tooling; it also exercises CLI bootstrap and credential-store
selection. Its prerequisites prepare the public TLS test boundary, and its CLI
tests enable the `test-credential-store` feature. `platform-release-test` runs
the release package tests through the repository clock wrapper.
`platform-release-check` checks the registry catalogue and release workflow.

## Configuration

The CLI reads JSON profiles through [cli/src/config.rs](cli/src/config.rs).
`LAYERX_CONFIG` selects the file; otherwise its location is determined by
`XDG_CONFIG_HOME` or `HOME`, using `layerx/config.json` or
`.config/layerx/config.json`, respectively. Profiles hold the selected environment,
endpoint, network identity, receipt trust anchor and public key metadata.

Other environment inputs read by the tools are:

- `LAYERX_CREDENTIAL_STORE` and `LAYERX_CREDENTIAL_PASSPHRASE` — credential storage selection and encrypted file storage.
- `LAYERX_CLIENT_CERTIFICATE_FILE` and `LAYERX_CLIENT_PRIVATE_KEY_FILE` — client TLS material.
- `LAYERX_EVM_KEY_FILE` — chain creation signing input.
- `LAYERX_GATEWAY_RPC_URL` and `LAYERX_GATEWAY_KEY_ID` — hosted tool transport and gateway key selection.
- `LAYERX_BETA_REGISTRY_URL`, `LAYERX_BETA_REGISTRY_PUBLICATION_TOKEN_FILE` and `LAYERX_BETA_REGISTRY_PUBLICATION_KEY_FILE` — program registry publication inputs.
- `LAYERX_PROGRAM_SDK` and `CARGO` — program scaffolding and build tool selection.
- `LAYERX_INSTALL_ROOT` — agent configuration installation root.
- `LAYERX_REPO_ROOT`, `PATH`, `PATHEXT` and `NO_COLOR` — workspace discovery, executable lookup and terminal output.
- `CC` and `AR` — native emulator compiler and archiver; Cargo supplies `CARGO_MANIFEST_DIR` and `OUT_DIR` to its build script.

Program commands read project `Cargo.toml`, `LayerX.toml` and
`layerx-program.json` files. Agent installation reads and updates the selected
client's `mcp.json`, `.claude.json` or `claude_desktop_config.json` configuration.
Release commands read [release/registries.kvx](release/registries.kvx) and
[the platform workflow](../../.github/workflows/platform.yml); artifact
verification reads `artifact-manifest.json`.

## Documentation

- [CLI package guide](cli/README.md)
- [CLI command reference](../../docs/system/cli.md)
- [Developer documentation](../../docs/platform/README.md)
- [Installation guide](../../docs/platform/content/install.md)
- [Emulator guide](../../docs/platform/content/environments/emulator.md)
- [Documentation site: CLI](../../docs/site/paxeer-docs/src/app/cli/page.tsx)
- [Documentation site: emulator](../../docs/site/paxeer-docs/src/app/emulator/page.tsx)
