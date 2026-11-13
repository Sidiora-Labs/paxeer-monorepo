# Inference market operator guide

The [inference-market daemon profile](README.md#availability) and its
configuration are implemented. This procedure also depends on F11 surfaces
whose availability must be checked on the target network.
Use a qualified release with F11 available before following the operational
sequence below.

## Configure and start the host

Run `layerx-agentd` as an off-chain host process. Configure its existing node,
program authority, signing and finality settings using the
[daemon guide](../../../services/agent/crates/layerx-agentd/README.md).
The node URL names the node root, with an explicitly checked chain ID.
Keep the host signing key and node credentials in the existing private key
configuration; room pages and browser configuration must not contain them.

The inference-market profile reads the settings below through
[InferenceConfig](../../../services/agent/crates/layerx-agentd/src/config.rs) and
the [daemon boot path](../../../services/agent/crates/layerx-agentd/src/main.rs).

| Variable | Meaning |
| --- | --- |
| `LAYERX_AGENT_MODE` | `inference-market` |
| `LAYERX_AGENT_INFERENCE_SIGNAL_LISTEN` | WebSocket signalling bind, default `127.0.0.1:8790` |
| `LAYERX_AGENT_INFERENCE_SERVE_LISTEN` | Completion bind, default `127.0.0.1:8791` |
| `LAYERX_AGENT_INFERENCE_MODEL_DIGEST` | Digest of the model served by this room |
| `LAYERX_AGENT_INFERENCE_RUNTIME_DIGEST` | Digest of the exact inference runtime |
| `LAYERX_AGENT_INFERENCE_ROOM_PROFILE` | Path to the JSON `RoomOffer` read at startup |
| `LAYERX_AGENT_INFERENCE_DRAFT_RUNTIME_KIND` | Optional manifest runtime kind marking draft-capable peers: `0`, `1` or `2`. Unset disables speculative slots; other values reject startup. |

With the required digests, room-offer file and existing daemon settings already
configured, start the profile with:

```sh
LAYERX_AGENT_MODE=inference-market layerx-agentd
```

The file named by `LAYERX_AGENT_INFERENCE_ROOM_PROFILE` is decoded as
[`RoomOffer`](../../../services/agent/crates/layerx-agentd/src/inference_market/signal.rs):
`room_id`, `model_digest`, `runtime_digest`, `layer_plan` and `price`.
The IDs and digests are 32-byte JSON arrays. Each layer assignment has
`peer_key`, `first_layer` and exclusive `end_layer`; `price` is a canonical
unsigned decimal string. The file is nonempty and at most 65536 bytes.
The on-chain `RoomProfile` is a separate binary policy containing bond,
settlement and serving limits; see the [protocol guide](protocol.md).

Configuration rejects missing, zero or malformed digest environment values;
each must encode a nonzero 32-byte hexadecimal value. Boot checks node
readiness, program authority and F11 availability before starting listeners.
A generic agent compatibility check alone does not establish F11 support.

## Enrol the host and peers

Optional host self-enrolment reads a signed document from
`LAYERX_AGENT_INFERENCE_ENROLMENT`. It measures the v2 capability manifest
using `LAYERX_AGENT_INFERENCE_REGION`,
`LAYERX_AGENT_INFERENCE_ATTESTATION_FAMILY` and
`LAYERX_AGENT_INFERENCE_BANDWIDTH_PROBE`, submits through the ai-market client
and waits for finality. The enrolment service persists its journal at
`LAYERX_AGENT_INFERENCE_ENROLMENT_JOURNAL`.
LockBond must succeed before PostBid at the configured price quote. A peer's
bid capability digest must match its enrolled manifest.

The [native peer CLI](../../../apps/inference-peer/README.md) provides `join <host> <room>`,
`serve` and `status`. The peer signs its own enrolment, even when the host
relays it through `enrol_peer(manifest, quote)`; it does not give its private
key to the host. Join may provide an attestation envelope file. Only the
development envelope is accepted; see the [threat statement](threats.md).

Peers connect to the signalling listener with a hello carrying a peer key,
signature over room ID and nonce, and v2 manifest. The host checks enrolment,
signature and manifest digest and returns the chain-anchored room offer with
model/runtime digests, layer plan and price. Verify that offer against the
chain before accepting it. Signalling relays WebRTC descriptions and ICE
candidates; model computation travels over peer data channels. The limits
are 64 connections and 65536 bytes per signalling frame.

## Open a room and admit work

Submit OpenRoom through the existing signed program-call path, binding the
model/runtime digests, profile and settlement account. Retain its canonical
disclosure and verify finality before advertising the room. Read RoomView or
`px_getInferenceRoom` to confirm the binding.

The host polls BookView for crossing asks and bids and assigns contiguous layer
ranges, largest accelerator memory first, retaining embedding and head itself.
RecordDeal binds the ask, ordered bids, price, layer plan and current roster
digest. Wait for chain finality and every peer's signed acknowledgement before
generation. A missing acknowledgement requires a new plan; a stale roster or
unfunded ask refuses the record.

Completion requests queue separately for each room at its profile's
positive `queue_depth`; it has no implicit default. The host rotates fairly between
asks, preserves arrival order within each ask, and runs at most one request
per ask at a time. Each room has one generation slot and up to four additional
speculative slots. A speculative request requires a peer in its live deal whose
enrolled manifest advertises the configured draft runtime kind; the host checks
that binding again before serving. Runtime kinds are numeric values without a
protocol-defined draft designation, so set the optional variable only to the
kind used for draft models by your peers. Without it, rooms use only the single
generation slot. A full waiting queue returns HTTP 429 with `Retry-After: 1`
on both completion routes.

## Fund, settle and claim

The asker posts its ask and funds the linked stream through `fundAsk` or the
wallet, authorizing `streamOpenForAsk` at `0x101B` with rate, rate unit,
total cap, the room settlement account and the host meter key. Verify the
ask-to-stream binding and funded state before accepting requests. Completion
routes return 402 for an absent, closed or capped stream.

The host collects countersigned lap receipts, seals the batch root, submits
SettleLaps, and only then submits streamMeter with the final settled cumulative
token count. It commits canary expectations before work and resolves faults
through ChallengeLap, RevealScore and ResolveChallenge. Keep receipt bodies,
signatures and reveal evidence through the challenge window.

Apply SplitPayout to eligible settlement with no open challenge. Read the reward
ledger for the credited epoch and connected peer; submit CLAIM (`0x0602`)
through the existing signed write path and verify its result. An estimated
earning or a submitted claim is not a paid claim. Close remaining deals before
CloseRoom; RefundBond waits for the challenge window and returns only the
unslashed remainder. See [protocol accounting](protocol.md#payout-split-and-bonds).

## Recovery and service limits

A lost node connection puts the profile in degraded state: no new deals,
while existing deals can keep serving. Local generation during a disconnect
does not establish new settlement or payment. Peer loss, a timeout or a slash
mark triggers a new layer plan and successor deal referring to the old deal.
Resume at the last countersigned lap and recompute the lost range's cache.
If remaining capacity cannot cover every layer, pause and return 503 with
Retry-After until capacity returns. This recovery policy cannot promise a
deadline or uninterrupted service.
