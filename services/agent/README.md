# LayerX agent workspace

The agent workspace provides non-authoritative clients, policy, custody,
proof verification and agent-facing tools for Paxeer mainnet-beta. There is
one BFT network, run by `paxd`; its kernel is part of the network and proposes
blocks. A receipt is final when its block is committed. Sub-chains are tenant
domains in the database, without a separate deployment for each tenant.

## Network boundary

The agent service's network interface uses node JSON-RPC and the authenticated
hosted gateway contracts. It does not open node storage, read private append-only
logs, mirror private C layouts or link the consensus core. Signed transactions
are submitted as exact bytes through `eth_sendRawTransaction`; only the network
applies protocol state. Local policy and approval decisions cannot grant
protocol authority.

Public JSON-RPC uses HTTPS at the root path of
`api1.mainnet-beta.paxeer.network` through
`api14.mainnet-beta.paxeer.network`, and
[`api-mainnet-beta.paxeer.network`](https://api-mainnet-beta.paxeer.network/).
The EVM chain ID is **125** (`0x7d`). Node
namespaces are `debug`, `echo`, `eth`, `net`, `pax`, `px`, `test` and `txpool`.
The agent service reads precompile views through `eth_call` at a specified height.
The node registers `px` reads for account, balance, receipt and consensus data;
asset and history queries use its configured indexer. `px_register` and
`px_requestFunds` require the hosted identity and faucet services declared in
[the gateway OpenRPC contract](../platform/hosted/gateway/openrpc.json).

`node.hyperpaxeer.com` is only the hpx library mirror, never a JSON-RPC endpoint.

Write evidence uses the rungs `submitted` (transaction hash known), `receipted`
(receipt signature verified against an authorized receipt key), and `committed`
(consensus commit verified at the receipt height). A timeout or unknown
transport outcome does not justify a higher rung. See
[SDK compatibility and offline verification](sdk/COMPATIBILITY.md).

## Crates

This is a Rust 2021 Cargo workspace, with Rust 1.91.1 pinned in the repository's
[`rust-toolchain.toml`](../../rust-toolchain.toml).

| Crate | Role |
| --- | --- |
| `layerx-types` | Canonical domain types, module identifiers and verification rungs |
| `layerx-wire` | Byte-exact canonical wire encoding |
| `layerx-crypto` | Key custody, disclosure-bound signing and EVM transaction codecs |
| `layerx-proof` | Verification primitives for receipts, inclusion and state evidence |
| `layerx-client` | Node JSON-RPC and hosted gateway clients |
| `layerx-identity-binding` | Unix-socket identity binding client shared with the Human plane |
| `layerx-agent-api` | Agent contract types and strict proof/export JSON codecs |
| `layerx-agentd` | Non-authoritative agent owner library ([README](crates/layerx-agentd/README.md)) |
| `layerx-agentd-host` | Builds the `layerx-agentd` binary from the library crate's `src/main.rs` |
| `layerx-mcp` | Tenant- and scope-bound MCP transport ([README](crates/layerx-mcp/README.md)) |
| `layerx-sdk` | Rust SDK for direct-node and agent deployments |

The Python and TypeScript SDKs live under [`sdk/`](sdk/). The
[compatibility matrix](../../docs/agent/COMPATIBILITY.md) pairs agent 0.1.0 and SDK 0.1.0 with
Agent API major 2 and the accepted node client-version major.

## Build and check

All commands below run from the repository root.

```sh
make agent-build
make agent-lint
make agent-check
```

`agent-build` builds the locked workspace. `agent-lint` runs Clippy and the
dependency policy. `agent-check` runs the boundary checker, secret checker,
node boundary harness and a workspace check of all targets. The boundary
checker rejects forbidden storage dependencies, node-private paths, private C
linkage and mirrored layouts. Its explicit exceptions are declared in
[`unsafe-allowlist.toml`](unsafe-allowlist.toml) and
[`nondeterminism-allowlist.toml`](nondeterminism-allowlist.toml), not in source
comments. The node boundary harness uses `AGENT_NODE_RPC_URL`, defaulting to
`http://127.0.0.1:8545`.

The boundary scanner traverses all Rust sources and Cargo manifests under
`crates/`, including integration tests, host tests and `src/bin/`, excluding
`target/` directories. Clock permission is an exact-file exception for
`crates/layerx-agentd/src/main.rs`; membership in a binary or test target
grants no additional exception. Library deadlines use an injected `Clock` and
`Deadline`. Test timestamps use the shared
[`wall_time`](tests/support/wall_clock.rs) and
[`monotonic_time`](tests/support/monotonic_clock.rs) adapters; those adapters
live outside the scanner's `crates/` traversal. Calling ambient clocks directly
inside a test remains a boundary violation.

## Tests

The full workspace target requires native prerequisites and a runtime clock.
Install the pinned Rust toolchain, a C compiler and Make, Go, Foundry (`forge`,
`cast`, `anvil`), and the Python dependencies in
[`tests/bridge/requirements.txt`](../../tests/bridge/requirements.txt).
The real-node entry point requires root so native fixtures can run the node
under a distinct test UID:

```sh
sh services/agent/tools/run-real-node-tests.sh test
```

Run that command in the prepared root test environment with its toolchains on
`PATH`. The script checks prerequisites and invokes `make agent-test`. Its
`prepare` mode, used by the Makefile, builds the program-admission and
metered-allowance fixtures and `paxd`; it exports the native binary locations.
`PAXEER_X_RUNTIME_MANIFEST`, when supplied, provides `chain_id` and the RPC port
from `ports[0]` or `chain_rpc`. Otherwise the script uses chain ID 125 and
`LAYERX_TEST_PAXEER_RPC_PORT`, default 8545.

`make agent-test` also builds the platform TLS boundary and runtime clock
binaries and runs Cargo through
[`run-with-clock.sh`](../../tools/runtime/run-with-clock.sh). The script's
`sanitizers` mode selects `agent-test-sanitize` and has the same prerequisites.
These are full qualification targets; focused targets are listed in the root
[`Makefile`](../../Makefile), including `agent-test-mcp-scope`,
`agent-test-mcp-readonly` and `agent-test-sdk-rust`.

## MCP

[`layerx-mcp`](crates/layerx-mcp/README.md) serves a daemon-owned catalogue for
one tenant and its session capability. Both the binary and
`layerx mcp serve --daemon-binding <path>` obtain the live description and
invoke tools through the authenticated agent envelope. Read-only mode narrows
the catalogue to read tools. Credentials and readiness requirements are in
the [MCP serving guide](crates/layerx-mcp/README.md#serving).

The interop MCP/A2A transports live in
[`services/interop/`](../interop/README.md); the install and serve commands
live in [`tools/platform/cli/`](../../tools/platform/cli/README.md).
Wallet, token and grant writes use the agent's authority, policy, preparation,
signature, submission and receipt tracking boundaries. They cannot acquire
extra authority through a tool call.
