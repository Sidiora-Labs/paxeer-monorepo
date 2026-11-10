# Inference peer

Standalone ES module npm package for the Paxeer inference peer. It has its own
lockfile and no build step or npm dependencies. Use Node.js 22 or newer:

```sh
npm --prefix apps/inference-peer install --no-audit --no-fund --ignore-scripts
node --test apps/inference-peer/test/vendored_engine.test.mjs
```

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
The fixture is byte-identical to upstream. Do not format or edit the snapshot;
put local runtime integration in `src/`. To update it, copy the dependency
closure from one upstream commit, refresh the origin comments, NOTICE hashes
and pinned test revision together, and retain its complete MIT notice.

The package manifest, regression tests and integration documentation are our
work, covered by the repository's Apache-2.0 licence. Room networking, chain
identity, browser UI and native CLI integration belong to subsequent tasks.
The copied upstream helpers retain Pooled's wire and model assumptions; the
Paxeer room adapter must live in `src/`.

The package entry point exports upstream's public API. Additional exports
include `@paxeer/inference-peer/dense`, `/qwen35`, `/gguf`, `/tokenizer`,
`/sampling` and `/generate`. For example, the generation-loop module is
`engine/engine/generate.js`, with its original relative imports intact.

Importing the core modules and shaders and running the tokenizer needs no GPU.
Executing inference needs a WebGPU device and compatible model weights, which
are downloaded separately and carry their own licences. Some experimental
Qwen4 modules also require WebGPU globals at import time. The declared Node
test imports the core engines, loaders, generation loop and shader modules,
checks the snapshot hashes, and verifies the tokenizer against upstream's
reference IDs and exact Unicode round trips. It does not qualify GPU inference.

Signalling uses the host listener's WebSocket request target `/`. `SignalClient`
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
run `inference-peer join <host> <room> --manifest <v2-file> --enrolment <signed-json-file>`.
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
hex digests and authenticated transaction IDs. Refusals preserve HTTP status and
JSON reason: framing/JSON errors use 400, a missing length 411, an oversized body
413, a non-JSON content type 415, invalid documents 422, program refusals 409,
relay failures 502, and missing host enrolment settings or unavailable chain
state 503. `test/fixtures/host-enrol-route.json` is shared by host and peer tests.

The host enables relaying with its existing enrolment settings, including
`LAYERX_AGENT_INFERENCE_ENROLMENT` and `LAYERX_AGENT_INFERENCE_ENROLMENT_JOURNAL`;
without them the route refuses with `enrolment_not_configured`. Preparing the
signed document for a peer owner remains open: the CLI does not construct,
authorise or sign these market transactions. An owner must obtain the correct
proposal, peer consent/publication and quote records for current chain state.

`test/fixtures/host-signal-frames.json` contains all eleven directional signalling
vectors, a public deterministic test seed/key, and the exact hello and deal
acknowledgement signing bytes and signatures. Tests use real loopback WebSockets
and verify hello bytes independently of the client's signing helper.
