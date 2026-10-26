# Interop specs contract

Source revision: `2a1033aa2`. This contract freezes the cited interface and acceptance blocks of the specification. Statuses include the specification overrides. A source marked planned has line 0; a positive source line is verified in the pinned revision.

The context and decision tables reproduce the rules linked by interface identifiers. An anchor without a linked context gains no additional storage, gas, activation or pause rule here. Acceptance qualification remains UNRUN until its declared gate succeeds.

## conformance-vectors

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-spec-committed-vectors-j47</code> | <code>file_format</code> | <code>new</code> | <code>services/interop/specs/conformance/committed/vector-v4-committed.json</code> (planned) | <code>committed-rung vectors: receipt, header, validator set, commit, batch proof</code> |
| <code>interface.11.fmt-spec-fiat-sequenced-journeys-j47</code> | <code>file_format</code> | <code>new</code> | <code>services/interop/specs/conformance/fiat/sequenced-journeys.json</code> (planned) | <code>fiat sequenced journey vectors</code> |
| <code>interface.11.fmt-spec-ucp-sequenced-order-j47</code> | <code>file_format</code> | <code>new</code> | <code>services/interop/specs/conformance/ucp/sequenced-order.json</code> (planned) | <code>UCP sequenced order vectors</code> |
| <code>interface.11.fmt-spec-x402-sequenced-settlement-j47</code> | <code>file_format</code> | <code>new</code> | <code>services/interop/specs/conformance/x402/sequenced-settlement.json</code> (planned) | <code>x402 sequenced settlement vectors</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-spec-committed-vectors-j47</code> | notes | Task 11.14.3; recorded via wiring.J47.1. |
| <code>interface.11.fmt-spec-fiat-sequenced-journeys-j47</code> | notes | Task 11.14.5. |
| <code>interface.11.fmt-spec-ucp-sequenced-order-j47</code> | notes | Task 11.14.4. |
| <code>interface.11.fmt-spec-x402-sequenced-settlement-j47</code> | notes | Task 11.14.1. |

Contract context <code>contract_context.11.x402-sequencer-settlement</code>:

