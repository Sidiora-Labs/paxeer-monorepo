# Payment middleware contract

The TypeScript packages under [services/platform/middleware](../services/platform/middleware) implement seller, buyer, merchant and agent payment flows for Paxeer. Mainnet-beta is one BFT network with EVM chain ID **125** (`0x7d`), identified in payment requirements as `eip155:125`. The kernel participates in block proposal inside the network. A signed pre-commit receipt is evidence of the `receipted` stage; finality requires the block to be `committed`. Sub-chains are tenant domains in the database.

Use node JSON-RPC at the HTTPS root of `api1.mainnet-beta.paxeer.network` through `api14.mainnet-beta.paxeer.network`, or `api-mainnet-beta.paxeer.network`. SDK `px_` reads use the node root. Hosted gateway discovery follows [the gateway contract](platform-gateway.md); configure the hosted gateway URL separately from the node endpoint for `px_register` and `px_requestFunds`. These packages retain historical `LayerX` names in wire fields and exports; those names do not establish a second network. See also [hosted webhooks](platform-hosted.md#webhooks-http-routes) and [independent ramps](platform-ramps.md).

## Middleware seller TS exports

[seller/src/index.ts](../services/platform/middleware/seller/src/index.ts) exports `SellerMiddleware`, `ReceiptPayloadAuthority`, `verifyPaymentReceipt`, header codecs, the payment requirement/payload/result types and `VerifiedWebhookConsumer`. It re-exports the [commitment](../services/platform/middleware/seller/src/commitment.ts), [ledger](../services/platform/middleware/seller/src/ledger.ts) and [fulfillment](../services/platform/middleware/seller/src/fulfillment.ts) contracts.

The protocol uses `X402_VERSION = 2` and the `PAYMENT-REQUIRED`, `PAYMENT-SIGNATURE` and `PAYMENT-RESPONSE` headers. Header values carry the package's base64 JSON encoding; use its codecs. Requirements include `scheme`, `network`, `asset`, `amount`, `payTo`, `maxTimeoutSeconds` and optional `extra`. Monetary amounts are exact integer strings, not floating-point prices. Supported receipt verification schemes are `exact`, `metered` and `subscription`.

`SellerMiddleware.handle(principal, paymentHeader, release)` produces these decisions:

| Kind | HTTP status | Meaning |
| --- | --- | --- |
| `payment-required` | 402 | No payment header; returns the requirements header and body |
| `pending` | 202 | Authority has not supplied a verified payment result |
| `refused` | 402 or 409 | Payment refusal or conflicting fulfillment |
| `released` | 200 | Payment verified and resource obtained through the fulfillment repository |

`PortableReceiptV2` uses `format: "layerx-receipt-proof-v2"`, `verificationLevel`, `rawTransaction`, `signedReceipt`, `receiptDigest` and a `receiptKey` containing `keyId`, `publicKey`, `firstHeight` and `lastHeight`. Verification binds the raw EIP-1559 transaction, recovered sender, chain, signed receipt, trusted receipt key and payment log to the accepted requirements. Matching a displayed transaction hash or trusting a caller-provided key is insufficient.

The ledger precompile is **`0x000000000000000000000000000000000000101a`**, as exported by `LAYERX_LEDGER_ADDRESS` in `ledger.ts`. Grant draws encode `draw(bytes32,bytes32,uint128,bytes32,uint64,bytes32,bytes32)`. Payment verification authenticates `PaymentExact` or `PaymentDrawn` log material. Metered and subscription offers require the purpose binding; the wire terms under `extra.layerx` also carry payer, currency and commitment where required. Preserve these fields and the ABI widths.

`paymentCommitment` defaults to `receipted` when no commitment terms are supplied. Request `extra.layerx.commitment: "committed"` when block commitment is required. That path needs a `PaymentCommitmentResolver` and its trusted validator resolver; it checks the receipt inclusion, block header, commit and validator set. A receipt signature alone must not satisfy the committed requirement.

Fulfillment must be durable and idempotent across requests and restarts. `FulfillmentRepository.fulfill` atomically associates the proof and request digest with the resource; a receipt cannot buy a different resource or principal's request on replay. An unknown resource-release outcome remains unknown rather than authorizing the side effect again.

### Webhook consumption

Pass the **exact raw body bytes** to `VerifiedWebhookConsumer.consume`. The authenticated headers are `layerx-webhook-id`, `layerx-webhook-timestamp`, `layerx-webhook-key-id` and `layerx-webhook-signature`. The signature is `v1=` followed by standard padded base64 for an Ed25519 signature over `<id>.<timestamp>.` followed by those bytes. The ID identifies the event across redeliveries, not an individual attempt. This agrees with the [producer scheme](../services/platform/hosted/webhooks/src/scheme.rs).

The consumer defaults to a five-minute maximum age and a one-minute processing lease, with at most 30 seconds of future clock skew. Supply trusted endpoint public keys and a durable `WebhookDeliveryStore`. Its atomic claim distinguishes claimed, processing, completed and conflicting deliveries. Successful results are `processed`, `duplicate` or `processing`; conflicting bodies under one ID raise `webhook-replay`. Apply financial state from the authenticated body and verified proof, not unsigned delivery metadata.

## Middleware seller HTTP routes

[seller/src/rpc.ts](../services/platform/middleware/seller/src/rpc.ts) is a client adapter, not a listening HTTP server. `PaymentRpc` posts JSON-RPC to a root origin, restricts calls to `eth_` and `pax_`, submits raw transactions with `eth_sendRawTransaction`, reads `pax_sequencerReceipt` and checks committed transaction status with `eth_getTransactionReceipt`. Its transport bounds response size and time, rejects redirects and checks the response ID and envelope.

`HostedGatewayRpc` posts to the gateway root and rejects `/rpc` endpoints. It accepts the configured gateway origin with or without a trailing slash and restricts calls to `px_` methods. Use that hosted gateway URL for discovery and hosted operations such as `px_register` and `px_requestFunds`, and the separate node endpoint with `PaymentRpc` for payment submission and receipt reads. No middleware package creates an extra public network endpoint.

## Middleware seller error codes

`MiddlewareError.code` is one of `invalid-payment-required`, `invalid-payment-payload`, `requirements-mismatch`, `unsupported-payment`, `payment-pending`, `payment-refused`, `verification-failure`, `fulfillment-conflict`, `fulfillment-outcome-unknown`, `invalid-webhook` or `webhook-replay`. `PaymentRpcError` separately preserves a numeric JSON-RPC code and its data. Transport failure, pending payment and a definitive refusal have different retry implications.

## Middleware buyer TS exports

[buyer/src/index.ts](../services/platform/middleware/buyer/src/index.ts) exports `BuyerMiddleware` and its offer, quote, journey and payment types. `BuyerMiddlewareConfig` requires a `ProductionClient`, source account, supported scheme/network pairs and trusted receipt keys; commitment verification and bounded retry policy are configurable. Offer selection must preserve the accepted asset, account, currency, amount, purpose and commitment. The buyer verifies the returned payment evidence instead of trusting an HTTP 200 or settlement boolean. Legacy journey state names such as `done-finalised` are wire vocabulary, not a separate finality system.

## Middleware merchant TS exports

[merchant/src/index.ts](../services/platform/middleware/merchant/src/index.ts) exports `MerchantMiddleware`, catalog/cart/quote types and `MerchantOrderStore`. The order states are `awaiting-payment`, `paid-verified` and `refused`. Quotes bind a checkout key to exact catalog terms. The implementation rejects duplicate SKUs, incompatible payment facts, more than 256 lines and quantities outside 1–1000000. Resource release and marking an order paid must preserve the store's idempotency and request digest. Webhook settlement follows the same locally verified receipt path.

## Middleware agent TS exports

[agent/src/index.ts](../services/platform/middleware/agent/src/index.ts) exports `AgentMiddleware`, `AgentGrantMiddleware`, budget reservation types and signing/submission contracts. [daemon.ts](../services/platform/middleware/agent/src/daemon.ts) decodes the agent signing boundary's preparations and observations. This signing agent is an application integration; it is not a second consensus process.

`AgentSpendRequest` explicitly includes `tenant`, preparation, network identity, authority, signer identity, submission idempotency key, approval sequence, asset, amount and recipient. Keep the tenant in budget and approval lookups. `AgentBudgetLedger` supports reserve, hold, commit and release transitions; held reservations retain approval and canonical-byte bindings. Uncertain submission keeps the reservation for reconciliation. Do not refund the budget merely because a post-submit transport call timed out.

## Middleware agent error codes

Agent refusals preserve SDK `code` and retry classification. Budget exhaustion returns `budget-refused` with code `budget-refusal`, retry `never` and the available amount. Missing required budget capability raises `unavailable-capability`. The exact `AgentMiddlewareErrorCode` union and `AgentMiddlewareError` in the source describe local validation failures; they are separate from a verified chain rejection. A post-submit unknown outcome must be resolved using the same signed transaction and idempotency identity.

<a id="examples-config-keys"></a>
<a id="examples-buyer-agent-config-keys"></a>
<a id="examples-marketplace-config-keys"></a>
<a id="examples-merchant-checkout-config-keys"></a>
<a id="examples-merchant-shop-config-keys"></a>
<a id="examples-paid-api-config-keys"></a>
<a id="examples-cli-commands"></a>
<a id="examples-support-cli-commands"></a>
<a id="examples-env-vars"></a>
<a id="examples-marketplace-env-vars"></a>
<a id="examples-agent-spend-env-vars"></a>
<a id="examples-support-env-vars"></a>
<a id="examples-paid-api-http-routes"></a>
<a id="examples-support-http-routes"></a>

## Reference applications

[reference-apps.json](../services/platform/examples/reference-apps.json) records the buyer-agent, paid-api, merchant-shop and marketplace applications, their paths, configuration filenames and emulator/beta command arrays. Merchant-checkout is the compatibility package name for merchant-shop. Use each application's actual `layerx.example.json` and its runtime validation; deployment origins and credentials must be supplied by the operator. Example placeholders and old service coordinates do not advertise running infrastructure.

The [reference runner](../services/platform/examples/run-reference-apps.mjs) and [shared runtime](../services/platform/examples/support/runtime.mjs) own command-line and environment handling; [merchant-app.mjs](../services/platform/examples/support/merchant-app.mjs) and [paid-api/index.mjs](../services/platform/examples/paid-api/index.mjs) own example HTTP routes. These application listeners are distinct from node JSON-RPC and have no consensus or custody authority.

<a id="middleware-examples-cli"></a>
<a id="middleware-examples-cli-commands"></a>
<a id="middleware-examples-env-vars"></a>
<a id="middleware-examples-http-routes"></a>

## Public RPC examples

[public-rpc.mjs](../services/platform/middleware/examples/public-rpc.mjs) and [public_rpc.py](../services/platform/middleware/examples/public_rpc.py) contain the executable example clients. Inspect their selected transport and endpoint validation before use: compatibility clients that require a retired path need migration to the root contract. Receipt keys and commitment validators are trusted inputs, not values to accept from an untrusted payment offer. Never use `node.hyperpaxeer.com` as RPC; it is the library mirror.

<a id="middleware-conformance-env-vars"></a>
<a id="middleware-conformance-environment"></a>
<a id="middleware-conformance-http-routes"></a>
<a id="middleware-conformance-ts-exports"></a>

## Conformance support

[conformance/service.mjs](../services/platform/middleware/conformance/service.mjs) and [conformance/src](../services/platform/middleware/conformance/src) provide the scenario service, receipt helpers and repositories for integration qualification. Their environment and listener routes are test support, not public network services. A scenario result proves only the scenario and deployment actually exercised; it cannot establish live provider settlement or chain finality by itself.
