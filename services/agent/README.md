# LayerX agent workspace

This directory holds the Rust agent workspace, Python and TypeScript SDKs,
API schemas, tests and boundary tools. It provides non-authoritative clients,
policy, custody, proof verification and agent-facing tools for Paxeer
mainnet-beta, the single BFT network run by `paxd`. Its kernel proposes blocks,
and a receipt is final when its block is committed. Sub-chains are tenant
domains in the database, without a separate deployment for each tenant.

## Layout

- [`crates/`](crates/) — Rust workspace packages listed below.
- [`fuzz/`](fuzz/) — Cargo-fuzz targets and corpora for wire, proof, contract,
  policy and tenant inputs.
- [`qualification/`](qualification/) — standalone funded-payment qualification
  package and fixtures.
- [`schema/`](schema/) — agent API schemas and golden JSON examples.
- [`sdk/`](sdk/) — Python and TypeScript SDKs and their compatibility matrix.
- [`tests/`](tests/) — boundary, isolation, parity and qualification packages,
  shared support code and fixtures.
- [`tools/`](tools/) — boundary, secret, dependency, policy, audit, SDK generation,
  documentation and test-runner tools.
- [`Cargo.toml`](Cargo.toml) — workspace members, package metadata and lint policy.
- [`Cargo.lock`](Cargo.lock) — locked workspace dependency versions.
- [`clippy.toml`](clippy.toml) — Clippy configuration.
- [`deny.toml`](deny.toml) — advisory, license, dependency and source policy.
- [`unsafe-allowlist.toml`](unsafe-allowlist.toml) — boundary scanner exceptions
  for unsafe operations.
- [`nondeterminism-allowlist.toml`](nondeterminism-allowlist.toml) — exact-file
  clock exception for the executable entry point.

### Crates

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

### Network boundary

The agent service's network interface uses node JSON-RPC and the authenticated
hosted gateway contracts. It does not open node storage, read private append-only
logs, mirror private C layouts or link the consensus core. Signed transactions
are submitted as exact bytes through `eth_sendRawTransaction`; only the network
applies protocol state. Local policy and approval decisions cannot grant
protocol authority.

Public JSON-RPC uses HTTPS at the node's root path.
The EVM chain ID is **125** (`0x7d`). Node
namespaces are `debug`, `echo`, `eth`, `net`, `pax`, `test` and `txpool`.
The agent service reads precompile views through `eth_call` at a specified height.
Hosted `px_` methods use the gateway contract; `px_register` and
`px_requestFunds` use its identity and faucet services, as declared in
[the gateway OpenRPC contract](../platform/hosted/gateway/openrpc.json).

Write evidence uses the rungs `submitted` (transaction hash known), `receipted`
(receipt signature verified against an authorized receipt key), and `committed`
(consensus commit verified at the receipt height). A timeout or unknown
transport outcome does not justify a higher rung. See
[SDK compatibility and offline verification](sdk/COMPATIBILITY.md).

## Build and test

All commands below run from the repository root.

```sh
make agent-build
make agent-lint
make agent-check
make agent-test
```

`agent-build` builds the locked workspace. `agent-test` runs its tests with
the native prerequisites and supervised runtime clock described below.
`agent-lint` runs Clippy and the dependency policy. `agent-check` runs the boundary checker, secret checker,
node boundary harness and a workspace check of all targets. The boundary
checker rejects forbidden storage dependencies, node-private paths, private C
linkage and mirrored layouts. Its explicit exceptions are declared in
[`unsafe-allowlist.toml`](unsafe-allowlist.toml) and
[`nondeterminism-allowlist.toml`](nondeterminism-allowlist.toml), not in source
comments. The node boundary harness uses the configured node JSON-RPC endpoint.

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

### Real-node tests

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
`prepare` mode, used by the Makefile, builds `paxd` and exports the native
binary locations. Runtime manifest and RPC selection are described below.

`make agent-test` also builds the platform TLS boundary and runtime clock
binaries and runs Cargo through
[`run-with-clock.sh`](../../tools/runtime/run-with-clock.sh). The script's
`sanitizers` mode selects `agent-test-sanitize` and has the same prerequisites.
These are full qualification targets; focused targets are listed in the root
[`Makefile`](../../Makefile), including `agent-test-mcp-scope`,
`agent-test-mcp-readonly` and `agent-test-sdk-rust`.

### SDK commands

From the repository root, the Makefile also provides:

