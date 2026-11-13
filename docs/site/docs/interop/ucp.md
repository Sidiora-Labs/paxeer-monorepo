# UCP

`layerx-ucp` adapts UCP merchant discovery, capability negotiation, checkout
completion and order reads to native Paxeer payments. Its adapter id is `ucp`;
the implementation is `services/interop/crates/layerx-ucp`.

Paxeer is one BFT network with EVM chain id `125` (`0x7d`). The kernel runs
inside the network and proposes blocks; a payment is final when its block is
committed. The adapter verifies native signed payment evidence and does not
write balances. See [x402 receipt binding](x402.md#receipt-binding) for the
shared evidence boundary and [public RPC endpoints](../reference/public-rpc.md)
for node access.

## What the crate binds

The crate pins UCP revision `2026-04-08`. `MerchantProfile::layerx` advertises
`dev.ucp.shopping.checkout` and `dev.ucp.shopping.order`, with specification
and schema URLs under `https://ucp.dev/2026-04-08/`. The merchant supplies its
HTTPS REST origin and an exact, versioned `PaymentHandler` declaration.
`NegotiatedCapabilities::negotiate` requires the supported checkout name and
version and a matching payment handler, including its specification and
schema. Order support is negotiated separately.

`CheckoutSubmission` binds a checkout id, uppercase three-letter currency,
positive `total_minor`, 32-byte asset and recipient, negotiated capabilities,
and a nonzero UUID idempotency key. `UcpIdempotencyKey::gateway_key` hashes
the UUID bytes under `LayerX/interop/ucp/idempotency/v1\0`. The checkout request
digest uses `LayerX/interop/ucp/checkout/v1\0` and includes the merchant currency
configuration, so changing the payment terms under the same key conflicts.

### Currency and asset units

Checkout totals are currency minor units; payment amounts are asset atomic
units. Configure `MerchantCurrencyBinding::new(currency, asset,
minor_decimals, atomic_decimals)` and wrap the payment plane with
`UcpServiceConfig::bind_plane`. The merchant owns this equivalence; a buyer
cannot supply an exchange rate or infer the exponents from a receipt.

Conversion multiplies or divides by the exact power of ten. For example,
`1000` minor units with currency exponent `2` and asset exponent `6` becomes
`10000000` atomic units. Division must be exact and multiplication must fit
`u128`. Missing, mismatched, inexact or overflowing bindings produce
`CurrencyBindingRequired`, `CurrencyBindingMismatch`, `InexactAmount` or
`AmountOverflow`; the adapter never rounds a total.

### Checkout evidence and retries

`UcpAdapter::complete_checkout` takes deployment-pinned
`&[SequencerAuthorization]` receipt keys. The configured payment plane returns
`Pending`, `Refused`, or `ExecutedUcpPayment { metadata, evidence }`, where
`evidence` is `SequencerEvidence`: transaction hash, canonical signed receipt,
logs and settlement rung.

Completion checks the receipt signature with `verify_sequencer_signature`,
binds its transaction hash, and checks logs against its signed effects with
`verify_logs`. It requires a successful native receipt and one matching
`PaymentExact` event from the ledger precompile at
`0x000000000000000000000000000000000000101a`. The event must bind the derived
idempotency key, configured asset, exact converted amount and recipient.
`GatewayCore::settle_with_sequencer_evidence` then records
`TranslationStatus::ReceiptVerified` with the canonical receipt digest.

Pending or `submitted` evidence leaves the checkout `CompleteInProgress`
without an order. Verified evidence can produce `Completed`, a `UcpOrder`
containing the transaction hash and receipt digest, and a `StoredOrder`
containing the signed evidence. A verified receipt at the `receipted` rung
does not itself establish block commitment. Retry with the same checkout,
configuration and UUID; the payment plane must recover the original payment.
A different receipt for an already settled identity is refused.

Order reads must use `UcpAdapter::read_order_with_config` with the merchant's
binding and pinned receipt keys. This re-verifies stored receipt bytes, logs,
transaction hash and payment facts on every read, and requires negotiated
order support. The unconfigured `read_order` convenience method uses an empty
configuration and refuses with `CurrencyBindingRequired`.

## Service integration

Merchant discovery and shopping REST endpoints belong to the merchant's
pinned UCP contract. The interop service registers the adapter wrapper
`POST /v1/http/ucp/checkouts/complete`; it does not register a public UCP
discovery or order-read endpoint. The wrapper uses `UcpServicePlane`
with the default empty service configuration. For an otherwise valid request,
the library refuses completion with `CurrencyBindingRequired`, which the HTTP
wrapper maps to `400` with code `invalid_ucp_request`. Configured embeddings use
`UcpServiceConfig::bind_plane` with the merchant currency binding above.

No public merchant origin is established by this crate. Deployment inputs
and trust configuration are described in
`services/interop/deploy/gateway/README.md`. [Visa TAP](visa-tap.md) can
authenticate a commerce agent, but that credential grants no payment authority.

## Status

Pinned vendor documents are in `services/interop/specs/vendor/ucp/`.
The crate's tests include merchant profiles, negotiated capabilities,
sequencer-backed checkout evidence and checked currency conversion;
first-party vectors live in `services/interop/specs/conformance/ucp/`.
These files do not establish a live hosted merchant or third-party UCP
qualification.

Sources: `services/interop/crates/layerx-ucp/src/lib.rs`,
`services/interop/crates/layerx-ucp/src/config.rs`, and
`services/interop/crates/layerx-interop-service/src/server.rs`.

[Interop](index.md) · [x402](x402.md) · [Visa TAP](visa-tap.md)
