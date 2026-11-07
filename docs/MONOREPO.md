# Monorepo layout

This repository contains one Paxeer X Network: the `paxd` node with the LayerX kernel embedded in it, contracts, hosted services, SDKs and applications. The kernel participates in block proposal and execution inside the network; a receipt is final when its block is committed. Source directories and language-specific builds are development boundaries within that network.

See [Paxeer Network](paxeer-network.md) for chain identity, public endpoints and tenant domains. The target directory layout is specified in [`spec/monorepo-layout/spec.kvx`](../spec/monorepo-layout/spec.kvx); the tables below distinguish existing paths from planned moves.

## Target groups

| Group | Holds |
| --- | --- |
| `apps/` | User-facing frontends |
| `services/` | Off-chain programs |
| `sdk/` | Client and guest SDKs |
| `core/` | The chain: kernel, node packages, framework fork, programs runtime and pinned upstream libraries |
| `tests/` | Shared test trees and fixtures; packages also retain local tests |
| `tools/` | Developer and operator tools |
| `contracts/` | On-chain sources |

`api/`, `deploy/`, `docker/`, `docs/`, `scripts/`, `spec/` and the root build and community files stay at the root.

## Current tree

| Path | Contents |
| --- | --- |
| `apps/perps` | The Hyperpax perps frontend |
| `apps/wallet`, `apps/web` | Wallet app and web app |
| `apps/explorer` | Block explorer frontend |
| `apps/inference-peer` | Standalone inference peer package with the pinned MIT Pooled WebGPU engine, room frame codec and WebSocket/WebRTC transport; see its `README.md`, `NOTICE` and `LICENSE-ENGINE` |
| `apps/bridge` | Bridge frontend |
| `apps/launchpad` | Launchpad web and mobile apps and the client workspace files |
| `services/agent` | Agent daemon, MCP server, shared Rust crates, TypeScript and Python SDKs |
| `services/interop` | Interop gateway, bridge relayer and agent-commerce services |
| `services/platform` | Hosted developer platform, middleware, perps data and ramps |
| `services/explorer` | Block explorer backend, services and deployment |
| `services/launchpad` | Launchpad services |
| `services/human` | Human control plane Rust workspace (crates, schema, tools) |
| `services/wallet` | Embedded wallet service: attestor, ceremony, gateway, demo, schema, deployment and runbooks |
| `services/bridge` | Bridge deployment |
| `sdk/platform`, `sdk/integrations` | Platform SDKs (Go, JVM, .NET, Swift) and framework integrations |
| `sdk/launchpad` | Launchpad protocol, shared libraries and client packages |
| `sdk/chainflow` | Chainflow client libraries for Go, Python, Rust and TypeScript |
| `sdk/js` | First-party `@pax-js/cosmos` package, edited in place and aligned to the protos of this repository |
| `sdk/wallet` | Wallet SDK (`@paxeer/wallet`), a member of the `services/wallet` pnpm workspace |
| `daemon/paxd`, `node`, `rpc` | Node entrypoint, application wiring and node JSON-RPC |
| `src`, `include`, `modules/lxphost` | C kernel implementation and headers, and its Go/cgo host inside `paxd` |
| `modules`, `precompiles`, `programs` | Chain modules, EVM precompiles and programs runtime |
| `core/deps` | Pinned snapshots of the upstream libraries the node builds from, each with a `VENDOR.kvx` pin record |
| `tests/` | Kernel C tests, `tests/fuzz`, `tests/integration`, `tests/occ`, `tests/testutil`, `tests/fixtures`, Solidity and vector suites |
| `tools/hpx`, `tools/platform`, `tools/launchpad` | Node installer and registry, platform CLI, emulator and release tooling, launchpad tools |
| `tools/loadtest`, `tools/paxctl`, `tools/uci`, `tools/wasm-tools` | Pinned tool snapshots with `VENDOR.kvx` pin records |
| `contracts/` | Root Foundry sources; `contracts/src` and `contracts/test` use `foundry.paxeer.toml` |
| `contracts/bridge` | Bridge vault contracts for EVM chains, the Solana program and test vectors |
| `contracts/launchpad` | Launchpad contracts |
| `deploy/` | Deployment descriptors, including `deploy/launchpad` |

