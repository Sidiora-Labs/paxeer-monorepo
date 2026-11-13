# Hosted gateway contract

The gateway in [services/platform/hosted/gateway](../services/platform/hosted/gateway) provides hosted account discovery, history, credentials and product ingress. It is an application boundary for the same Paxeer network served by `paxd`. It does not introduce another execution or settlement chain. See [hosted services](platform-hosted.md), [payment middleware](platform-middleware.md) and [protobuf APIs](proto-api.md) for the adjacent boundaries.

Paxeer mainnet-beta has EVM chain ID **125** (`0x7d`). Its kernel is part of the network and proposes blocks. A receipt is final when its block is committed. Sub-chains are tenant domains in the database, launchable without separate chain infrastructure.

## Gateway HTTP routes

The current [HTTP dispatcher](../services/platform/hosted/gateway/src/main.rs) accepts JSON-RPC at `POST /`, with `Content-Type: application/json`. The hosted method schema is available at `GET /openrpc.json` and in the checked-in [OpenRPC document](../services/platform/hosted/gateway/openrpc.json).

| Route | Meaning |
| --- | --- |
| `GET /openrpc.json` | Hosted JSON-RPC method schema |
| `GET /livez` | Process liveness, service identifier and package version |
| `GET /readyz` | Backend and product dependency readiness |
| `GET /readyz/core` | Backend readiness without product route checks |
| `GET /v1/status` | Gateway status |
| `POST /v1/keys`, `GET /v1/keys`, key-specific operations | Session-authorized key issue, listing, rotation and revocation; see `manage_keys` in the dispatcher |
| `GET /internal/v1/principal` | Principal lookup authenticated against the durable API-key store |
| `GET /metrics` | Event-producer health counters |

Additional product and proxy routes are admitted through [routes.rs](../services/platform/hosted/gateway/src/routes.rs) and its service catalogue. A catalogue entry is not proof that a deployment has configured or readied its upstream. The native explorer binding verifies its owner's readiness and identity responses before proxying. [Explorer proxy](../services/platform/hosted/gateway/src/explorer_proxy.rs) and [UI proxy](../services/platform/hosted/gateway/src/ui_proxy.rs) have their own forwarding policies; they are not arbitrary upstream forwarding facilities.

<a id="gateway-jsonrpc-methods"></a>

## Gateway JSON-RPC methods

The [dispatcher](../services/platform/hosted/gateway/src/rpc.rs) checks JSON-RPC 2.0 envelopes, permits batches of **1–32** entries, and omits responses to valid notifications. An all-notification request returns HTTP 204. JSON-RPC errors otherwise travel in a JSON response, commonly with HTTP 200; callers must inspect `error` as well as HTTP status. The `lx_` namespace is rejected with method-not-found.

The OpenRPC contract declares these hosted methods:

| Family | Methods |
| --- | --- |
| Registration and funding | `px_register`, `px_requestFunds` |
| Consensus and receipt material | `px_getCommit`, `px_getValidators`, `px_getReceiptKeys`, `px_getSequenceBatch`, `pax_sequencerReceipt` |
| Accounts and assets | `px_resolveAccount`, `px_getAccount`, `px_getBalances`, `px_listAssets` |
| History and holders | `px_getHistory`, `px_getUnifiedHistory`, `px_listHolderRecords` |
| Discovery | `px_getNetwork`, `px_getCapabilities`, `px_getRouteCatalogue` |

