<!-- Source: https://docs.paxeer.app/paid-apis/ -->

# Paid APIs and x402

Monetize HTTP resources, agent tools, and recurring access with payment-required offers and receipt-verified fulfillment on Paxeer X.

A Paxeer X paid endpoint advertises its price through x402 version 2. The buyer selects an accepted offer, authorizes payment, and retries with payment evidence. The seller releases the resource after verifying the receipt and required commitment. Exact, metered, and subscription schemes share this evidence-bound lifecycle.

## The HTTP payment exchange

1. The buyer requests the resource without payment.
2. The seller returns HTTP `402` with a `PAYMENT-REQUIRED` header and matching JSON body.
3. The buyer selects a supported scheme and network, verifies payee terms, and constructs a payment payload.
4. The buyer retries with `PAYMENT-SIGNATURE`, preserving the accepted offer and required extensions.
5. The seller verifies settlement and records fulfillment before returning the resource with `PAYMENT-RESPONSE`.
6. The buyer independently verifies the response receipt and settlement reference.

HTTP headers carry standard-base64 JSON. MCP and A2A bindings carry the corresponding JSON values. Settlement identity stays consistent across transports, so redelivering the same payment through another binding retains the same economic identity.

## Define the offer

| Field | Contract |
| --- | --- |
| `x402Version` | `2`. |
| `resource` | HTTP(S) URL, with optional description, MIME type, service name, tags, and icon URL. |
| `accepts` | One to 32 offers; buyer selection follows seller order among supported scheme/network pairs. |
| `scheme`, `network` | `exact`, `metered`, or `subscription`, with the intended network identifier. |
| `amount` | Positive canonical decimal string, represented exactly as an unsigned 128-bit integer. |
| `asset`, `payTo` | 32-byte hexadecimal Asset and recipient identifiers. |
| `maxTimeoutSeconds` | Positive request timeout bound. |
| `extra.layerx` | Commitment, account reference, currency, and scheme-specific grant terms. |

For a `layerx:*` network, `extra.layerx.account` names the destination account, such as `agent:<did>:main`, and `currency` names its money currency. The offered `payTo` must derive from that account under a supported account-ID derivation. Buyers refuse a mismatched recipient.

## Add seller middleware

The Node package `@sidiora/layerx-seller-middleware` exports `SellerMiddleware`, `ReceiptPayloadAuthority`, and the payment-header codecs. Supply a trusted authority resolver, payment requirements, and a durable fulfillment repository. The following handler is the resource-release pattern used by the paid API example; `middleware` is the configured seller instance.

```
import { PAYMENT_SIGNATURE_HEADER } from "@sidiora/layerx-seller-middleware";

const header = request.headers[PAYMENT_SIGNATURE_HEADER.toLowerCase()];
const paymentHeader = Array.isArray(header) ? undefined : header;
const decision = await middleware.handle(
  "public-paid-api",
  paymentHeader,
  async () => resourceBody,
);
const headers = decision.kind === "payment-required"
  || decision.kind === "refused"
  || decision.kind === "released"
  ? decision.headers
  : { "retry-after": "1" };
response.writeHead(decision.status, {
  "content-type": "application/json", ...headers,
});
response.end(JSON.stringify(
  decision.kind === "payment-required" ? decision.body
    : decision.kind === "released" ? decision.resource
    : { state: decision.kind },
));
```

The example serves `GET /paid`. Its configuration selects protocol version, network, price, Asset, recipient, account, currency, receipt authority, and fulfillment directory. Keep credentials in the configured environment variables. Set the resource URL to the externally reachable endpoint your customer requests.

### Persist fulfillment

A fulfillment record binds the caller's idempotency key to the request digest, canonical receipt, authorized batch, and returned resource. Persist it durably and return the stored resource for a matching retry. Refuse reuse of the same key for a different request. The example writes an exclusive file, synchronizes it and its directory, and checks the existing record after a concurrent write.

## Integrate the buyer

`BuyerMiddleware` accepts a production client, source account, protocol selection, supported scheme/network pairs, configured receipt authorities, and retry policy. `prepare(paymentRequiredHeader, callerIdempotencyKey)` parses the offer and prepares payment through the typed quote workflow. `fetch` drives the HTTP exchange, while `captureSettlement` verifies the result. Use `fetchGrant` and `captureGrantSettlement` for grant-backed receives.