## Inference peer room transport

The local adapter lives in [`src/wire.mjs`](../apps/inference-peer/src/wire.mjs)
and [`src/transport.mjs`](../apps/inference-peer/src/transport.mjs); the MIT
engine snapshot remains separate. Import these source modules directly from a
browser ES module or the native runtime. There is no compilation step.

`encodeFrame({ kind, roomId, sequence, payload })` returns a `Uint8Array`;
`decodeFrame(bytes)` accepts an `ArrayBuffer` or a byte view and returns those
four fields. All integers use network byte order. One WebSocket message or data
channel message carries exactly one frame, at most **65536 bytes including its
header**. Decoding rejects unknown kinds, malformed UTF-8/JSON, truncated bodies
and extra bytes. The binary layout is:

| Field | Encoding |
| --- | --- |
| Kind | u16; hello=1, offer=2, deal=3, deal-ack=4, activation=5, lap-receipt=6, countersign=7, canary-answer=8, bye=9 |
| Body length | u32, excludes the six-byte kind/length header |
| Room ID length, room ID | u16 byte length followed by nonempty UTF-8 text |
| Sequence | u32 |
| Payload | Raw bytes for activation; a UTF-8 JSON object for every other kind |

`SignalClient(url, { roomId, hello })` uses the browser or Node 22+ WebSocket
API. `hello` carries `peerKey`, `signature`, `nonce` and `manifest` with version
2; the caller supplies the signature over the room ID and nonce. `connect()`
sends hello and resolves with the host's offer frame. The application must
validate that offer against the chain's room record before accepting a deal.
The host offer carries the model/runtime digests, layer plan and price; a deal
payload carries `dealId` and `peers` (distinct peer-key strings). Frame events
arrive as `event.detail`; a received deal can be acknowledged once through
`acknowledgeDeal(dealId)`. The client enforces hello → offer → deal → deal-ack
order, matching room IDs and increasing incoming signalling sequences. Its
ten-second offer deadline is configurable as `timeoutMs`.

WebRTC relays use kind `offer` with payload `{ to, relay }`; the host supplies
the admitted sender as `{ from, to, relay }` when forwarding. `relay` contains
either `description: { type, sdp }` (offer/answer) or
`candidate: { candidate, sdpMid, ... }`. The host must authenticate the sender
and forward only to the named admitted peer. The planned host listener is
not present in this checkout; these envelopes define the peer side of that
integration. The declared transport tests use a loopback WebSocket stand-in,
without qualifying chain admission or live host interoperability.

After receiving and validating a deal, create a browser mesh with
`await PeerMesh.create(signal, { mode: "browser", rtcConfig })` and call
`await mesh.join(deal.payload)`. Joining allocates connections for the deal's
other peers, sends deal-ack, and starts SDP/ICE exchange. The lexicographically
smaller peer key initiates each connection; candidates arriving before the
remote description are buffered with a bound. Listen for `open` events before
calling `mesh.send(peerKey, kind, payload)`. Mesh frame events carry
`{ peerKey, frame }`. Native callers use `mode: "native"` and must provision
the optional `node-datachannel` package and its native binary so that it is
resolvable from this package. Missing WebRTC support or native dependencies
fails explicitly. Browser and native `rtcConfig` values use their respective
backend's ICE server configuration format.

Each connection has a reliable ordered `control` channel and a reliable
unordered `activations` channel, both with protocol `paxeer-room-v1`.
Activations retain their sequence for the layer scheduler to handle arrival
order; control sequences must increase. Both signalling and mesh sends reject
backpressure above four maximum-size frames. Mesh sends also respect the
negotiated SCTP message limit. Applications split larger tensors into bounded
activation messages. Invalid incoming mesh traffic or connection failure
closes the mesh and emits an `error` event; signalling loss closes it too.
`close()` releases sockets or peer connections and can be called repeatedly.
The focused gate covers every frame kind, size and decode refusals, the real
WebSocket client handshake and named-peer relay; it does not exercise a
browser or native WebRTC connection.

## Trees that move under core in a later release

These are planned directory moves, not additional deployments. In particular, the current kernel sources still live at the repository root.

