# Inference market protocol

The ai-market program implements the F11 interface described here. Check
[availability](README.md#availability) on the target network before use.
F11 selectors belong to the ai-market program operation envelope; they are not
standalone EVM method selectors. Submit program calls through the existing
program signing and finality path.

## Room lifecycle

The host enrols with a capability manifest and opens a room binding model and
runtime digests, a room profile and settlement account. An asker posts its price
ceiling and binds a funded stream. Enrolled peers lock bonds before posting
bids with capability digests, layer capacity and price floors. The placement
book groups asks and bids by model digest.

The host records a crossing deal with ordered bids, price, layer split and the
current admitted roster digest. A zero ask stream ID, incompatible bid floor
or mismatched roster digest refuses the deal. It becomes live after the write
reaches finality and every peer acknowledges. Unacknowledged peers are dropped
and the plan is recomputed.

Generation proceeds in laps. The host seals and settles batches, advances the
stream meter, and splits paid rewards after resolving any challenges. CloseRoom
refuses with open deals. Bond refunds additionally wait for the peer's last deal
to close and its challenge window to pass. A local recovery state can pause a
room without changing the room record's open/closed state.

## F11 operation table and payload layouts

The selectors and authority classes are declared in
[`dispatch.rs`](../../../programs/crates/layerx-programs-ai-market/src/dispatch.rs).
Binary request and view layouts are defined by
[`rooms.rs`](../../../programs/crates/layerx-programs-ai-market/src/rooms.rs),
[`laps.rs`](../../../programs/crates/layerx-programs-ai-market/src/laps.rs),
[`canary.rs`](../../../programs/crates/layerx-programs-ai-market/src/canary.rs)
and the [reward](../../../programs/crates/layerx-programs-ai-market/src/rewards.rs)
and [bond](../../../programs/crates/layerx-programs-ai-market/src/bonds.rs) modules.
Use those codecs for field order, widths and bounds.

| Selector | Operation | Authority |
| --- | --- | --- |
| `0x0B01` | OpenRoom | Host |
| `0x0B02` | PostAsk | Any account |
| `0x0B03` | PostBid | Enrolled peer |
| `0x0B04` | RecordDeal | Host |
| `0x0B05` | SettleLaps | Host |
| `0x0B06` | ChallengeLap | Any account |
| `0x0B07` | ResolveChallenge | Any account |
| `0x0B08` | SplitPayout | Host, or any account after the challenge window |
| `0x0B09` | LockBond | Enrolled peer |
| `0x0B0A` | SlashBond | Resolved fault |
| `0x0B0B` | RefundBond | Bond owner |
| `0x0B0C` | CloseRoom | Host |
| `0x0B10` | RoomView | Public read |
| `0x0B11` | DealView | Public read |
| `0x0B12` | PeerView | Public read |
| `0x0B13` | ReceiptBatchView | Public read |
| `0x0B14` | BookView | Public read |

Mutations also enforce the operation's state and evidence conditions. Room,
deal and peer views take a 32-byte identifier; ReceiptBatchView takes its
64-byte receipt-batch key, and BookView takes a 32-byte model digest.
The program state bound is 245760 bytes. The room book permits at most
16 rooms, 64 asks, 128 bids and 32 peers per deal.

The version-2 [RoomProfile](../../../programs/crates/layerx-programs-ai-market/src/policy.rs)
contains `minimum_bond`, `canary_ratio_bps`, `lap_size_tokens`,
`challenge_window_blocks`, `slash_fraction_bps`, `host_share_bps`,
`protocol_share_bps`, `settlement_account`, `max_generated_tokens`, `queue_depth`,
`liveness_timeout_ms`, `acknowledgement_window_ms` and
`canary_relative_tolerance_bps`. Bond, lap size, challenge window, token limit,
queue depth and both timeouts must be positive. Ratios and tolerance are bounded
by 10000 basis points, and host plus protocol shares cannot exceed 10000.

## Lap receipts and sealed evidence

A LapReceipt carries deal ID, lap index, prompt tokens, generated tokens,
cumulative tokens, the layer-range digest per peer, host signature, and peer
countersignatures with a bitmap over the deal roster. A ReceiptBatch carries
deal ID, first and last lap indices, the final cumulative token count, a Merkle
root of the receipts and countersign quorum. Its root must already be sealed
through SealEvidence as an artifact manifest chunk root for the deal task set.

SettleLaps refuses unsealed roots, an index below the last settled index and a
quorum below roster size minus one. Successful settlement records cumulative
tokens and the amount owed at the deal price. The host settles before metering.
A peer that misses the countersign deadline twice in a batch is marked for
slashing and removal. A countersignature proves agreement with receipt bytes;
it does not establish semantic correctness of every generated token.

## Canary derivation and challenge

The host derives a secret-keyed deterministic sampling key with
[`canary_sampling_key`](../../../services/agent/crates/layerx-client/src/ai_market.rs).
It hashes `PAXAI/canary-sampling/v1` followed by a zero byte, the 32-byte deal ID
and the big-endian u64 lap index with SHA-256, signs that digest with its
Ed25519 host meter key, then hashes the signature with SHA-256. The signature
is not sent to peers.

The [daemon sampler](../../../services/agent/crates/layerx-agentd/src/inference_market/canary.rs)
uses SHA-256 with the `PAXAI/canary-position/v1` domain and a zero byte, that key,
the `lap` or `position` domain and a big-endian u64 counter. Rejection sampling
selects a lap with probability `canary_ratio_bps / 10000`, then chooses one
position uniformly in that lap. Each selected lap has one probe.
The host computes expectations on its model or uses a canary table bound to
the model digest. It commits expectations through CommitScore before work,
then uses ChallengeLap and RevealScore for a mismatch.

The reveal carries the top eight logits at the canary position as signed i32
fixed-point values with 16 fractional bits. ResolveChallenge compares the peer
answer to the expectation using relative tolerance in basis points; zero
tolerance requires equality. A miss marks that peer at fault for the lap; a hit
returns the challenge deposit. Challenges outside the window refuse. Canary
checks bound but do not eliminate wrong answers; see [threats](threats.md).

## Metered stream rule

The pay methods at `0x101B` include:

```text
streamOpenForAsk(bytes32 askRef, address settlementAccount, uint256 rate,
                 uint8 rateUnit, uint256 totalCap, bytes32 hostAuthorityKey)
    returns (bytes32 streamId)
askStreamOf(bytes32 askRef) view returns (bytes32)
```

Each ask pays through one metered stream. The host key is its single meter
authority. **A reading equals the cumulative token count of the last settled
lap batch.** Only streamMeter advances that reading; another key or a regressing
reading refuses. Streaming response chunks and locally estimated usage never
authorize a meter increment. Preserve the stream's rate unit and cap when
encoding a price quoted per thousand tokens.
`AskStreamOpened(bytes32 indexed askRef, bytes32 indexed streamId, address settlementAccount)`
binds ask reference, stream ID and settlement account.

## Payout split and bonds

SplitPayout credits only the settled amount. For amount `S`, host share `h`
and protocol share `p`, the integer host credit is `floor(S*h/10000)` and the
protocol credit is `floor(S*p/10000)`. The remaining peer pool is
`S - host_credit - protocol_credit`.

[Reward math](../../../programs/crates/layerx-programs-ai-market/src/reward_math.rs)
distributes that pool using layer weights, excluding peers at fault for a lap.
It divides lap budgets with earlier laps receiving any remainder, then assigns
weighted peer floors and distributes remaining units by largest remainder,
breaking ties by canonical peer order. Credits must sum exactly to `S`.
A split exceeding settlement or carrying an open batch challenge refuses.

Credits enter the existing epoch reward ledger. Peers withdraw through CLAIM
(`0x0602`), unchanged; FUND is not the settlement path. The keeper-side draw
handles settlement accounting at the room account. Metering, splitting and
claiming are distinct transitions, each requiring its own evidence.

LockBond precedes PostBid. A resolved canary miss or missed countersign deadline
permits SlashBond at the room slash fraction, credited to the other deal peers'
reward ledger. A second slash for one lap refuses. RefundBond returns the
remainder only after the last deal closes and the challenge window passes;
closing a room alone does not bypass that window.
