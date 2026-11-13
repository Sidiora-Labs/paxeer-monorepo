# Inference market contract: rooms, deals, laps, canaries, bonds, streams and the orchestration profile

| Field | Value |
|---|---|
| Domain | 22 (inference market), feature managers fm-programs, fm-ledger-auth-pay, fm-rpc-api, fm-agent, fm-platform, fm-web-explorer-bridge, fm-wallet, fm-tools-ops |
| Precompiles | layerxpay 0x101B (ask streams), teeattestor 0x0907 (peer binding) and the ai-market program family F11 |
| Revision | repository at 2bec29631dcd8426714cd9fe43570d820949aeba |
| Change control | Changes only by amendment to this contract; every amendment updates the interface, contract_context and acceptance blocks that point here. |
| Status meaning | served: the implementation symbol and its callers are present; stable: code and intent agree, no task touches it; change: behaviour or shape must change; new: absent at the revision and created by a task; retire: removed or refused by a task. |

## Scope and reading guide

This contract freezes the surface of the pooled inference market (decision 2752, requirements 22998.1 and 22380.1 to 22380.6, tasks 22.38.1 to 22.38.39). The chain verifies, pays and connects peers: room, ask, bid, deal, lap receipt, canary, bond and payout records in the ai-market program, metered ask streams in 0x101B and peer attestation bindings in 0x0907. Orchestration runs off chain in the agent daemon profile inference-market and in the vendored peer runtime under apps/inference-peer; validators and nodes never take part in a room.

Each `##` heading below is the anchor named by the `contract` key of the rows it lists. The fifteen `interface.22.inference-*` rows are served at the revision. Their paths, declaration lines, signatures and callers are recorded in `interfaces.kvx`. Other rows retain their recorded status.

## Surface summary

| Item | Count |
|---|---|
| Anchors | 21 |
| Interfaces | 43 |
| Interfaces with status served | 15 |
| Interfaces with status new | 28 |

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

| Interface | Kind | Defined at | Signature | Task | Status |
|---|---|---|---|---|---|
| interface.22.inference-f11-operation-table | rust_pub_item | programs/crates/layerx-programs-ai-market/src/dispatch.rs:194 | pub const OpenRoom: Operation = Operation(0x0B01); | 22.38.1 | served |
| interface.22.inference-f11-errors | rust_pub_item | programs/crates/layerx-programs-ai-market/src/errors.rs:244 | pub const F11_ROOM_UNKNOWN: ApplicationError = application_error(0x0B01); | 22.38.1 | served |
| interface.22.inference-f11-availability | rust_pub_item | programs/crates/layerx-programs-ai-market/src/queries.rs:1084 | pub fn feature_availability(state_bytes: &[u8]) -> CodecResult<[Availability; 11]> | 22.38.1 | served |
| interface.22.inference-room-profile | rust_pub_item | programs/crates/layerx-programs-ai-market/src/policy.rs:430 | pub struct RoomProfile | 22.38.1 | served |

## manifest-v2

**Participant manifest v2.** The v1 manifest followed by runtime digest, model digest, device class, memory bytes, bandwidth class and price quote, signed under a v2 metadata domain so v1 manifests stay valid.

| Interface | Kind | Defined at | Signature | Task | Status |
|---|---|---|---|---|---|
| interface.22.inference-manifest-v2 | file_format | programs/crates/layerx-programs-ai-market/src/workers.rs:756 | pub struct ManifestV2<'a> | 22.38.2 | served |

## room-records

**Rooms, asks, bids and deals.** A room binds a model digest, a runtime digest, a room profile and the roster digest; asks and bids reference rooms; a deal binds a layer plan to enrolled peers and an ask reference, recorded by the host.

| Interface | Kind | Defined at | Signature | Task | Status |
|---|---|---|---|---|---|
| interface.22.inference-room-record | rust_pub_item | programs/crates/layerx-programs-ai-market/src/rooms.rs:103 | pub struct RoomRecord | 22.38.3 | served |

## lap-receipts

**Lap receipts and settlement.** Lap receipts are countersigned off chain by the contributing peers, batched by the host and settled through sealed evidence; a batch records the cumulative token count the metered stream reads.

| Interface | Kind | Defined at | Signature | Task | Status |
|---|---|---|---|---|---|
| interface.22.inference-lap-receipt | rust_pub_item | programs/crates/layerx-programs-ai-market/src/laps.rs:25 | pub struct LapReceipt<'a> | 22.38.4 | served |

## canary

**Canary challenges.** Commit-reveal challenges on lap receipts with fixed-point logit tolerance; a failed reveal slashes the peer bond after the challenge window.

| Interface | Kind | Defined at | Signature | Task | Status |
|---|---|---|---|---|---|
| interface.22.inference-canary-commitment | rust_pub_item | programs/crates/layerx-programs-ai-market/src/canary.rs:228 | pub struct CanaryCommitment | 22.38.5 | served |

