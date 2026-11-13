# Inference peer

This directory holds the standalone ES module package `@paxeer/inference-peer`,
its pinned Pooled WebGPU engine, browser page and native CLI. Peers provide
compute to the Paxeer inference market: the runtime executes assigned model
layers over WebRTC and countersigns contributed laps, while the host keeps
embedding and head and the chain verifies records and settles payment. Paxeer
identity, signalling, transport and deal integration live in `src/`.

## Layout

- [bin/](bin/): native CLI entry point, `inference-peer.mjs`.
- [engine/](engine/): vendored Pooled engine, WGSL shader modules, room and harness helpers, and tokenizer reference data.
- [scripts/](scripts/): browser development server, `serve.mjs`.
- [src/](src/): identity, signalling, binary wire codec, WebRTC transport, deal execution, laps, canaries, browser page and CLI integration.
- [test/](test/): Node regression tests, host signalling support and protocol fixtures in `fixtures/`.
- [tests/](tests/): scripted attention model and runtime used by the deal tests.
- [index.html](index.html): browser peer page.
- [package.json](package.json): package exports, Node requirement, dependency and script declarations.
- [package-lock.json](package-lock.json): npm dependency lockfile.
- [LICENSE-ENGINE](LICENSE-ENGINE): complete MIT notice for the vendored engine.
- [NOTICE](NOTICE): engine source pin and SHA-256 hashes.

### Engine and package exports