| Field | Frozen rule |
|---|---|
| interface | x402 seller, facilitator and buyer settle and capture on SequencerEvidence with transaction set to the transaction hash. |
| storage | None beyond the gateway records. |
| errors | X402Error::EvidenceMismatch, X402Error::TransactionForm. |
| events | None. |
| gas | Paid by the payer-signed transaction. |
| activation | Wave 2; committed rung in wave 5. |
| invariants | transaction is a 32-byte hash; committed only with a verified proof. |
| pause | Not pausable. |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.11.26.1</code> | req | 11026.1 |
| <code>acceptance.11.26.1</code> | scenario | Given each case of sequenced-order.json, when it is replayed through complete_checkout, then the stored order or refusal code equals the expected field. |
| <code>acceptance.11.26.1</code> | test | services/interop/crates/layerx-ucp/tests/sequenced_vectors.rs::ucp_sequenced_order_vectors_replay (planned) |
| <code>acceptance.11.26.1</code> | qualification | UNRUN |
| <code>req.11026</code> | ac_1 | Every case of the UCP sequenced order vectors yields its expected order or refusal (planned) (services/interop/crates/layerx-ucp/src/lib.rs:542). |
| <code>acceptance.11.27.1</code> | req | 11027.1 |
| <code>acceptance.11.27.1</code> | scenario | Given each journey of sequenced-journeys.json, when its steps are applied, then the final FiatJourneyState equals the expected field and the reverted journey is refused. |
| <code>acceptance.11.27.1</code> | test | services/interop/crates/layerx-fiat/tests/sequenced_vectors.rs::fiat_sequenced_journey_vectors_replay (planned) |
| <code>acceptance.11.27.1</code> | qualification | UNRUN |
| <code>req.11027</code> | ac_1 | Every fiat sequenced journey ends in its expected state (planned) (services/interop/crates/layerx-fiat/src/lib.rs:293). |
| <code>acceptance.11.35.1</code> | req | 11035.1 |
| <code>acceptance.11.35.1</code> | scenario | Given every case of vector-v4-committed.json, when verify_commit runs on its header and commit, then accepting cases return their power and refusal cases return their expected ProofError. |
| <code>acceptance.11.35.1</code> | test | services/interop/crates/layerx-committed-proof/tests/committed_vectors.rs::committed_vector_commits_verify (planned) |
| <code>acceptance.11.35.1</code> | qualification | UNRUN |
| <code>req.11035</code> | ac_1 | Each accepting committed vector's commit verifies and each refusal case is refused (planned) (consensus/types/block.go:853). |
| <code>acceptance.11.5.1</code> | req | 11005.1 |
| <code>acceptance.11.5.1</code> | scenario | Given sequenced-settlement.json, when each case is decoded, then its transaction equals the recorded txHash and its receiptDigest equals the recorded receiptDigest. |
| <code>acceptance.11.5.1</code> | test | services/interop/crates/layerx-x402/tests/sequenced_vectors.rs::sequenced_settlement_vectors_carry_the_recorded_receipt (planned) |
| <code>acceptance.11.5.1</code> | qualification | UNRUN |
| <code>req.11005</code> | ac_1 | Every case of sequenced-settlement.json (planned) carries the txHash and receiptDigest recorded at sdk/platform/conformance/fixtures/sequencer-receipt-v1.json:150. |
| <code>acceptance.11.5.2</code> | req | 11005.2 |
| <code>acceptance.11.5.2</code> | scenario | Given the three cases, when SettlementResponse::validate_wire runs on each body, then receipted and submitted validate and committed_without_proof is refused. |
| <code>acceptance.11.5.2</code> | test | services/interop/crates/layerx-x402/tests/sequenced_vectors.rs::sequenced_settlement_vectors_validate_on_the_wire (planned) |
| <code>acceptance.11.5.2</code> | qualification | UNRUN |
| <code>req.11005</code> | ac_2 | Each case of sequenced-settlement.json (planned) passes or fails SettlementResponse::validate_wire (services/interop/crates/layerx-x402/src/model.rs:447) as its expected verdict states. |
| <code>acceptance.11.54.1</code> | req | 11054.1 |
| <code>acceptance.11.54.1</code> | scenario | Given the committed case, when it is replayed through the facilitator, then the response level is committed with its block fields. |
| <code>acceptance.11.54.1</code> | test | services/interop/crates/layerx-x402/tests/sequenced_vectors.rs::x402_committed_vector_settles_as_committed (planned) |
| <code>acceptance.11.54.1</code> | qualification | UNRUN |
| <code>req.11054</code> | ac_1 | The committed x402 case settles with level committed (planned) (services/interop/crates/layerx-x402/src/facilitator.rs:484). |
| <code>capability.11.14.1</code> | title | Record x402 sequenced settlement vectors from the recorded payExact receipt |
| <code>capability.11.14.1</code> | reqs | 11005.1<br>11005.2 |
| <code>capability.11.14.2</code> | title | Publish UCP sequenced order vectors and replay them against the adapter |
| <code>capability.11.14.2</code> | reqs | 11026.1 |
| <code>capability.11.14.3</code> | title | Publish fiat sequenced journey vectors and replay them against the adapter |
| <code>capability.11.14.3</code> | reqs | 11027.1 |
| <code>capability.11.14.4</code> | title | Publish committed-rung vectors recorded from a local chain |
| <code>capability.11.14.4</code> | reqs | 11035.1 |
| <code>capability.11.14.5</code> | title | Add the committed rung case to the x402 sequenced settlement vectors |
| <code>capability.11.14.5</code> | reqs | 11054.1 |

## fmt-specs-conformance-ap2-autonomous-001-line-items

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-specs-conformance-ap2-autonomous-001-line-items</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/specs/conformance/ap2/autonomous/001-line-items.json:1</code> | <code>JSON object {description, test_case_id, generator, issuer_key_scalar, merchant_key_scalar, agent_key_scalar, verification_context, expected_outcome, expected_values, notes}</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-specs-conformance-ap2-autonomous-001-line-items</code> | notes | first-party conformance vectors, read by the crate tests listed in callers |

