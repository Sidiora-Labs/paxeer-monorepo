# UCP Checkout Conformance Vectors

The first-party UCP suite contains checkout model values and receipt-backed
order records with their declared outcomes.

## Files

- `capabilities.json` — capability names, versions, specification and schema URLs.
- `checkout-status.json` — checkout status tokens and their meanings.
- `idempotency-keys.json` — idempotency-key strings and their refusals.
- `merchant-profiles.json` — merchant profiles and their refusals.
- `payment-handlers.json` — payment handler declarations and digest grouping.
- `sequenced-order.json` — checkout, payment evidence, order metadata and expected outcomes.

## Vector format

Each file holds a JSON array. Constructor records use `name`, `valid` and,
for invalid inputs, the exact `UcpError` in `refusal`. Their wire fields include
`capability`, `version`, `spec`, `schema`, `rest_endpoint`, `payment_handler`,
`key` and `id`. Merchant profiles also declare expected versions and capabilities.
Payment handler records use `digest_group`: members of one group produce the
same digest, and distinct groups produce different digests.

Status records use `name`, `status` and `meaning`. Order records instead carry
`checkout`, `metadata`, `evidence`, signed receipt material and `expected`;
`source_case` identifies their producer case. Submission-only evidence yields
`complete_in_progress`; authenticated matching payment evidence is required
for a completed order.

## Conformance

`services/interop/crates/layerx-ucp/tests/conformance_vectors.rs` reads the five
model files with `include_str!`. It exercises `MerchantProfile`, `PaymentHandler`,
`UcpIdempotencyKey` and `Capability`, and checks the complete `CheckoutStatus`
vocabulary. Its descriptor, exported identifier, vendored-revision and receipt
checks are additional Rust cases outside the JSON record count.

`services/interop/crates/layerx-ucp/tests/sequenced_vectors.rs` exercises receipt
and order settlement from its crate-local `fixtures/sequenced-order.json`, using
producer recordings in `sdk/platform/conformance/fixtures/ucp-checkout-receipt-v1.json`
and `sdk/platform/conformance/fixtures/sequencer-receipt-v1.json`.

`services/interop/deploy/gateway/render.py` derives the suite identifier, vector
count and SHA-256 from all six JSON files in this directory.
