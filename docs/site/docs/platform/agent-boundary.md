# Hosted agent boundary

`layerx-agent-boundary` is an internal TLS HTTP adapter for receipts from the
merged Paxeer node. The kernel belongs to that network; this adapter does not
execute payments or supply another finality authority. The crate is
`layerx-platform-agent-boundary` in
`services/platform/hosted/agent-boundary/`, with binary entry point
`src/main.rs` and service name `agent-boundary`.

The current implementation reads `pax_sequencerReceipt` over chain JSON-RPC and
verifies signed receipt bytes against configured keys. The old activity submit,
simulation, account-state relay and local idempotency-journal routes are removed.
For the local agent runtime, see [Agentd](../agents/agentd.md).

See also [Paxeer settlement](../concepts/paxeer-settlement.md),
[Assets and tokens](../concepts/assets.md), and
[Settlement evidence](../operators/settlement-evidence.md).

## TLS

Inbound TLS uses rustls. Configure the server certificate in DER and its private
key in PKCS#8 DER. An optional client CA validates certificates that clients
present, but `allow_unauthenticated` also permits clients without certificates.
Bearer authorization therefore remains required for protected routes. An
untrusted presented certificate fails TLS validation.

The listener defaults to `0.0.0.0:9446`. At most 128 connections are active;
additional accepts are dropped without an HTTP response. Incoming socket I/O
has a 15-second timeout. The HTTP parser accepts HTTP/1.1 with a Host header,
refuses duplicate headers and `Transfer-Encoding`, and bounds the full request
to `1_048_576 + 16 KiB`. Headers and request bodies are zeroized on drop.
Responses use JSON, `Cache-Control: no-store` and `Connection: close`.

The outbound chain adapter supports HTTP and HTTPS. HTTPS uses the system CA
bundle at `/etc/ssl/certs/ca-certificates.crt` with server-name validation.
Chain DNS resolution and connection attempts are bounded by three seconds;
receipt lookup uses a 15-second deadline and a 4 MiB response bound. See
`src/chain_receipt.rs` for the parser and verification boundary.

## Tokens

All three inbound Bearer secrets are required and must be pairwise distinct.
Files have trailing CR/LF removed and must contain 1–4096 bytes. Token comparison
uses constant-time equality. `GET /livez` and `GET /readyz` precede authentication.
There is no current outbound node Bearer configuration.

| Credential file key | Plane | Permitted protected reads |
| --- | --- | --- |
| `LAYERX_AGENT_BOUNDARY_GATEWAY_TOKEN_FILE` | Gateway | `GET /v1/receipts/{transaction}` |
| `LAYERX_AGENT_BOUNDARY_REGISTRY_TOKEN_FILE` | Registry | Internal receipt and deployment-proof reads |
| `LAYERX_AGENT_BOUNDARY_WEBHOOK_TOKEN_FILE` | Webhook | Internal receipt reads only |

Missing or wrong Bearer returns `401` `identity_required`. A valid token on a
route owned by another plane returns `403` `entitlement_denied`. Unmatched
routes remain `404` `not_found`; authentication does not enable old routes.

## Routes

The route implementations are `route` in `src/main.rs` and `route`/`proof` in
`src/deployment.rs`. Query strings are not accepted on these routes.

| Method and path | Plane | Success document |
| --- | --- | --- |
| `GET /livez` | None | `{"status":"live","service":"agent-boundary"}` |
| `GET /readyz` | None | `{"ready":true,"network_id":"<configured name>","chain_receipts":true}` |
| `GET /v1/receipts/{transaction}` | Gateway | `{"result":{"activity_id":"<hex64>","receipt":"<hex>"}}` |
| `GET /internal/v1/receipts/{transaction}` | Registry or Webhook | `{"activity_id":"<hex64>","receipt":"<hex>"}` |
| `GET /internal/v1/deployment-proof/{transaction}` | Registry | `{"transaction_hash":"<hex64>","receipt":"<hex>"}` |

The identifier is the 32-byte chain transaction hash, supplied as 64 hex digits
with an optional lowercase `0x` prefix. Response identifiers and receipt bytes
are lowercase hex without a prefix. Deployment-proof lookup additionally refuses
a zero transaction hash. These routes use the transaction identity even where
the response field retains the name `activity_id`.

The deployment-proof route validates a receipt's transaction identity and
signature. It does not itself establish a successful deploy, program id,
code hash or BFT commit. The caller must verify those deployment facts and the
committed execution before accepting a program registration.

## Removed submission and relay routes

Authenticated calls to `POST /v1/activities`,
`POST /v1/programs/{call,deploy,upgrade,wind-down,simulate}` and the old program
activity/idempotency lookup routes return `404` `not_found`. Internal program
deploy/upgrade paths also return `404` for the Registry plane and `403` for other
planes. The retained internal deployment-proof read does not submit a deployment.

The old account-state head, receipt account-state, program state/change and
batch receipt-authority relays are removed. An `Idempotency-Key` header cannot
restore any removed operation. The adapter no longer writes a journal or
replays submission attempts after restart.

Submit signed transactions through the chain's supported transaction entry
points. Keep the request digest, transaction hash and application idempotency
record at the submitting caller, then resume receipt lookup after reconnect or
restart. A missing receipt does not justify blindly issuing a second payment.

