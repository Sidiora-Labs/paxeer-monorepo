# Hosted agent boundary

`layerx-agent-boundary` is the internal TLS HTTP service that reads and
verifies signed chain receipts for the hosted gateway, program registry,
and webhooks. Its crate is `layerx-platform-agent-boundary`, with the binary
in [`src/main.rs`](../../services/platform/hosted/agent-boundary/src/main.rs).
The service identifies itself as `agent-boundary`.

Paxeer is one BFT network: the kernel is integrated into `paxd` and proposes
blocks. EVM chain id is `125` (`0x7d`), and block commit establishes finality.
Callers submit signed EIP-1559 transactions through node JSON-RPC and use
this service to retrieve signed receipts. The current boundary's live routes
are reads; the old activity submission, program execution, simulation, and
account-state relay routes are retired.

See [Assets and tokens](Assets.md) for bank and pointer funding,
[Public JSON-RPC](PublicRpc.md) for chain access, and
[Hosted gateway](HostedGateway.md) for the hosted application surface.
[Agentd](Agentd.md) is the client agent, distinct from this service.

## Deployment

The [Dockerfile](../../docker/platform-agent-boundary/Dockerfile) builds the
binary and starts `/usr/local/bin/layerx-agent-boundary` as user `4020:4020`.
The default listener is `0.0.0.0:9446`. Configure inbound TLS, all three service
credentials, the network label, chain JSON-RPC URL and governed receipt-key
file listed below. Configure this service separately from the native node
described in [Hosted node](HostedNode.md).

## TLS

Inbound TLS uses rustls. `LAYERX_AGENT_BOUNDARY_TLS_CERT_DER` points to the
server certificate in DER format; `LAYERX_AGENT_BOUNDARY_TLS_KEY_DER` points
to a PKCS#8 DER private key.

When `LAYERX_AGENT_BOUNDARY_CLIENT_CA_DER` is set, a presented client
certificate must chain to that CA. The verifier permits clients that present
no certificate (`allow_unauthenticated`); the Bearer token remains required
for receipt routes. Without the CA variable the listener uses no TLS client
authentication.

Outbound HTTPS to the chain RPC validates server certificates using
`/etc/ssl/certs/ca-certificates.crt`. The receipt client accepts HTTP or
HTTPS configuration; public node connections should use HTTPS. It does not
use the inbound client CA as its outbound trust store.

## Credentials and entitlements

All three credential files are required, and their token values must be
pairwise distinct:

| Configuration key | Plane |
| --- | --- |
| `LAYERX_AGENT_BOUNDARY_GATEWAY_TOKEN_FILE` | Gateway |
| `LAYERX_AGENT_BOUNDARY_REGISTRY_TOKEN_FILE` | Registry |
| `LAYERX_AGENT_BOUNDARY_WEBHOOK_TOKEN_FILE` | Webhook |

Requests use `Authorization: Bearer <token>`. Comparisons are constant-time.
Secret files have trailing CR/LF removed; an empty value or a value over
4096 bytes refuses startup. Missing or invalid configuration exits with
code `2`.

## Routes

The parser requires HTTP/1.1 and a Host header. Duplicate headers,
`Transfer-Encoding`, invalid framing, and requests exceeding the total
`1_048_576 + 16 KiB` byte bound are refused. Query strings are not accepted
by the supported routes. Responses are JSON with `Cache-Control: no-store`
and close the connection.

| Method and path | Plane | Success body |
| --- | --- | --- |
| `GET /livez` | None | `{"status":"live","service":"agent-boundary"}` |
| `GET /readyz` | None | Readiness document below |
| `GET /v1/receipts/{hash}` | Gateway | `{"result":{"activity_id":"<hash>","receipt":"<bytes>"}}` |
| `GET /internal/v1/receipts/{hash}` | Registry or Webhook | `{"activity_id":"<hash>","receipt":"<bytes>"}` |
| `GET /internal/v1/deployment-proof/{hash}` | Registry | `{"transaction_hash":"<hash>","receipt":"<bytes>"}` |

Hashes are 32 bytes expressed as 64 hexadecimal characters, optionally
prefixed with `0x`. Returned hashes and receipt bytes are lowercase hex
without that prefix. The compatibility field `activity_id` contains the
chain transaction hash. The deployment-proof route additionally rejects an
all-zero hash.

The deployment-proof route returns a verified receipt, not a complete
program deployment attestation. Registry ingestion separately requires a canonical
`DeploymentProof` and verifies it against native chain trust and the current
authenticated head. See [Registry deployment journal](RegistryDeploymentJournal.md)
and [`node_state.rs`](../../services/platform/hosted/registry/src/node_state.rs).

The old `POST /v1/activities`, program submit/simulate routes, and
idempotency-key receipt routes are retired. The old account-state relay
allowlist is also retired. Authenticated requests to removed routes return
`404 not_found`, subject to the plane checks in
[`deployment.rs`](../../services/platform/hosted/agent-boundary/src/deployment.rs):
for example, the retained internal deploy/upgrade path recognizer gives
non-registry credentials `403 entitlement_denied`, while registry POSTs
receive `404 not_found`. It does not execute a deployment.

## Chain transactions and receipt verification

Use node JSON-RPC at the root URL listed in [Public JSON-RPC](PublicRpc.md).
Chain transactions use
`eth_sendRawTransaction`; `eth_getTransactionReceipt` reports the EVM
execution outcome in a block. Hosted `px_` methods are separately declared
in the gateway's [`OpenRPC contract`](../../services/platform/hosted/gateway/openrpc.json).

