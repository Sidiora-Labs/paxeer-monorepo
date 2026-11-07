# Inference market contract: rooms, deals, laps, canaries, bonds, streams and the orchestration profile

| Field | Value |
|---|---|
| Domain | 22 (inference market), feature managers fm-programs, fm-ledger-auth-pay, fm-rpc-api, fm-agent, fm-platform, fm-web-explorer-bridge, fm-wallet, fm-tools-ops |
| Precompiles | layerxpay 0x101B (ask streams), teeattestor 0x0907 (peer binding) and the ai-market program family F11 |
| Revision | repository at 8e40540496 |
| Change control | Changes only by amendment to this contract; every amendment updates the interface, contract_context and acceptance blocks that point here. |
| Status meaning | stable: code and intent agree, no task touches it; change: behaviour or shape must change; new: absent at the revision and created by a task; retire: removed or refused by a task. |

## Scope and reading guide

This contract freezes the surface of the pooled inference market (decision 2752, requirements 22998.1 and 22380.1 to 22380.6, tasks 22.38.1 to 22.38.39). The chain verifies, pays and connects peers: room, ask, bid, deal, lap receipt, canary, bond and payout records in the ai-market program, metered ask streams in 0x101B and peer attestation bindings in 0x0907. Orchestration runs off chain in the agent daemon profile inference-market and in the vendored peer runtime under apps/inference-peer; validators and nodes never take part in a room.

Each `##` heading below is the anchor named by the `contract` key of the rows it lists. Every row is status new at the revision: it is created by the task named in its notes and reconciled to served by task 22.38.39.

## Surface summary

| Item | Count |
|---|---|
| Anchors | 21 |
| Interfaces | 43 |
| Interfaces with status new | 43 |

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
| interface.22.inference-f11-operation-table | rust_pub_item | programs/crates/layerx-programs-ai-market/src/dispatch.rs (planned) | pub const OpenRoom: Operation = Operation(0x0B01) through CloseRoom = Operation(0x0B0C) and the reads RoomView = Operation(0x0B10) through BookView = Operation(0x0B14) | 22.38.1 | new |
| interface.22.inference-f11-errors | rust_pub_item | programs/crates/layerx-programs-ai-market/src/errors.rs (planned) | pub const F11_ROOM_UNKNOWN and the other F11 error codes | 22.38.1 | new |
| interface.22.inference-f11-availability | rust_pub_item | programs/crates/layerx-programs-ai-market/src/queries.rs (planned) | pub fn feature_availability(state_bytes: &[u8]) -> CodecResult<[Availability; 11]> | 22.38.1 | new |
| interface.22.inference-room-profile | rust_pub_item | programs/crates/layerx-programs-ai-market/src/policy.rs (planned) | room profile fields of the market policy (host share, protocol share, bond, lap size, challenge window, maximum tokens) | 22.38.1 | new |

## manifest-v2

**Participant manifest v2.** The v1 manifest followed by runtime digest, model digest, device class, memory bytes, bandwidth class and price quote, signed under a v2 metadata domain so v1 manifests stay valid.

| Interface | Kind | Defined at | Signature | Task | Status |
|---|---|---|---|---|---|
| interface.22.inference-manifest-v2 | file_format | programs/crates/layerx-programs-ai-market/src/workers.rs (planned) | ManifestV2: v1 fields then runtime digest, model digest, device class, memory bytes, bandwidth class, price quote; domain PARTICIPANT_METADATA_DOMAIN_V2 | 22.38.2 | new |

## room-records

**Rooms, asks, bids and deals.** A room binds a model digest, a runtime digest, a room profile and the roster digest; asks and bids reference rooms; a deal binds a layer plan to enrolled peers and an ask reference, recorded by the host.

| Interface | Kind | Defined at | Signature | Task | Status |
|---|---|---|---|---|---|
| interface.22.inference-room-record | rust_pub_item | programs/crates/layerx-programs-ai-market/src/rooms.rs (planned) | pub struct RoomRecord, AskRecord, BidRecord, DealRecord and pub fn apply_record_deal | 22.38.3 | new |

## lap-receipts

**Lap receipts and settlement.** Lap receipts are countersigned off chain by the contributing peers, batched by the host and settled through sealed evidence; a batch records the cumulative token count the metered stream reads.

| Interface | Kind | Defined at | Signature | Task | Status |
|---|---|---|---|---|---|
| interface.22.inference-lap-receipt | rust_pub_item | programs/crates/layerx-programs-ai-market/src/laps.rs (planned) | pub struct LapReceipt, ReceiptBatch and pub fn apply_settle_laps | 22.38.4 | new |

## canary

**Canary challenges.** Commit-reveal challenges on lap receipts with fixed-point logit tolerance; a failed reveal slashes the peer bond after the challenge window.

| Interface | Kind | Defined at | Signature | Task | Status |
|---|---|---|---|---|---|
| interface.22.inference-canary-commitment | rust_pub_item | programs/crates/layerx-programs-ai-market/src/canary.rs (planned) | pub struct CanaryCommitment, pub fn apply_challenge_lap, apply_resolve_challenge, relative_tolerance_hit | 22.38.5 | new |

## payout-split