## fmt-specs-conformance-ap2-constraints-001-amount-range-exceeded

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-specs-conformance-ap2-constraints-001-amount-range-exceeded</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/specs/conformance/ap2/constraints/001-amount-range-exceeded.json:1</code> | <code>JSON object {description, test_case_id, generator, authentic_amount_minor_units, constraint_max_minor_units, verification_context, expected_outcome, expected_error, notes}</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-specs-conformance-ap2-constraints-001-amount-range-exceeded</code> | notes | first-party conformance vectors, read by the crate tests listed in callers |

## fmt-specs-conformance-ap2-direct-001-minimal-valid

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-specs-conformance-ap2-direct-001-minimal-valid</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/specs/conformance/ap2/direct/001-minimal-valid.json:1</code> | <code>JSON object {description, test_case_id, generator, issuer_key_scalar, merchant_key_scalar, verification_context, expected_outcome, expected_values, notes}</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-specs-conformance-ap2-direct-001-minimal-valid</code> | notes | first-party conformance vectors, read by the crate tests listed in callers |

## fmt-specs-conformance-ap2-readme

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-specs-conformance-ap2-readme</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/specs/conformance/ap2/README.md:1</code> | <code>Markdown document: AP2 Golden Vectors</code> |

## fmt-specs-conformance-ap2-refusals-001-expired

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-specs-conformance-ap2-refusals-001-expired</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/specs/conformance/ap2/refusals/001-expired.json:1</code> | <code>JSON object {description, test_case_id, generator, signed_checkout_iat, signed_checkout_exp, verification_context, expected_outcome, notes}</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-specs-conformance-ap2-refusals-001-expired</code> | notes | first-party conformance vectors, read by the crate tests listed in callers |

## fmt-specs-conformance-ap2-refusals-002-invalid-signature

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-specs-conformance-ap2-refusals-002-invalid-signature</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/specs/conformance/ap2/refusals/002-invalid-signature.json:1</code> | <code>JSON object {description, test_case_id, generator, mutation, verification_context, expected_outcome, notes}</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-specs-conformance-ap2-refusals-002-invalid-signature</code> | notes | first-party conformance vectors, read by the crate tests listed in callers |

## fmt-specs-conformance-ap2-refusals-003-binding-mismatch

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-specs-conformance-ap2-refusals-003-binding-mismatch</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/specs/conformance/ap2/refusals/003-binding-mismatch.json:1</code> | <code>JSON object {description, test_case_id, generator, mutation, verification_context, expected_outcome, notes}</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-specs-conformance-ap2-refusals-003-binding-mismatch</code> | notes | first-party conformance vectors, read by the crate tests listed in callers |

## fmt-specs-conformance-fiat-journeys

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-specs-conformance-fiat-journeys</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/specs/conformance/fiat/journeys.json:1</code> | <code>JSON array of 18 objects {evidence, expect, name, now, plane, principal, provider_facts, replay_at, token, trace}</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-specs-conformance-fiat-journeys</code> | notes | first-party conformance vectors, read by the crate tests listed in callers |

## fmt-specs-conformance-fiat-readme

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-specs-conformance-fiat-readme</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/specs/conformance/fiat/README.md:1</code> | <code>Markdown document: Fiat Provider Callback Conformance Vectors</code> |

## fmt-specs-conformance-fiat-token-references

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-specs-conformance-fiat-token-references</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/specs/conformance/fiat/token-references.json:1</code> | <code>JSON array of 8 objects {name, redaction_probe, refusal, token, token_hex, valid}</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-specs-conformance-fiat-token-references</code> | notes | first-party conformance vectors, read by the crate tests listed in callers |

## fmt-specs-conformance-transport-a2a-readme

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-specs-conformance-transport-a2a-readme</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/specs/conformance/transport-a2a/README.md:1</code> | <code>Markdown document: x402 A2A Transport Conformance Vectors</code> |

