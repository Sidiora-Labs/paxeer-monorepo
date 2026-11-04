# Interoperability adapters

Interop crates translate foreign protocols into typed requests and verifiable
Paxeer evidence. They never write balances themselves. Paxeer is one BFT network,
EVM chain id `125` (`0x7d`); its kernel participates in block proposal. Receipts
are final once their block is committed.

The service package is `layerx-interop-service`; its executable is
`layerx-interop-gateway`, as declared in its
[Cargo manifest](../../services/interop/crates/layerx-interop-service/Cargo.toml).
The `layerx-interop-gateway` library owns adapter registration, principal-scoped
translation state and receipt verification. The executable composes that library
with adapters and the payment relay.

Public node JSON-RPC uses the root of
`https://api-mainnet-beta.paxeer.network` or
`https://api1.mainnet-beta.paxeer.network` through
`https://api14.mainnet-beta.paxeer.network`. See the
[public RPC reference](../site/docs/reference/public-rpc.md).
Interop's versioned HTTP routes below belong to the configured interop service;
their presence in the code does not make them routes on every public node.
Hosted `px_` methods have their own
[OpenRPC contract](../../services/platform/hosted/gateway/openrpc.json).
Sub-chains are tenant domains in the database and retain the network's chain id.

---

## Gateway routes

[interop_gateway_routes()](../../services/interop/crates/layerx-interop-gateway/src/server.rs)
defines the route contract. Runtime HTTP handling, authentication, dispatch and
continuations live in
[the service's server.rs](../../services/interop/crates/layerx-interop-service/src/server.rs).
In this table, `t` means one of `http`, `mcp`, or `a2a`.

| Method | Path |
| --- | --- |
| GET | `/livez`, `/readyz` |
| GET | `/v1/adapters` |
| GET | `/v1/operations/{64-hex}` |
| GET | `/v1/{t}/x402/supported`, `/v1/{t}/x402/facilitator/supported` |
| POST | `/v1/{t}/x402/buyer/build` |
| POST | `/v1/{t}/x402/seller/offer` |
| POST | `/v1/{t}/x402/verify`, `/v1/{t}/x402/facilitator/verify` |
| POST | `/v1/{t}/x402/settle`, `/v1/{t}/x402/seller/settle`, `/v1/{t}/x402/facilitator/settle` |
| POST | `/v1/http/ap2/mandates/verify`, `/v1/http/ap2/execute` |
| POST | `/v1/http/ucp/checkouts/complete` |
| POST | `/v1/http/visa-tap/intents/verify`, `/v1/http/visa-tap/intents/execute` |
| POST | `/v1/http/fiat/card/callbacks`, `/v1/http/fiat/bank/callbacks`, `/v1/http/fiat/rtp/callbacks` |
| POST | `/v2/migration/accounts` |
| POST | `/v2/migration/assets` |
| POST | `/v2/migration/history` |
| POST | `/v2/migration/history/read` |

The route contract identifies migration as `migration-v2`. The
[service configuration](../../services/interop/crates/layerx-interop-service/src/config.rs)
requires the base adapter declarations `x402`, `ap2`, `ucp`, `visa-tap`, and
`fiat`, plus all three transport pins. Migration uses the optional
`migration_v2` profile with separately configured source verifiers, Paxeer
binding verification and durable journals; a route can be declared while its
required profile is unavailable.

Evidence-policy labels are `layerx-receipt`,
`verified-mandate+layerx-receipt`, `trusted-agent-credential`, and
`external-settlement+layerx-receipt`. Startup checks the policy against the
adapter, its specification pin and conformance declaration.

## Authentication, retries and evidence

The service authenticates adapter calls with a bearer API key. x402, UCP and
migration calls require `Idempotency-Key`, including their read-only operations.
AP2, Visa TAP and fiat callbacks instead derive ingress identity from the exact
request digest. Operation records and continuations are scoped to the
authenticated principal. Resume the returned operation with the same principal
and preserve the original request and payment identity. Read-only verification
routes cannot execute settlement.

A state-changing payment reaches `TranslationStatus::ReceiptVerified` only
through receipt verification. The gateway verifies the canonical receipt
signature under configured authority, binds returned logs to signed effects,
and requires exactly one payment log matching the translation's idempotency
key. Protocol adapters additionally bind their own payment terms. Replaying
the same verified digest returns the same result; another digest for that
identity is refused. `submitted` or `pending` is not payment completion, and
`ReceiptVerified` alone does not claim BFT commitment. See
[x402 receipt binding](X402Transport.md#receipt-binding).

## Migration source observation

`POST /v2/migration/accounts` accepts a source chain and base64
`source_evidence`. The handler obtains the owner identity from the authenticated
key, verifies source ownership and the committed Paxeer address binding, then
persists an external mapping. Request data cannot supply its own owner or
receipt authority.

`POST /v2/migration/assets` accepts `order_digest`, `chain`, and
`source_evidence`. It forwards authenticated source evidence to the configured
ramp intake's `/internal/v2/source-settlements` endpoint, preserving customer
authorization and idempotency. The validated response contains
`source_settlement`, `provenance: "external-custody"`, a custody label, and
`layerx_receipt: false`. Upstream `done` maps to `external_source_recorded`;
`layerx_refused` maps to `refused`; intermediate outcomes remain `pending`.
Recording source settlement does not prove a Paxeer credit.

History import verifies `ownership_evidence` and `source_evidence`, persists
principal-scoped external records, and returns `provenance: "external"` with
`layerx_receipt: false`. History read uses a bounded page `limit` and optional
`cursor`. Imported history is external provenance, not native payment evidence.

The tree contains Ethereum and Solana source clients, a Paxeer binding verifier,
a durable mapping store, history storage and ramp intake integration. Deployment
still needs configured network/genesis and contract/program identities, trusted
RPC sources, authentication, durable storage and funding/custody arrangements.
Local tests and recorded fixtures do not qualify a live independent provider.

---

## Pages

| Topic | Page | Implementation |
| --- | --- | --- |
| x402 buyer, seller, facilitator | [x402 transport](X402Transport.md) | `layerx-x402` |
| AP2 mandates | [AP2 mandates](Ap2Mandates.md) | `layerx-ap2` |
| UCP checkout | [UCP](Ucp.md) | `layerx-ucp` |
| Visa TAP | [Visa TAP](VisaTap.md) | `layerx-visa-tap` |
| Portable receipts | [Portable receipt verifier](PortableVerifier.md) | `layerx-portable` |
| Fiat ramps | [Fiat ramps](FiatRamps.md) | `layerx-fiat` |
| Market-maker ramps | [Market-maker ramps](MarketMakerRamps.md) | `services/platform/ramps` |
| Ethereum and Solana mirrors | [Mirrors](Mirrors.md) | `layerx-mirror` |
| Ethereum and Solana migration | [Migration](Migration.md) | `layerx-migrate` |

[Programs porting](Porting.md) covers guest-code porting under `programs/porting`.

[Home](Home.md)