The buyer validates offered account and currency terms before quoting. Required extensions are echoed unchanged. Choose a stable caller idempotency key for one purchase and retain it with the accepted offer and activity identifier until the outcome is resolved.

## Metered requests and subscriptions

A metered or subscription offer additionally commits to a nonzero payer and purpose hash. Subscription offers require a positive decimal-string `windowSeconds` below `2^64`; metered offers omit the field.

The payer signs a canonical 346-byte Asset ordinal-7 grant. The receiver submits a canonical 733-byte Asset ordinal-6 receive that embeds the grant and receiver authorization. Every metered request is one draw. Every subscription renewal uses a distinct signed receive and idempotency key; a recurring allowance alone does not establish one renewal per period.

```
{
  "receive": "<733-byte canonical receive as hex>",
  "idempotencyKey": "<64-hex key>"
}
```

Validation binds the payer, recipient, Asset, amount, purpose, network, grant identity, allowance, per-draw maximum, expiry, controller, and signed context. Metered grants are non-recurring with a zero window. Unknown, paused, or unregistered Assets and changed payment facts are refused.

## Select settlement assurance

| Commitment | Required evidence |
| --- | --- |
| `executed` | An authorized sequencer-signed successful receipt for the exact activity. |
| `batched` | Executed evidence plus receipt inclusion in the authorized signed batch header. |
| `finalised` | Batched evidence plus the guarantor checkpoint certificate for that batch. |

Set `extra.layerx.commitment` to the assurance your resource requires. Authority keys come from verifier configuration. Missing evidence keeps settlement pending or refused; it never silently lowers the requested assurance.

### Receipt verification and capture

The seller verifies canonical receipt encoding, activity binding, successful result, protocol body, authorized sequencer signature, and the accepted Asset, amount, and recipient. The gateway records a receipt-verified settlement. Success returns `transaction: "lxp:<receipt_digest>"`.

The buyer checks `success`, the `layerx` extension, `verificationLevel: "sequencer-signed"`, canonical receipt bytes, payer, network, Asset, recipient, and amount. It recomputes the receipt digest and requires both the declared digest and `lxp:` reference to match. An HTTP 200 or acknowledgement by itself does not establish settlement.

## Recover uncertain payment outcomes

There is no public `lx_*` JSON-RPC transport in this checkout. The hosted gateway refuses every `lx_*` method name with JSON-RPC `-32601` (`Method not found`), including `lx_sendActivity`, `lx_getActivityStatus`, `lx_getReceipt`, and `lx_getProof`. Do not build submission or recovery flows on those names.

Recover through the identity you already hold: the purchase idempotency key, the accepted offer, and the original activity or transaction reference. Re-resolve the original payment through the SDK or buyer-middleware settlement capture, and use published hosted reads such as `pax_sequencerReceipt` where they apply. For batched assurance, require the recovered receipt evidence to verify against the authorized signed batch header before release. Verify recovered evidence before releasing or capturing the resource. Preserve payment identity throughout recovery; a replacement debit does not resolve the original purchase.

| Seller outcome | HTTP | Application action |
| --- | --- | --- |
| Payment required | 402 | Select and authorize an offered payment. |
| Pending | 202 | Honor retry guidance and recover the same activity. |
| Refused | 402 | Read settlement reason; correct the failed condition. |
| Released | 200 | Verify PAYMENT-RESPONSE and consume the stored resource. |

## Facilitators and transport interoperability

The x402 facilitator exposes `/supported`, `/verify`, and `/settle`. Verification is read-only; settlement is state-changing and completes against verified canonical receipts. HTTP, MCP, and A2A share the same message semantics. An identical settlement identity and receipt digest recover the existing settlement; a swapped receipt for that identity is an idempotency conflict.

Use [service agreements](https://docs.paxeer.app/services) for work that needs deadlines, delivered artifacts, review, and disputes. Continue with the [SDK](https://docs.paxeer.app/sdk) and [native protocol modules](https://docs.paxeer.app/modules) for account, Asset, and activity integration.
