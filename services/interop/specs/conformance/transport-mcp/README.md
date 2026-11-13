# x402 MCP Transport Conformance Vectors

The first-party x402 MCP binding suite: every vector is one role message as it
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
- `validationScope`: `envelope-only` for retained representation cases or
  `current-offer` for chain-125 offers bound to recorded payer bytes
- `semanticExpectation`: the separate requirements, binding and settlement
  expectations; `accepted` retains the envelope expectation
- `producerFixture`: the committed producer case, transaction hash and recorded
  observation time used by the current offers
- `ingress`: the mounted route and expected response, when applicable

This binding carries JSON on every message.

## Conformance

`services/interop/crates/layerx-x402/tests/transports.rs` reads these files with
`include_str!` and runs every record through the production x402 model types
and the production encoder and decoder for this transport. The current codecs
also invoke semantic validation. `profile_refusal` assigns `ProfileMissing`
to the three `eip155_125_*` payment cases, and the harness checks that both
encoding and decoding refuse them even when their envelope representation is
classified as accepted.

The fault-injected settlement cases in that file are not vectors and are not
counted: they drive a live sequencer signer and a canonical receipt encoder
rather than a transport representation.

`services/interop/deploy/gateway/render.py` derives the vector count and the SHA-256
the gateway configuration declares from these files.

## Current offers and semantic refusals

The `eip155_125_*` records carry `eip155:125` offers without `extra.layerx`.
Their amount, asset and payee match the unmodified signed `payExact` transaction
in `V1-layerxledger-pay-exact` from
`sdk/platform/conformance/fixtures/sequencer-receipt-v1.json`. Both `transaction`
(the facilitator input) and `layerxTransaction` (the mounted service input)
contain those same bytes; `layerxIdempotencyKey` is the recorded payment reference.
The recorded observation time is historical and must be used for deterministic
expiry checks. These bytes must not be submitted to a live endpoint.

`PaymentRequirements::validate` requires `extra.layerx.account` and
`extra.layerx.currency` on `eip155:125`, and binds `payTo` to that account's
identifier. Offers on another network must omit the LayerX profile. The
profile-free `eip155_125_*` records therefore produce `ProfileMissing`.

The original pending settlement responses and opaque authorization fields are
role representations; they do not establish successful settlement. Mounted
payment ingress additionally requires authentication, typed signed payer bytes,
a payment idempotency key and verified receipt evidence from its configured
relay.
