<!-- Source: https://docs.paxeer.app/paid-apis/ -->

# Paid APIs and x402

Use x402 version 2 offers, EVM payments, and verified receipts to release paid HTTP resources on Paxeer mainnet-beta.

A paid endpoint advertises its price through x402 version 2. The buyer selects an accepted offer, authorizes payment on the named EVM network, and retries with payment evidence. The seller releases the resource after verifying the receipt and required commitment. Exact, metered, and subscription schemes use this evidence-bound lifecycle.

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
| `scheme`, `network` | `exact`, `metered`, or `subscription`, with an `eip155:` network identifier. Paxeer mainnet-beta uses `eip155:125`. |
| `amount` | Positive canonical decimal string, represented exactly as an unsigned 128-bit integer. |
| `asset`, `payTo` | 32-byte hexadecimal Asset and recipient identifiers. |
| `maxTimeoutSeconds` | Positive integer request timeout, at most `2^32 - 1` seconds. |
| `extra.layerx` | Commitment, account reference, currency, and scheme-specific grant terms. |

The network is an `eip155:` chain identifier with a positive chain number that fits unsigned 32-bit range. For the typed money-movement buyer flow, `extra.layerx.account` names the destination account, such as `agent:<did>:main`, and `currency` names its money currency. The buyer checks that `payTo` corresponds to the offered account or its asset account before preparing payment.

## Add seller middleware

The Node package `@sidiora/layerx-seller-middleware` lives at `services/platform/middleware/seller/`. It exports `SellerMiddleware`, `ReceiptPayloadAuthority`, and payment-header codecs. Configure trusted receipt keys, a payment authority, payment requirements, and a durable fulfillment repository. This handler follows `services/platform/examples/paid-api/index.mjs`; `middleware` is the configured seller instance.

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

The example serves `GET /paid`. Its configuration selects network, scheme, resource URL, port, and the environment-variable names for price, asset, recipient, account, currency, and receipt keys. Its offer requests `receipted` assurance. Set the resource URL to the endpoint the customer requests.

### Persist fulfillment

A fulfillment record binds payment identity to the request digest, transaction hash, canonical receipt evidence, optional committed-block evidence, and returned resource. Matching retries return the stored resource; conflicting reuse is refused. The example durably records a claim before resource release, then synchronizes and publishes the result. An unresolved claim remains an unknown outcome until reconciled, so an uncertain release is not repeated.

## Integrate the buyer

`BuyerMiddleware` in `services/platform/middleware/buyer/` accepts a production client, source account, supported scheme/network pairs, trusted receipt keys, an optional commitment resolver, and retry policy. `prepare(paymentRequiredHeader, callerIdempotencyKey)` prepares an exact payment through the typed quote workflow. `fetch` drives the HTTP exchange, while `captureSettlement` verifies the result. Use `fetchGrant` and `captureGrantSettlement` for grant-backed draw transactions.

The buyer validates offered account and currency terms before quoting. Required extensions are echoed unchanged. Choose a stable caller idempotency key for one purchase and retain it with the accepted offer and activity identifier until the outcome is resolved.

## Metered requests and subscriptions

A metered or subscription offer requires nonzero 32-byte hexadecimal `payer` and `purposeHash` terms. Subscription offers require a positive decimal-string `windowSeconds` below `2^64`; metered offers omit that field.

Grant-backed payments use a signed EIP-1559 transaction calling the ledger precompile’s `draw` method. Its 228-byte calldata encodes the grant ID, recipient, unsigned 128-bit amount, idempotency key, unsigned 64-bit sequence, purpose hash, and context hash. Each metered request or subscription payment carries its own authorized draw. The seller requires the grant to be active and bound to the signed executor, payer, recipient, and asset.

```
{
  "rawTransaction": "<signed EIP-1559 draw transaction as hex>",
  "idempotencyKey": "<64-hex key>"
}
```

The grant payment authority checks the transaction’s chain ID, ledger destination, zero native value, calldata, amount, recipient, purpose, idempotency key, and executor. Receipt verification requires the matching `PaymentDrawn` event, including grant ID, payer, asset, amount, and context. A request with changed payment facts is refused.

## Select settlement assurance

| Commitment | Required evidence |
| --- | --- |
| `receipted` | An authorized signed successful receipt bound to the raw EVM transaction and matching payment event. |
| `committed` | Receipted evidence plus verified transaction and result inclusion, block headers, validator sets, and BFT commits. |
| Trust configuration | Receipt keys are configured locally; committed verification also resolves the trusted chain identity and validator-set hash. |

Set `extra.layerx.commitment` to `receipted` or `committed`. Committed verification requires a commitment resolver and a trusted validator resolver. A receipt is final when its block is committed; a signed receipt alone does not supply that proof. Missing evidence cannot lower the requested assurance.

### Receipt verification and capture

The seller verifies canonical receipt encoding, the raw transaction and its chain ID, sender and destination binding, successful result, authorized receipt signature, receipt digest, and the accepted payment facts in ledger log effects. Successful settlement returns the verified EVM transaction hash in `transaction`.

The buyer checks `success`, network, amount, payer, transaction hash, and the `layerx` extension. Portable evidence uses `format: "layerx-receipt-proof-v2"` and `verificationLevel: "receipted"` or `"committed"`, with raw transaction, signed receipt, receipt digest, and receipt-key metadata. The buyer verifies these against its prepared payment and configured trust. HTTP 200 alone does not establish payment.

## Recover uncertain payment outcomes

Preserve the original raw transaction, its EVM hash, the accepted offer, and purchase idempotency key when a payment outcome is uncertain. The node’s `pax_sequencerReceipt` method supplies receipt data for the transaction; the hosted gateway also accepts that method. Verify recovered material before using it as payment evidence.

Recover the original payment through the SDK or buyer-middleware settlement capture. Committed assurance requires recovered evidence to verify against the chain’s block headers, BFT commits, inclusion proofs, and trusted validator set. The seller’s durable fulfillment record resolves matching retries to the same resource. A replacement debit does not resolve the original purchase.

| Seller outcome | HTTP | Application action |
| --- | --- | --- |
| Payment required | 402 | Select and authorize an offered payment. |
| Pending | 202 | Honor retry guidance and recover the same activity. |
| Refused or conflicting reuse | 402 or 409 | Inspect the refusal reason and preserve the original payment identity. |
| Released | 200 | Verify PAYMENT-RESPONSE and consume the stored resource. |

## Facilitators and transport interoperability

The interoperability service in `services/interop/` supplies x402 facilitator operations for supported offers, verification, and settlement. Verification validates the signed payment without submitting it; settlement uses the chain relay and verified receipt evidence. HTTP, MCP, and A2A bindings share payment semantics. Reuse the same settlement identity for recovery and reject a conflicting receipt for that identity.

Use [service agreements](https://docs.paxeer.app/services) for work that needs deadlines, delivered artifacts, review, and disputes. Continue with the [SDK](https://docs.paxeer.app/sdk) and [native protocol modules](https://docs.paxeer.app/modules) for account, Asset, and activity integration.
