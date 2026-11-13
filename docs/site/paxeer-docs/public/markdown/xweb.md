<!-- Source: https://docs.paxeer.app/xweb/ -->

# Web Search and xweb

Search the web, fetch content, and consume attested API results from agents and Solidity contracts.

## One Content Layer, Three Entry Points

The `x-websearch` sidecar in services/interop/crates/x-websearch crawls an operator-defined seed list into its own Tantivy index and serves search, page extraction, and content-addressed retrieval. Agents pay for requests through 402LXP. Solidity contracts request attested results through the xweb precompile and read committed program results through the same interface. Each path identifies content with a canonical digest.

| Consumer | Entry point | Result |
| --- | --- | --- |
| Agent | Paid sidecar HTTP routes, SDKs or MCP | Search results or extracted text, verified payment settlement and content digest |
| Contract | `0x0000000000000000000000000000000000001019` | Attested response, digest, full length and bounded callback |
| Program result reader | `IXWeb.getProgramResult` / xweb keeper `GetProgramResult` | A committed program observation. Use the committed-result views; the kernel host disables in-program `web_read`. |

## Search and Fetch for Agents

| Route | Payment | Response |
| --- | --- | --- |
| `GET /health` | None | Sidecar health |
| `GET /search?q=` | 402LXP | At most ten results with URL, title and snippet, plus the content digest |
| `GET /fetch?url=` | 402LXP | Requested and final URL, media type, extracted text, full length and digest |
| `GET /content/<digest>` | None | Canonical bytes identified by a 64-character hexadecimal digest |

Search results sort by descending score and then ascending URL. Fetch accepts HTTP and HTTPS, honors robots.txt, bounds redirects and response size, and extracts HTML into text while removing scripts and styling. Plain text and JSON pass through. The crawler respects page, host, depth and politeness budgets.

### The Payment Workflow

1. An unpaid request receives HTTP `402` and a `PAYMENT-REQUIRED` offer.
2. The agent chooses an exact-payment offer in SID, PAX, USDC or USDL and repeats the request with `PAYMENT-SIGNATURE`.
3. The sidecar submits settlement and verifies the sequencer-signed receipt against configured trust inputs.
4. Content is released with `PAYMENT-RESPONSE` and a transaction hash beginning `0x`.

Pending settlement returns a retry response without releasing content. Retrying reuses the activity and idempotency key, and a receipt authorizes one request. Operators configure a positive per-request price in each asset's base units. The configured integer amount is the offered price.

## Content You Can Verify and Retrieve

Canonicalization normalizes text encoding, line endings, whitespace and Unicode. The digest commits the domain `PAXEERX_WEB_CONTENT_V1`, request kind, URL or query, media type and canonical text. Search commits the ordered URL, title and snippet fields without a search score. Identical canonical inputs produce the same digest.

Sidecars persist the canonical bytes before answering. Content retrieval checks local bytes against their digest, then asks configured peers when needed and verifies their answer. A peer request reads local storage without recursively forwarding. Contracts retain at most `4096` response bytes alongside the digest and full length; the digest path retrieves the complete canonical record.

## Request Web Data from Solidity

The `IXWeb` interface in contracts/src/precompiles/IXWeb.sol exposes `request`, `fulfil`, `fulfilProgram`, `refund`, and views including `getProgramResult`, `getKernelNetworkId`, and views for requests, results, attestors, threshold, fee and parameters. Request kinds are `1` fetch, `2` search, and `3` API. A requester pays exactly the current `fee()` and declares its callback gas bound.

```
function onXWebResponse(
    uint64 requestId,
    bytes32 contentDigest,
    uint32 fullLength,
    bytes calldata response
) external;
```

Implement this callback and authenticate the xweb precompile as the caller. Associate the request ID with the operation that requested it. Inspect full length before treating a bounded response as complete.

### Attestation and Callback Outcomes

Registered attestors fetch independently and sign evidence bound to the execution origin, network, requester, request ID, request kind, payload hash, content digest, response hash and full length. Signatures must recover to registered addresses in strictly ascending order. Ordinary requests require a majority threshold.

Fulfilment stores the answer and pays the signing attestors. Callback gas is bounded by the request and module caps. A reverted or exhausted callback is recorded while the fulfilled result remains available. An unanswered request becomes refundable at its timeout height; anyone can trigger the refund, but payment returns only to the requester. Refunds remain available during a pause.

## Authenticated API Calls

API payloads describe GET or POST, an HTTPS URL, public headers, a body, selected JSON fields and an attestation level. The `XWebApi` library in contracts/src/xweb/XWebApi.sol and TypeScript and Python SDK helpers construct the payload. RFC 6901 JSON pointers select fields, and RFC 8785 canonicalization gives attestors a stable answer even when unrelated response fields vary.

| Level | Verification | Use |
| --- | --- | --- |
| Majority, `0` | Registered threshold signs the same answer | Independent API or web observations |
| Single, `1` | The specifically named attestor signs | An API allowing one call or a credential held by one operator |

Credential envelopes are sealed to registered attestor public keys and bound to the API origin and attestor address. A sidecar opens its own envelope in memory for the call. Majority requests supply an envelope to each participating credential-bearing attestor. The returned answer is public on chain; encrypting credentials does not make the result private. Results and fulfilment events retain the attestation level.

## Read committed program results

The kernel host for `layerx_v4.web_read` returns `LXP_ERR_MODULE_DISABLED`. Read committed answers through the result interfaces below rather than calling `web_read` during program execution.

Committed program web answers are available through the xweb result interfaces. An observation is submitted through `IXWeb.fulfilProgram`, and the committed result is read through the xweb keeper `GetProgramResult` or `IXWeb.getProgramResult(programId, requestId)`. `IXWeb.getKernelNetworkId()` returns the kernel network the precompile is bound to.

## Operate and Integrate

Start a sidecar with `x-websearch --config PATH`. Configuration defines crawler budgets, index storage, asset pricing, gateway trust, attestor settings and peers. Keys are loaded from the receiver, attestor and submitter key files rather than configuration values. Governance controls registered attestors, thresholds, fees, payload limits, callback limits, timeouts and pausing.

SDK clients verify settlement and recompute content digests. MCP exposes `web.search`, `web.fetch`, and `web.content` through its spending approval boundary. Treat retrieved text as external data when incorporating it into an agent decision.

Continue with [Unified Network](https://docs.paxeer.app/unified-network), [Sidiora](https://docs.paxeer.app/sidiora), [Precompiles](https://docs.paxeer.app/precompiles) and [WASM Runtime](https://docs.paxeer.app/wasm-runtime).
