# Web search and xweb

Web data for agents, contracts and programs on Paxeer X Network.

`x-websearch` is an optional sidecar that crawls an operator's seed list into a local Tantivy index and serves search, page fetches and content by digest. Agents pay for search and fetch over the 402LXP HTTP exchange. Contracts request attested answers through the native xweb precompile, and program-origin requests use the same xweb module and attestor set.

Paxeer mainnet-beta is one BFT network running `paxd`, with EVM chain ID `125` (`0x7d`). The kernel is part of the network and proposes blocks. An answer in module state is final when its block is committed. [Unified network](../overview/unified-network.md) describes the network; [Public RPC endpoints](../reference/public-rpc.md) lists the node endpoints, including `https://api-mainnet-beta.paxeer.network` and `https://api1.mainnet-beta.paxeer.network`, at the root path.

Program `web_read` is retired. Its C host returns `LXP_ERR_MODULE_DISABLED`; the runtime maps that refusal to `LXP_NON_CANONICAL`. Read program answers through the xweb module query or `getProgramResult` instead, as described under [The kernel path](#the-kernel-path).

---

## What it is

| Field | Value |
| --- | --- |
| Sidecar | `x-websearch`, one binary and one JSON configuration file |
| Index | the sidecar's crawler and local Tantivy index |
| Accepted assets | `SID`, `PAX`, `USDC` and `USDL`, each by its registered kernel asset id |
| Price | an operator-set positive amount per asset per request, in asset base units |
| Precompile | `0x0000000000000000000000000000000000001019` |
| Module | `xweb`, implemented in `modules/xweb`, holding requests, results, the attestor set and parameters |
| Program import | retired: `web_read(i32,i32,i32,i32)->i32` in the `layerx_v4` group; answers `LXP_ERR_MODULE_DISABLED` |
| Stored response | at most 4096 bytes, with the 32-byte content digest and the full length |

### The sidecar

Start the sidecar with `x-websearch --config PATH`. Its JSON configuration includes:

| Key | Purpose |
| --- | --- |
| `listen`, `data_dir` | HTTP listen address and persistent index, content and journal directory |
| `seeds`, `crawl` | Seed URLs and per-cycle, per-host, depth and politeness limits |
| `crawl_interval_seconds` | Optional crawl interval, default `900` seconds, accepted range `1..86400` |
| `fetch` | Connect, total-time, body-size and redirect limits, plus `allow_loopback` for local tests |
| `assets` | Exactly SID, PAX, USDC and USDL, with distinct registered `asset_id` values and positive `price` strings |
| `evm.endpoint`, `evm.chain_id`, `evm.confirmations` | Node JSON-RPC endpoint, expected EVM chain ID and watcher confirmation depth; use chain ID `125` on mainnet-beta |
| `gateway.endpoint` | Endpoint checked by the readiness probe; settlement and attestation calls use `evm.endpoint` |
| `gateway.receipt_keys` | Receipt-signing authorizations: `key_id`, `public_key`, `first_height`, `last_height` |
| `kernel_network_id` | Optional expected program-origin network ID; startup reads `getKernelNetworkId()` from `0x1019` and rejects a zero value or a configured mismatch |
| `payment` | Optional payment settings, including `payer_address`, `draw_fee_limit` and `conformance_suite`; these do not enable metered offers |
| `peers` | Peer sidecars for content retrieval and attestation exchange |

The program-origin network ID is a module parameter, distinct from the EVM chain ID. Startup requires the node and its configured module state; an HTTP listener alone does not establish readiness. `gateway.receipt_keys` supplies height-scoped Ed25519 keys for signed transaction receipts. The obsolete single-key fields `gateway.sequencer_id` and `gateway.sequencer_public_key` are rejected.

The loader rejects unknown fields, missing required fields, placeholders and key material in configuration. Private keys come only from files named by `X_WEBSEARCH_RECEIVER_KEY_FILE`, `X_WEBSEARCH_ATTESTOR_KEY_FILE` and `X_WEBSEARCH_SUBMITTER_KEY_FILE`. The receiver key is required. Enabling the program attestor relay also requires the submitter key. Key files must not be readable by group or others, and roles must use different keys.

The HTTP server has a bounded worker pool, request-line and header limits, read and write timeouts, and one response per connection. Its routes accept `GET`.

### How it reads the web

- **The crawler walks the seed list inside its budget.** The budget is pages per cycle, pages per host, a maximum depth and a per-host politeness delay. A cycle starts immediately and repeats at `crawl_interval_seconds` (fifteen minutes by default), and every page it keeps goes into the index with its URL, title and body.
- **robots.txt is honoured.** Every crawl and every fetch checks robots.txt for the user-agent token `x-websearch`, and a disallowed URL is refused.
- **A fetch is bounded.** Only `http` and `https` are accepted. Resolved destinations are checked before connecting and on every redirect. Loopback is allowed only with `fetch.allow_loopback`; private, link-local, multicast and unspecified addresses remain refused. Configured limits cannot exceed 3 seconds to connect, 10 seconds total, 2 MiB of body data and 3 redirects.
- **Extraction is deterministic.** `text/html` drops `script`, `style`, `noscript` and `template` elements, turns block elements into line breaks and decodes entities; `text/plain` and `application/json` pass through; every other media type is refused.

---

## Paying for a request

The sidecar currently advertises one `exact` offer for each configured asset. An unpaid search or fetch returns HTTP `402` with a `PAYMENT-REQUIRED` header. The offer identifies the asset, base-unit amount, receiver account and network (`eip155:125` on mainnet-beta).

1. **Pay on the network.** Submit `payExact(bytes32,bytes32,bytes32,uint128,uint64,bytes32)` to the ledger precompile at `0x000000000000000000000000000000000000101a`, using the offered accounts, asset and amount. Bind its reference to the request's idempotency key; expiry is a Unix timestamp in milliseconds.
2. **Retry with payment evidence.** Send `PAYMENT-SIGNATURE` with the x402 payment payload matching the offer. The exact scheme payload names `txHash`, `receiptDigest` and `verificationLevel: "receipted"`, with an optional `idempotencyKey`. The digest is the canonical signed receipt digest.
3. **The sidecar verifies the result.** Through `evm.endpoint`, it checks `eth_chainId` and queries `pax_sequencerReceipt(txHash)`. It verifies the receipt against `gateway.receipt_keys`, verifies the bound logs, and requires a successful `PaymentExact` event from `0x101a` matching the accounts, asset, amount and reference. A transaction hash or an ordinary unverified RPC response is insufficient.
4. **Release follows verification.** The response includes `PAYMENT-RESPONSE` with settlement reference `lxp:<receipt_digest>`. Pending evidence returns `503 payment_pending` with `Retry-After: 1` and releases no content.

The HTTP payment gate accepts verified signed execution evidence at the `receipted` rung; that is earlier than committed-block finality. Do not interpret HTTP resource release as a BFT commit proof. [Commitment levels](commitment-levels.md) and [Finality](finality.md) explain that distinction.

Payment and delivery journals under `data_dir` bind retries to the same request. A receipt cannot pay for another request. Successful search and fetch responses can be replayed from their recorded delivery without charging again, and pending deliveries resume after restart.

Metered payer grants and draws are retired ledger operations. They are not offered by the current sidecar. If a buyer address is supplied, the accepted header is `LAYERX-PAYER-ADDRESS` with a 20-byte `0x`-prefixed EVM address, or the operator can set `payment.payer_address`. Older DID headers and activity-payment payloads are not the current payment contract.

### The price rule

Each operator chooses a positive `price` in each asset's base units. The implementation validates the amount and asset IDs; it does not perform a fiat conversion or enforce a dollar price. Configuration refuses a missing or extra asset, a zero or placeholder asset ID, duplicate IDs and zero prices. Use the amounts in the offer when paying.

---

## The routes

| Route | Payment | What it returns |
| --- | --- | --- |
| `GET /health` | none | Process liveness and network/role metadata; also available at `/xweb/health` |
| `GET /readyz` | none | Dependency and role readiness; HTTP `200` when ready, `503` otherwise |
| `GET /search?q=` | 402LXP | the query, the media type, the content digest and at most ten results, each with `url`, `title` and `snippet` |
| `GET /fetch?url=` | 402LXP | the URL, the final URL after redirects, the media type, the content digest, the text length and the extracted text |
| `GET /content/<digest>` | none | the canonical bytes stored under that digest |

When serving dependencies are not ready, search and fetch return `503 service_not_ready`; content-by-digest retrieval remains available. A search queries the index and returns at most ten results ordered by score descending and then by URL ascending. A search and a fetch both write their canonical bytes to the content store under their digest before they answer, so the same bytes can be asked for again later.

### Content by digest

`GET /content/<digest>` takes the digest as 64 hexadecimal characters and returns exactly the canonical bytes, as `application/octet-stream`.

- A sidecar that holds the digest serves it from its own store. A stored file whose bytes no longer hash to its name is removed and reads as absent.
- A sidecar that does not hold it asks its configured peer sidecars in turn, recomputes the digest of what each one answers, keeps the first answer whose digest matches, and serves it.
- An unavailable digest returns `404` after the configured retrieval attempts. This does not prove that no other sidecar holds it. A peer request marked `X-Websearch-Peer` is answered from the local store only and never forwarded further.

Full canonical content can be retrieved independently of the 4096-byte on-chain response bound, while a reachable sidecar retains it. A digest authenticates bytes; it does not guarantee their availability.

---

## The canonical content digest

Identical kind, request payload, normalized media type and canonical text produce the same digest. No clock or node identity enters it. Different local indexes or changing upstream pages can produce different answers, so attestors must agree on the same digest.

The extracted text is canonicalised first: decoded to UTF-8 from the declared charset with invalid input refused, CRLF and CR turned into LF, control characters other than LF and TAB removed, runs of spaces and tabs collapsed to one space, trailing spaces removed from each line, three or more consecutive LF collapsed to two, leading and trailing whitespace trimmed, and the result put in Unicode NFC. For a search, the text is the compact JSON array of the results with the keys `url`, `title` and `snippet` in that order, and no score in the bytes.

The canonical bytes are then:

| Part | Encoding |
| --- | --- |
| domain | the ASCII bytes `PAXEERX_WEB_CONTENT_V1` |
| kind | one byte: `1` fetch, `2` search, `3` API |
| payload | a big-endian `uint32` length, then the request payload: URL, query or encoded API request |
| media type | a big-endian `uint32` length, then the media type in lower case without parameters |
| text | a big-endian `uint64` length, then the canonical text |

The content digest is `keccak256` of those bytes. API answers use kind byte `3` and the API payload, with the selected JSON values or bounded raw body described below. Storage-receipt answers use the fetch content encoding with the storage payload; the attestation request kind remains `4`.

---

## The contract path

Contracts call the xweb precompile at `0x0000000000000000000000000000000000001019`. The Solidity interface is `contracts/src/precompiles/IXWeb.sol`, and `contracts/src/xweb/XWebConsumer.sol` is a reference consumer.

| Call | What it does |
| --- | --- |
| `request(uint8 kind, bytes payload, uint64 callbackGas)` | payable with exactly `fee()`; stores the request under the next id and returns it |
| `fulfil(uint64 requestId, bytes response, bytes32 contentDigest, uint32 fullLength, bytes[] signatures)` | verifies the attestor signatures, stores the result, pays the signers and delivers the callback |
| `refund(uint64 requestId)` | returns the fee of a request not fulfilled by its timeout height to its requester |
| `getRequest`, `getResult`, `getAttestors`, `threshold`, `fee`, `getParams` | Contract request, result, attestor and parameter views |
| `fulfilProgram(bytes observation)` | Validates and stores an origin-2 answer through the module keeper |
| `getProgramResult(bytes32 programId, uint64 requestId)` | Reads an origin-2 answer: digest, full length, response length and response |
| `getKernelNetworkId()` | Reads the module's program-origin network ID |

### Request

A contract names the kind, payload and callback gas, and sends exactly `fee()` in EVM wei. The fee is represented in whole bank base units, where one base unit is `10^12` wei.

| Kind | Payload |
| --- | --- |
| `1` fetch | URL bytes |
| `2` search | Query bytes |
| `3` API | Encoded HTTP request, selectors and optional credential envelopes; see [Calling an API](#calling-an-api) |
| `4` storage receipt | Nonzero 32-byte object digest followed directly by a UTF-8 HTTPS base URL of 1 to 512 bytes, without userinfo, query, fragment or trailing slash |

For kind `4`, the sidecar fetches `<baseURL>/manifests/<digest>.json`, checks its SHA-256 against the object digest, and attests the receipt text `pns-receipt/v1\n<digest>\n<byteLength>\n<baseURL>\n`. This is distinct from fetching and attesting page text.

The module rejects paused requests, unknown kinds, empty or oversized payloads, malformed API or storage payloads, wrong fees and callback gas outside `1..maxCallbackGas`. On success it escrows the fee in the module account, stores the next request ID with its requester, kind, payload hash, callback gas, fee, height and timeout height, and emits `XWebRequested` for attestors.

### Fulfil

`fulfil` rebuilds the 188-byte attestation preimage from the stored request and the submitted response, digest and length, and verifies the signatures over its `keccak256`. It refuses a response longer than 4096 bytes, a full length shorter than the response, a request that is unknown, fulfilled or refunded, and a paused module. On success it stores the response, the digest, the full length and the signers, marks the request fulfilled, and splits the fee equally among the signing attestors' payout accounts, with the integer remainder to the lowest signer.

### The callback

The precompile then calls the requester, from `0x0000000000000000000000000000000000001019`:

```solidity
function onXWebResponse(uint64 requestId, bytes32 contentDigest, uint32 fullLength, bytes calldata response) external;
```

The callback's gas is bounded by the smaller of the request's `callbackGas` and the module's maximum, and `fulfil` refuses unless the gas left covers that full bound plus the 10000 gas callback-recording cost. A callback that reverts or runs out of gas is recorded in the result - delivered, reverted or out of gas, with the gas it used - and the fulfilment stands. `XWebFulfilled` carries the same outcome together with the attestation level, `0` majority or `1` single, which `getResult` also returns.

### Refund

A request not fulfilled by its timeout height is refundable exactly once. Anyone may trigger the refund; the fee only ever goes back to the requester. A refunded request is never fulfilled, and refunds stay open while the module is paused, so a pause never holds a fee.

### Gas

Calls cost a base plus 16 gas per byte of calldata after the selector. The base is 3000 for views and 30000 for writes. `fulfil` adds 8000 per signature and the callback's gas used plus 10000 to record it; `fulfilProgram` also charges 8000 per observation signature and has no callback.

---

## The attestors

Attestors are sidecars holding a web-attestor secp256k1 key, separate from any consensus, validator or bridge key. Governance registers each attestor's 20-byte signer address together with its payout account and, for an attestor that accepts credential envelopes, the 33-byte compressed public key of the same key, which must derive the signer address. `getAttestors` returns all three.

**The threshold is always a majority.** Only an api request under the single level is answered by one attestor, as [Calling an API](#calling-an-api) describes. The keeper refuses a threshold at or below half of the registered set or above it. Registering an attestor raises the threshold to the majority of the new set when it would otherwise fall below it; removing one lowers a threshold above the new set size to that size, which is still a majority.

Each attestor watches the request event, fetches independently, and signs one raw digest: `keccak256` of a 188-byte preimage, with no EIP-191 prefix and no EIP-712 domain.

| Field | Size | Encoding |
| --- | --- | --- |
| domain | 14 | the ASCII bytes `PAXEERX_WEB_V1` |
| origin | 1 | `1` for the EVM precompile, `2` for a program-origin request in the xweb module |
| network id | 32 | the EVM chain id for origin 1, the module's `kernel_network_id` for origin 2 |
| requester | 32 | the EVM address left-padded with zeros for origin 1, the program id for origin 2 |
| request id | 8 | `uint64` |
| kind | 1 | `1` fetch, `2` search, `3` API, `4` storage receipt; origin 2 supports only `1` and `2` |
| payload hash | 32 | `keccak256` of the request payload |
| content digest | 32 | the canonical content digest |
| response hash | 32 | `keccak256` of the stored response bytes |
| full length | 4 | `uint32`, the length of the full text |

Every integer is big-endian. Each signature is 65 bytes, `r` then `s` then `v`, with `v` in 27 or 28 and `s` at or below half the secp256k1 order. Signatures are ordered by strictly ascending recovered signer, which also refuses a repeated signer; every signer must be registered. Majority requests require at least the threshold of signatures; single-level API requests require exactly the named attestor's signature. The attestors exchange signatures, and a submitter that is part of the sidecar posts `fulfil` once the threshold is reached, ordering the signatures by ascending signer and treating an already-fulfilled request as completed. The current codec is `modules/xweb/types/attestation.go`, with shared vectors in `modules/xweb/types/testdata/preimage-vectors.json`. Both origins use the module's registered attestor set; they do not have separate finality authorities.

---

## The kernel path

Program-origin web work is origin `2` in `modules/xweb`. Programs execute inside the network and never perform outbound HTTP during execution.

1. **Record the request and escrow together.** A hosted program call emits `PAXEERX_WEB_REQUEST_V1` through `event_emit`. Event data is a big-endian `uint64` request ID, one-byte kind (`1` fetch or `2` search), big-endian `uint32` payload length and exactly that many payload bytes. The Programs bridge in `precompiles/layerxprograms/xweb.go` passes the event and actual transfer legs to `Keeper.RequestFromProgram`. A positive fee in one configured asset must debit the call principal and reach that asset's web fee account; the module records the asset, bank denom, amount, payload hash and sequence. Admission and bank transfers commit atomically, and a call can record at most one web request.
2. **Watch native request events.** The xweb precompile's `RequestFromProgram` adapter emits `XWebProgramRequested(bytes32,uint64,uint8,bytes,uint64)` with program ID, request ID, kind, payload and recording sequence. The sidecar's program relay watches that event in node logs, fetches or searches independently and exchanges origin-2 signatures. The signed network ID comes from `getKernelNetworkId()`, and the requester is the 32-byte program ID.
3. **Submit to the module.** The submitter sends `fulfilProgram(bytes observation)` at `0x1019`. `Keeper.FulfilProgram` checks activation, pause state, the canonical observation, network ID, pending program/request pair, kind, payload hash and the shared attestor majority. Responses are bounded to 4096 bytes. It stores the answer, splits the escrow among signer payout accounts and marks the request fulfilled in one cached state update. Duplicate fulfilment is refused before a second payout, including after a restart.
4. **Read the committed answer.** `getProgramResult(programId, requestId)` at `0x1019`, the module's `ProgramResult` query and `Keeper.GetProgramResult` expose the digest, full length, response length and bytes. `ProgramRequest` exposes the persisted request record and fee denom. These are module records under the committed application state; block commit supplies finality. Completed answers moved out of the bounded hot store remain queryable from the module archive.

The current Programs bridge calls `Keeper.RequestFromProgram` directly, while the event-emitting adapter lives in `precompiles/xweb/xweb.go`. The direct keeper call does not emit `XWebProgramRequested`. Automatic relay discovery therefore requires that event integration; persisting a pending request alone does not notify the watcher.

The CLI exposes `paxd query xweb program-request <program-id> <request-id>`, `program-result` with the same arguments, and `program-counters`. Use a 32-byte `0x`-prefixed program ID and decimal request ID.

Program-origin requests require the `v7.0` activation, an unpaused module, a nonzero `kernel_network_id` and configured `program_fee_denoms`. Before activation the origin refuses with `LXP_MODULE_DISABLED`. Program request IDs remain replay-protected after fulfilment. The contract `refund(uint64)` entry operates on origin-1 request IDs; it is not a program escrow refund API.

Program `web_read` remains retired. The ABI retains `web_read(i32,i32,i32,i32)->i32` in `layerx_v4`, but `src/modules/programs/call.c` returns `LXP_ERR_MODULE_DISABLED` from its C web-view hook. The runtime maps that refusal to `LXP_NON_CANONICAL`. The Rust SDK's `web::read` and the web-reader example's frozen read entrypoint cannot retrieve module answers through this import. Use the module query or EVM view above. [Programs](../programs/index.md) describes program execution.

---

## Calling an API

A contract can ask for an HTTP API call beyond the network with the request kind `3`, api. The payload names the method (`GET` or `POST`), an `https` URL, public request headers, a request body, the JSON fields to attest and the attestation level, and it may carry credential envelopes. The Solidity library `XWebApi` builds that payload and submits it in one statement:

```solidity
import {IXWebConsumer, XWEB_CONTRACT, XWEB_PRECOMPILE_ADDRESS} from "../../precompiles/IXWeb.sol";
import {XWebApi} from "../XWebApi.sol";

contract ApiConsumer is IXWebConsumer {
    error WrongFee(uint256 supplied, uint256 expected);

    mapping(uint64 => bytes) public prices;

    function askPrice(bytes calldata envelope, address attestor) external payable returns (uint64) {
        uint256 fee = XWEB_CONTRACT.fee();
        if (msg.value != fee) revert WrongFee(msg.value, fee);
        return XWebApi.get("https://paxeer.app/api/v1/price?asset=PAX").select("/data/price").withCredential(envelope)
            .single(attestor).submit(200_000);
    }

    function onXWebResponse(uint64 requestId, bytes32, uint32, bytes calldata response) external {
        require(msg.sender == XWEB_PRECOMPILE_ADDRESS, "only xweb");
        prices[requestId] = response;
    }
}
```

`get` and `post` start the call, `header` adds a public header, `select` adds a JSON pointer, `withCredential` adds an envelope and `single` names the one attestor that answers. `submit` pays the current `fee()` from the contract's balance and returns the request id. The example lives at [`contracts/src/xweb/examples/ApiConsumer.sol`](https://github.com/Sidiora-Labs/paxeer-monorepo/blob/main/contracts/src/xweb/examples/ApiConsumer.sol).

| Part | Bound |
| --- | --- |
| URL | 1 to 2048 bytes of printable ASCII, `https://` with a lower-case host, no fragment |
| public headers | at most 16, names of at most 64 token characters, values of at most 1024 bytes; `Host`, `Content-Length`, `Transfer-Encoding`, `Connection`, `Keep-Alive`, `TE`, `Trailer` and `Upgrade` are refused |
| body | at most 4096 bytes, empty under `GET` |
| JSON pointers | at most 16, each at most 256 bytes |
| envelopes | at most one per registered attestor, never two for one attestor |
| whole payload | the payload cap, `8192` bytes by default |

The module refuses a payload that breaks one of those bounds, names an attestor outside the registered set or carries more envelopes than there are registered attestors.

### The envelope rule

Put secret API keys and bearer tokens in credential envelopes, never in public headers or the body. The module does not identify secrets placed in public fields. Only the sealed envelope belongs in chain state and request events; the sidecar decrypts it in memory for the outbound call. It travels as one envelope per attestor, sealed to that attestor's registered public key:

- **The seal.** A fresh ephemeral secp256k1 key per envelope; the shared secret is the x coordinate of the ephemeral key times the attestor's key; HKDF-SHA256 over it, salted with the compressed ephemeral key under the info `PAXEERX_WEB_API_ENVELOPE_V1`, gives the AES-256-GCM key; a fresh 12-byte nonce.
- **The binding.** The associated data is the attestor's address and the origin of the URL - `https://` and its host and port - so an envelope copied into a request for another origin does not open.
- **The layout.** The attestor's 20-byte address in the clear, so each sidecar picks its own, then the 33-byte ephemeral key, the nonce, the ciphertext and the 16-byte tag.
- **The use.** The sidecar opens its envelope in memory for the one call and adds the credential headers to the public ones. A request that carries envelopes but none for a sidecar is refused by that sidecar rather than called without the credential.

When credentials are used under the majority level, every attestor that makes the call needs its own envelope. An attestor replaced through governance stops receiving envelopes as soon as `getAttestors` stops returning it, and the holder of a credential rotates it at the API as with any other client.

### The selector rule

The attested answer is not the whole response. Each JSON pointer (RFC 6901) selects one field of a JSON response, and the selected values are put in RFC 8785 canonical form, so attestors agree on the answer even when an API's full responses differ from call to call. A response that is not JSON when pointers are given, or a pointer that resolves to nothing, is refused and the pointer is named. With no pointer the attested answer is the raw bounded body.

### The two attestation levels

| Level | Who answers | What `fulfil` needs |
| --- | --- | --- |
| majority (`0`) | Registered attestors independently; each needs its own envelope if credentials are used | the threshold of signatures over the same answer, exactly as for a fetch |
| single (`1`) | the one attestor `single` names | one signature, from that attestor only |

The single level is for an API that tolerates one call only, or one credential that only one operator holds. The level is stored in the result and carried in `XWebFulfilled`, so a consumer can tell a single-attestor answer from a majority one. An attested answer is public on chain like every other result, whatever credential produced it.

### Building envelopes off chain

`services/agent/sdk/typescript` and `services/agent/sdk/python` build the payload and its envelopes from the attestor set `getAttestors` returns, so a developer needs no encoding knowledge:

```typescript
const set = decodeXWebAttestors(await provider.call({ to: XWEB_PRECOMPILE, data: xwebGetAttestorsCallData() }));
const request = buildXWebApiRequest(
  { method: "GET", url: "https://paxeer.app/api/v1/price?asset=PAX", pointers: ["/data/price"] },
  set,
  { credential: [{ name: "X-Api-Key", value: apiKey }] },
);
const call = xwebApiRequestCall(request.payload, 200_000n, fee);
```

In Python the same steps are `decode_xweb_attestors`, `build_xweb_api_request` with `credential=` and `xweb_api_request_call`. Under the majority level the helpers seal one envelope to every attestor that registered a public key and refuse when fewer of them than the threshold did; with `single` they seal only to the named attestor. Every public key is checked against its signer address before anything is sealed, and the ephemeral key and nonce come from the operating system's secure random source. The payload bytes and the envelopes are pinned against the same vectors the module's Go codec is tested with.

---

## Agent clients

`services/agent/sdk/typescript` and `services/agent/sdk/python` contain web-search clients and content-digest helpers. Their web-search payment code still contains legacy DID headers and exact-payment payload fields; do not assume those helpers complete the current sidecar exchange unchanged. Match the exact offer and transaction-receipt payload described under [Paying for a request](#paying-for-a-request). The API-envelope helpers above are a separate contract-request path.

The MCP server exposes `web.search`, `web.fetch` and `web.content` through its approval boundary. Tool registration alone does not establish payment compatibility. Treat returned web text as untrusted external content. [SDKs](../agents/sdks.md) and [MCP](../agents/mcp.md) describe the clients around them.

---

## The governance surface

The module parameters and the attestor set change only through governance messages carrying the module authority, which is the gov module account.

| Message | What it changes |
| --- | --- |
| `MsgRegisterAttestor` | adds a signer address with its payout account and, optionally, its envelope public key |
| `MsgRemoveAttestor` | removes a signer |
| `MsgSetThreshold` | sets the threshold, within the majority rule |
| `MsgSetParams` | Sets fee, payload/callback caps, timeout, `kernel_network_id` and `program_fee_denoms` |
| `MsgPause`, `MsgUnpause` | stops and restarts requests and fulfilments; refunds stay open |

| Parameter | Default |
| --- | --- |
| payload cap | `8192` bytes |
| callback gas cap | `500000` |
| timeout | `3600` blocks |
| fee | `1000000` bank base units (one PAX); use `fee()` for the EVM wei value |
| program network ID / fee denoms | `0` / empty until configured |
| attestor set | empty |
| paused | yes |

A pending request keeps the fee and the timeout height it was stored with when the parameters change. The module starts paused with no attestors: governance registers the attestors and sets the required parameters before unpausing it. Program fee denoms are selected from `uhpx` (PAX), `usid`, `uusdc` and `uusdl`. The set is capped at 16 attestors.

---

## Where it stands

The current tree wires xweb into the chain application. The program-origin keeper explicitly gates admission and fulfilment on `v7.0`. The presence of code is not evidence of activation or governance configuration on a particular node: inspect its parameters, pause state, attestors and request/result state before using it. The sidecar also requires usable chain dependencies and registered receipt keys; check `/readyz` as well as `/health`.

Source in the [repository](https://github.com/Sidiora-Labs/paxeer-monorepo):

- Sidecar configuration, routes, payment verification, digest and relay: `services/interop/crates/x-websearch/src`.
- Native requests, escrow, results, queries and governance: `modules/xweb/keeper` and `modules/xweb/types`.
- EVM dispatch, result views and callbacks: `precompiles/xweb`.
- Program event/transfer bridge: `precompiles/layerxprograms/xweb.go` and `precompiles/layerxprograms/events_call.go`.
- Solidity ABI and consumers: `contracts/src/precompiles/IXWeb.sol` and `contracts/src/xweb`.
- Retired guest read hook and its runtime mapping: `src/modules/programs/call.c` and `programs/crates/layerx-programs-runtime/src/ffi_call.rs`.
- Guest SDK and frozen example: `programs/sdk/rust/src/web.rs` and `programs/sdk/rust/examples/web-reader`.
- Agent SDKs and MCP tools: `services/agent/sdk` and `services/agent/crates/layerx-mcp`.