```sh
make agent-sdk-ts-build
make agent-test-sdk-ts
make agent-test-sdk-py
make agent-test-sdk-parity
make agent-test-sdk-compat
make agent-sdk-generate
make agent-test-sdk-generate
```

The TypeScript build target installs locked dependencies when needed and runs
`npm run build` from [`sdk/typescript/`](sdk/typescript/). Its
[`package.json`](sdk/typescript/package.json) declares `npm test` and the
self-contained, Anvil and external-node test groups. Python tests use pytest;
the root target also checks compilation of the SDK and examples. The Python
package metadata and test paths are in
[`pyproject.toml`](sdk/python/pyproject.toml).

## Configuration

The executable reads environment settings in
[`main.rs`](crates/layerx-agentd/src/main.rs), with startup configuration types
in [`config.rs`](crates/layerx-agentd/src/config.rs). Settings depend on the
selected mode and enabled surfaces. The
[agent owner guide](crates/layerx-agentd/README.md) describes mode requirements,
and the [MCP serving guide](crates/layerx-mcp/README.md#serving) describes session
bindings.

| Purpose | Environment variables |
| --- | --- |
| Mode | `LAYERX_AGENT_MODE` |
| Human node connection | `LAYERX_AGENT_NODE_RPC_URL`, `LAYERX_AGENT_NODE_CHAIN_ID`, `LAYERX_AGENT_HUMAN_NETWORK_ID`, `LAYERX_AGENT_HUMAN_PROTOCOL_VERSION` |
| Human authority | `LAYERX_AGENT_HUMAN_AUTHORITY_ENDPOINT`, `LAYERX_AGENT_HUMAN_AUTHORITY_BEARER`, `LAYERX_AGENT_HUMAN_AUTHORITY_CA_DER`, `LAYERX_AGENT_HUMAN_AUTHORITY_MAX_BYTES`, `LAYERX_AGENT_HUMAN_DEADLINE_MS`, `LAYERX_AGENT_HUMAN_PEERS` |
| Human persistence and sessions | `LAYERX_AGENT_HUMAN_STORE`, `LAYERX_AGENT_HUMAN_SESSION_KEY_ROOT`, `LAYERX_AGENT_HUMAN_SESSION_OPERATOR_SECRET_FILE` |
| Human listener | `LAYERX_AGENT_HUMAN_SOCKET`, `LAYERX_AGENT_HUMAN_SOCKET_UID`, `LAYERX_AGENT_HUMAN_SOCKET_GID`, `LAYERX_AGENT_HUMAN_SOCKET_MODE`, `LAYERX_AGENT_HUMAN_MAX_CONNECTIONS`, `LAYERX_AGENT_HUMAN_MAX_STREAMS`, `LAYERX_AGENT_HUMAN_MAX_FRAME_BYTES`, `LAYERX_AGENT_HUMAN_MAX_QUEUED_BYTES`, `LAYERX_AGENT_HUMAN_MAX_PAYLOAD_BYTES`, `LAYERX_AGENT_HUMAN_TIMESTAMP_SPAN` |
| Human limits | `LAYERX_AGENT_HUMAN_LIMIT_ID`, `LAYERX_AGENT_HUMAN_LIMIT_NAME`, `LAYERX_AGENT_HUMAN_LIMIT_SCOPE`, `LAYERX_AGENT_HUMAN_LIMIT_SCOPE_ID`, `LAYERX_AGENT_HUMAN_LIMIT_CEILING`, `LAYERX_AGENT_HUMAN_LIMIT_CONSUMED` |
| Programs node and authority | `LAYERX_AGENT_NODE_ENDPOINT`, `LAYERX_EXPECTED_CHAIN_ID`, `LAYERX_AGENT_NODE_BEARER_TOKEN`, `LAYERX_AGENT_NODE_CONSENSUS_ENDPOINT`, `LAYERX_AGENT_CONSENSUS_ENDPOINT`, `LAYERX_AGENT_AUTHORITY_ENDPOINT`, `LAYERX_AGENT_AUTHORITY_BEARER_TOKEN`, `LAYERX_AGENT_AUTHORITY_CA_DER`, `LAYERX_AGENT_AUTHORITY_REPLICA_ID` |
| Programs listener and state | `LAYERX_AGENT_PROGRAM_LISTEN`, `LAYERX_AGENT_PROGRAM_BEARER_TOKEN`, `LAYERX_AGENT_PROGRAM_PROBE_ID`, `LAYERX_AGENT_PROGRAM_MAX_STALENESS_MS`, `LAYERX_AGENT_CHAIN_TRUST_ROOT`, `LAYERX_AGENT_DEPLOYMENT_JOURNAL`, `LAYERX_AGENT_REGISTRY_SOURCE_CONFIG`, `LAYERX_AGENT_STATE_EXPORT_SOURCE` |
| Tenant policies and budget denominations | `LAYERX_POLICY_SOURCES`, `LAYERX_NATIVE_POLICY_SOURCES`, `LAYERX_NATIVE_EFFECT_POLICY_SOURCES`, `LAYERX_PROGRAM_BUDGET_DENOMINATION_SOURCES` |
| Runtime clock | `LAYERX_RUNTIME_CLOCK_SOCKET`, `LAYERX_RUNTIME_CLOCK_PID`, `LAYERX_RUNTIME_CLOCK_UID` |
| Agent RPC listener | `LAYERX_AGENTD_RPC_LISTEN`, `LAYERX_AGENTD_RPC_TLS_CERT`, `LAYERX_AGENTD_RPC_TLS_KEY`, `LAYERX_AGENTD_RPC_TLS_CLIENT_CA`, `LAYERX_AGENTD_RPC_PEER`, `LAYERX_NODE_NETWORK_NAME` |
| Agent RPC persistence and MCP | `LAYERX_AGENTD_RPC_IDEMPOTENCY_ROOT`, `LAYERX_AGENTD_RPC_IDEMPOTENCY_DAEMON_SEQUENCES`, `LAYERX_AGENTD_RPC_IDEMPOTENCY_PROTOCOL_SEQUENCES`, `LAYERX_AGENTD_MCP_BINDINGS`, `LAYERX_AGENTD_MCP_PAYERS` |
| MCP binding | `LAYERX_AGENT_MCP_BINDING_ROOT`, `LAYERX_AGENT_MCP_AUDIT_ROOT`, `LAYERX_AGENT_MCP_MODE`, `LAYERX_AGENT_MCP_AGENT_DID`, `LAYERX_AGENT_MCP_SESSION_ID`, `LAYERX_AGENT_MCP_CAPABILITY_ID`, `LAYERX_AGENT_MCP_PEER_UID`, `LAYERX_AGENT_MCP_OPENING_CLIENT`, `LAYERX_AGENT_MCP_POLICY_VERSION`, `LAYERX_AGENT_MCP_ACTIVITY_TYPES`, `LAYERX_AGENT_MCP_SCOPES`, `LAYERX_AGENT_MCP_DEADLINE_MS` |
| MCP heights | `LAYERX_AGENT_MCP_CORE_HEIGHT`, `LAYERX_AGENT_MCP_EXPIRY_HEIGHT` |
| MCP limits | `LAYERX_AGENT_MCP_LIMIT_ID`, `LAYERX_AGENT_MCP_LIMIT_NAME`, `LAYERX_AGENT_MCP_LIMIT_SCOPE`, `LAYERX_AGENT_MCP_LIMIT_SCOPE_ID`, `LAYERX_AGENT_MCP_LIMIT_CEILING`, `LAYERX_AGENT_MCP_LIMIT_CONSUMED` |
| MCP listener | `LAYERX_AGENT_MCP_LISTENER_SOCKET`, `LAYERX_AGENT_MCP_LISTENER_OWNER_UID`, `LAYERX_AGENT_MCP_LISTENER_OWNER_GID`, `LAYERX_AGENT_MCP_LISTENER_MODE`, `LAYERX_AGENT_MCP_LISTENER_ADMITTED_UIDS` |
| MCP web access | `LAYERX_AGENT_MCP_WEB_ENDPOINT`, `LAYERX_AGENT_MCP_WEB_NETWORK`, `LAYERX_AGENT_MCP_WEB_PAYER_ADDRESS`, `LAYERX_AGENT_MCP_WEB_RECEIVER_ADDRESS`, `LAYERX_AGENT_MCP_WEB_RECEIPT_KEY_INDEX`, `LAYERX_AGENT_MCP_WEB_TIMEOUT_MS`, `LAYERX_AGENT_MCP_WEB_PENDING_ATTEMPTS`, `LAYERX_AGENT_MCP_WEB_APPROVAL_THRESHOLD` |
| Inference service | `LAYERX_AGENT_INFERENCE_MODEL_DIGEST`, `LAYERX_AGENT_INFERENCE_RUNTIME_DIGEST`, `LAYERX_AGENT_INFERENCE_ROOM_PROFILE`, `LAYERX_AGENT_INFERENCE_SIGNAL_LISTEN`, `LAYERX_AGENT_INFERENCE_SERVE_LISTEN`, `LAYERX_AGENT_INFERENCE_DEALS`, `LAYERX_AGENT_INFERENCE_DRAFT_RUNTIME_KIND` |
| Inference enrolment and completion | `LAYERX_AGENT_INFERENCE_ENROLMENT`, `LAYERX_AGENT_INFERENCE_REGION`, `LAYERX_AGENT_INFERENCE_ATTESTATION_FAMILY`, `LAYERX_AGENT_INFERENCE_BANDWIDTH_PROBE`, `LAYERX_AGENT_INFERENCE_ENROLMENT_JOURNAL`, `LAYERX_AGENT_INFERENCE_COMPLETION_SOCKET`, `LAYERX_AGENT_INFERENCE_COMPLETION_RECEIPTS` |
| Inference gateway | `LAYERX_AGENT_INFERENCE_GATEWAY_ENDPOINT`, `LAYERX_AGENT_INFERENCE_GATEWAY_API_KEY`, `LAYERX_AGENT_INFERENCE_GATEWAY_CA_FILE` |

The library's `config::load` reads an explicitly supplied protected UTF-8
configuration file containing `key=value` declarations. It accepts
`network_id`, `node_endpoint`, `expected_chain_id`, `tenants`, `policy_sources`,
`signer_configurations`, `verification_defaults` and `receipt_key_source`.
Their environment overrides are `LAYERX_NETWORK_ID`, `LAYERX_NODE_ENDPOINT`,
`LAYERX_EXPECTED_CHAIN_ID`, `LAYERX_TENANTS`, `LAYERX_POLICY_SOURCES`,
`LAYERX_SIGNER_CONFIGURATIONS`, `LAYERX_VERIFICATION_DEFAULTS` and
`LAYERX_RECEIPT_KEY_SOURCE`. Policy, signer, trust-root, registry-source, CA,
TLS, room-profile and session-operator files use explicitly supplied paths.

The boundary checker reads [`unsafe-allowlist.toml`](unsafe-allowlist.toml) and
[`nondeterminism-allowlist.toml`](nondeterminism-allowlist.toml). Clippy uses
[`clippy.toml`](clippy.toml), and the dependency policy reads
[`deny.toml`](deny.toml).

Build and test settings include `AGENT_CARGO`, `AGENT_NODE_RPC_URL`,
`CARGO_TARGET_DIR`, `LAYERX_TEST_NATIVE_BUILD_DIR`, `LAYERX_TEST_NATIVE_BIN_DIR`,
`PAXD`, `LAYERX_PAXEER_BOUNDARY_BIN`, `LAYERX_RUNTIME_CLOCK_BIN` and
`LAYERX_TEST_RUNTIME_CLOCK_BIN`. The real-node script reads
`PAXEER_X_RUNTIME_MANIFEST` when supplied, taking `chain_id` and the RPC port
from `ports[0]` or `chain_rpc`; otherwise it reads
`LAYERX_TEST_PAXEER_RPC_PORT`. The sanitizer script also reads
`LAYERX_AGENT_TSAN_TARGET_DIR` and `LAYERX_TEST_SANITIZER`.

## Documentation

- [Agent runtime on the documentation site](../../docs/site/paxeer-docs/src/app/agent-runtime/page.tsx)
- [Agent integration on the documentation site](../../docs/site/paxeer-docs/src/app/agents/page.tsx)
- [MCP tools on the documentation site](../../docs/site/paxeer-docs/src/app/mcp/page.tsx)
- [Agent API compatibility](../../docs/agent/COMPATIBILITY.md)
- [Inference market overview](../../docs/agent/inference-market/README.md)
- [Inference market client guide](../../docs/agent/inference-market/client.md)
- [Inference market operator guide](../../docs/agent/inference-market/operator.md)
- [Inference market protocol](../../docs/agent/inference-market/protocol.md)
- [Inference market threats](../../docs/agent/inference-market/threats.md)
- [SDK compatibility and offline verification](sdk/COMPATIBILITY.md)
- [Python SDK testing](sdk/python/TESTING.md)
- [Agent owner](crates/layerx-agentd/README.md)
- [MCP transport](crates/layerx-mcp/README.md)

### MCP integration

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
