<!-- Source: https://docs.paxeer.app/developer-funding/ -->

# Developer funding and allocations

Request a bounded development allocation, track its signed treasury transfer, and verify the funded account before your first Paxeer X payment.

## Choose the right funding path

The hosted allocation service distributes operator-approved development funds in its configured environment. A developer or agent authenticates, names a recipient identity and public key, and receives a funded result or a typed pending or refusal outcome. The service controls the allocation amount, treasury, eligibility, and quota policy.

| Funding need | Mechanism | What the result establishes |
| --- | --- | --- |
| Approved developer allocation | Authenticated claim backed by a signed treasury SEND | The configured environment allocated funds to the named identity |
| User deposit | Custody deposit evidence and authenticated custody credit | A deposit-backed credit for its beneficiary |
| Market-maker inventory | Inventory and liquidity arrangements for the selected venue and Asset | The balance and obligations of that arrangement |
| Fiat purchase or conversion | The selected provider and payment rail | The provider-specific funded or converted outcome |

A development allocation is scoped to its environment and policy. It does not establish a general entitlement to free real-value Assets, fiat conversion, or unlimited liquidity. Public account deposit funding uses the custody credit path. Use the allocation endpoint supplied for your approved development environment rather than assuming a public faucet hostname.

## Developer journey

1. Obtain access to the configured allocation environment and an active hosted identity session.
2. Create or select a recipient signing identity. Record its public DID and 64-hex Ed25519 public key without exposing the seed.
3. Choose and persist an idempotency key for this allocation request.
4. Submit the claim and retain the funding identifier and transaction identifier.
5. Resolve pending outcomes under the same request key.
6. Verify the transfer receipt and recipient balance before sending a payment.

## Authentication and roles

| Actor | Credential | Authority |
| --- | --- | --- |
| Developer or agent | Hosted identity session Bearer | Request an allocation subject to the session principal and quota policy |
| Gateway service | Dedicated service-claim Bearer | Submit a claim naming the authenticated principal |
| Allocation service | Identity introspection service credential | Validate the session and resolve its active principal |
| Funding controller | Control-admin credential | Authorize the funding operation against the configured treasury |
| Treasury owner | Treasury signing authority | Sign the canonical transfer and its owner authorization |

The caller's session token is introspected as request data; it is never reused as an upstream service authorization credential. The direct claim resolves an active session subject. The service claim accepts its separate configured credential and supplies a principal explicitly. Keep service and treasury credentials on trusted servers.

## Submit a direct claim

```
POST /v1/faucet/claims
Authorization: Bearer <hosted-session-token>
Idempotency-Key: developer-allocation-order-01
Content-Type: application/json

{
  "did": "did:layerx:<lowercase-public-key-hex>",
  "public_key": "<64-hex-public-key>"
}
```

The JSON body accepts only `did` and `public_key`. The allocation amount is configured by the service rather than chosen in the claim. Use the key-derived DID that the treasury funding path validates. `Idempotency-Key` is 1–128 characters from letters, digits, hyphen, underscore, dot, and colon.

### Service and JSON-RPC access

A configured service caller uses `POST /v1/faucet/service-claims` with the same headers and a body containing exactly `principal`, `did`, and `public_key`. Direct and service requests for the same principal share its identity quota. The service route must be configured with its dedicated credential.

```
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "px_requestFunds",
  "params": ["did:layerx:<lowercase-public-key-hex>", "<64-hex-public-key>"]
}
```

The configured gateway proxies this method to the service-claim flow. The `layerx faucet` CLI command and the `faucet.request` tool use the same allocation capability. Note: Rust `RpcClient::request_funds` currently still calls the retired `lx_requestFunds` name, which the gateway refuses with `-32601`; call `px_requestFunds` directly until the SDK is updated. Gateway and environment eligibility still apply.

## Amounts and configurable ceilings

Amounts are integer base units of the configured treasury Asset. Interpret them using that Asset's decimals and metadata; do not assume an allocation amount is a fiat amount or a whole-token count. Responses encode amounts as decimal strings.