## Receipt validation

`ChainReceipts::lookup_receipt` posts `pax_sequencerReceipt` for the exact
`0x`-prefixed transaction hash. It validates the JSON-RPC envelope and transaction
identity. Null results and the `submitted` rung produce no available receipt.
Only `receipted` and `committed` results proceed to signature verification.

The adapter decodes the canonical receipt, checks its Ed25519 signature under a
configured key authorized at that height, and requires these bindings:

- Signed activity id equals the requested transaction hash.
- Signed context hash equals the returned `keyId` and a configured key id.
- Returned `signingKey` equals that pinned key's public key.

Both receipted and committed results can return HTTP `200` here. The adapter
returns the signed bytes and does not pass the commitment rung or independently
verify a validator commit. An HTTP `200` receipt is not proof of settlement or
successful execution. Use the commit and merchant checks in
[Paxeer settlement](../concepts/paxeer-settlement.md#merchant-verification).

## Config keys

These are the configuration keys read by the current binary:

| Key | Role |
| --- | --- |
| `LAYERX_AGENT_BOUNDARY_LISTEN` | Socket address; default `0.0.0.0:9446` |
| `LAYERX_AGENT_BOUNDARY_TLS_CERT_DER` | Required server certificate path |
| `LAYERX_AGENT_BOUNDARY_TLS_KEY_DER` | Required PKCS#8 key path |
| `LAYERX_AGENT_BOUNDARY_CLIENT_CA_DER` | Optional client CA path |
| `LAYERX_AGENT_BOUNDARY_GATEWAY_TOKEN_FILE` | Required Gateway secret file |
| `LAYERX_AGENT_BOUNDARY_REGISTRY_TOKEN_FILE` | Required Registry secret file |
| `LAYERX_AGENT_BOUNDARY_WEBHOOK_TOKEN_FILE` | Required Webhook secret file |
| `LAYERX_AGENT_BOUNDARY_NETWORK_ID` | Required network name; 1–64 characters from `[A-Za-z0-9_.:-]` |
| `LAYERX_AGENT_BOUNDARY_CHAIN_RPC_URL` | Required chain JSON-RPC URL, including the intended root path |
| `LAYERX_AGENT_BOUNDARY_RECEIPT_KEYS_FILE` | Required pinned receipt-key JSON file |

Use the operator-provided mainnet-beta node root; the EVM chain id is `125`
(`0x7d`). The hosted gateway has its own `px_` method contract; nodes also
expose registered `px_` reads.
The configured network name is a label, not a chain-id check: this adapter's
startup does not query `eth_chainId`, so validate the target network when
provisioning the URL and pins.

The key file is a nonempty JSON array bounded to 256 KiB:

```text
[{"keyId":"<padded base64 of 32 bytes>",
  "publicKey":"<padded base64 of 32 bytes>",
  "firstHeight":<u64 JSON number>,"lastHeight":<u64 JSON number>}]
```

Snake-case field aliases are accepted. Unknown fields, duplicate ids, invalid
base64 and reversed height ranges refuse startup. Height bounds are inclusive.
The file is not the hexadecimal `{keys:[...]}` RPC envelope returned by
`px_getReceiptKeys`; provisioning must validate and convert that contract.
Configuration and pins load at startup; changing the file requires a restart.
Configuration failure exits with status `2`.

The old socket, protocol-network-id, node URL/Bearer, state directory, module
registry and receipt-wait/deadline settings are removed from the binary's
configuration. They are not aliases for the chain URL or key file.

## Deployment and readiness

Deploy the boundary as a separate service with its TLS listener on port
`9446` by default. Configure its chain URL, receipt-key file and credentials
using the keys above before starting it.

`/livez` checks only that the boundary is serving HTTP. `/readyz` performs a
chain receipt lookup for a fixed probe hash. A valid null result is sufficient;
transport or validation failure returns `503` `node_unavailable` with retry 5.
Readiness does not require a journal, a separate replica or a pending transaction,
and does not certify balance proofs, block commit or an enabled program surface.

## Typed refusals

Refusals use `{"error":{"code":"…","retry":"never"|"after",…}}`.
Retriable errors also carry `retry_after_seconds` and an HTTP `Retry-After`.

| HTTP | Code | Condition |
| --- | --- | --- |
| `400` | `invalid_request` | Invalid HTTP framing, request line or missing Host |
| `400` | `invalid_activity_id` | Invalid transaction hash; zero hash on deployment-proof lookup |
| `401` | `identity_required` | Missing or nonmatching Bearer |
| `403` | `entitlement_denied` | Token plane does not own the route |
| `404` | `not_found` | Removed or unmatched route, method or query |
| `404` | `receipt_not_found` | Null result or only submitted evidence |
| `503` | `node_unavailable` | Chain transport, envelope or receipt-validation failure; retry after five seconds |

The last error intentionally covers verification failures too; it must not be
interpreted as evidence that a payment failed or that resubmission is safe.

## Receipt coverage

The crate's receipt tests cover TLS reads, unknown transactions,
deployment-proof lookup, chain readiness and receipt-key height enforcement.

[Home](../index.md)
