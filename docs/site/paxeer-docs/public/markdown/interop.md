<!-- Source: https://docs.paxeer.app/interop/ -->

# Interoperability

Connect agent payments, commerce mandates, card and bank rails, and external chains to Paxeer X through evidence-backed adapters.

## One execution ledger, many entry points

Interoperability adapters validate external payment, mandate and settlement evidence and submit authorized transactions to Paxeer X. The kernel executes within the node, and a transaction receipt becomes final when its block commits. An external provider's settlement evidence remains a separate input to reconciliation.

The interoperability gateway provides HTTP, MCP, and A2A ingress. Transport selects how a request arrives; the adapter selects how its evidence is validated. Applications keep the same operation identity and reconciliation process across these interfaces.

## Connector catalogue

| Adapter | Application journey | Evidence boundary |
| --- | --- | --- |
| `x402` | Buyer builds payment, seller offers a resource, facilitator verifies and settles | Payment evidence plus verified LayerX receipt; HTTP, MCP, and A2A transports |
| `ap2` | Verify an agent commerce mandate and execute within its authority | Verified mandate and LayerX receipt |
| `ucp` | Complete a commerce checkout | Checkout evidence and receipt-gated execution |
| `visa-tap` | Verify a trusted agent intent and execute its authorized action | Trusted agent credential, canonical target, and policy checks |
| `fiat` | Process card, bank, and real-time-payment callbacks | External settlement evidence and LayerX receipt |
| `migration-v2` | Import verified account, asset, and history state | Source-chain verification and migration rules |

## Gateway discovery and operation tracking

`GET /v1/adapters` exposes adapter metadata and readiness. Track or resume an operation with `GET /v1/operations/{id}`, where `id` is exactly 64 hexadecimal characters and belongs to the authenticated principal. The service also exposes `GET /livez` and `GET /readyz` for process and readiness checks. These paths are relative to the configured interop service origin.

```
# Set these shell variables to your service origin, API key and operation ID.
curl "$INTEROP_GATEWAY/v1/adapters"
curl -H "Authorization: Bearer $INTEROP_API_KEY" \
  "$INTEROP_GATEWAY/v1/operations/$OPERATION_ID"
```

Build the request body from the selected adapter's schema. Adapter requests require a gateway API key in the Authorization header; requests other than provider callbacks also require an Idempotency-Key header. Preserve operation identifiers and evidence digests through retries. A successful HTTP exchange alone does not establish a committed state change.

### Evidence policies

| Policy | Required basis |
| --- | --- |
| `layerx-receipt` | A verified LayerX execution receipt |
| `verified-mandate+layerx-receipt` | Accepted mandate authority and a verified execution receipt |
| `trusted-agent-credential` | Agent credential accepted by the configured trust policy |
| `external-settlement+layerx-receipt` | External rail settlement evidence paired with a verified execution receipt |

## x402 over HTTP, MCP, and A2A

x402 supports buyer, seller, and facilitator roles. The same route family accepts `http`, `mcp`, or `a2a` as its transport segment. Capability discovery comes before constructing a payment, so clients can select the supported scheme and network.

| Method | Path | Purpose |
| --- | --- | --- |
| GET | `/v1/{transport}/x402/supported` | Discover payment support |
| GET | `/v1/{transport}/x402/facilitator/supported` | Discover facilitator support |
| POST | `/v1/{transport}/x402/buyer/build` | Build buyer evidence |
| POST | `/v1/{transport}/x402/seller/offer` | Produce a seller offer |
| POST | `/v1/{transport}/x402/verify` | Verify payment evidence |
| POST | `/v1/{transport}/x402/settle` | Settle through LayerX execution |

Verification also has the `/facilitator/verify` variant; settlement has `/seller/settle` and `/facilitator/settle` variants beneath the same x402 prefix. Commitment extras and metered or subscription grant draws let applications bind payments to a resource and consume an authorized allowance over time.

### Paid resource workflow

1. The seller publishes an offer identifying the resource and payment terms.
2. The buyer discovers support and builds evidence for the selected terms.
3. The seller or facilitator verifies the evidence before accepting the purchase.
4. Settlement submits the LayerX operation and verifies its receipt.
5. The application associates resource delivery with the accepted payment operation and retains its evidence for reconciliation.

## Agent commerce and fiat rails

| Method | HTTP route | Action |
| --- | --- | --- |
| POST | `/v1/http/ap2/mandates/verify` | Verify mandate |
| POST | `/v1/http/ap2/execute` | Execute authorized mandate |
| POST | `/v1/http/ucp/checkouts/complete` | Complete checkout |
| POST | `/v1/http/visa-tap/intents/verify` | Verify trusted-agent intent |
| POST | `/v1/http/visa-tap/intents/execute` | Execute accepted intent |
| POST | `/v1/http/fiat/card/callbacks` | Accept card-rail evidence |
| POST | `/v1/http/fiat/bank/callbacks` | Accept bank-rail evidence |
| POST | `/v1/http/fiat/rtp/callbacks` | Accept real-time-payment evidence |

Unlike x402, these adapters are HTTP-only: there is no MCP or A2A variant of the routes above. Mandate verification establishes what an agent may do before execution changes state. TAP policy binds the trusted agent and canonical request target, with clock-skew limits owned by the service. Fiat callbacks bind external settlement to the LayerX operation; repeating a provider notification must reconcile the original operation rather than credit it twice.

## Migration imports

The hosted migration adapter is `migration-v2`. It is served under `/v2`, not the `/v1` adapter prefix, and every route is a POST. Other methods, a `/v1` prefix, or a trailing slash are unknown routes.

| Method | Route | Action |
| --- | --- | --- |
| POST | `/v2/migration/accounts` | Import accounts |
| POST | `/v2/migration/assets` | Import assets |
| POST | `/v2/migration/history` | Import history |
| POST | `/v2/migration/history/read` | Read imported history |

## Portable receipts and mirrors

Portable receipt verification checks supplied receipt evidence against independently trusted keys and inclusion facts. Ethereum and Solana mirror implementations publish batch archives and commitments for independent verification; see [Ethereum & Solana Mirrors](https://docs.paxeer.app/mirrors). Mirrors carry verification data; custody remains on Paxeer X.

Choose migration for a source-state transition, a mirror for independent archive verification, and the [bridge](https://docs.paxeer.app/bridge) for a custody-backed asset transfer. Preserve the source chain identity and proof material in each case.

## Admission and receipts

Adapters submit transactions to the node. A submitted or pending response requires further receipt lookup; it does not establish a committed state change. Verify the receipt's result and block commitment before treating execution as final. Keep the gateway operation identifier, transaction hash, receipt and external settlement evidence together for reconciliation.

## Continue building

Explore [bridging and custody](https://docs.paxeer.app/bridge), [execution and settlement architecture](https://docs.paxeer.app/paxeer-vs-layerx), and [native chain APIs](https://docs.paxeer.app/rest-grpc) to connect protocol evidence to chain state.