## payout-split

**Payout split.** The settled amount of a batch is credited to the reward ledger per peer in layer-weight proportions with the host and protocol shares of the room profile; peers claim through the existing CLAIM operation.

| Interface | Kind | Defined at | Signature | Task | Status |
|---|---|---|---|---|---|
| interface.22.inference-payout-split | rust_pub_item | programs/crates/layerx-programs-ai-market/src/rewards.rs:3646 | pub fn apply_split_payout( ctx: &CallContext, envelope: &ValidatedEnvelope<'_>, current: &[u8], next: &mut [u8], scratch: &mut [u8], event: &mut [u8], ) -> CodecResult<crate::rooms::Outcome> | 22.38.6 | served |

## bonds

**Peer bonds.** A peer locks a bond at enrolment; a failed canary slashes it; closing a room with no open challenge refunds it.

| Interface | Kind | Defined at | Signature | Task | Status |
|---|---|---|---|---|---|
| interface.22.inference-bond-record | rust_pub_item | programs/crates/layerx-programs-ai-market/src/bonds.rs:21 | pub struct BondRecord | 22.38.7 | served |

## placement-book

**Placement book.** Open asks and bids per model digest inside the program state; crossing pairs are the deal inputs the off-chain profile negotiates from.

| Interface | Kind | Defined at | Signature | Task | Status |
|---|---|---|---|---|---|
| interface.22.inference-placement-book | rust_pub_item | programs/crates/layerx-programs-ai-market/src/placement.rs:74 | pub struct PlacementBook<'a> | 22.38.11 | served |

## views

**Fixed-width views.** Room, deal, peer and receipt batch views answered as fixed-width records that the node px_ reads decode.

| Interface | Kind | Defined at | Signature | Task | Status |
|---|---|---|---|---|---|
| interface.22.inference-f11-views | rust_pub_item | programs/crates/layerx-programs-ai-market/src/queries.rs:67 | pub struct RoomView | 22.38.8 | served |

## streams-0x101b

**Ask streams (0x101B).** An ask is funded by a metered stream whose single meter authority is the host key; a reading equals the cumulative token count of the last settled lap batch.

| Interface | Kind | Defined at | Signature | Task | Status |
|---|---|---|---|---|---|
| interface.22.pc-0x101b-streamopenforask | precompile_method | precompiles/layerxpay/stream.go (planned) | streamOpenForAsk(bytes32,address,uint256,uint8,uint256,bytes32) returns (bytes32) (planned) | 22.38.9 | new |
| interface.22.pc-0x101b-askstreamof | precompile_method | precompiles/layerxpay/stream.go (planned) | askStreamOf(bytes32) view returns (bytes32) (planned) | 22.38.9 | new |
| interface.22.pc-0x101b-evt-askstreamopened | precompile_event | precompiles/layerxpay/events.go (planned) | AskStreamOpened(bytes32 indexed,bytes32 indexed,address) (planned) | 22.38.9 | new |

The bytes32 identifier returned by `streamOpenForAsk` and `askStreamOf` is the call idempotency key with its first sixteen bytes set to zero, so it fits the ask record’s u128 stream field. The assignment is in `precompiles/layerxpay/stream.go:22`; `node/app.go:3860` configures inference reads from `evm.px_inference_program` and `evm.px_inference_reader`; `services/agent/crates/layerx-agentd/src/main.rs:2130` uses `LAYERX_AGENT_CONSENSUS_ENDPOINT` for consensus reads, with the node endpoint as its default. The committed source retries historical proof rate limits at most eight times, 250 ms apart, with a one-second node request timeout and a two-second socket I/O timeout.

Inference reads in `rpc/px_account.go` use the quoted call fee; `rpc/simulate.go` evaluates their private native simulation in batch H+1 over the store pinned at committed height H. `rpc/px_inference.go` decodes the version-2 room profile and closed height.

The daemon's native historical reads retry a proof rate limit at most eight times, 250 ms apart; each native RPC request retains its thirty-second timeout.

The daemon verifies each deployment admission and replays its journal by program identifier and version.

The inference book uses the node's committed header below its latest height and authenticates the preceding store height through the configured consensus endpoint, with unprefixed hexadecimal store keys and at most eight retries of a historical proof rate limit, 250 ms apart.

Availability checks independently verify the node header at the gateway publication's height under the configured freshness bound; readiness requires that publication's exact height and block hash.

The committed-header RPC validates the chain identifier from the initialized node context on each request.

The signalling client uses node RPC independently of the gateway snapshot client. Its header, room and peer reads use the verified availability height; `px_getInferenceRoomAt` and `px_getInferencePeerAt` accept a positive committed height and apply the same program and peer-binding decoders as current-height reads. Native peers retry `chain_unavailable` admission responses at most seven times, 250 ms apart, within a twenty-second admission deadline.

Human-session node readiness uses the consensus chain identifier of the configured trust root and separately requires EVM chain 125.

CPU layer planning uses the manifest's system memory; accelerator layer planning uses its accelerator memory. Deal and lap envelopes derive their protocol principal from the signer's DID; a room host's DID must bind the signer's Ed25519 public key, and evidence envelopes use their own signer's DID.

## attestation-0x0907

**Peer attestation binding (0x0907).** A verified attestation envelope is bound to a peer key, a model digest and a runtime digest; only the development envelope family is accepted today.

| Interface | Kind | Defined at | Signature | Task | Status |
|---|---|---|---|---|---|
| interface.22.pc-0x0907-bindpeer | precompile_method | precompiles/layerxagent/teeattestor/teeattestor.go (planned) | bindPeer(uint8,bytes,bytes32,bytes32,bytes32) returns (bool) (planned) | 22.38.10 | new |
| interface.22.pc-0x0907-peerbinding | precompile_method | precompiles/layerxagent/teeattestor/teeattestor.go (planned) | peerBinding(bytes32) view returns (uint8,bytes32,bytes32,uint64) (planned) | 22.38.10 | new |
| interface.22.pc-0x0907-evt-peerattestationbound | precompile_event | precompiles/layerxagent/teeattestor/teeattestor.go (planned) | PeerAttestationBound(bytes32 indexed,uint8,bytes32,bytes32) (planned) | 22.38.10 | new |

## px-reads

**Node root px_ reads.** Five px_ methods served at the node root (decision 2521) decode the F11 views and add the ask stream state and the peer binding.

| Interface | Kind | Defined at | Signature | Task | Status |
|---|---|---|---|---|---|
| interface.22.rpc-px-getinferencerooms | jsonrpc_method | rpc/px_inference.go (planned) | px_getInferenceRooms() -> room summaries (planned) | 22.38.12 | new |
| interface.22.rpc-px-getinferenceroom | jsonrpc_method | rpc/px_inference.go (planned) | px_getInferenceRoom(roomId) -> room view with ask stream state (planned) | 22.38.12 | new |
| interface.22.rpc-px-getinferencedeal | jsonrpc_method | rpc/px_inference.go (planned) | px_getInferenceDeal(dealId) -> deal view (planned) | 22.38.12 | new |
| interface.22.rpc-px-getinferencepeer | jsonrpc_method | rpc/px_inference.go (planned) | px_getInferencePeer(peerKey) -> peer view with attestation binding (planned) | 22.38.12 | new |
| interface.22.rpc-px-getinferencereceipts | jsonrpc_method | rpc/px_inference.go (planned) | px_getInferenceReceipts(dealId) -> receipt batch views (planned) | 22.38.12 | new |

## daemon-profile

**Agent daemon profile inference-market.** LAYERX_AGENT_MODE=inference-market runs the orchestration profile off chain: enrolment, signalling, deal negotiation, lap settlement, canaries, recovery and the completion routes. It is never part of a validator or node.

| Interface | Kind | Defined at | Signature | Task | Status |
|---|---|---|---|---|---|
| interface.22.inference-mode-inference-market | rust_pub_item | services/agent/crates/layerx-agentd/src/human_owner_mode.rs:4 | pub enum Mode | 22.38.14 | served |
| interface.22.env-layerx-agent-inference-signal-listen | env_var | services/agent/crates/layerx-agentd/src/inference_market/mod.rs (planned) | LAYERX_AGENT_INFERENCE_SIGNAL_LISTEN | 22.38.14 | new |
| interface.22.env-layerx-agent-inference-serve-listen | env_var | services/agent/crates/layerx-agentd/src/inference_market/mod.rs (planned) | LAYERX_AGENT_INFERENCE_SERVE_LISTEN | 22.38.14 | new |
| interface.22.env-layerx-agent-inference-model-digest | env_var | services/agent/crates/layerx-agentd/src/inference_market/mod.rs (planned) | LAYERX_AGENT_INFERENCE_MODEL_DIGEST | 22.38.14 | new |
| interface.22.env-layerx-agent-inference-runtime-digest | env_var | services/agent/crates/layerx-agentd/src/inference_market/mod.rs (planned) | LAYERX_AGENT_INFERENCE_RUNTIME_DIGEST | 22.38.14 | new |
| interface.22.env-layerx-agent-inference-room-profile | env_var | services/agent/crates/layerx-agentd/src/inference_market/mod.rs (planned) | LAYERX_AGENT_INFERENCE_ROOM_PROFILE | 22.38.14 | new |

## signalling

**Signalling.** A plain WebSocket listener for browser peers; hello frames answered with the room offer whose digest is on chain; WebRTC descriptions relayed between peers.

| Interface | Kind | Defined at | Signature | Task | Status |
|---|---|---|---|---|---|
| interface.22.inference-signal-hello-frame | file_format | services/agent/crates/layerx-agentd/src/inference_market/signal.rs:25 | pub struct HelloFrame | 22.38.16 | served |
| interface.22.ws-inference-signal | ws_route | services/agent/crates/layerx-agentd/src/inference_market/signal.rs (planned) | WS / on the signalling listener (planned) | 22.38.16 | new |

## completion-routes

**Completion routes.** POST /v1/chat/completions and POST /v1/messages on the completion listener, gated by the ask stream (402 when absent, closed or capped), usage from the lap counts.

| Interface | Kind | Defined at | Signature | Task | Status |
|---|---|---|---|---|---|
| interface.22.http-inference-chat-completions | http_route | services/agent/crates/layerx-agentd/src/inference_market/serve.rs (planned) | POST /v1/chat/completions (planned) | 22.38.21 | new |
| interface.22.http-inference-messages | http_route | services/agent/crates/layerx-agentd/src/inference_market/serve.rs (planned) | POST /v1/messages (planned) | 22.38.21 | new |

## peer-wire

**Peer wire protocol.** Frames exchanged between the peer runtime under apps/inference-peer and the host over the signalling and data channels.

| Interface | Kind | Defined at | Signature | Task | Status |
|---|---|---|---|---|---|
| interface.22.inference-peer-wire-frame | file_format | apps/inference-peer/src/wire.mjs:45 | export function encodeFrame({ kind, roomId, sequence, payload }) | 22.38.24 | served |

## peer-cli

**Native peer command line.** join, serve and status commands of the native peer; join may submit an attestation envelope through the host before enrolling.

| Interface | Kind | Defined at | Signature | Task | Status |
|---|---|---|---|---|---|
| interface.22.cli-inference-peer-join | cli_command | apps/inference-peer/bin/inference-peer.mjs (planned) | 'join <host> <room>' -- join a room, optionally with an attestation envelope file | 22.38.30 | new |
| interface.22.cli-inference-peer-serve | cli_command | apps/inference-peer/bin/inference-peer.mjs (planned) | 'serve' -- local OpenAI-compatible route forwarding to the host completion route | 22.38.30 | new |
| interface.22.cli-inference-peer-status | cli_command | apps/inference-peer/bin/inference-peer.mjs (planned) | 'status' -- print the joined room, layer range and lap counters | 22.38.30 | new |

## sdks

**SDK surfaces.** The TypeScript and Rust SDKs expose the market operations and the px_ reads.

| Interface | Kind | Defined at | Signature | Task | Status |
|---|---|---|---|---|---|
| interface.22.ts-sdk-inference-market | ts_export | services/agent/sdk/typescript/src/inference_market.ts (planned) | export openRoom, postAsk, fundAsk, recordDeal, settleLaps, getInferenceRooms | 22.38.31 | new |
| interface.22.rs-sdk-inference-market | rust_pub_item | services/agent/crates/layerx-sdk/src/inference_market.rs (planned) | pub struct InferenceMarket with open_room, post_ask, get_inference_rooms | 22.38.32 | new |

## explorer

**Explorer surface.** A server route in the shape of the assistant route calls the px_ reads on the configured node root; the page lists rooms, deals and receipt batches.

| Interface | Kind | Defined at | Signature | Task | Status |
|---|---|---|---|---|---|
| interface.22.http-explorer-api-inference | http_route | apps/explorer/pages/api/inference.ts (planned) | ANY /api/inference (planned) | 22.38.33 | new |
| interface.22.ts-explorer-paxeerxinference | ts_export | apps/explorer/ui/pages/PaxeerXInference.tsx (planned) | export default function PaxeerXInference() | 22.38.33 | new |

## wallet

**Wallet surface.** The wallet funds an ask through the metered stream and shows peer earnings from the reward ledger.

| Interface | Kind | Defined at | Signature | Task | Status |
|---|---|---|---|---|---|
| interface.22.ts-wallet-inferenceview | ts_export | apps/wallet/src/surfaces/InferenceView.tsx (planned) | export function InferenceView() | 22.38.34 | new |

## qualification

**Isolated-chain proofs.** The room driver on the isolated-chain fixture proves room, canary and settlement flows end to end with one host, two native peers and one asker.

| Interface | Kind | Defined at | Signature | Task | Status |
|---|---|---|---|---|---|
| interface.22.cli-inference-market-room-driver | cli_command | tools/qualification/paxeer-x/inference_market_room.py (planned) | 'inference_market_room.py --proof roomorcanaryorsettlement' -- isolated-chain proof driver | 22.38.36 | new |