The `engine/` directory vendors [Pooled](https://github.com/Nehanth/pooled) at
commit `45b5ee275a8754a37fce4a6ce174f31e4a216ef4` under the MIT licence.
[LICENSE-ENGINE](LICENSE-ENGINE) preserves the complete upstream copyright and
permission notice; [NOTICE](NOTICE) records the source pin and SHA-256 hashes.

The snapshot preserves upstream paths beneath `engine/`:

| Path                                        | Vendored contents                                                                                                                                                   |
| ------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `engine/engine/`                            | Complete WebGPU engine: dense and Qwen hybrid engines, safetensors and quantised GGUF loaders, tokenizer, quantisation, generation loop, worker and storage helpers |
| `engine/engine/wgsl/`                       | WGSL shaders exported by upstream JavaScript modules                                                                                                                |
| `engine/room/`, `engine/harness/`           | Transitive helpers used by the upstream generation loop, including sampling, conversation and layer planning                                                        |
| `engine/tests/unit/tokenizer_llamacpp.json` | Upstream reference corpus, vocabulary subsets and token IDs generated using llama.cpp                                                                               |

Every vendored JavaScript file differs only by its first-line origin comment.
The fixture is byte-identical to upstream. Local runtime integration lives in
`src/`, outside the vendored snapshot.

The package manifest, regression tests and integration documentation are covered
by the repository's Apache-2.0 licence. Paxeer identity, signalling, WebRTC room
transport, deal execution, browser UI and CLI integration live in [src](src/).
The copied upstream helpers retain Pooled's wire and model assumptions.

The package entry point exports upstream's public API. Additional exports
include `@paxeer/inference-peer/dense`, `/qwen35`, `/gguf`, `/tokenizer`,
`/sampling` and `/generate`. For example, the generation-loop module is
`engine/engine/generate.js`, with its original relative imports intact.

Importing the core modules and shaders and running the tokenizer needs no GPU.
Executing the vendored GPU engines needs a WebGPU device and compatible model
weights, which are downloaded separately and carry their own licences. Some
experimental Qwen4 modules also require WebGPU globals at import time. The
vendored-engine Node test imports the core engines, loaders, generation loop and
shader modules, checks the snapshot hashes, and verifies the tokenizer against
upstream's reference IDs and exact Unicode round trips. These tests exercise
imports and tokenization without running GPU inference.

## Build and test

The package runs directly as ES modules and has no build step or runtime npm
dependencies. It has its own lockfile and uses `jsdom` as a development
dependency. Use Node.js 22 or newer. From the repository root:

```sh
npm --prefix apps/inference-peer install --no-audit --no-fund --ignore-scripts
npm --prefix apps/inference-peer test
npm --prefix apps/inference-peer run serve
```

The `test` script in [package.json](package.json) runs
`node --test test/*.test.mjs`; the `serve` script runs `node scripts/serve.mjs`.
The vendored engine tests can also be selected from the repository root:

```sh
node --test apps/inference-peer/test/vendored_engine.test.mjs
```

### Browser and native entry points

From this directory, `npm run serve` serves [index.html](index.html), `src/` and
`engine/` on loopback port 4173. `PORT` overrides that port. The server sets the
cross-origin isolation headers needed by the browser runtime.

The native entry point is [bin/inference-peer.mjs](bin/inference-peer.mjs).
Invoke it with `node`; the package manifest does not register a command in `bin`.
`join` uses a supplied native v2 manifest and exposes `/status` on loopback port
8788 by default (`--status-port` overrides it). Execution requires a chain-checked
`--deal`, `--host-key` and `--model-manifest-url`.

`serve --host <host>` forwards `/v1/chat/completions` and `/v1/completions` on
loopback port 8787 by default (`--port` overrides it). It reads the host's ask
bearer through `PAXAI_ASK_BEARER` or `--ask-bearer`. `status --port <port>` reads a
running join session's status.

### Protocol fixtures

`test/fixtures/host-signal-frames.json` contains eleven directional signalling
vectors for connection setup, relay and deal acknowledgement, a deterministic
signing identity, and the exact hello and deal acknowledgement signing bytes and
signatures. Tests use real loopback WebSockets and verify hello bytes
independently of the client's signing helper.

### Room signalling

Signalling uses the host listener's WebSocket request target `/`.
[`SignalClient`](src/transport.mjs)
takes `{ roomId, identity, manifest }`: the room is 32-byte lowercase hex, the
identity is a `PeerIdentity`, and the manifest contains the raw v2 bytes. It
waits for `challenge` and signs `PAXAI/inference-signal/hello/v1\0`, the 32 room
bytes and the 32 challenge nonce bytes, then sends `hello` and expects `offer`.
The nonce comes from the host; the CLI has no `--nonce` option.

`src/signal.mjs` validates text JSON frames with the host's snake-case members
and JSON byte lists. It rejects unknown or missing members, invalid byte lengths,
binary messages and messages exceeding 65536 UTF-8 bytes. `description` and
`candidate` use `to` when sent and `from` when received; ICE fields are `sdp_mid`
and `sdp_m_line_index`. The WebRTC adapter translates those fields to browser
names. Only data channels use the binary codec in `src/wire.mjs` and its sequence
numbers.

A received `deal` wraps the host's `DealFrame`. After chain-term validation and
model loading, `DealClient` signs `PAXAI/inference-deal-ack/v1\0` followed by the
deal digest, and sends `deal_ack` with `deal_id`, `digest` and `signature` on the
same socket. Execution waits for the matching `deal_live`. An `error` rejects or
closes the session and exposes the host's `code` on the error object.

The host's `RoomOffer` provides room/model/runtime digests, layer assignments and
price. It serves no model download URL or host peer key. Configure the model
manifest locally with `--model-manifest-url` (or `modelSource.manifest_url` in
`DealClient`), and supply the existing chain-checked `--deal` and `--host-key`
inputs for CLI execution. Missing model-source configuration fails explicitly;
it is never added to the signalling offer.

The signalling listener also serves `POST /enrol_peer`. To enrol before joining,
run from the repository root:

```sh
node apps/inference-peer/bin/inference-peer.mjs join <host> <room> --manifest <v2-file> --enrolment <signed-json-file>
```

Without `--enrolment`, join sends no enrolment request; the hello still undergoes
chain admission. `--attestation` checks the PXVE binding locally before either
request. There is no `bind_peer` request or host attestation-binding operation.

The enrolment file is one JSON object containing hex `manifest_v1` and
`manifest_v2`, signed EIP-1559 transactions `enroll`, `accept`, `publish`,
`lock_bond` and `post_bid`, and numeric block-height `expiry`. Each transaction
must bind the configured chain, program and market; both manifests must match
the configured model and runtime. The host relays the five records in that order
through its enrolment journal and receipt verifier, signing nothing for the peer.
The request requires Content-Length and application/json and is limited to
262144 bytes, checked before reading the body.

`await identity.enrolPeer(enrolUrl, { document, headers })` sends the document
unchanged when supplied as text or UTF-8 bytes, or JSON-encodes an object. It
returns `{ enrolled: true, manifest_digest, acceptance, bond_lock, bid }` with
hex digests and authenticated transaction IDs. On an HTTP refusal, `enrolPeer`
throws an `Error` whose message includes the status and up to 4096 characters
of response text. The host returns JSON errors: framing/JSON errors use 400,
a missing length 411, an oversized body
413, a non-JSON content type 415, invalid documents 422, program refusals 409,
relay failures 502, and missing host enrolment settings or unavailable chain
state 503. `test/fixtures/host-enrol-route.json` is shared by host and peer tests.

The host relays the supplied transactions through its configured enrolment
service. The peer owner prepares the signed proposal, peer consent/publication
and quote records for current chain state. Host configuration is described in
the [operator guide](../../docs/agent/inference-market/operator.md).

## Configuration

| Environment variable | Reader and purpose                                                                                                                |
| -------------------- | --------------------------------------------------------------------------------------------------------------------------------- |
| `PORT`               | [scripts/serve.mjs](scripts/serve.mjs): browser development server port.                                                          |
| `PAXAI_ASK_BEARER`   | [src/cli.mjs](src/cli.mjs): host ask bearer for `serve`, also accepted through `--ask-bearer`.                                    |
| `PREFILL_MATH`       | [engine/engine/qwen35.js](engine/engine/qwen35.js): prefill arithmetic selection when running under Deno with environment access. |

The CLI reads these user-supplied files and model sources:

| Option                 | Input                                                                                                                          |
| ---------------------- | ------------------------------------------------------------------------------------------------------------------------------ |
| `--manifest`           | Native participant manifest v2 bytes.                                                                                          |
| `--enrolment`          | Signed enrolment JSON document.                                                                                                |
| `--attestation`        | PXVE attestation binding envelope, checked locally.                                                                            |
| `--deal`               | JSON deal terms checked against chain state by the caller.                                                                     |
| `--identity`           | Persistent native identity file in PKCS#8 format.                                                                              |
| `--model-manifest-url` | Model JSON manifest and its referenced weight shards; `DealClient` accepts the same source through `modelSource.manifest_url`. |

[src/identity.mjs](src/identity.mjs) stores native identity in
`~/.paxeer/inference-peer/identity.pkcs8` unless a path is supplied. Browser
identity uses the IndexedDB database `paxeer-inference-peer`.
The browser page accepts participant manifest v1 bytes, region, bandwidth probe,
signalling host and room through its form. The CLI also accepts `--model-digest`,
`--runtime-digest` and `--host-key` for its local binding checks, plus
`--status-port` and `--port` for its local listeners.

Host enrolment settings are separate from the peer's inputs. The host reads
`LAYERX_AGENT_INFERENCE_ENROLMENT` and
`LAYERX_AGENT_INFERENCE_ENROLMENT_JOURNAL`; its enrolment route returns
`enrolment_not_configured` when the service is not configured.

## Documentation

- [Inference market](../../docs/agent/inference-market/README.md): network roles and the protocol, orchestration and inference planes.
- [Protocol](../../docs/agent/inference-market/protocol.md): deals, lap evidence, canaries, payouts and bonds.
- [Operator guide](../../docs/agent/inference-market/operator.md): host configuration, enrolment and recovery.
- [Client guide](../../docs/agent/inference-market/client.md): SDK calls, completion routes, wallet and explorer surfaces.
- [Threat statement](../../docs/agent/inference-market/threats.md): attestation, model correctness, liveness and privacy.