## fmt-specs-conformance-transport-a2a-role-messages

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-specs-conformance-transport-a2a-role-messages</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/specs/conformance/transport-a2a/role-messages.json:1</code> | <code>JSON array of 8 objects {accepted, document, header, name, representation, role, transport}</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-specs-conformance-transport-a2a-role-messages</code> | notes | first-party conformance vectors, read by the crate tests listed in callers |

## fmt-specs-conformance-transport-http-readme

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-specs-conformance-transport-http-readme</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/specs/conformance/transport-http/README.md:1</code> | <code>Markdown document: x402 HTTP Transport Conformance Vectors</code> |

## fmt-specs-conformance-transport-http-role-messages

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-specs-conformance-transport-http-role-messages</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/specs/conformance/transport-http/role-messages.json:1</code> | <code>JSON array of 8 objects {accepted, document, header, name, representation, role, transport}</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-specs-conformance-transport-http-role-messages</code> | notes | first-party conformance vectors, read by the crate tests listed in callers |

## fmt-specs-conformance-transport-mcp-readme

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-specs-conformance-transport-mcp-readme</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/specs/conformance/transport-mcp/README.md:1</code> | <code>Markdown document: x402 MCP Transport Conformance Vectors</code> |

## fmt-specs-conformance-transport-mcp-role-messages

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-specs-conformance-transport-mcp-role-messages</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/specs/conformance/transport-mcp/role-messages.json:1</code> | <code>JSON array of 8 objects {accepted, document, header, name, representation, role, transport}</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-specs-conformance-transport-mcp-role-messages</code> | notes | first-party conformance vectors, read by the crate tests listed in callers |

## fmt-specs-conformance-ucp-capabilities

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-specs-conformance-ucp-capabilities</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/specs/conformance/ucp/capabilities.json:1</code> | <code>JSON array of 5 objects {capability, name, refusal, reverse_domain_prefix, schema, spec, valid, version}</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-specs-conformance-ucp-capabilities</code> | notes | first-party conformance vectors, read by the crate tests listed in callers |

## fmt-specs-conformance-ucp-checkout-status

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-specs-conformance-ucp-checkout-status</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/specs/conformance/ucp/checkout-status.json:1</code> | <code>JSON array of 6 objects {meaning, name, status}</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-specs-conformance-ucp-checkout-status</code> | notes | first-party conformance vectors, read by the crate tests listed in callers |

## fmt-specs-conformance-ucp-idempotency-keys

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-specs-conformance-ucp-idempotency-keys</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/specs/conformance/ucp/idempotency-keys.json:1</code> | <code>JSON array of 6 objects {key, name, refusal, valid}</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-specs-conformance-ucp-idempotency-keys</code> | notes | first-party conformance vectors, read by the crate tests listed in callers |

## fmt-specs-conformance-ucp-merchant-profiles

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-specs-conformance-ucp-merchant-profiles</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/specs/conformance/ucp/merchant-profiles.json:1</code> | <code>JSON array of 4 objects {expected_capabilities, expected_version, name, payment_handler, refusal, rest_endpoint, valid}</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-specs-conformance-ucp-merchant-profiles</code> | notes | first-party conformance vectors, read by the crate tests listed in callers |

## fmt-specs-conformance-ucp-payment-handlers

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-specs-conformance-ucp-payment-handlers</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/specs/conformance/ucp/payment-handlers.json:1</code> | <code>JSON array of 5 objects {digest_group, id, name, refusal, schema, spec, valid, version}</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-specs-conformance-ucp-payment-handlers</code> | notes | first-party conformance vectors, read by the crate tests listed in callers |

## fmt-specs-conformance-ucp-readme

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-specs-conformance-ucp-readme</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/specs/conformance/ucp/README.md:1</code> | <code>Markdown document: UCP Checkout Conformance Vectors</code> |

## fmt-specs-conformance-visa-tap-credential-verification

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-specs-conformance-visa-tap-credential-verification</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/specs/conformance/visa-tap/credential-verification.json:1</code> | <code>JSON array of 16 objects {authority, clock_skew_seconds, expect, intent, key_expires_at, key_status, name, now, path, registry_agent_domain, registry_agent_id, registry_key_id, registry_layerx_agent, replays, signature_params, signed_authority, signed_path, signing_key_seed}</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-specs-conformance-visa-tap-credential-verification</code> | notes | first-party conformance vectors, read by the crate tests listed in callers |