Submit native writes directly to the node with `eth_sendRawTransaction`; read precompile views with `eth_call`. Program reads use the [Programs precompile](programs.md#layerxprograms-0x101d). Proof-backed records follow the [registry evidence contract](programs.md#registry-evidence). Verify the block commit rather than relying on a separate checkpoint or settlement service.

Preserve method-specific parameter and result types from OpenRPC. For example, registration takes `signer_public_key` and `registration_signature`; receipt lookup takes `txHash`; history requires `account` and accepts `cursor`, `limit` and `kind`. History uses the [indexer's position order](../services/platform/hosted/gateway/src/history.rs), `(blockHeight, txIndex, logIndex)`, with opaque 32-character lowercase hexadecimal cursors, a default limit of 50 and a maximum of 100. Pass returned cursors through unchanged.

Public node JSON-RPC uses HTTPS at the configured node origin's root path. Node namespaces are `debug`, `echo`, `eth`, `net`, `pax`, `px`, `test` and `txpool`. The `px` namespace answers from node state; `px_register` and `px_requestFunds` are operations of the hosted identity and faucet services declared in `services/platform/hosted/gateway/openrpc.json`.

## Gateway WS routes

Use the WebSocket transport enabled by the node operator for node subscriptions. [ws.rs](../services/platform/hosted/gateway/src/ws.rs) owns gateway forwarding. Product streams and explorer subscriptions are separate, configured service routes and must retain their authentication, origin and upstream readiness checks.

## Gateway config keys

[routes.rs](../services/platform/hosted/gateway/src/routes.rs) reads a protected binding document with schema `paxeer-x.route-bindings.v1`. Its fields include `chain_id` (125), `network_id`, `wire_version`, `allowed_origins`, `services`, optional `upstreams`, `passthrough` and optional `mcp`. Unknown fields, duplicate origins, unknown service bindings and network/version mismatches are rejected. Upstream origins cannot carry a base path; readiness and identity paths are separate binding fields. The library's service catalogue and ingress policies live in [lib.rs](../services/platform/hosted/gateway/src/lib.rs).

## Gateway env vars

The `LAYERX_` prefix remains a configuration spelling; it does not identify a separate network. The actual readers in `main.rs` and the linked modules define required combinations:

| Variable | Contract |
| --- | --- |
| `LAYERX_GATEWAY_LISTEN` | Listen address; otherwise listens on all IPv6 interfaces at `PORT`, defaulting to 9443 |
| `LAYERX_GATEWAY_LISTENER` | `tls` by default, or `plain`; plain mode rejects configured listener certificate variables |
| `LAYERX_GATEWAY_TLS_CERT_DER`, `LAYERX_GATEWAY_TLS_KEY_DER` | TLS certificate and PKCS#8 key inputs for TLS mode |
| `LAYERX_GATEWAY_OUTBOUND_CA_DER` | Required outbound trust root |
| `LAYERX_GATEWAY_REDIS_URL`, `LAYERX_GATEWAY_REDIS_USERNAME_FILE`, `LAYERX_GATEWAY_REDIS_PASSWORD_FILE` | Durable key, quota and idempotency store and its credentials |
| `LAYERX_GATEWAY_NETWORK_ID`, `LAYERX_GATEWAY_LXP_WIRE_VERSION`, `LAYERX_GATEWAY_PROTOCOL_NETWORK_ID` | Deployment protocol identity; validated together by `configured_protocol` |
| `LAYERX_GATEWAY_PAXEER_RPC_URLS` | JSON array of two to eight distinct configured node RPC URLs; public nodes use root origins |
| `LAYERX_GATEWAY_CONSENSUS_RPC_URL` | Consensus readiness source; must answer a non-catching-up status |
| `LAYERX_GATEWAY_INDEXER_URL` | History read API; plaintext is restricted to an explicit loopback IP and port |
| `LAYERX_GATEWAY_RECEIPT_KEYS_FILE` | Bounded receipt-key document for proof verification |
| `LAYERX_GATEWAY_ROUTE_BINDINGS_FILE` | Protected product route bindings |
| `LAYERX_GATEWAY_HPX_INGRESS_FILE` | Optional protected HPX ingress policy |
| `LAYERX_GATEWAY_IDEMPOTENCY_SECONDS` | Retention, default 604800 seconds, subject to the source bounds |

Optional component integrations require their complete configuration and credentials; setting one URL does not provision a backend. Key authentication uses `Authorization: LayerX-Key <id>:<secret>`, with secrets beginning `lxp_live_`. Key management uses the identity session boundary rather than treating a caller-supplied principal header as authority.

## Gateway error codes

[rpc.rs](../services/platform/hosted/gateway/src/rpc.rs) emits `-32700` for parse errors, `-32600` for invalid envelopes or batch size, and `-32601` for unsupported methods. Method-specific invalid parameters use `-32602`. Backend refusals carry typed error data: for example, [paxeer.rs](../services/platform/hosted/gateway/src/paxeer.rs) uses `-32001` with `data.code` such as `paxeer_rpc_not_configured` or `paxeer_unreachable`. Preserve that data instead of substituting an empty successful result.

HTTP refusals include `untrusted_identity_header` (400), `api_key_required` (401), `insufficient_scope` (403), `not_found` (404), `idempotency_conflict` (409), `json_content_type_required` (415) and `persistence_unavailable` (503). The dispatcher's `KernelUnavailable` distinguishes missing configuration from unreachable dependencies. Liveness does not establish backend readiness or receipt validity.

## Gateway metrics

`GET /metrics` currently returns JSON containing `event_producer_failures` and `event_producer_overflow`. Treat these as producer health counters, not proof of delivery or chain commitment. Readiness reports configuration and probe outcomes separately.

## Gateway Rust pub items

[lib.rs](../services/platform/hosted/gateway/src/lib.rs) owns API-key authentication, principal and quota types, production route classification and receipt-verifying operations. [authority_evidence.rs](../services/platform/hosted/gateway/src/authority_evidence.rs) and [evm.rs](../services/platform/hosted/gateway/src/evm.rs) define proof and EVM adapters; [store.rs](../services/platform/hosted/gateway/src/store.rs) owns Redis persistence. A verified signed response and a committed block prove different stages. Never infer finality from successful submission, gateway availability or a receipt-key signature alone.