The boundary's receipt client calls `pax_sequencerReceipt` with the requested
`0x`-prefixed transaction hash. This method name is retained in the node
contract; it does not imply a separate consensus service. The implementation
is [`chain_receipt.rs`](../../services/platform/hosted/agent-boundary/src/chain_receipt.rs).
It enforces:

- A matching JSON-RPC response identity and transaction hash.
- A signed protocol receipt verified against the configured keys and their
  authorized height ranges.
- A receipt activity id equal to the requested transaction hash, its context
  hash equal to the returned `keyId`, and the returned `signingKey` equal to
  the pinned public key.

A null result or `rung: "submitted"` means no receipt is available yet.
`receipted` and `committed` results are accepted only after signature and
identity verification. A successful boundary HTTP response means receipt
retrieval succeeded; callers must still inspect the protocol result and
establish committed-block inclusion for a final successful operation. The
boundary does not upgrade a `receipted` observation into committed evidence.

Each lookup has a 15-second deadline, with a 3-second connection budget.
Responses are bounded to 4 MiB and headers to 16 KiB. Invalid envelopes,
framing, signatures, identities, and expired or unpinned keys fail closed.

## Config keys

These are the variables read by the current configuration path:

| Key | Role |
| --- | --- |
| `LAYERX_AGENT_BOUNDARY_LISTEN` | Socket address; default `0.0.0.0:9446` |
| `LAYERX_AGENT_BOUNDARY_TLS_CERT_DER` | Required inbound DER certificate file |
| `LAYERX_AGENT_BOUNDARY_TLS_KEY_DER` | Required inbound PKCS#8 DER key file |
| `LAYERX_AGENT_BOUNDARY_CLIENT_CA_DER` | Optional inbound client CA DER file |
| `LAYERX_AGENT_BOUNDARY_GATEWAY_TOKEN_FILE` | Required Gateway Bearer file |
| `LAYERX_AGENT_BOUNDARY_REGISTRY_TOKEN_FILE` | Required Registry Bearer file |
| `LAYERX_AGENT_BOUNDARY_WEBHOOK_TOKEN_FILE` | Required Webhook Bearer file |
| `LAYERX_AGENT_BOUNDARY_NETWORK_ID` | Required network label, 1–64 ASCII letters, digits, or `-_.:`; not the numeric EVM chain id |
| `LAYERX_AGENT_BOUNDARY_CHAIN_RPC_URL` | Required chain JSON-RPC URL; use the node root |
| `LAYERX_AGENT_BOUNDARY_RECEIPT_KEYS_FILE` | Required JSON file of authorized receipt keys |

The receipt-key file is a non-empty JSON array, at most 256 KiB. Entries have
`keyId`, `publicKey`, `firstHeight`, and `lastHeight`; the corresponding
snake_case aliases are also accepted. Keys and ids are canonical padded
base64 encodings of 32 bytes. Key ids must be unique and
`firstHeight <= lastHeight`. Unknown entry fields are rejected.
Provision keys and height ranges from the network's governed receipt-key
configuration, not from an untrusted response. The boundary loads this file
at startup, so a key rotation requires an updated file and process restart.

## Typed refusals

Errors use `{"error":{"code":"…","retry":"never"}}`. Retriable errors
use `retry: "after"`, `retry_after_seconds: 5`, and `Retry-After: 5`.

| HTTP | Code | Condition |
| --- | --- | --- |
| 400 | `invalid_request` | HTTP request parsing or framing failed |
| 400 | `invalid_activity_id` | Receipt hash encoding is invalid; deployment proof also rejects zero |
| 401 | `identity_required` | Bearer identity is missing or invalid |
| 403 | `entitlement_denied` | Credential plane does not own the matched route |
| 404 | `not_found` | Unsupported route, method, or query |
| 404 | `receipt_not_found` | Chain lookup returned null or only a submitted transaction |
| 503 | `node_unavailable` | Chain connection, response, or receipt verification failed; retry after 5 seconds |

The current receipt handler maps invalid signatures to `node_unavailable`;
it does not expose the old activity-submit refusal table. The listener permits
at most 128 simultaneous connections and drops further accepts without an
HTTP response. Inbound socket read and write timeouts are 15 seconds.

## Readiness and recovery

`GET /livez` checks only the running HTTP service. `GET /readyz` performs a
receipt lookup using `SHA-256("LayerX-AgentBoundary-Readiness/v1")` as a probe
transaction hash. A valid absent result is sufficient; success is:

```json
{"ready":true,"network_id":"<configured network label>","chain_receipts":true}
```

A lookup error returns `503 node_unavailable`. Readiness does not prove block
progress or successful execution of a payment. Losing chain access makes
readiness fail while liveness can remain `200`.

The active configuration creates no submission journal or activity index.
Recovery belongs to the transaction producer: persist signed bytes and the
transaction hash before submission, and reconcile that exact hash after an
uncertain send or restart. A missing receipt is not authorization to create
a second payment. Restarting the boundary reloads its trust configuration and
continues reading chain receipts without replaying submissions. For funding
producers and the distinction between wallet and subaccount destinations,
see [Assets and tokens](Assets.md#transfers-and-derived-subaccount-funding).

## Tests

Current receipt behavior is covered in
[`tests/chain_receipt.rs`](../../services/platform/hosted/agent-boundary/tests/chain_receipt.rs):
receipt identity and signature checks, unknown transactions, deployment proof
reads, authorization, and startup without the retired socket configuration.
[`tests/repair_22_31_191.rs`](../../services/platform/hosted/agent-boundary/tests/repair_22_31_191.rs)
covers retired account-state routes, readiness using only chain dependencies,
absence of journal directories, upstream loss with liveness retained, and
pinned key-height enforcement.

[Home](Home.md)
