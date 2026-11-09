# Inference market client guide

The inference market is not yet available. The SDK, HTTP and UI surfaces below
describe its [planned interface](README.md#availability). A missing operation or
unsupported F11 family must be reported to the caller. Check the target release
and network's availability before preparing market calls.

## SDK calls and node reads

The planned TypeScript SDK module, `inference_market`, will be exported from the
SDK index. Its planned helpers include `openRoom`, `postAsk`, `postBid`,
`recordDeal`, `settleLaps`, `challengeLap`, `resolveChallenge`, `splitPayout`,
`lockBond`, `slashBond`, `refundBond`, `closeRoom`, view decoders, and `fundAsk`
for the `streamOpenForAsk` payload. These are encoders/helpers; use the existing
prepare, disclose, sign, submit and finality path to execute writes.
Their argument types must come from the landed SDK; no runnable imports or
object shapes are available yet.

The planned Rust SDK `InferenceMarket` interface includes `open_room`,
`post_ask` and `get_inference_rooms`, with corresponding F11 codecs and reads
built on the existing ai-market client. Encoders must reproduce producer
fixture bytes, not infer binary layouts from page descriptions.

| TypeScript read | Node root JSON-RPC method | Parameters |
| --- | --- | --- |
| `getInferenceRooms` | `px_getInferenceRooms` | None |
| `getInferenceRoom` | `px_getInferenceRoom` | Room ID |
| `getInferenceDeal` | `px_getInferenceDeal` | Deal ID |
| `getInferencePeer` | `px_getInferencePeer` | Peer key |
| `getInferenceReceipts` | `px_getInferenceReceipts` | Deal ID |

These methods decode F11 program views. Room answers add ask stream state from
`0x101B`; peer answers add the attestation binding from `0x0907`. Use the
configured node root, preserving strict method parameters. For example, on a
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
capacity returns **503** with Retry-After. The planned per-room queue returns
**429** with Retry-After on overflow. Compatibility covers these routes, not
every upstream API feature. The native peer's planned `serve` command forwards
the local OpenAI-compatible route to the host; it does not bypass funding.

All writes retain the existing `submitted`, `receipted`, `committed` evidence
rungs described in [agent compatibility](../COMPATIBILITY.md). A lap receipt
is inference evidence, distinct from a transaction's protocol receipt. Finality
comes from a committed block on the connected network.

## Wallet and explorer paths

The planned wallet inference surface will be registered in the wallet routes.
Its ask form accepts model, price cap per thousand tokens and total cap, then
uses `fundAsk` through the existing wallet send path. The ask list shows stream
reading and remaining cap; earnings combine `px_getInferencePeer` and reward
ledger credits for the connected key. The route URL is not frozen in the
contract; use the registered Inference surface when available. CLAIM remains
the separate signed operation that withdraws credited rewards.

The planned explorer page **`/paxeer-x/inference`** will have an Inference
navigation item. The browser calls **`/api/inference`**; its server
calls the five allowed node methods and keeps the configured node address on
the server. The page presents rooms (model, host, peers, open asks and settled
tokens), the selected deal's layer split and receipt batches. A page listing
records does not verify model correctness or prove that a deployment is ready.
