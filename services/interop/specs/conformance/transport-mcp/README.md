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
also invoke semantic validation. The retained envelope-only profile cases are
therefore expected to fail with `ProfileMismatch`, despite their historical
`accepted: true` envelope classification. The consumer must assert the explicit
semantic expectation separately; its existing single-boolean harness needs the
owning-domain wiring recorded in `repair_22_33_97.kvx`.

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

The original profile-bearing records remain intentional envelope-only cases.
`PaymentRequirements::validate` rejects `extra.layerx` on `eip155:125` with
`ProfileMismatch`. The original pending settlement responses and opaque
authorization are representations, with no proof of a successful settlement.

The current offer passes the requirements rules. The seller and facilitator
currently call `layerx_terms()` unconditionally, so even the corrected offer
reaches `ProfileMissing`: mounted `POST /v1/mcp/x402/seller/offer` returns
`400 invalid_x402_offer`, and mounted verify/settle returns
`400 invalid_x402_request`. These pre-existing consumer mismatches require
out-of-scope repairs. No current fixture claims successful settlement through
those blockers.

The focused Python qualification parses the fixtures and recorded transaction,
checks their byte-for-byte provenance and payment facts, and asserts production
source connections through the codec, route, service and relay. It does not
execute the Rust validator or mounted ingress. Runtime semantic validation,
ingress coverage and deployment digest regeneration are owning-domain wiring
requests in `tools/qualification/audit_repairs/repair_22_33_97.kvx`.
