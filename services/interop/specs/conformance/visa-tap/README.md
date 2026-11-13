# Visa Trusted Agent Protocol Conformance Vectors

The first-party Visa TAP suite: every vector is an HTTP Message Signatures
credential presentation or a signed-target representation, together with the
verification outcome the production types must reach.

## Files

- `credential-verification.json` — full credential presentations and outcomes
- `target-canonicalization.json` — authority and path representations

## Vector Format

Each file holds a JSON array of records. Credential records carry the exact
wire strings of the presentation and the registry it is verified against:

- `name`: the case the vector covers
- `signing_key_seed`: the 32-byte Ed25519 seed the record is signed with
- `authority`, `path`: the covered target components
- `signed_authority`, `signed_path`: what the signature actually covers, so a
  tampered presentation can be expressed as data
- `signature_params`: the exact RFC 9421 `Signature-Input` parameters
- `registry_*`, `key_status`, `key_expires_at`: the registered agent key
- `clock_skew_seconds`: a number verifies through `TapVerifier::verify_credential`, `null`
  verifies through `TapVerifier::verify` and a shared nonce window
- `now`, `replays`: verification time and how many accepted deliveries precede
  this one
- `expect`: `verified` or the exact `TapError` variant
- `intent`: the intent a verified record must carry

Target records carry `authority`, `path`, `signature_input`, `signature`,
`accepted` and, when refused, `refusal`.

## Conformance

`services/interop/crates/layerx-visa-tap/tests/conformance.rs` reads these files with
`include_str!` and runs every record through the production `TapRequest`,
`SignatureInput`, `RegisteredAgentKey`, `NonceWindow` and `TapVerifier` types.
The tests sign the canonical signature base at runtime using each record's
deterministic test-key seed; the remaining presentation fields come from data.

The harness also includes tests outside the JSON vector count.
`binding_is_scoped_non_authoritative_and_success_requires_a_real_receipt`
checks principal-scoped credential binding and refusal of invalid receipt bytes.
The `v2_*` tests verify exact payments against a governed receipt-key list,
inclusive height bounds and signed ledger logs. They also cover invalid payment
terms, failed or unverifiable receipts, and missing or duplicate payment logs.

`services/interop/deploy/gateway/render.py` derives the suite identifier, the vector
count and the SHA-256 the gateway configuration declares from these files.