## fmt-specs-conformance-visa-tap-readme

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-specs-conformance-visa-tap-readme</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/specs/conformance/visa-tap/README.md:1</code> | <code>Markdown document: Visa Trusted Agent Protocol Conformance Vectors</code> |

## fmt-specs-conformance-visa-tap-target-canonicalization

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-specs-conformance-visa-tap-target-canonicalization</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/specs/conformance/visa-tap/target-canonicalization.json:1</code> | <code>JSON array of 7 objects {accepted, authority, name, path, refusal, signature, signature_input}</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-specs-conformance-visa-tap-target-canonicalization</code> | notes | first-party conformance vectors, read by the crate tests listed in callers |

## fmt-specs-conformance-x402-payment-payload

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-specs-conformance-x402-payment-payload</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/specs/conformance/x402/payment-payload.json:1</code> | <code>JSON array of 5 objects {document, name, valid}</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-specs-conformance-x402-payment-payload</code> | notes | first-party conformance vectors, read by the crate tests listed in callers |

## fmt-specs-conformance-x402-payment-required

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-specs-conformance-x402-payment-required</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/specs/conformance/x402/payment-required.json:1</code> | <code>JSON array of 12 objects {document, name, valid}</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-specs-conformance-x402-payment-required</code> | notes | first-party conformance vectors, read by the crate tests listed in callers |

## fmt-specs-conformance-x402-readme

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-specs-conformance-x402-readme</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/specs/conformance/x402/README.md:1</code> | <code>Markdown document: x402 v2 Conformance Vectors</code> |

## fmt-specs-conformance-x402-settlement-response

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-specs-conformance-x402-settlement-response</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/specs/conformance/x402/settlement-response.json:1</code> | <code>JSON array of 5 objects {document, name, valid}</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-specs-conformance-x402-settlement-response</code> | notes | first-party conformance vectors, read by the crate tests listed in callers |

## fmt-specs-vendor-ap2-provenance

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-specs-vendor-ap2-provenance</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/specs/vendor/ap2/PROVENANCE.md:1</code> | <code>Markdown document: AP2 vendored specification provenance</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-specs-vendor-ap2-provenance</code> | notes | pinned protocol specification copy; crates embed and digest-check it |

## fmt-specs-vendor-ap2-specification

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-specs-vendor-ap2-specification</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/specs/vendor/ap2/specification.md:1</code> | <code>Markdown document: Agentic Payment Protocol (v0.2)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-specs-vendor-ap2-specification</code> | notes | pinned protocol specification copy; crates embed and digest-check it |

## fmt-specs-vendor-conformance

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-specs-vendor-conformance</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/specs/vendor/CONFORMANCE.md:1</code> | <code>Markdown document: conformance suite survey</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-specs-vendor-conformance</code> | notes | pinned protocol specification copy; crates embed and digest-check it |

## fmt-specs-vendor-ucp-checkout-schema

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-specs-vendor-ucp-checkout-schema</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/specs/vendor/ucp/checkout.schema.json:1</code> | <code>JSON object {$schema, $id, name, title, description, type, required, additionalProperties, properties, version}</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-specs-vendor-ucp-checkout-schema</code> | notes | pinned protocol specification copy; crates embed and digest-check it |

## fmt-specs-vendor-ucp-order-schema

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-specs-vendor-ucp-order-schema</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/specs/vendor/ucp/order.schema.json:1</code> | <code>JSON object {$schema, $id, name, title, description, $defs, type, required, properties, version}</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-specs-vendor-ucp-order-schema</code> | notes | pinned protocol specification copy; crates embed and digest-check it |

## fmt-specs-vendor-ucp-provenance

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-specs-vendor-ucp-provenance</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/specs/vendor/ucp/PROVENANCE.md:1</code> | <code>Markdown document: UCP vendored specification provenance</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-specs-vendor-ucp-provenance</code> | notes | pinned protocol specification copy; crates embed and digest-check it |