| Current path | Destination |
| --- | --- |
| `src`, `include`, `cmd`, `migrations` | `core/kernel` |
| `sdk` framework fork entries | `core/cosmos-sdk` |
| `consensus`, `interchain`, `wasm`, `wasm-runtime`, `storage`, `modules`, `precompiles`, `node`, `rpc`, `engine`, `daemon`, `admin`, `custodyproof`, `layerxproof`, `utils` | `core/<name>` |
| `programs/sdk` | `sdk/programs` |
| `programs` | `core/programs` |

## Build boundaries

- **Kernel library**: C17 (`-std=c17` in the root `Makefile`); `make build` produces `build/liblayerx.a`. Native tests and replay, arithmetic, fault and fuzz qualification exercise this library. These checks alone do not qualify the integrated node; see [Qualification](QUALIFICATION.md).
- **Integrated node**: Go 1.25.8 (`go.mod`), built through `chain.mk`. `make paxeer-build` builds `build/paxd`; `make paxd-build` also stamps the release version from `tools/release/version.json`. The native prerequisite `make paxd-native-libs` builds the C library and release programs sandbox archive, linked by `modules/lxphost/host.go`. `make paxeer-lint`, `make paxeer-test` and `make paxeer-ci` cover the Go tree; `make test-paxd-domain` includes the kernel host, node, EVM keeper and RPC tests.
- **Workspaces and contracts**: Rust 1.91.1 (`rust-toolchain.toml`), Solidity 0.8.27 (`foundry.toml` and `foundry.paxeer.toml`), and the npm workspaces listed in `package.json`. `make test-contracts` selects `tests/solidity` for the root Foundry project. The separate Foundry configuration selects `contracts/src` and `contracts/test`; see [Build and test](paxeer-network.md#build-and-test).
- **Aggregate checks**: `make workspace-build` composes the declared kernel, node, service, tool and contract builds. `make monorepo-ci` depends on `workspace-ci`, which composes source audit, inventory, workspace build, test and lint targets. It does not replace the explicit replay, arithmetic, fault or fuzz qualification matrices. Run aggregate and sanitizer gates at wave or release qualification; an individual task runs only its declared build and verification.
- `make ci` runs `public-audit`, kernel tests, a two-build archive comparison, consensus symbol checks and sanitizer suites. `scripts/ci/layout-check.sh` and `scripts/ci/vendor-verify.sh` check the layout and the vendored snapshots.

## Trust boundaries

The network's consensus commit is the authority for finality. There is no separate execution-to-settlement hop between the kernel and `paxd`.

- Kernel execution is reached through the node's `modules/lxphost` integration and chain modules. Native balances, asset-ledger state and receipts must be interpreted in that committed chain state.
- Hosted services, explorers and bridge clients consume node APIs and committed evidence. Their databases, caches and request acknowledgements do not establish network finality. The node serves the `px` namespace at its root; the hosted `px_register` and `px_requestFunds` operations are declared in `services/platform/hosted/gateway/openrpc.json`.
- A sub-chain is a tenant domain in the database, with its own namespace and allowances under the same network security. Creating one needs no new node infrastructure.
- Retained legacy source or migration code is not a deployment inventory. The standalone `layerxd` daemon and checkpoint/guarantor settlement model are retired.
- Imported code keeps its license and provenance boundaries. In particular, the explorer backend and frontend run as separate processes; consult `services/explorer/LICENSE`, `apps/explorer/LICENSE` and the imported service licenses before redistribution.

## Workflows

Workflows live in `.github/workflows/`. `node-build.yml` builds native libraries and `paxd` and exercises the host integration; its pull-request paths include kernel, programs and node sources. Other package workflows include `agent.yml`, `human.yml`, `platform.yml`, `programs-conformance.yml`, `explorer-build.yml` and `bridge-test.yml`, alongside the `paxeer-*.yml` workflows. Consult each workflow's triggers and commands for its actual coverage.

## Further reading

- [`README.md`](../README.md): network overview, build and test commands
- [`CONTRIBUTING.md`](../CONTRIBUTING.md): contribution guidelines
- [`QUALIFICATION.md`](QUALIFICATION.md): evidence levels and qualification gates
- [`paxeer-network.md`](paxeer-network.md): merged runtime, root node endpoints and tenant domains
