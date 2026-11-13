# Inference market contract: rooms, deals, laps, canaries, bonds, streams and the orchestration profile

## Scope and reading guide

The pooled inference market uses native program state and payment streams. The chain verifies, pays and connects peers: room, ask, bid, deal, lap receipt, canary, bond and payout records in the ai-market program, metered ask streams in 0x101B and peer attestation bindings in 0x0907. Orchestration runs off chain in the agent daemon profile inference-market and in the vendored peer runtime under apps/inference-peer; validators and nodes never take part in a room.

## Contents

- [f11-operations](#f11-operations)
- [manifest-v2](#manifest-v2)
- [room-records](#room-records)
- [lap-receipts](#lap-receipts)
- [canary](#canary)
- [payout-split](#payout-split)
- [bonds](#bonds)
- [placement-book](#placement-book)
- [views](#views)
- [streams-0x101b](#streams-0x101b)
- [attestation-0x0907](#attestation-0x0907)
- [px-reads](#px-reads)
- [daemon-profile](#daemon-profile)
- [signalling](#signalling)
- [completion-routes](#completion-routes)
- [peer-wire](#peer-wire)
- [peer-cli](#peer-cli)
- [sdks](#sdks)
- [explorer](#explorer)
- [wallet](#wallet)
- [qualification](#qualification)

## f11-operations

**F11 room market family (ai-market program).** Operations 0x0B01 to 0x0B0C and reads 0x0B10 to 0x0B14 of the ai-market program: OpenRoom, PostAsk, PostBid, RecordDeal, SettleLaps, ChallengeLap, ResolveChallenge, SplitPayout, LockBond, SlashBond, RefundBond, CloseRoom; RoomView, DealView, PeerView, ReceiptBatchView, BookView. Availability answers eleven families. The chain verifies and settles; it never runs inference.

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.22.inference-f11-operation-table</code> | <code>rust_pub_item</code> | [programs/crates/layerx-programs-ai-market/src/dispatch.rs](../../../programs/crates/layerx-programs-ai-market/src/dispatch.rs) | <code>pub const OpenRoom: Operation = Operation(0x0B01);</code> |
| <code>interface.22.inference-f11-errors</code> | <code>rust_pub_item</code> | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) | <code>pub const F11_ROOM_UNKNOWN: ApplicationError = application_error(0x0B01);</code> |
| <code>interface.22.inference-f11-availability</code> | <code>rust_pub_item</code> | [programs/crates/layerx-programs-ai-market/src/queries.rs](../../../programs/crates/layerx-programs-ai-market/src/queries.rs) | <code>pub fn feature_availability(state_bytes: &amp;[u8]) -&gt; CodecResult&lt;[Availability; 11]&gt;</code> |
| <code>interface.22.inference-room-profile</code> | <code>rust_pub_item</code> | [programs/crates/layerx-programs-ai-market/src/policy.rs](../../../programs/crates/layerx-programs-ai-market/src/policy.rs) | <code>pub struct RoomProfile</code> |

## manifest-v2

**Participant manifest v2.** The version-2 manifest extends the version-1 fields with `CapabilityFields`: accelerator and system memory in MiB, bandwidth class, region code, attestation family and runtime kind. The capability extension occupies 13 bytes. Metadata digests use the domain for the declared version; the decoder accepts versions 1 and 2.

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.22.inference-manifest-v2</code> | <code>file_format</code> | [programs/crates/layerx-programs-ai-market/src/workers.rs](../../../programs/crates/layerx-programs-ai-market/src/workers.rs) | <code>pub struct ManifestV2&lt;'a&gt;</code> |

## room-records

**Rooms, asks, bids and deals.** A room binds a model digest, a runtime digest, a room profile and the roster digest; asks and bids reference rooms; a deal binds a layer plan to enrolled peers and an ask reference, recorded by the host.

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.22.inference-room-record</code> | <code>rust_pub_item</code> | [programs/crates/layerx-programs-ai-market/src/rooms.rs](../../../programs/crates/layerx-programs-ai-market/src/rooms.rs) | <code>pub struct RoomRecord</code> |

## lap-receipts

**Lap receipts and settlement.** Lap receipts are countersigned off chain by the contributing peers, batched by the host and settled through sealed evidence; a batch records the cumulative token count the metered stream reads.

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.22.inference-lap-receipt</code> | <code>rust_pub_item</code> | [programs/crates/layerx-programs-ai-market/src/laps.rs](../../../programs/crates/layerx-programs-ai-market/src/laps.rs) | <code>pub struct LapReceipt&lt;'a&gt;</code> |

## canary

**Canary challenges.** Commit-reveal challenges on lap receipts compare fixed-point logits under the room’s relative tolerance. A resolved miss records a peer fault that can be used as bond-slashing evidence.

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.22.inference-canary-commitment</code> | <code>rust_pub_item</code> | [programs/crates/layerx-programs-ai-market/src/canary.rs](../../../programs/crates/layerx-programs-ai-market/src/canary.rs) | <code>pub struct CanaryCommitment</code> |

## payout-split

**Payout split.** The settled amount of a batch is credited to the reward ledger per peer in layer-weight proportions with the host and protocol shares of the room profile; peers claim through the existing CLAIM operation.

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.22.inference-payout-split</code> | <code>rust_pub_item</code> | [programs/crates/layerx-programs-ai-market/src/rewards.rs](../../../programs/crates/layerx-programs-ai-market/src/rewards.rs) | <code>pub fn apply_split_payout( ctx: &amp;CallContext, envelope: &amp;ValidatedEnvelope&lt;'_&gt;, current: &amp;[u8], next: &amp;mut [u8], scratch: &amp;mut [u8], event: &amp;mut [u8], ) -&gt; CodecResult&lt;crate::rooms::Outcome&gt;</code> |

## bonds

**Peer bonds.** A peer locks a bond for its room bid. Slashing requires recorded peer-fault evidence. RefundBond returns the remaining bond after the room is closed, its challenge window has elapsed, involved deals are settled and relevant challenges are resolved.

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.22.inference-bond-record</code> | <code>rust_pub_item</code> | [programs/crates/layerx-programs-ai-market/src/bonds.rs](../../../programs/crates/layerx-programs-ai-market/src/bonds.rs) | <code>pub struct BondRecord</code> |

## placement-book

**Placement book.** Open asks and bids per model digest inside the program state; crossing pairs are the deal inputs the off-chain profile negotiates from.

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.22.inference-placement-book</code> | <code>rust_pub_item</code> | [programs/crates/layerx-programs-ai-market/src/placement.rs](../../../programs/crates/layerx-programs-ai-market/src/placement.rs) | <code>pub struct PlacementBook&lt;'a&gt;</code> |

## views

**Fixed-width views.** Room, deal, peer and receipt batch views answered as fixed-width records that the node px_ reads decode.

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.22.inference-f11-views</code> | <code>rust_pub_item</code> | [programs/crates/layerx-programs-ai-market/src/queries.rs](../../../programs/crates/layerx-programs-ai-market/src/queries.rs) | <code>pub struct RoomView</code> |

## streams-0x101b

**Ask streams (0x101B).** An ask is funded by a metered stream whose single meter authority is the host key; a reading equals the cumulative token count of the last settled lap batch.

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.22.pc-0x101b-streamopenforask</code> | <code>precompile_method</code> | [precompiles/layerxpay/stream.go](../../../precompiles/layerxpay/stream.go) | <code>streamOpenForAsk(bytes32,address,uint256,uint8,uint256,bytes32) returns (bytes32)</code> |
| <code>interface.22.pc-0x101b-askstreamof</code> | <code>precompile_method</code> | [precompiles/layerxpay/stream.go](../../../precompiles/layerxpay/stream.go) | <code>askStreamOf(bytes32) view returns (bytes32)</code> |
| <code>interface.22.pc-0x101b-evt-askstreamopened</code> | <code>precompile_event</code> | [precompiles/layerxpay/events.go](../../../precompiles/layerxpay/events.go) | <code>AskStreamOpened(bytes32 indexed,bytes32 indexed,address)</code> |

The `bytes32` identifier returned by `streamOpenForAsk` and `askStreamOf` is the call idempotency key with its first sixteen bytes set to zero, so it fits the ask record’s u128 stream field. The assignment is in `precompiles/layerxpay/stream.go`; `node/app.go` configures inference reads from `evm.px_inference_program` and `evm.px_inference_reader`; `services/agent/crates/layerx-agentd/src/main.rs` uses `LAYERX_AGENT_CONSENSUS_ENDPOINT` for consensus reads, with the node endpoint as its default. The committed source retries historical proof rate limits at most eight times, 250 ms apart, with a one-second node request timeout and a two-second socket I/O timeout.

Inference reads in `rpc/px_account.go` use the quoted call fee; `rpc/simulate.go` evaluates their private native simulation in batch H+1 over the store pinned at committed height H. `rpc/px_inference.go` decodes the version-2 room profile and closed height.

The daemon's native historical reads retry a proof rate limit at most eight times, 250 ms apart; each native RPC request retains its thirty-second timeout.

The daemon verifies each deployment admission and replays its journal by program identifier and version.

The inference book uses the node's committed header below its latest height and authenticates the preceding store height through the configured consensus endpoint, with unprefixed hexadecimal store keys and at most eight retries of a historical proof rate limit, 250 ms apart.

Availability checks independently verify the node header at the gateway publication's height under the configured freshness bound; readiness requires that publication's exact height and block hash.

The committed-header RPC validates the chain identifier from the initialized node context on each request.

The signalling client uses node RPC independently of the gateway snapshot client. Its header, room and peer reads use the verified availability height; `px_getInferenceRoomAt` and `px_getInferencePeerAt` accept a positive committed height and apply the same program and peer-binding decoders as current-height reads. The isolated-chain room driver’s native peers retry `chain_unavailable` admission responses at most seven times, 250 ms apart, within a twenty-second admission deadline.

Human-session node readiness uses the consensus chain identifier of the configured trust root and separately requires EVM chain 125.

CPU layer planning uses the manifest's system memory; accelerator layer planning uses its accelerator memory. Deal and lap envelopes derive their protocol principal from the signer's DID; a room host's DID must bind the signer's Ed25519 public key, and evidence envelopes use their own signer's DID.

## attestation-0x0907

**Peer attestation binding (0x0907).** A verified attestation envelope is bound to a peer key, a model digest and a runtime digest; only the development envelope family is accepted today.

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.22.pc-0x0907-bindpeer</code> | <code>precompile_method</code> | [precompiles/layerxagent/teeattestor/teeattestor.go](../../../precompiles/layerxagent/teeattestor/teeattestor.go) | <code>bindPeer(uint8,bytes,bytes32,bytes32,bytes32) returns (bool)</code> |
| <code>interface.22.pc-0x0907-peerbinding</code> | <code>precompile_method</code> | [precompiles/layerxagent/teeattestor/teeattestor.go](../../../precompiles/layerxagent/teeattestor/teeattestor.go) | <code>peerBinding(bytes32) view returns (uint8,bytes32,bytes32,uint64)</code> |
| <code>interface.22.pc-0x0907-evt-peerattestationbound</code> | <code>precompile_event</code> | [precompiles/layerxagent/teeattestor/teeattestor.go](../../../precompiles/layerxagent/teeattestor/teeattestor.go) | <code>PeerAttestationBound(bytes32 indexed,uint8,bytes32,bytes32)</code> |

## px-reads

**Node root px_ reads.** The node serves seven inference `px_` methods: the five current-height reads below, plus `px_getInferenceRoomAt` and `px_getInferencePeerAt` for an explicit committed height. They decode F11 views and add ask-stream state and peer bindings.

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.22.rpc-px-getinferencerooms</code> | <code>jsonrpc_method</code> | [rpc/px_inference.go](../../../rpc/px_inference.go) | <code>px_getInferenceRooms() -&gt; room summaries</code> |
| <code>interface.22.rpc-px-getinferenceroom</code> | <code>jsonrpc_method</code> | [rpc/px_inference.go](../../../rpc/px_inference.go) | <code>px_getInferenceRoom(roomId) -&gt; room view with ask stream state</code> |
| <code>interface.22.rpc-px-getinferencedeal</code> | <code>jsonrpc_method</code> | [rpc/px_inference.go](../../../rpc/px_inference.go) | <code>px_getInferenceDeal(dealId) -&gt; deal view</code> |
| <code>interface.22.rpc-px-getinferencepeer</code> | <code>jsonrpc_method</code> | [rpc/px_inference.go](../../../rpc/px_inference.go) | <code>px_getInferencePeer(peerKey) -&gt; peer view with attestation binding</code> |
| <code>interface.22.rpc-px-getinferencereceipts</code> | <code>jsonrpc_method</code> | [rpc/px_inference.go](../../../rpc/px_inference.go) | <code>px_getInferenceReceipts(dealId) -&gt; receipt batch views</code> |

## daemon-profile

**Agent daemon profile inference-market.** LAYERX_AGENT_MODE=inference-market runs the orchestration profile off chain: enrolment, signalling, deal negotiation, lap settlement, canaries, recovery and the completion routes. It is never part of a validator or node.

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.22.inference-mode-inference-market</code> | <code>rust_pub_item</code> | [services/agent/crates/layerx-agentd/src/human_owner_mode.rs](../../../services/agent/crates/layerx-agentd/src/human_owner_mode.rs) | <code>pub enum Mode</code> |
| <code>interface.22.env-layerx-agent-inference-signal-listen</code> | <code>env_var</code> | [services/agent/crates/layerx-agentd/src/config.rs](../../../services/agent/crates/layerx-agentd/src/config.rs) | <code>LAYERX_AGENT_INFERENCE_SIGNAL_LISTEN</code> |
| <code>interface.22.env-layerx-agent-inference-serve-listen</code> | <code>env_var</code> | [services/agent/crates/layerx-agentd/src/config.rs](../../../services/agent/crates/layerx-agentd/src/config.rs) | <code>LAYERX_AGENT_INFERENCE_SERVE_LISTEN</code> |
| <code>interface.22.env-layerx-agent-inference-model-digest</code> | <code>env_var</code> | [services/agent/crates/layerx-agentd/src/config.rs](../../../services/agent/crates/layerx-agentd/src/config.rs) | <code>LAYERX_AGENT_INFERENCE_MODEL_DIGEST</code> |
| <code>interface.22.env-layerx-agent-inference-runtime-digest</code> | <code>env_var</code> | [services/agent/crates/layerx-agentd/src/config.rs](../../../services/agent/crates/layerx-agentd/src/config.rs) | <code>LAYERX_AGENT_INFERENCE_RUNTIME_DIGEST</code> |
| <code>interface.22.env-layerx-agent-inference-room-profile</code> | <code>env_var</code> | [services/agent/crates/layerx-agentd/src/config.rs](../../../services/agent/crates/layerx-agentd/src/config.rs) | <code>LAYERX_AGENT_INFERENCE_ROOM_PROFILE</code> |

## signalling

**Signalling.** A plain WebSocket listener for browser peers; hello frames answered with the room offer whose digest is on chain; WebRTC descriptions relayed between peers.

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.22.inference-signal-hello-frame</code> | <code>file_format</code> | [services/agent/crates/layerx-agentd/src/inference_market/signal.rs](../../../services/agent/crates/layerx-agentd/src/inference_market/signal.rs) | <code>pub struct HelloFrame</code> |
| <code>interface.22.ws-inference-signal</code> | <code>ws_route</code> | [services/agent/crates/layerx-agentd/src/inference_market/signal.rs](../../../services/agent/crates/layerx-agentd/src/inference_market/signal.rs) | <code>WS / on the signalling listener</code> |

## completion-routes

**Completion routes.** POST /v1/chat/completions and POST /v1/messages on the completion listener, gated by the ask stream (402 when absent, closed or capped), usage from the lap counts.

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.22.http-inference-chat-completions</code> | <code>http_route</code> | [services/agent/crates/layerx-agentd/src/inference_market/serve.rs](../../../services/agent/crates/layerx-agentd/src/inference_market/serve.rs) | <code>POST /v1/chat/completions</code> |
| <code>interface.22.http-inference-messages</code> | <code>http_route</code> | [services/agent/crates/layerx-agentd/src/inference_market/serve.rs](../../../services/agent/crates/layerx-agentd/src/inference_market/serve.rs) | <code>POST /v1/messages</code> |

## peer-wire

**Peer wire protocol.** Frames exchanged between the peer runtime under apps/inference-peer and the host over the signalling and data channels.

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.22.inference-peer-wire-frame</code> | <code>file_format</code> | [apps/inference-peer/src/wire.mjs](../../../apps/inference-peer/src/wire.mjs) | <code>export function encodeFrame({ kind, roomId, sequence, payload })</code> |

## peer-cli

**Native peer command line.** join, serve and status commands of the native peer; join may submit an attestation envelope through the host before enrolling.

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.22.cli-inference-peer-join</code> | <code>cli_command</code> | [apps/inference-peer/bin/inference-peer.mjs](../../../apps/inference-peer/bin/inference-peer.mjs) | <code>join &lt;host&gt; &lt;room&gt; --manifest &lt;v2-file&gt; [--enrolment &lt;signed-json-file&gt;] [--attestation &lt;PXVE-file&gt;]</code> |
| <code>interface.22.cli-inference-peer-serve</code> | <code>cli_command</code> | [apps/inference-peer/bin/inference-peer.mjs](../../../apps/inference-peer/bin/inference-peer.mjs) | <code>serve --host &lt;host&gt; --ask-bearer &lt;bearer&gt; (the bearer may instead come from PAXAI_ASK_BEARER)</code> |
| <code>interface.22.cli-inference-peer-status</code> | <code>cli_command</code> | [apps/inference-peer/bin/inference-peer.mjs](../../../apps/inference-peer/bin/inference-peer.mjs) | <code>status [--port &lt;port&gt;] -- print the room, peer key, connection state, backend, layer range and activation count</code> |

## sdks

**SDK surfaces.** The TypeScript and Rust SDKs expose the market operations and the px_ reads.

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.22.ts-sdk-inference-market</code> | <code>ts_export</code> | [services/agent/sdk/typescript/src/inference_market.ts](../../../services/agent/sdk/typescript/src/inference_market.ts) | <code>export openRoom, postAsk, fundAsk, recordDeal, settleLaps, getInferenceRooms</code> |
| <code>interface.22.rs-sdk-inference-market</code> | <code>rust_pub_item</code> | [services/agent/crates/layerx-sdk/src/inference_market.rs](../../../services/agent/crates/layerx-sdk/src/inference_market.rs) | <code>pub struct InferenceMarket; associated read constructor: pub fn get_inference_rooms() -&gt; InferenceRead</code> |

## explorer

**Explorer surface.** The POST-only inference handler calls the px_ reads on the node root configured by `EXPLORER_INFERENCE_NODE_URL`; the page lists rooms, deals and receipt batches. The route below is relative to the frontend’s `/explorer` base path and is also available through `/explorer/node-api/inference`.

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.22.http-explorer-api-inference</code> | <code>http_route</code> | [apps/explorer/pages/api/inference.ts](../../../apps/explorer/pages/api/inference.ts) | <code>POST /api/inference</code> |
| <code>interface.22.ts-explorer-paxeerxinference</code> | <code>ts_export</code> | [apps/explorer/ui/pages/PaxeerXInference.tsx](../../../apps/explorer/ui/pages/PaxeerXInference.tsx) | <code>export default function PaxeerXInference()</code> |

## wallet

**Wallet surface.** The wallet funds an ask through the metered stream and shows peer earnings from the reward ledger.

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.22.ts-wallet-inferenceview</code> | <code>ts_export</code> | [apps/wallet/src/surfaces/InferenceView.tsx](../../../apps/wallet/src/surfaces/InferenceView.tsx) | <code>export function InferenceView({ sidRate = null, rewardSource }: { readonly sidRate?: string &#124; null; readonly rewardSource?: InferenceRewardSource &#124; null; })</code> |

## qualification

**Room driver.** The isolated-chain driver accepts `--proof room` and `--proof settlement` and checks the corresponding room and payment flows.

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.22.cli-inference-market-room-driver</code> | <code>cli_command</code> | [tools/qualification/paxeer-x/inference_market_room.py](../../../tools/qualification/paxeer-x/inference_market_room.py) | <code>python3 tools/qualification/paxeer-x/inference_market_room.py --proof room; alternatively select --proof settlement</code> |
