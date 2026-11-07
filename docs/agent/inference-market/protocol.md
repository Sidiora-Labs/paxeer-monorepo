# Inference market protocol

The inference market is not yet available. This page describes the planned F11
interface; see [availability](README.md#availability).
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

The table describes planned payload fields and authority requirements.
Exact byte offsets, integer widths, padding, length bounds and read-key encodings
remain to be established by their pending codecs and `OperationMetadata` in
[dispatch.rs](../../../programs/crates/layerx-programs-ai-market/src/dispatch.rs).
Use the qualified producer codecs and SDK encoders when available; this field
inventory is not a substitute binary encoder. The governing room/deal/batch
reference is shown for operations whose full argument order is not yet frozen.

| Selector | Operation | Payload fields or query key | Authority / condition |
| --- | --- | --- | --- |
| `0x0B01` | OpenRoom | Room binding: model digest, runtime digest, room profile, settlement account | Host key |
| `0x0B02` | PostAsk | Room/model reference, asker, maximum price per thousand tokens, stream reference | Any account for its ask |
| `0x0B03` | PostBid | Room reference, peer, capability digest, minimum price per thousand tokens, bond, layer capacity | Enrolled peer; matching manifest and locked bond |
| `0x0B04` | RecordDeal | Ask ID, ordered bid IDs, agreed price, per-peer layer split, roster digest | Host; funded ask and current roster |
| `0x0B05` | SettleLaps | Deal ID, batch first/last lap, final cumulative tokens, Merkle root, countersign quorum | Host; sealed evidence |
| `0x0B06` | ChallengeLap | Deal ID, lap index, canary commitment | Any account inside the challenge window |
| `0x0B07` | ResolveChallenge | Challenge reference and revealed canary evidence | After reveal; tolerance comparison |
| `0x0B08` | SplitPayout | Settled batch reference and payout shares | Host, or any account after the window; no open challenge |
| `0x0B09` | LockBond | Peer/bid reference and bond amount | Peer; at least room minimum |
| `0x0B0A` | SlashBond | Bond reference and resolved lap fault or missed countersign deadline | Valid fault evidence; one slash per lap |
| `0x0B0B` | RefundBond | Peer bond reference | Last deal closed and challenge window passed |
| `0x0B0C` | CloseRoom | Room ID | Host; no open deals |
| `0x0B10` | RoomView | Room ID | Program read: room, open asks and bids |
| `0x0B11` | DealView | Deal ID | Program read: layer split and settled totals |
| `0x0B12` | PeerView | Peer key | Program read: manifest, bond and fault count |
| `0x0B13` | ReceiptBatchView | Receipt batch reference | Program read: batch digest and quorum |
| `0x0B14` | BookView | Placement book selection | Program read: open asks and bids by model |

Room records contain room ID, host key, model/runtime digests, room profile,
settlement account, open/closed state and opening epoch. Ask records contain ask
ID, asker key, model digest, price ceiling, stream ID and state. Bid records
contain bid ID, peer key, capability digest, price floor, bond amount and layer
capacity. The planned state limit is 196608 bytes with at most 16 rooms,
64 asks and 128 bids; these are planned codec bounds.

The room profile includes minimum bond, canary ratio in basis points, lap size
in generated tokens, challenge window in blocks, slash fraction, host and
protocol shares in basis points, and settlement account. The planned daemon will
also use tolerance, liveness/acknowledgement windows and maximum generated
tokens. The profile file schema must come from the implementation, not from
invented configuration keys here.

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

The planned canary check derives a deterministic sampling key from the **deal ID
and lap index**, then samples positions at the room's canary ratio. The host
computes expected logits locally on the full model or obtains them from the
canary table bound to the model digest. It commits prompt and expected-logit digests through
CommitScore before the lap, compares the peer answer, and on a miss submits
ChallengeLap followed by RevealScore in the existing commit/reveal window.

The specification fixes the derivation inputs but does not yet define a hash,
domain tag, input byte encoding, sampling algorithm or secret input; the pending
daemon canary implementation must provide that interoperable derivation. Public
deal ID and lap index alone do not make a deterministic key unpredictable to
peers. No cryptographic unpredictability claim follows from these identifiers.

The reveal carries the top eight logits at the canary position as signed i32
fixed-point values with 16 fractional bits. ResolveChallenge compares the peer
answer to the expectation using relative tolerance in basis points; zero
tolerance requires equality. A miss marks that peer at fault for the lap; a hit
returns the challenge deposit. Challenges outside the window refuse. Canary
checks bound but do not eliminate wrong answers; see [threats](threats.md).

## Metered stream rule

The planned pay methods at `0x101B` are:

```text
streamOpenForAsk(bytes32 askRef, address settlementAccount, uint256 rate,
                 uint8 rateUnit, uint256 totalCap, bytes32 hostAuthorityKey)
    returns (uint64 streamId)
askStreamOf(bytes32 askRef) view returns (uint64)
```

Each ask pays through one metered stream. The host key is its single meter
authority. **A reading equals the cumulative token count of the last settled
lap batch.** Only streamMeter advances that reading; another key or a regressing
reading refuses. Streaming response chunks and locally estimated usage never
authorize a meter increment. Preserve the stream's rate unit and cap when
encoding a price quoted per thousand tokens. AskStreamOpened binds ask
reference, stream ID and settlement account.

## Payout split and bonds

SplitPayout credits only the settled amount. Host and protocol basis-point
shares come from the room profile; the remaining peer pool is allocated in
the deal's layer-weight proportions. For settled amount `S`, host share `h`
and protocol share `p`, the accounting relation before integer rounding is
`peer_pool = S * (10000 - h - p) / 10000`; peer `i` receives
`peer_pool * weight_i / sum(weights)`. Producer reward math must resolve
integer remainders so the credits sum to `S`; no rounding convention is frozen
by this page. A peer at fault loses that lap share to the other peers. A split
that exceeds settlement or has an open batch challenge refuses.

Credits enter the existing epoch reward ledger. Peers withdraw through CLAIM
(`0x0602`), unchanged; FUND is not the settlement path. The keeper-side draw
handles settlement accounting at the room account. Metering, splitting and
claiming are distinct transitions, each requiring its own evidence.

LockBond precedes PostBid. A resolved canary miss or missed countersign deadline
permits SlashBond at the room slash fraction, credited to the other deal peers'
reward ledger. A second slash for one lap refuses. RefundBond returns the
remainder only after the last deal closes and the challenge window passes;
closing a room alone does not bypass that window.
