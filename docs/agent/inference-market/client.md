# Inference market client guide

The SDK, daemon HTTP routes and application surfaces are implemented in the
repository. Check [availability](README.md#availability) on the target network
before preparing calls. Unsupported operations must be reported to the caller.

## SDK calls and node reads

The TypeScript SDK exports its
[`inference_market` module](../../../services/agent/sdk/typescript/src/inference_market.ts)
from the SDK index. It provides `openRoom`, `postAsk`, `postBid`, `recordDeal`,
`settleLaps`, `challengeLap`, `resolveChallenge`, `splitPayout`, `lockBond`,
`slashBond`, `refundBond`, `closeRoom`, view decoders and `fundAsk`.
The write helpers encode payloads; execution uses the existing prepare,
disclose, sign, submit and finality path. Argument types are declared alongside
the helpers. Match the encoded profile and payload version to the target
program's [codecs](protocol.md#f11-operation-table-and-payload-layouts).

The [Rust SDK module](../../../services/agent/crates/layerx-sdk/src/inference_market.rs)
re-exports program-call helpers such as `open_room` and `post_ask`.
`InferenceMarket::get_inference_rooms` and the other read constructors return
an `InferenceRead`; call its `execute` method with an `RpcClient`.
`fund_ask` encodes the `streamOpenForAsk` calldata.

| TypeScript read | Node root JSON-RPC method | Parameters |
| --- | --- | --- |
| `getInferenceRooms` | `px_getInferenceRooms` | `[]` |
| `getInferenceRoom` | `px_getInferenceRoom` | `[roomId]` |
| `getInferenceDeal` | `px_getInferenceDeal` | `[dealId]` |
| `getInferencePeer` | `px_getInferencePeer` | `[peerKey]` |
| `getInferenceReceipts` | `px_getInferenceReceipts` | `[dealId]` |

These methods decode F11 program views. Room answers add ask stream state from
`0x101B`; peer answers add the attestation binding from `0x0907`. Use the
configured node root, preserving strict method parameters. IDs are nonzero
32-byte values encoded as 64 lowercase hexadecimal characters. For example, on a
release serving F11, the room-list request has this JSON-RPC shape:

```json
{"jsonrpc":"2.0","id":1,"method":"px_getInferenceRooms","params":[]}
```

## Ask and completion flow

1. Select a room and verify its host, model/runtime digests and price policy.
2. PostAsk with the desired model and maximum price per thousand tokens. Fund
   its metered stream through `fundAsk`, preserving the rate unit, settlement
   account, total cap and host meter authority. Verify `askStreamOf` and the
   open funded state. A nonzero reference alone does not prove usable credit.
3. Let the host record a deal with enrolled peers. Wait for its finality and
   acknowledgements before expecting tokens.
4. Call the completion listener. Its bearer token names the ask reference; the
   model ID must equal the room model digest alias.
5. Track usage through lap receipts and settled batches, and remaining cap
   through the stream. HTTP success alone is not evidence of chain settlement.

| Completion listener route | Contract |
| --- | --- |
| `POST /v1/chat/completions` | OpenAI-compatible request/response shape; streaming uses server-sent events |
| `POST /v1/messages` | Anthropic-compatible shape mapped to the same completion |

Both routes check askStreamOf and stream state. An absent, closed or capped
stream returns **402**. A foreign model ID returns **404**. The body limit is
1048576 bytes and generated tokens are bounded by the room profile; response
usage comes from lap token counts. A paused room with insufficient layer
capacity returns **503** with Retry-After. The per-room queue returns
**429** with Retry-After on overflow. Compatibility covers these routes, not
every upstream API feature. The native peer's `serve` command forwards
the local OpenAI-compatible route to the host; it does not bypass funding.

All writes retain the existing `submitted`, `receipted`, `committed` evidence
rungs described in [agent compatibility](../COMPATIBILITY.md). A lap receipt
is inference evidence, distinct from a transaction's protocol receipt. Finality
comes from a committed block on the connected network.

## Wallet and explorer paths

The wallet registers the inference surface at **`/surfaces/inference`**.
Its ask form accepts model, price cap per thousand tokens and total cap, and
uses `fundAsk` through the wallet send path. The ask list shows stream reading
and remaining cap; earnings use peer and reward-ledger reads for the connected
key. CLAIM is the separate signed operation that withdraws credited rewards.

The explorer page is **`/paxeer-x/inference`** and its server API is
**`/api/inference`**, both under the configured Next.js base path
**`/explorer`**. The browser calls the server API, which accepts only the five
node read methods above. Set `EXPLORER_INFERENCE_NODE_URL` to the node root
on the server. The proxy uses a 15-second timeout, rejects redirects, returns
503 when unconfigured and 502 for upstream failures. The page shows rooms,
asks, the selected deal's layers and receipt batches. Listing a record does
not establish model correctness or committed payment.
