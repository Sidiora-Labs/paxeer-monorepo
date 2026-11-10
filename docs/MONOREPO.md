# Monorepo layout

This repository contains one Paxeer X Network: the `paxd` node with the LayerX kernel embedded in it, contracts, hosted services, SDKs and applications. The kernel participates in block proposal and execution inside the network; a receipt is final when its block is committed. Source directories and language-specific builds are development boundaries within that network.

See [Paxeer Network](paxeer-network.md) for chain identity, public endpoints and tenant domains. The tables below distinguish existing paths from planned moves. See [Contributing](../CONTRIBUTING.md#specifications-come-first) for the specifications that govern layout changes.

## Root layout

Every tracked top-level directory and file has one row below, including hidden
configuration entries. Generated build output and untracked local state are outside
this inventory. The current checkout has 43 directories and 41 files; the inventory
changes as tracked root entries change.

| Entry                     | Kind      | Purpose and configuration                                                                                                                                                                                                                                                                                                    |
| ------------------------- | --------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `.agents/`                | Directory | Local coding-agent skills, including the Codify workflow.                                                                                                                                                                                                                                                                    |
| `.cgignore`               | File      | Codify graph indexing include and exclude rules.                                                                                                                                                                                                                                                                             |
| `.claude/`                | Directory | Claude settings and local ignore rules.                                                                                                                                                                                                                                                                                      |
| `.coderabbit.yaml`        | File      | Code review automation settings and language-specific review guidance.                                                                                                                                                                                                                                                       |
| `.codex/`                 | Directory | Codex hook configuration.                                                                                                                                                                                                                                                                                                    |
| `.codify/`                | Directory | Codify agent context and lifecycle hooks.                                                                                                                                                                                                                                                                                    |
| `.cursor/`                | Directory | Cursor MCP configuration and generated workflow rules.                                                                                                                                                                                                                                                                       |
| `.devcontainer/`          | Directory | Development container definition, Dockerfile and environment template.                                                                                                                                                                                                                                                       |
| `.editorconfig`           | File      | Shared encoding, whitespace, indentation and newline conventions.                                                                                                                                                                                                                                                            |
| `.gemini/`                | Directory | Gemini coding-agent settings.                                                                                                                                                                                                                                                                                                |
| `.gitattributes`          | File      | Git file attributes and text handling rules.                                                                                                                                                                                                                                                                                 |
| `.github/`                | Directory | CI workflows, contribution templates and GitHub automation.                                                                                                                                                                                                                                                                  |
| `.gitignore`              | File      | Untracked dependency, build and local-state exclusions.                                                                                                                                                                                                                                                                      |
| `.gitmodules`             | File      | Git submodule configuration retained at the repository root.                                                                                                                                                                                                                                                                 |
| `.golangci.yml`           | File      | Go lint configuration for the node and related packages.                                                                                                                                                                                                                                                                     |
| `.markdownlint.yaml`      | File      | Markdown lint rules.                                                                                                                                                                                                                                                                                                         |
| `.mcp.json`               | File      | Root MCP server configuration for Codify.                                                                                                                                                                                                                                                                                    |
| `.opencode/`              | Directory | OpenCode coding-agent configuration.                                                                                                                                                                                                                                                                                         |
| `.pre-commit-config.yaml` | File      | Pre-commit whitespace, Markdown, YAML, Rust and Solidity checks.                                                                                                                                                                                                                                                             |
| `.yamllint.yaml`          | File      | YAML lint rules.                                                                                                                                                                                                                                                                                                             |
| `AGENTS.md`               | File      | The same generated workflow pointer as CLAUDE.md, rendered from spec/workflow.kvx by tools/specgen; edit the source specification.                                                                                                                                                                                           |
| `CHANGELOG.md`            | File      | Release change history.                                                                                                                                                                                                                                                                                                      |
| `CITATION.cff`            | File      | Machine-readable project citation metadata.                                                                                                                                                                                                                                                                                  |
| `CLAUDE.md`               | File      | The same generated workflow pointer as AGENTS.md, rendered from spec/workflow.kvx by tools/specgen; edit the source specification.                                                                                                                                                                                           |
| `CODEMAP.md`              | File      | Repository code map and component navigation.                                                                                                                                                                                                                                                                                |
| `CODE_OF_CONDUCT.md`      | File      | Community conduct policy.                                                                                                                                                                                                                                                                                                    |
| `CONTRIBUTING.md`         | File      | Contributor setup, development and contribution guidance.                                                                                                                                                                                                                                                                    |
| `CONTRIBUTORS.md`         | File      | Contributor acknowledgements.                                                                                                                                                                                                                                                                                                |
| `GOTCHA.kvx`              | File      | Append-only engineering gotcha ledger.                                                                                                                                                                                                                                                                                       |
| `GOVERNANCE.md`           | File      | Project governance and decision-making policy.                                                                                                                                                                                                                                                                               |
| `LICENSE`                 | File      | Repository Apache-2.0 license; imported components retain their own licenses.                                                                                                                                                                                                                                                |
| `Makefile`                | File      | C17 kernel library and tool builds, focused tests, and workspace orchestration; includes chain.mk.                                                                                                                                                                                                                           |
| `NOTICE`                  | File      | Copyright and third-party attribution notices.                                                                                                                                                                                                                                                                               |
| `README.md`               | File      | Network overview, entry points, build commands and documentation links.                                                                                                                                                                                                                                                      |
| `RELEASE.md`              | File      | Release artifacts, qualification and network upgrade procedure.                                                                                                                                                                                                                                                              |
| `SECURITY.md`             | File      | Security policy and vulnerability reporting instructions.                                                                                                                                                                                                                                                                    |
| `SUPPORT.md`              | File      | Support channels and issue guidance.                                                                                                                                                                                                                                                                                         |
| `admin/`                  | Directory | Top-level Go package implementing the node administration gRPC server, including runtime log-level control and generated admin types.                                                                                                                                                                                        |
| `api/`                    | Directory | Protobuf API sources and compatibility tests for chain modules and administration; root Buf configuration generates Go packages in modules, precompiles and admin/types.                                                                                                                                                     |
| `apps/`                   | Directory | User-facing applications, including launchpad, perps, wallet, bridge, explorer, names and the inference peer; several own pnpm sub-project workspaces.                                                                                                                                                                       |
| `buf.baseline.binpb`      | File      | Binary protobuf descriptor baseline for API compatibility checks.                                                                                                                                                                                                                                                            |
| `buf.gen.yaml`            | File      | Root Buf code-generation inputs and plugin settings.                                                                                                                                                                                                                                                                         |
| `buf.lock`                | File      | Pinned remote protobuf dependencies for the root Buf workspace.                                                                                                                                                                                                                                                              |
| `buf.yaml`                | File      | Root multi-module protobuf workspace, lint and breaking-change settings.                                                                                                                                                                                                                                                     |
| `chain.mk`                | File      | Go node build, version stamping, lint and test targets used by the root Makefile.                                                                                                                                                                                                                                            |
| `cmd/`                    | Directory | C-kernel development tools: layerx-genesis and layerx-module-registry; also the Rust layerxctl CLI and retirement checks. The production node entrypoint is daemon/paxd.                                                                                                                                                     |
| `codify.kvx`              | File      | Codify project paths, graph indexing and sync settings.                                                                                                                                                                                                                                                                      |
| `consensus/`              | Directory | BFT consensus implementation, networking, ABCI, consensus types and RPC support used by paxd.                                                                                                                                                                                                                                |
| `contracts/`              | Directory | Solidity contracts, Foundry dependencies and tests, plus separate bridge and launchpad contract projects.                                                                                                                                                                                                                    |
| `core/`                   | Directory | Pinned upstream dependencies under core/deps, with provenance and vendor pin records.                                                                                                                                                                                                                                        |
| `custodyproof/`           | Directory | Top-level Go package for decoding and verifying custody and bank-state proofs, including retained historical proof formats.                                                                                                                                                                                                  |
| `daemon/`                 | Directory | Production paxd command entrypoint and node lifecycle commands.                                                                                                                                                                                                                                                              |
| `deploy/`                 | Directory | Deployment descriptors and launchpad production topology and observability material.                                                                                                                                                                                                                                         |
| `docker/`                 | Directory | Container builds, compose files and container deployment support.                                                                                                                                                                                                                                                            |
| `docs/`                   | Directory | Architecture and component guides, wiki, documentation site, translations and engineering records.                                                                                                                                                                                                                           |
| `engine/`                 | Directory | EVM execution engine packages integrated into the Go node.                                                                                                                                                                                                                                                                   |
| `foundry.paxeer.toml`     | File      | Separate Paxeer chain Foundry project: contracts/src sources, contracts/test tests, contracts/lib dependencies, contracts/out artifacts, forge-std/OpenZeppelin remappings and Solidity 0.8.27. Select with FOUNDRY_CONFIG=foundry.paxeer.toml.                                                                              |
| `foundry.toml`            | File      | Root kernel-contract Foundry project: contracts sources excluding the separate Paxeer, bridge and launchpad trees; Solidity 0.8.27 with optimization and via_ir, build/forge-artifacts output and build/forge-cache cache. Names and governance profiles select their own tests; make test-contracts selects tests/solidity. |
| `go.mod`                  | File      | Root Go module identity, Go toolchain and dependency requirements for node and top-level Go packages.                                                                                                                                                                                                                        |
| `go.sum`                  | File      | Go dependency checksums corresponding to go.mod.                                                                                                                                                                                                                                                                             |
| `include/`                | Directory | Public C17 LayerX kernel headers under include/layerx, including the ABI and module interfaces.                                                                                                                                                                                                                              |
| `interchain/`             | Directory | IBC protocol, client, channel and transfer packages for the node.                                                                                                                                                                                                                                                            |
| `launchpad/`              | Directory | Launchpad production system specification under launchpad/spec; implementation services live in services/launchpad and clients in apps/launchpad.                                                                                                                                                                            |
| `layerxproof/`            | Directory | Top-level Go packages for proof codecs, portable exports and receipt verification, with test vectors and explicit retirement guards.                                                                                                                                                                                         |
| `migrations/`             | Directory | C-kernel SQLite schemas, owned by retained kernel tooling rather than an off-chain service: 0001_projection.sql is loaded by src/storage/lxp_projection.c; 0001_genesis_sections.sql and 0007_history_index.sql retain historical schemas. Three SQL files and README.md are tracked here; see the migration guide.          |
| `modules/`                | Directory | Native chain modules and the modules/lxphost Go/cgo host for the embedded C kernel.                                                                                                                                                                                                                                          |
| `node/`                   | Directory | Go application wiring, integrated sequencer, block proposal, upgrades and chain-state execution.                                                                                                                                                                                                                             |
| `package-lock.json`       | File      | Tracked npm dependency lockfile for the root package.json workspaces.                                                                                                                                                                                                                                                        |
| `package.json`            | File      | Declares npm@11.19.1 as the root package manager, using package-lock.json; lists npm SDK, middleware and example workspaces. pnpm-workspace.yaml and pnpm-lock.yaml belong to the pnpm sub-projects, not this root.                                                                                                          |
| `precompiles/`            | Directory | Native EVM precompiles and ABI definitions, including ledger, payments, authority and Programs interfaces.                                                                                                                                                                                                                   |
| `programs/`               | Directory | The kernel Programs WASM tree: deterministic Rust guest runtime, registry, C/Rust bridge, SDKs, reference programs and sandbox linked into paxd. Distinct from the CosmWasm module and its VM wrapper.                                                                                                                       |
| `rpc/`                    | Directory | Node JSON-RPC server, methods and compatibility tests.                                                                                                                                                                                                                                                                       |
| `rust-toolchain.toml`     | File      | Pinned Rust toolchain and components for the Rust workspaces.                                                                                                                                                                                                                                                                |
| `scripts/`                | Directory | CI, release, qualification, protobuf generation and workspace scripts.                                                                                                                                                                                                                                                       |
| `sdk/`                    | Directory | Cosmos SDK framework fork plus client SDKs and integrations, including launchpad, wallet and chainflow packages.                                                                                                                                                                                                             |
| `services/`               | Directory | Off-chain services: agent, interop, hosted platform, launchpad, explorer, human control plane, embedded wallet and bridge deployment; some own pnpm workspaces.                                                                                                                                                              |
| `spec/`                   | Directory | Feature requirements, acceptance criteria, dependency-ordered tasks and the workflow source.                                                                                                                                                                                                                                 |
| `src/`                    | Directory | C17 LayerX kernel implementation: deterministic execution, modules, storage and supporting runtime code embedded through modules/lxphost.                                                                                                                                                                                    |
| `storage/`                | Directory | Go database and storage implementations used by node packages.                                                                                                                                                                                                                                                               |
| `tests/`                  | Directory | Shared C kernel, Solidity, integration, CI regression, fixture and vector suites; packages also keep local tests.                                                                                                                                                                                                            |
| `tools/`                  | Directory | Developer, operator, release, launchpad and platform tools, including Hyperfide and pinned tool snapshots; tools/hyperfide owns its pnpm workspace.                                                                                                                                                                          |
| `utils/`                  | Directory | Shared Go utility packages used by the node and its components.                                                                                                                                                                                                                                                              |
| `wasm/`                   | Directory | CosmWasm chain module and application integration in Go, including wasm/x/wasm and protobuf sources; executes contracts through wasm-runtime.                                                                                                                                                                                |
| `wasm-runtime/`           | Directory | CosmWasm VM wrapper: Go APIs and cgo bindings backed by the Rust libwasmvm library. Distinct from the wasm chain module and kernel Programs runtime.                                                                                                                                                                         |

The root uses npm and `package-lock.json`. Each pnpm sub-project keeps its own
`pnpm-workspace.yaml` and `pnpm-lock.yaml`, for example
[`apps/launchpad/`](../apps/launchpad/), [`apps/perps/`](../apps/perps/),
[`services/wallet/`](../services/wallet/) and
[`tools/hyperfide/`](../tools/hyperfide/). Neither pnpm file is tracked at the root.

The three WASM-related trees serve distinct layers: `wasm/` integrates CosmWasm
with the chain, `wasm-runtime/` binds the CosmWasm VM, and `programs/` hosts the
kernel's deterministic WASM programs. For the retained SQL schemas and their
consumers, see [`migrations/README.md`](../migrations/README.md).

## Target groups

| Group        | Holds                                                                                            |
| ------------ | ------------------------------------------------------------------------------------------------ |
| `apps/`      | User-facing frontends                                                                            |
| `services/`  | Off-chain programs                                                                               |
| `sdk/`       | Client and guest SDKs                                                                            |
| `core/`      | The chain: kernel, node packages, framework fork, programs runtime and pinned upstream libraries |
| `tests/`     | Shared test trees and fixtures; packages also retain local tests                                 |
| `tools/`     | Developer and operator tools                                                                     |
| `contracts/` | On-chain sources                                                                                 |

`api/`, `deploy/`, `docker/`, `docs/`, `scripts/`, `spec/` and the root build and community files stay at the root.

## Current tree

| Path                                                              | Contents                                                                                                                                                                                              |
| ----------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `apps/perps`                                                      | The Hyperpax perps frontend                                                                                                                                                                           |
| `apps/wallet`, `apps/web`                                         | Wallet app and web app                                                                                                                                                                                |
| `apps/explorer`                                                   | Block explorer frontend                                                                                                                                                                               |
| `apps/inference-peer`                                             | Standalone inference peer package with the pinned MIT Pooled WebGPU engine, room transport, persistent Ed25519 identity and measured v2 manifests; see its `README.md`, `NOTICE` and `LICENSE-ENGINE` |
| `apps/bridge`                                                     | Bridge frontend                                                                                                                                                                                       |
| `apps/launchpad`                                                  | Launchpad web and mobile apps and the client workspace files                                                                                                                                          |
| `services/agent`                                                  | Agent daemon, MCP server, shared Rust crates, TypeScript and Python SDKs                                                                                                                              |
| `services/interop`                                                | Interop gateway, bridge relayer and agent-commerce services                                                                                                                                           |
| `services/platform`                                               | Hosted developer platform, middleware, perps data and ramps                                                                                                                                           |
| `services/explorer`                                               | Block explorer backend, services and deployment                                                                                                                                                       |
| `services/launchpad`                                              | Launchpad services                                                                                                                                                                                    |
| `services/human`                                                  | Human control plane Rust workspace (crates, schema, tools)                                                                                                                                            |
| `services/wallet`                                                 | Embedded wallet service: attestor, ceremony, gateway, demo, schema, deployment and runbooks                                                                                                           |
| `services/bridge`                                                 | Bridge deployment                                                                                                                                                                                     |
| `sdk/platform`, `sdk/integrations`                                | Platform SDKs (Go, JVM, .NET, Swift) and framework integrations                                                                                                                                       |
| `sdk/launchpad`                                                   | Launchpad protocol, shared libraries and client packages                                                                                                                                              |
| `sdk/chainflow`                                                   | Chainflow client libraries for Go, Python, Rust and TypeScript                                                                                                                                        |
| `sdk/js`                                                          | First-party `@pax-js/cosmos` package, edited in place and aligned to the protos of this repository                                                                                                    |
| `sdk/wallet`                                                      | Wallet SDK (`@paxeer/wallet`), a member of the `services/wallet` pnpm workspace                                                                                                                       |
| `daemon/paxd`, `node`, `rpc`                                      | Node entrypoint, application wiring and node JSON-RPC                                                                                                                                                 |
| `src`, `include`, `modules/lxphost`                               | C kernel implementation and headers, and its Go/cgo host inside `paxd`                                                                                                                                |
| `modules`, `precompiles`, `programs`                              | Chain modules, EVM precompiles and programs runtime                                                                                                                                                   |
| `core/deps`                                                       | Pinned snapshots of the upstream libraries the node builds from, each with a `VENDOR.kvx` pin record                                                                                                  |
| `tests/`                                                          | Kernel C tests, `tests/fuzz`, `tests/integration`, `tests/occ`, `tests/testutil`, `tests/fixtures`, Solidity and vector suites                                                                        |
| `tools/hpx`, `tools/platform`, `tools/launchpad`                  | Node installer and registry, platform CLI, emulator and release tooling, launchpad tools                                                                                                              |
| `tools/loadtest`, `tools/paxctl`, `tools/uci`, `tools/wasm-tools` | Pinned tool snapshots with `VENDOR.kvx` pin records                                                                                                                                                   |
| `contracts/`                                                      | Root Foundry sources; `contracts/src` and `contracts/test` use `foundry.paxeer.toml`                                                                                                                  |
| `contracts/bridge`                                                | Bridge vault contracts for EVM chains, the Solana program and test vectors                                                                                                                            |
| `contracts/launchpad`                                             | Launchpad contracts                                                                                                                                                                                   |
| `deploy/`                                                         | Deployment descriptors, including `deploy/launchpad`                                                                                                                                                  |

## Inference peer room transport

[`src/identity.mjs`](../apps/inference-peer/src/identity.mjs) provides
`PeerIdentity`, `measureManifest` and `signHello`. Open an identity with
`await PeerIdentity.open()`: Node uses an owner-only PKCS#8 file at
`~/.paxeer/inference-peer/identity.pkcs8` (override with `{ path }`), and browsers
store a nonextractable Ed25519 private key in IndexedDB (override with
`{ databaseName }`). `{ mode: "browser" | "native" }` can select the runtime
explicitly. Native storage requires POSIX file permissions and rejects symlinks,
exposed keys and damaged keys; concurrent creation publishes one complete file
without replacing an existing identity. Storage failures surface to the caller.

`await measureManifest({ manifestV1, regionCode, probeUrl })` takes the canonical
v1 manifest bytes and returns v2 bytes. It preserves every v1 field after the
version and appends the chain's 13-byte capability layout, in network byte order:
u32 accelerator MiB, u32 system MiB, u8 bandwidth class, two ASCII region letters,
u8 attestation family (default 255, none), and u8 runtime (browser 0, native 1).
The total stays within 8192 bytes. System memory comes from
`navigator.deviceMemory` in browsers or `os.totalmem()` in Node. Accelerator memory
uses the WebGPU adapter's `maxBufferSize` limit, an allocation limit rather than
total physical VRAM; a runtime without the WebGPU API advertises zero. An exposed
GPU without an adapter, or unavailable browser system memory, fails explicitly.
The probe downloads 64 KiB–1 MiB without redirects, compression or caching, within
two seconds. Classes 1–4 mean below 10 Mbit/s, below 100 Mbit/s, below 1 Gbit/s,
and at least 1 Gbit/s. Probe failures are errors. Endpoints require HTTPS or HTTP
loopback; `timeoutMs` may shorten the two-second limit.

`await signHello(identity, { roomId, nonce, manifest })` accepts these v2 bytes
and returns the payload for `SignalClient`. Omitting `nonce` generates a UUID;
hosts should supply and consume a fresh challenge nonce for replay prevention.
`peerKey`, `signature` and binary manifest fields use lowercase hex. The signed
manifest contains `{ version: 2, bytes, metadataDigest, peerKey, signature }`;
its digest is SHA-256 of `PAXAI/participant-metadata/v2`, a zero byte and the exact
manifest bytes. Its Ed25519 signature signs that digest. The hello signature
signs `PAXAI/peer-hello/v1`, a zero byte, the room's u32 UTF-8 byte length and
bytes, the nonce's u32 UTF-8 byte length and bytes, then the 32-byte manifest
digest. `helloSigningBytes` exposes that preimage for verifiers. Receivers must
verify both signatures, recompute the manifest digest and check the key against
chain admission before accepting a peer.

`await identity.enrolPeer(enrolUrl, { document, headers })` POSTs one enrolment
JSON document to `/enrol_peer` on the host signalling listener. The document
contains hex `manifest_v1`, `manifest_v2`, owner-signed native transactions
`enroll`, `accept`, `publish`, `lock_bond`, `post_bid`, and block-height `expiry`.
The host shares its self-enrolment parser, checks chain/program/model/runtime and
expiry before submission, then relays the five records in order through
`EnrolmentRelay::enrol_peer`. It holds no peer signing key. Content-Length is
required and checked against 262144 bytes before reading the JSON body.
Success returns `enrolled`, `manifest_digest`, `acceptance`, `bond_lock` and `bid`;
refusals carry stable JSON reasons and HTTP statuses, including 503
`enrolment_not_configured` when the host lacks its enrolment settings.

CLI `join --enrolment <signed-json-file>` forwards the file before the WebSocket
hello. Omitting the option sends no enrolment request; normal hello admission
still applies. PXVE attestation checks remain local and no `bind_peer` request
is sent. Both sides consume `apps/inference-peer/test/fixtures/host-enrol-route.json`.
Preparing these signed documents for peer owners outside a proof remains open;
the CLI cannot derive native market transactions from its Ed25519 hello identity.

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

| Field                   | Encoding                                                                                                      |
| ----------------------- | ------------------------------------------------------------------------------------------------------------- |
| Kind                    | u16; hello=1, offer=2, deal=3, deal-ack=4, activation=5, lap-receipt=6, countersign=7, canary-answer=8, bye=9 |
| Body length             | u32, excludes the six-byte kind/length header                                                                 |
| Room ID length, room ID | u16 byte length followed by nonempty UTF-8 text                                                               |
| Sequence                | u32                                                                                                           |
| Payload                 | Raw bytes for activation; a UTF-8 JSON object for every other kind                                            |

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

## Inference peer deal execution

[`src/deal.mjs`](../apps/inference-peer/src/deal.mjs) exports `DealClient`,
`loadLayerRange`, `runActivation`, the activation codec and `createDenseRuntime`.
Construct a client with `{ roomId, identity, offer, runtime, mesh, hostPeerKey,
validateDeal, acknowledge }`. Room IDs and peer keys are lowercase 32-byte hex.
The mesh provides `send(peerKey, kind, payload)` and `frame` events containing
`{ peerKey, frame }`, as `PeerMesh` does. Feed signalling deals through
`client.handleFrame(frame)`; mesh activations and bye frames are consumed
through the mesh listener. `validateDeal(deal)` must explicitly return `true`
after the application checks the chain authority and agreed terms. The host
peer key is supplied separately from the plan's host owner identity.

The deal payload is the Rust host's `DealFrame` (or `{ deal: DealFrame }`), with
`plan.layers` assigning contiguous `[first_layer, end_layer)` ranges. Digests
accept the host's 32-byte arrays or lowercase hex. The offer must carry matching
`model_digest`, `runtime_digest`, and
`model_source: { manifest_url: "https://.../manifest.json" }`. The model digest
is SHA-256 of the exact manifest response bytes. Manifest version 1 has
`layer_count`, `hidden_size`, `format`, and ordered `shards`, each with `path`,
`first_layer`, `end_layer`, byte `size` and `sha256`. Shards partition all layers;
assignment boundaries must align with shard boundaries. The client downloads
only the manifest and shards wholly inside its range, bounds each response and
verifies its digest before handing any weights to the runtime. Sources require
HTTPS (HTTP loopback for local use); redirects, cross-origin shards, incomplete
coverage and shards crossing an assignment boundary refuse explicitly.

After verification, `acknowledge` receives the host's signed JSON payload
`{ type: "deal_ack", deal_id, digest, signature }` with byte arrays. The signature
covers `PAXAI/inference-deal-ack/v1`, a zero byte and the SHA-256 deal digest,
using the exact field ordering and network-byte-order integers of the Rust
`DealFrame::digest`. Unsafe JavaScript integers are refused; use decimal strings
for integer values beyond JavaScript's exact range before decoding a host JSON
message. Prices remain canonical decimal strings.

Activation payload version 1 is binary: u8 version, u16 request-ID byte length,
UTF-8 request ID (1–256 bytes), 32-byte deal digest, u32 token position, u16 next
layer index, u32 float count, then finite float32 values, all in network byte
order. `encodeActivation` and `decodeActivation` produce and consume this payload
inside the existing bounded room frame. One payload holds one token's hidden
vector; oversized vectors require a separate chunking protocol and are refused.
The first peer receives embeddings from `hostPeerKey`; intermediate results go
to the next assigned peer, and the last peer returns hidden vectors to the host
for its head and token selection. Sender, deal, layer and dimensions are checked.
Up to 32 positions of unordered arrivals are buffered per request; duplicates
and larger gaps refuse. At most 64 requests are active. Each request owns its
cache and position. Host bye with `{ requestId }` drops that request's cache
and buffered activations and prevents its reuse; room bye or mesh closure drops
all caches. The existing `PeerMesh` closes on any bye, so request-specific bye
through that transport also closes the mesh and clears all requests.

`createDenseRuntime({ device, maxSeq })` runs the vendored `DenseEngine` on an
existing WebGPU device. Its manifest format is `safetensors-dense-v1` and includes
`config` with the matching hidden size and layer count. It accepts only tensors
for the assigned layers, disables embedding and head loading, and saves separate
KV slots for each request. Other runtimes implement `loadLayerRange` returning
`createCache`, `runActivation`, `dropCache`, and `close`; the caller supplies the
runtime explicitly. GPU execution requires WebGPU and its own integration gate.

The focused deal test uses the executable CPU attention model in
[`tests/scripted_model.mjs`](../apps/inference-peer/tests/scripted_model.mjs),
real HTTP shard downloads, real Ed25519 identities and encoded room frames over
loopback TCP sockets. It checks a two-peer split against single-process hidden
vectors and tokens, signed acknowledgements, digest refusals, per-request cache
isolation, unordered delivery and bye cleanup. It does not qualify WebRTC or a
live chain host. The current Rust `RoomOffer` has no model-source field, its
listener uses JSON signalling envelopes, and `SignalClient.acknowledgeDeal`
sends an unsigned `{ dealId }` payload. Those existing interfaces require host
and signalling integration before this client can join a live room; a caller
must transport the signed acknowledgement explicitly rather than route it
through the unsigned acknowledgement method.

## Native inference peer CLI

Run the native peer directly with Node.js 22 or newer:

```sh
node apps/inference-peer/bin/inference-peer.mjs join https://host.example ROOM_HEX \
  --manifest /private/participant-v2.bin --nonce HOST_CHALLENGE
node apps/inference-peer/bin/inference-peer.mjs status
PAXAI_ASK_BEARER=ASK_STREAM_TOKEN node apps/inference-peer/bin/inference-peer.mjs serve \
  --host https://host.example
```

[`src/cli.mjs`](../apps/inference-peer/src/cli.mjs) exports `parseCLI`,
`joinCommand`, `serveCommand`, `statusCommand` and `runCLI`. `join` takes the host
base URL and a lowercase 32-byte room ID. `--manifest` is the canonical binary
v2 manifest with native runtime kind 1; create and measure it through the identity
API above. `--identity` overrides the persistent owner-only identity file and
`--nonce` supplies the host's challenge. The host must consume that challenge.
The command POSTs the signed manifest to `enrol_peer`, then sends the signed
hello over the existing binary room codec to `rooms/ROOM_HEX/signal` (WebSocket).
Host base path prefixes are retained. Hosts require HTTPS; HTTP is accepted only
on IP loopback. Redirects and HTTP refusals fail explicitly.

To bind an attestation, add `--attestation /private/quote.pxve`,
`--model-digest MODEL_HEX` and `--runtime-digest RUNTIME_HEX`. The file contains
the complete signed binary PXVE development envelope accepted by `bindPeer` at
`0x0907`, including its 96-byte header and 65-byte signatures. The CLI checks
its family/version/length and compares bytes 39–70 against
`keccak256(peerKey || modelDigest || runtimeDigest)` **before any network send**.
A matching report is POSTed to `bind_peer` as
`{ family, envelope, peerKey, modelDigest, runtimeDigest }`, with lowercase hex
byte fields. The host must submit `bindPeer` and return `{ "bound": true }`
only after successful chain execution. Only then does the CLI enrol and send
hello. Local matching does not verify attestor trust, signatures, freshness or
hardware evidence; these remain chain checks. JSON claims about report data
cannot replace the bytes inside the envelope.

Deal execution additionally requires `--deal /private/accepted-deal.json` and
`--host-key HOST_PEER_HEX`. The file is the complete Rust `DealFrame` whose
chain authority and terms the operator has already checked. Its exact deal
digest pins incoming terms; it also supplies attestation model/runtime digests
when their flags are omitted. Execution refuses a deal without this pin and
host key. The host's binary deal payload must contain `{ dealId, deal }`:
`deal` is the full frame, and `dealId` is the signalling acknowledgement ID.
Its offer must include the matching model/runtime digests and model-source
manifest URL described above. The CLI verifies/downloads its assigned range
through `DealClient`, POSTs the signed acknowledgement to
`rooms/ROOM_HEX/deal_ack`, then uses `PeerMesh` in native mode for WebRTC.
Provision `node-datachannel` and its native binary so it resolves from this
package; it is required when a deal starts and is not bundled by the CLI.

When the optional `webgpu` package resolves, native execution uses its adapter
and device with the vendored `DenseEngine`. An installed but broken GPU backend
fails explicitly. If the package is absent, the CPU dense path uses the vendored
safetensors parser/converter and executes RMSNorm, grouped causal attention,
RoPE, residuals and SiLU feed-forward layers. Both paths keep per-request KV
caches and leave embeddings and the head on the host. CPU support covers
`safetensors-dense-v1` with F32/BF16 tensors, standard RoPE and SiLU; unsupported
formats, extra bias/head-normalisation tensors and scaled RoPE refuse. It is a
correctness path for compatible models, not a throughput guarantee.

`join` prints a JSON snapshot and keeps a loopback status listener on port 8788;
`--status-port` overrides it (0 chooses a free port). `status --port PORT` reads
`GET /status` and prints room, peer key, state, backend, assigned layer range,
processed activation count and any error. No signing key or bearer is exposed.
SIGINT/SIGTERM closes the signalling connection, mesh, model caches and local
listener. An asynchronous transport or execution failure closes the session and
makes the CLI exit unsuccessfully.

`serve --host URL` listens on `127.0.0.1:8787`; `--port` overrides it. It forwards
`POST /v1/chat/completions` and `POST /v1/completions` to those same paths below
the host base URL, replacing incoming authorization with the ask bearer from
`PAXAI_ASK_BEARER` or `--ask-bearer`. The host must support the requested route.
Requests are bounded to 1 MiB. Responses preserve upstream status, content type,
retry hints and bytes, including SSE chunks and `[DONE]`, with backpressure.
Disconnecting the local client cancels the upstream request. Host refusals such
as 402 remain refusals; unavailable hosts return 502. Other local routes return 404. The bridge does not open or fund an ask stream.

The task gate is `node --test apps/inference-peer/test/cli.test.mjs`. It exercises
the executable's signed hello over real loopback HTTP/WebSocket, identity-bound
PXVE refusal before any send, binding-before-enrolment ordering, a separate
status process, CPU dense attention/cache isolation and a streamed completion
against a stand-in host. The existing Rust host still lacks the binding and
enrolment HTTP adapters and uses different signalling envelopes, as noted
above. These tests do not qualify a live chain, native WebRTC/GPU execution,
lap countersigning or canary integration.

## Inference peer browser page

[`apps/inference-peer/index.html`](../apps/inference-peer/index.html) loads
[`src/page.mjs`](../apps/inference-peer/src/page.mjs) as a plain ES module.
Start its loopback static server from the repository root with
`npm --prefix apps/inference-peer run serve`, then open
`http://127.0.0.1:4173`. `PORT` overrides 4173. The server supplies JavaScript
module MIME types, cross-origin isolation headers and a bounded 64 KiB `/probe`
for manifest measurement. It serves only the page, `src/` and `engine/`;
participant keys, package metadata and local tests are not served.

The page opens the existing browser IndexedDB identity, displays the WebGPU
adapter, and measures a v2 manifest from the operator's canonical v1 hex bytes,
two-letter region and bandwidth probe URL. Browser measurement requires
`navigator.deviceMemory`; a failed probe or unavailable measurement is shown
explicitly and prevents joining with stale capabilities. Paste the host's
`ws://` or `wss://` signalling origin (request target `/`) and 32-byte room ID
as 64 lowercase hex characters after measurement. `SignalClient` waits for
the host challenge and signs its hello with the persisted Ed25519 identity.
Signalling uses bounded JSON text frames from `src/signal.mjs`.

`PeerPage` and `renderStatus` display the host offer, the assigned layer range
with an exclusive end, the deal, `deal_live` and the exact host error code.
The offer/deal price is shown as an exact string, with no earnings calculation.
The host's `ServerFrame` in
`services/agent/crates/layerx-agentd/src/inference_market/signal.rs` and
`DealServerFrame` in its `deals.rs` publish no status frame. Consequently the
page's `#laps`, `#receipts` and `#earnings` fields explicitly display unavailable
state. No countersigned receipt or income is inferred from a received deal.
The page does not automatically accept a deal: the browser still lacks the
chain validation and trusted model-source/host-key integration required to
acknowledge it and start GPU work. `renderStatus` can display `deal_live` once
an acknowledged session supplies it; the page does not fabricate acceptance.

Leaving, pagehide and unload close the signalling socket once. The host accepts
no signalling `bye` (the `ClientFrame` and `DealClientFrame` enums have no such
variant); the old page's signalling-bye requirement therefore has no producer
interface at `PeerPage.leave`. Data-channel bye remains in the mesh protocol,
which this page does not start before deal validation.

The jsdom test uses the declared, exactly pinned development dependency. It
exercises persisted identity reopening, challenge-first signed hellos over real
loopback WebSockets using the existing host-frame vectors, measured memory and
a real HTTP probe, offer/deal/error rendering, the `deal_live` renderer, unload
closure and static-server boundaries. It uses the existing native identity
store because jsdom has no IndexedDB or WebGPU. It does not qualify browser GPU
execution, chain admission, deal acceptance or lap countersigning.

## Trees that move under core in a later release

These are planned directory moves, not additional deployments. In particular, the current kernel sources still live at the repository root.

| Current path                                                                                                                                                               | Destination       |
| -------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----------------- |
| `src`, `include`, `cmd`, `migrations`                                                                                                                                      | `core/kernel`     |
| `sdk` framework fork entries                                                                                                                                               | `core/cosmos-sdk` |
| `consensus`, `interchain`, `wasm`, `wasm-runtime`, `storage`, `modules`, `precompiles`, `node`, `rpc`, `engine`, `daemon`, `admin`, `custodyproof`, `layerxproof`, `utils` | `core/<name>`     |
| `programs/sdk`                                                                                                                                                             | `sdk/programs`    |
| `programs`                                                                                                                                                                 | `core/programs`   |

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

## Inference peer lap countersignatures

[`src/laps.mjs`](../apps/inference-peer/src/laps.mjs) exports `LapTracker`,
`countersignLap` and the program-compatible receipt and countersign digest
functions. Construct a tracker with `{ identity, deal, hostMeterKey }`, using
the accepted deal and independently trusted host meter key. Pass it to
`DealClient` as `lapTracker`, with `countersign: ack => signal.countersignLap(ack)`.
Forward `lap_receipt` signalling events to `DealClient.handleFrame`, alongside
`deal` and `deal_live`. The client records each successfully forwarded encoded
output activation in processing order. A range digest is SHA-256 of the
concatenated activation payloads for that lap; the host supplies these digests
through `LapInput.layer_range_digests`, in deal-plan order.

The tracker starts at lap zero and checks the deal binding, pinned meter key,
host signature, deadline, continuous lap index, its assigned range digest and
local activation count. Cumulative tokens include prompt tokens on the first
lap and advance only by generated tokens thereafter. Refusals preserve the
activation evidence and return `{ type: "refusal", reason }` locally, surfaced
as `lap-refusal` events. The host has no refusal message on its socket. Identical
receipt retransmissions return the cached countersignature without advancing
accounting; conflicting retransmissions refuse. Activations already processed
for a later lap remain buffered, up to a total of 64 MiB. Tracker state is
in memory and requires the original processing evidence to countersign.

The socket carries JSON text `{ type: "lap_receipt", lap }` and
`{ type: "lap_countersign", deal_id, lap_index, digest, signature }` through
`src/signal.mjs`, using the Rust host's exact snake-case members and byte arrays.
The peer signs the SHA-256 digest of `PAXAI/lap-countersign/v1`, a zero byte,
deal digest, host meter key, big-endian deadline and program receipt digest.
The program receipt encoding and its two hash domains match task 22.38.4;
the binary lap kinds in `src/wire.mjs` are not sent on signalling sockets.

[`test/laps.test.mjs`](../apps/inference-peer/test/laps.test.mjs) exercises these
checks with the fixed vectors in `test/fixtures/host-lap-frames.json`, the real
signalling client over loopback WebSockets, encoded activations over TCP and
the CPU model's assigned layers loaded from verified HTTP shards. Its fixture
recipe is `test/fixtures/generate-host-lap-frames.mjs`; it independently encodes
the Rust layouts, including `f11_lap_settlement.rs::batch(0, 1, 0)`.
The declared gate runs `node --test apps/inference-peer/test/laps.test.mjs`.

## Inference peer canary answers

[`src/canary.mjs`](../apps/inference-peer/src/canary.mjs) provides
`encodeFixedPointLogits`, `CanaryTracker` and `canaryAnswer`. Encoding selects
the eight greatest finite logits in descending order, rounds to Q16 with
halfway values away from zero, and writes eight signed big-endian i32 values
in the program's `canary::encode_logits` layout. Values that overflow i32 refuse.
The fixture uses the program's `f11_canary_challenge.rs::EXPECTED` vector.

Construct a tracker with `{ identity, deal }` and record actual model-head
output with `recordLogits({ lapIndex, position, logits })`. The tracker copies
the encoded values, retains at most 8192 positions, refuses conflicting
records and clears them on `close()`. `canaryAnswer(plan, tracker)` consumes
the host's existing `CanaryPlan` payload, checks the deal, selected peer,
frozen binding, lap and prompt context, and returns its `CanaryAnswer` payload.
It signs the SHA-256 domain `PAXAI/canary-answer/v1` over the 248-byte host
context and the `PAXAI/canary-logits/q16/v1` digest. Unknown positions return
`{ type: "refusal", reason: "unknown_position" }` locally.

The signalling codec accepts the host's text JSON frames
`{ "type": "canary_request", "plan": ... }` and
`{ "type": "canary_answer", "answer": ... }`, matching `LapServerFrame` and
`LapAcknowledgement` in the daemon's `inference_market/laps.rs`. The socket
client calls `canaryAnswer` for the requested peer key and sends only successful
answers. Refusals, including unknown positions, emit a local `canary-refusal`
event and send no frame. A successful answer emits `canary`. The binary canary
kind in `src/wire.mjs` is not used on the signalling socket.

`SignalClient` creates a tracker when a deal arrives and clears it when the
next deal arrives or the socket closes. A caller with an already bound deal
can supply `{ canaryTracker }` at construction; it must use the socket's peer
identity. Record outputs with `signal.recordCanaryLogits({ lapIndex, position,
logits })`. The logits come from the runtime of the peer. Hidden layer
activations are not logits; a runtime that has not supplied actual model-head
logits leaves the position unknown and the request refuses locally. Canary
positions use the ordinary activation path without a special binary request.

[`test/fixtures/host-canary-frames.json`](../apps/inference-peer/test/fixtures/host-canary-frames.json)
keeps the host's producer-defined vectors unchanged and carries the program
encoding vector in `program_codec`. The declared `test/canary.test.mjs` gate
checks encoding, signing, refusals, strict frame members and both directions
of a real loopback WebSocket exchange against those vectors.
