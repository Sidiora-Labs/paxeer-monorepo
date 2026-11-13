# x402 HTTP Transport Conformance Vectors

The first-party x402 HTTP binding suite: every vector is one role message as it
travels on this transport, together with whether the binding must accept that
representation.

## Files

- `role-messages.json` — one record per x402 role on this transport

## Vector Format

Each file holds a JSON array of records:

- `name`: the case the vector covers
- `transport`: the transport under test, so a record cannot be read by the
  wrong binding
- `role`: the x402 role the document plays
- `accepted`: whether the binding must accept this representation
- `representation`: `http-header` or `json`
- `header`: the exact header name a header representation travels in
- `document`: the wire document, exactly as it travels
- `semantic`: separate model and payment expectations; model acceptance never
  establishes that an opaque authorization can pay
- `semantic_cases`: additional complete role records with their own `document`,
  `accepted` and `semantic` expectations, plus mounted `ingress` expectations
  where the gateway exposes that role

This binding carries base64 of the JSON document in the `PAYMENT-REQUIRED`, `PAYMENT-SIGNATURE` and `PAYMENT-RESPONSE` headers for the three payment messages, and JSON for the facilitator bodies.

## Conformance

`services/interop/crates/layerx-x402/tests/transports.rs` reads these files with
`include_str!` and runs every record through the production x402 model types
and the production encoder and decoder for this transport, so the suite the
deployment pins is the suite the tests exercise.

The fault-injected settlement cases in that file are not vectors and are not
counted: they drive a live sequencer signer and a canonical receipt encoder
rather than a transport representation.

`services/interop/deploy/gateway/render.py` derives the vector count and the SHA-256
the gateway configuration declares from these files.

## Semantic and payment expectations

The current production `LAYERX_NETWORK` is `eip155:125`.
`PaymentRequirements::validate` requires `extra.layerx.account` and
`extra.layerx.currency` on that network, and binds `payTo` to an account id
derived from the account reference. The production transport encoders and
decoders call semantic validation.
The profile-free `eip155:125` cases therefore explicitly expect `ProfileMissing`;
a LayerX profile on the foreign `eip155:1` network expects `ProfileMismatch`.

The original role documents remain transport compatibility fixtures. Their opaque
`authorization` payloads have explicit payment refusal expectations:
`typed_intent_required`. They must never be presented as successful payments.
The additional `recorded_native_*` cases carry matching offers and unchanged
signed payer bytes from `V1-layerxledger-pay-exact` in
`sdk/platform/conformance/fixtures/sequencer-receipt-v1.json`. Their payee account,
asset, amount and idempotency reference match that transaction and its recorded
PaymentExact log. The settlement case names the same transaction hash and remains
pending until the production relay supplies verified evidence.

`ingress` describes POST JSON bodies sent to `/v1/http/x402/seller/offer` and
`/v1/http/x402/verify`. The nested case's `document` is the body; fixture metadata
is never sent. A valid seller offer returns an HTTP 200 dispatch containing a
payment-required 402 signal; refused profiles return HTTP 400. Settlement needs
authenticated `/v1/http/x402/settle` ingress, the recorded
`layerxIdempotencyKey`, and a relay. Replay uses the fixture's
`recorded_payer.observed_at_ms`, before expiry, rather than the wall clock.