## fmt-specs-vendor-ucp-rest-openapi

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-specs-vendor-ucp-rest-openapi</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/specs/vendor/ucp/rest.openapi.json:1</code> | <code>JSON object {openapi, info, servers, paths, webhooks, components}</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-specs-vendor-ucp-rest-openapi</code> | notes | pinned protocol specification copy; crates embed and digest-check it |

## fmt-specs-vendor-ucp-specification-checkout

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-specs-vendor-ucp-specification-checkout</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/specs/vendor/ucp/specification-checkout.html:1</code> | <code>HTML document (pinned specification text)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-specs-vendor-ucp-specification-checkout</code> | notes | pinned protocol specification copy; crates embed and digest-check it |

## fmt-specs-vendor-ucp-specification-order

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-specs-vendor-ucp-specification-order</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/specs/vendor/ucp/specification-order.html:1</code> | <code>HTML document (pinned specification text)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-specs-vendor-ucp-specification-order</code> | notes | pinned protocol specification copy; crates embed and digest-check it |

## fmt-specs-vendor-visa-tap-provenance

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-specs-vendor-visa-tap-provenance</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/specs/vendor/visa-tap/PROVENANCE.md:1</code> | <code>Markdown document: Visa Trusted Agent Protocol vendored document provenance</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-specs-vendor-visa-tap-provenance</code> | notes | pinned protocol specification copy; crates embed and digest-check it |

## fmt-specs-vendor-visa-tap-readme

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-specs-vendor-visa-tap-readme</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/specs/vendor/visa-tap/README.md:1</code> | <code>Markdown document: Trusted Agent Protocol</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-specs-vendor-visa-tap-readme</code> | notes | pinned protocol specification copy; crates embed and digest-check it |

## fmt-specs-vendor-x402-license

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-specs-vendor-x402-license</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/specs/vendor/x402/LICENSE:1</code> | <code>license text for the pinned specification copy</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-specs-vendor-x402-license</code> | notes | pinned protocol specification copy; crates embed and digest-check it |

## fmt-specs-vendor-x402-notice

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-specs-vendor-x402-notice</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/specs/vendor/x402/NOTICE:1</code> | <code>notice text for the pinned specification copy</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-specs-vendor-x402-notice</code> | notes | pinned protocol specification copy; crates embed and digest-check it |

## fmt-specs-vendor-x402-provenance

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-specs-vendor-x402-provenance</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/specs/vendor/x402/PROVENANCE.md:1</code> | <code>Markdown document: x402 vendored specification provenance</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-specs-vendor-x402-provenance</code> | notes | pinned protocol specification copy; crates embed and digest-check it |

## fmt-specs-vendor-x402-transports-a2a

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-specs-vendor-x402-transports-a2a</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/specs/vendor/x402/transports/a2a.md:1</code> | <code>Markdown document: Transport: A2A (Agent-to-Agent Protocol)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-specs-vendor-x402-transports-a2a</code> | notes | pinned protocol specification copy; crates embed and digest-check it |

## fmt-specs-vendor-x402-transports-http

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-specs-vendor-x402-transports-http</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/specs/vendor/x402/transports/http.md:1</code> | <code>Markdown document: Transport: HTTP</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-specs-vendor-x402-transports-http</code> | notes | pinned protocol specification copy; crates embed and digest-check it |

## fmt-specs-vendor-x402-transports-mcp

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-specs-vendor-x402-transports-mcp</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/specs/vendor/x402/transports/mcp.md:1</code> | <code>Markdown document: Transport: MCP (Model Context Protocol)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-specs-vendor-x402-transports-mcp</code> | notes | pinned protocol specification copy; crates embed and digest-check it |

## fmt-specs-vendor-x402-x402-specification-v2

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-specs-vendor-x402-x402-specification-v2</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/specs/vendor/x402/x402-specification-v2.md:1</code> | <code>Markdown document: X402 Protocol Specification</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-specs-vendor-x402-x402-specification-v2</code> | notes | pinned protocol specification copy; crates embed and digest-check it |
