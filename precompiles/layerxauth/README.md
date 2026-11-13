# LayerX authority precompile

This directory holds the Go EVM precompile for native account authority in the Paxeer network. It connects Solidity ABI calls to the application’s LayerX auth keeper for identity lifecycle updates, session and capability grants, revocation, and authority queries, and emits the corresponding EVM logs. The node supplies the keeper and activates the registered constructor through the merge upgrade.

## Layout

- [layerxauth.go](layerxauth.go) — precompile registration, keeper wiring, caller checks, and method dispatch.
- [abi.json](abi.json) — function and event ABI for callers and log consumers.
- [abi.go](abi.go) — embeds and parses the ABI.
- [grants.go](grants.go) — session, successor session, and capability grant handlers and receipt event encoding.
- [lifecycle.go](lifecycle.go) — identity lifecycle updates and their events.
- [revoke.go](revoke.go) — grant revocation and EVM log emission.
- [views.go](views.go) — grant, liveness, session key, capability scope, and activity queries.
- [errors.go](errors.go) — canonical LXP revert reasons with the underlying error identity preserved.
- [gas.go](gas.go) — method gas table.
- [internal/](internal/) — native governance and fee admission helpers used by integration tests.
- [harness_test.go](harness_test.go) — shared application keeper and EVM test harness.
- [abi_test.go](abi_test.go) — ABI signatures and event encoding tests.
- [activation_test.go](activation_test.go) — merge activation and constructor tests.
- [client_test.go](client_test.go) — client ABI calls and event decoding tests.
- [layerxauth_test.go](layerxauth_test.go) — precompile construction and execution checks.
- [grants_test.go](grants_test.go) — grant creation and successor session tests.
- [lifecycle_test.go](lifecycle_test.go) — identity lifecycle tests.
- [revoke_test.go](revoke_test.go) — revocation tests.
- [views_test.go](views_test.go) — authority query tests.
- [errors_test.go](errors_test.go) — error mapping tests.
- [gas_test.go](gas_test.go) — method gas tests.
- [gas_conformance_test.go](gas_conformance_test.go) — gas conformance tests.
- [parity_test.go](parity_test.go) — native session fixture parity tests.
- [pause_test.go](pause_test.go) — governance pause behavior tests.
- [surface_test.go](surface_test.go) — ABI surface and keeper context checks.
- [proof_layerxauth_0x101c_test.go](proof_layerxauth_0x101c_test.go) — application transactions, EVM receipts, and indexer consumer integration.
- [proof_layerxauth_errors_test.go](proof_layerxauth_errors_test.go) — kernel errors through keeper and precompile transport.

## Build and test

This package belongs to the root [Go module](../../go.mod) and has no separate build manifest or Makefile. Run these targets from the repository root:

```sh
make paxd-build
make paxeer-test
```

The root [Makefile](../../Makefile) builds the native libraries and node through `paxd-build`. Its `paxeer-test` target delegates to [chain.mk](../../chain.mk), whose Go test groups include this package. The receipt integration test also runs the Rust indexer consumer from [services/platform/Cargo.toml](../../services/platform/Cargo.toml) through the [runtime clock helper](../../tools/runtime/run-with-clock.sh).

## Configuration

The precompile reads no runtime environment variables or configuration files. The node supplies its auth keeper through `AuthKeepers`; [abi.json](abi.json) is embedded at compile time.

The receipt integration test reads `LAYERX_AUTH_PROOF_INPUT` to select a file for the application’s receipt output when invoked by the indexer test. Without this variable, it creates a temporary receipt file and runs the indexer consumer itself.

## Documentation

- [Precompile ABI reference](../../docs/system/precompiles.md) — authority functions, events, and activation rules.
- [Chain modules](../../docs/system/chain-modules.md) — native module state and parameters.
- [Documentation site: precompiles](../../docs/site/paxeer-docs/src/app/precompiles/page.tsx).
- [Documentation site: precompiles Markdown](../../docs/site/paxeer-docs/public/markdown/precompiles.md).
- [Documentation site: identity](../../docs/site/paxeer-docs/public/markdown/identity.md).
