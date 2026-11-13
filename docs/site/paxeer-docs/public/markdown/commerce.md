<!-- Source: https://docs.paxeer.app/commerce/ -->

# Agent commerce

Connect merchant checkout, trusted shopping agents, delegated authorization, and receipt-backed payment through Paxeer X.

## One commerce journey, explicit authority at every step

The commerce adapters under services/interop/crates/ connect protocol requests to the network payment path: a merchant advertises its capabilities, an agent identifies the intended operation, the payer supplies bounded authorization, and the payment plane produces verifiable settlement evidence. Checkout and order experiences use that evidence to report a completed purchase.

| Protocol or integration | Role | Result |
| --- | --- | --- |
| UCP | Merchant profiles, capability negotiation, checkout completion, and order reads. | A checkout outcome and an order tied to verified payment evidence. |
| AP2 | Signed payment and checkout mandates with evaluated constraints. | An authorized payment mapped to a typed payment intent. |
| Visa Trusted Agent Protocol | Signed agent identity and browse or pay intent verification. | A verified agent credential handed to the merchant authority path. |
| Express and FastAPI | Paid-resource release, fulfillment persistence, and webhook processing. | A resource associated with its canonical receipt and request digest. |

## Merchant discovery and capability negotiation

The UCP adapter uses revision `2026-04-08`. A merchant profile declares REST services, payment handlers, and supported capabilities. A platform profile is negotiated against it before checkout completion. The checkout capability is `dev.ucp.shopping.checkout`; order reads use `dev.ucp.shopping.order`. Negotiation binds the selected payment handler by digest, so checkout completion carries the payment configuration that the participants agreed to use.

Publish a merchant profile with the supported protocol revision and HTTPS service endpoint. Have the buying platform negotiate its capabilities before it submits a payment. Preserve the negotiated result alongside the checkout for subsequent order verification.

## Complete a checkout

1. Prepare the checkout identifier, uppercase three-letter currency, and positive total in ISO-4217 minor units. Configure the merchant currency-to-asset binding and exact conversion to atomic asset units.
2. Bind the checkout to the LayerX asset and recipient, negotiated payment handler, and a stable UCP idempotency key.
3. Submit completion through `POST /v1/http/ucp/checkouts/complete`.
4. The payment plane executes the typed payment intent. Pending or refused execution produces the corresponding checkout outcome.
5. For executed payment, verify the signed canonical receipt, transaction identity, logs and checkout binding. Return the completed order only after that evidence passes verification.

At the adapter boundary, the completion request has the following fields. These fields are declared by the Rust submission type in services/interop/crates/layerx-ucp/src/lib.rs:

```
CheckoutSubmission {
    checkout_id,
    currency,            // [u8; 3], ISO-4217 code
    total_minor,         // u128, positive minor-unit amount
    layerx_asset,        // [u8; 32]
    layerx_recipient,    // [u8; 32]
    idempotency_key,     // UcpIdempotencyKey
    negotiated,          // NegotiatedCapabilities
}
```

Submission validation also requires a nonzero LayerX asset, a nonzero recipient, and a nonzero negotiated payment-handler digest. A submission that fails validation is refused before payment execution.

### Retries and completion states

The gateway binds an idempotency key to a digest of the submitted request. Repeated attempts retain their payment identity. An execution already settled by receipt must present consistent order evidence on a retry; a pending response cannot replace previously verified settlement.

| Checkout status vocabulary | Meaning in the purchase journey |
| --- | --- |
| `Incomplete` | The checkout still requires information. |
| `RequiresEscalation` | The flow requires escalation before completion. |
| `ReadyForComplete` | The checkout is ready for completion submission. |
| `CompleteInProgress` | Completion awaits a payment result. |
| `Completed` | Verified payment evidence supports the order. |
| `Canceled` | The checkout is canceled. |

## Read an order with its evidence

An order combines merchant-owned metadata, including its order identifier and HTTPS permalink, with verified settlement evidence. `UcpAdapter::read_order_with_config` checks the negotiated order capability, validates metadata and the merchant currency binding, and re-verifies the stored signed receipt and logs against the transaction and original checkout. Reading an order is a read operation; it does not debit balances or execute a second payment.

## Release paid resources

The Express and FastAPI integrations connect settlement to merchant fulfillment. The integrations live in sdk/integrations/express/ and sdk/integrations/fastapi/. Their fulfillment repository receives a proposed record and a resource-release function. Persist the idempotency key, request digest, canonical receipt, and returned resource together. On repeat delivery, return the existing record only when its request digest matches; a different digest produces a fulfillment conflict.

The Express example uses exclusive file creation (`wx`) for fulfillment records. The FastAPI example serializes fulfillment by key and persists records through atomic replacement. Choose a durable fulfillment repository and webhook delivery store for services that run across restarts or multiple workers. Keep fulfillment identity consistent with the payment operation.

## Plan the merchant integration

1. Define the merchant profile, payment handler, asset mapping, and recipient.
2. Choose the buying-agent credential and [mandate authorization](https://docs.paxeer.app/mandates) policy.
3. Persist checkout submissions, idempotency keys, and merchant order metadata.
4. Store signed canonical receipts, transaction hashes, logs, receipt-key authorizations and merchant currency bindings for checkout completion and order reads.
5. Connect fulfillment and webhook delivery to durable application records.

Use [EVM applications](https://docs.paxeer.app/evm) for contract-facing experiences and [the SDK](https://docs.paxeer.app/sdk) for network integration. Commerce adapters preserve protocol-specific identities while submitting typed payment operations. Paxeer mainnet-beta is one BFT chain, and its receipts are final when their blocks are committed.
