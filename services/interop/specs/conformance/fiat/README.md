# Fiat Provider Callback Conformance Vectors

The first-party fiat suite covers token references, provider-evidence journeys
and recorded payment evidence consumed by the fiat adapter.

## Files

- `token-references.json` — exact token bytes and their acceptance or refusal.
- `journeys.json` — provider facts, payment-plane outcomes and expected journey states.
- `sequenced-journeys.json` — signed provider facts bound to recorded native payments.

## Vector format

Each file holds a JSON array. Token records carry `name`, `valid`, either
`token` or `token_hex`, the exact `refusal` for an invalid record, and an optional
`redaction_probe` for debug rendering.

Records in `journeys.json` carry:

- `name`, `principal`, `trace`, `token`, `evidence` and `now`.
- `provider_facts`: `provider`, `settlement`, `rail`, `evidence_class`, `amount`,
  `asset`, `destination`, `observed_at`, `hold_until` and an optional `fault`.
- `plane`: an `outcome` of `pending`, `refused` or `executed`; executed outcomes
  include a `receipt` with `signing_key_seed`, `sequence`, `amount`, `from` and `to`.
- `replay_at`: an optional second delivery time.
- `expect`: the journey `state`, its time bound when applicable, or an exact `refusal`.

Records in `sequenced-journeys.json` carry `provider_public_key`,
`provider_signature` and `provider_facts`, including a token digest. Their
`sequencer` object identifies a producer fixture and case, receipt rung,
transaction hash, receipt digest and payment reference. `now`, `replay_at` and
`expect` define replay and outcome checks. These are public test vectors;
production provider trust is configured separately.

## Conformance

`services/interop/crates/layerx-fiat/tests/adapter.rs` reads the token and
`journeys.json` files with `include_str!` and exercises `TokenReference`,
`VerifiedProviderFacts` and `FiatAdapter` with `GatewayCore`. Executed journey
receipts are signed at runtime from each record's deterministic test-key seed.

`services/interop/crates/layerx-fiat/tests/sequenced_vectors.rs` reads
`sequenced-journeys.json`, verifies the provider signature and token binding,
and consumes the named payment evidence from
`sdk/platform/conformance/fixtures/fiat-receipts-v1.json`. Its cases cover
receipted payments, submission-only evidence, reverted payments and replay.

`services/interop/deploy/gateway/render.py` derives the suite identifier, vector
count and SHA-256 from all JSON files in this directory.