**Payout split.** The settled amount of a batch is credited to the reward ledger per peer in layer-weight proportions with the host and protocol shares of the room profile; peers claim through the existing CLAIM operation.

| Interface | Kind | Defined at | Signature | Task | Status |
|---|---|---|---|---|---|
| interface.22.inference-payout-split | rust_pub_item | programs/crates/layerx-programs-ai-market/src/rewards.rs (planned) | pub fn apply_split_payout and pub struct PayoutShares | 22.38.6 | new |

## bonds

**Peer bonds.** A peer locks a bond at enrolment; a failed canary slashes it; closing a room with no open challenge refunds it.

| Interface | Kind | Defined at | Signature | Task | Status |
|---|---|---|---|---|---|
| interface.22.inference-bond-record | rust_pub_item | programs/crates/layerx-programs-ai-market/src/bonds.rs (planned) | pub struct BondRecord, pub fn apply_lock_bond, apply_slash_bond, apply_refund_bond | 22.38.7 | new |

## placement-book

**Placement book.** Open asks and bids per model digest inside the program state; crossing pairs are the deal inputs the off-chain profile negotiates from.

| Interface | Kind | Defined at | Signature | Task | Status |
|---|---|---|---|---|---|
| interface.22.inference-placement-book | rust_pub_item | programs/crates/layerx-programs-ai-market/src/placement.rs (planned) | pub struct PlacementBook, BookView and pub fn crossing_pairs | 22.38.11 | new |

## views

**Fixed-width views.** Room, deal, peer and receipt batch views answered as fixed-width records that the node px_ reads decode.

| Interface | Kind | Defined at | Signature | Task | Status |
|---|---|---|---|---|---|
| interface.22.inference-f11-views | rust_pub_item | programs/crates/layerx-programs-ai-market/src/queries.rs (planned) | pub struct RoomView, DealView, PeerView, ReceiptBatchView and pub fn answer_room_view | 22.38.8 | new |

## streams-0x101b

**Ask streams (0x101B).** An ask is funded by a metered stream whose single meter authority is the host key; a reading equals the cumulative token count of the last settled lap batch.

| Interface | Kind | Defined at | Signature | Task | Status |
|---|---|---|---|---|---|
| interface.22.pc-0x101b-streamopenforask | precompile_method | precompiles/layerxpay/stream.go (planned) | streamOpenForAsk(bytes32,address,uint256,uint8,uint256,bytes32) returns (uint64) (planned) | 22.38.9 | new |
| interface.22.pc-0x101b-askstreamof | precompile_method | precompiles/layerxpay/stream.go (planned) | askStreamOf(bytes32) view returns (uint64) (planned) | 22.38.9 | new |
| interface.22.pc-0x101b-evt-askstreamopened | precompile_event | precompiles/layerxpay/events.go (planned) | AskStreamOpened(bytes32 indexed,uint64 indexed,address) (planned) | 22.38.9 | new |

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
| interface.22.inference-mode-inference-market | rust_pub_item | services/agent/crates/layerx-agentd/src/human_owner_mode.rs (planned) | Mode::InferenceMarket parsed from the mode value inference-market | 22.38.14 | new |
| interface.22.env-layerx-agent-inference-signal-listen | env_var | services/agent/crates/layerx-agentd/src/inference_market/mod.rs (planned) | LAYERX_AGENT_INFERENCE_SIGNAL_LISTEN | 22.38.14 | new |
| interface.22.env-layerx-agent-inference-serve-listen | env_var | services/agent/crates/layerx-agentd/src/inference_market/mod.rs (planned) | LAYERX_AGENT_INFERENCE_SERVE_LISTEN | 22.38.14 | new |
| interface.22.env-layerx-agent-inference-model-digest | env_var | services/agent/crates/layerx-agentd/src/inference_market/mod.rs (planned) | LAYERX_AGENT_INFERENCE_MODEL_DIGEST | 22.38.14 | new |
| interface.22.env-layerx-agent-inference-runtime-digest | env_var | services/agent/crates/layerx-agentd/src/inference_market/mod.rs (planned) | LAYERX_AGENT_INFERENCE_RUNTIME_DIGEST | 22.38.14 | new |
| interface.22.env-layerx-agent-inference-room-profile | env_var | services/agent/crates/layerx-agentd/src/inference_market/mod.rs (planned) | LAYERX_AGENT_INFERENCE_ROOM_PROFILE | 22.38.14 | new |

## signalling

**Signalling.** A plain WebSocket listener for browser peers; hello frames answered with the room offer whose digest is on chain; WebRTC descriptions relayed between peers.

| Interface | Kind | Defined at | Signature | Task | Status |
|---|---|---|---|---|---|
| interface.22.inference-signal-hello-frame | file_format | services/agent/crates/layerx-agentd/src/inference_market/signal.rs (planned) | hello frame (peer key, signature over room id and nonce, v2 manifest) answered by the room offer (room id, model digest, runtime digest, layer plan, price); frames at most 65536 bytes | 22.38.16 | new |
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
| interface.22.inference-peer-wire-frame | file_format | apps/inference-peer/src/wire.mjs (planned) | room wire frame encoded by encodeFrame and decoded by decodeFrame (kind, room id, sequence, payload) | 22.38.24 | new |

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
