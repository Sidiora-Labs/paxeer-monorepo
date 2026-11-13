# WebAssembly integration

The CosmWasm Go wrapper connects the Rust VM to the chain's Wasm module.
[`node/app.go`](../../node/app.go) registers that module in `paxd`. CosmWasm,
the EVM and the kernel execute within the same network; a receipt is final when
its block is committed.

## Components

| Component | Location | Role |
| --- | --- | --- |
| Go wrapper | [`lib.go`](../lib.go) | Code storage, VM entry points, JSON serialization and gas reports |
| Contract data types | [`types`](../types) | Environment, messages, responses, storage and host callback interfaces |
| Native bridge | [`internal/api`](../internal/api) and [`libwasmvm`](../libwasmvm) | cgo bindings and Rust VM calls |
| Chain integration | [`wasm/x/wasm`](../../wasm/x/wasm) | Authorization, contract state, message dispatch, queries, governance and IBC |

The native library's dependency versions and capabilities are declared in
[`libwasmvm/Cargo.toml`](../libwasmvm/Cargo.toml). Its `cosmwasm-std` and
`cosmwasm-vm` dependencies use the pinned Paxeer CosmWasm fork.

## Contents

- [Architecture](Architecture.md): state isolation, host queries, response
  dispatch and contract lifecycle.
- [Specification](Specification.md): wrapper methods, JSON fields and callback
  interfaces.
- [Build and platform guide](../README.md): native libraries and Go builds.
- [Module configuration](../../wasm/x/wasm/README.md): node configuration and CLI.
- [Governance](../../wasm/x/wasm/Governance.md): privileged contract operations.
- [IBC](../../wasm/x/wasm/IBC.md): contract ports and packet callbacks.