| Configuration | Default | Meaning |
| --- | --- | --- |
| `LAYERX_FAUCET_CLAIM_AMOUNT` | 1,000,000 base units | Positive allocation size per accepted claim |
| `LAYERX_FAUCET_IDENTITY_LIMIT` | 10,000,000 base units | Principal total per quota window |
| `LAYERX_FAUCET_ADDRESS_LIMIT` | 10,000,000 base units | Recipient public-key total per quota window |
| `LAYERX_FAUCET_NETWORK_LIMIT` | 50,000,000 base units | Network bucket total per quota window |
| `LAYERX_FAUCET_WINDOW_SECONDS` | 86,400 seconds | Allocation quota window |
| `LAYERX_FAUCET_NETWORK_REQUEST_LIMIT` | 60 requests | Network admission ceiling |
| `LAYERX_FAUCET_NETWORK_REQUEST_WINDOW_SECONDS` | 60 seconds | Admission-count window |
| `LAYERX_FAUCET_IDEMPOTENCY_SECONDS` | 604,800 seconds | Request retention; must cover the quota window |

These are service configuration defaults, not a promised allowance for every deployment. The gateway additionally bounds its claim request rate at 30 per minute. The direct service derives network admission from the TCP peer; service claims use a principal-scoped network bucket. Forwarded identity or IP headers are refused rather than trusted as quota authority.

## Signed canonical funding

The allocation service reserves quota and an idempotency record atomically, then sends its controller a funding command containing `funding_id`, `did`, `public_key`, and `amount`. The controller constructs an owner-authorized native Asset SEND from the treasury main account to the recipient main account.

Preparation reads the treasury identity sequence separately from its state-proven source-account sequence and verifies sufficient treasury Asset balance. The signing disclosure binds the canonical envelope, debit authorization, recipient, amount, validity interval, fee limit, and idempotency key. The transfer uses current sequences and the validity interval set at preparation.

The core submission follows the ordinary signed activity and receipt verification path. Result code zero produces a funded result. A refused transfer produces a refusal, and a receipt timeout remains pending. Allocation does not mint arbitrary supply or bypass normal transfer authorization.

## Response verification and recovery

```
{
  "funded": true,
  "funding_id": "<funding-id>",
  "transaction_id": "<transaction-id>",
  "amount": "1000000",
  "network": "layerx-testnet"
}
```

This illustrates the default claim amount and the allocation response's environment label. Direct HTTP success is an unwrapped body rather than an `ok` envelope; its transaction identifier is optional at that layer. The gateway additionally requires a transaction identifier and a complete funded response before returning JSON-RPC success.

Retain the returned identifiers and verify the transaction's canonical receipt, recipient, Asset, amount, result code, network authority, and requested commitment. Confirm the recipient account with `px_getAccount` or `px_getBalances`. The retired `lx_*` names return `-32601`. A funding identifier is a correlation reference, not an independent finality proof.

```
HTTP/1.1 202 Accepted
Retry-After: 10

{
  "state": "still_checking",
  "retry": "after",
  "retry_after_seconds": 10
}
```

Repeat the original claim with the same idempotency key and unchanged body after the carried delay. The service recovers the same funding identifier. A stored funded response is replayed; a different request digest under the same key returns `409 idempotency_conflict`. The gateway maps pending allocation to JSON-RPC `-32001` with `faucet_claim_pending`, never a funded result.

## Typed refusals

| HTTP | Code | Resolution |
| --- | --- | --- |
| 400 | `idempotency_key_required`, `invalid_idempotency_key`, `invalid_argument` | Correct the request before retrying |
| 400 | `untrusted_identity_header` | Remove caller-supplied forwarded identity headers |
| 401 | `identity_required`, `service_token_required` | Authenticate with the correct credential for the route |
| 409 | `idempotency_conflict` | Recover the original request rather than modifying its body |
| 429 | `network_request_rate`, `identity_quota`, `address_quota`, `network_quota` | Respect the supplied retry delay and configured allowance |
| 503 | `identity_unavailable`, `persistence_unavailable` | Retain the request key while dependencies recover |
| 503 | `funding_rejected` | Inspect the destination and treasury funding refusal |

## Owner operations

Configure the allocation amount and caps, supply a funded treasury with fee balance, and provision separate identity, service-claim, and control credentials. Persist quota and idempotency state in the configured TLS Redis service. Keep the funding journal durable across controller restarts and retain its request digest bindings.

The service exposes `GET /livez` for process liveness and `GET /readyz` for Redis readiness. A ready response establishes Redis availability; independently monitor identity introspection, controller connectivity, treasury balances, and receipt recovery. Its hash-chained allocation audit records event, result, and funding reference without recording authentication values.

## Continue building

- [Send your first payment](https://docs.paxeer.app/payments-quickstart)

- [Deposit-backed custody funding](https://docs.paxeer.app/custody)

- [Asset metadata and account balances](https://docs.paxeer.app/assets)

- [Execution fees and fee limits](https://docs.paxeer.app/fees)

- [Public payment API](https://docs.paxeer.app/platform-api)
