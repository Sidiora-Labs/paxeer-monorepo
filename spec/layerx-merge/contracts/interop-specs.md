# Interop specs contract

The [interop specifications](../../../services/interop/specs) contain pinned protocol documents and conformance data used by the protocol adapters. File-format entries below describe the checked-in documents and fixture shapes.

## conformance-vectors

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-spec-committed-vectors-j47</code> | <code>file_format</code> | [services/interop/specs/conformance/committed/vector-v4-committed.json](../../../services/interop/specs/conformance/committed/vector-v4-committed.json) | <code>committed-rung vectors: receipt, header, validator set, commit, batch proof</code> |
| <code>interface.11.fmt-spec-fiat-sequenced-journeys-j47</code> | <code>file_format</code> | [services/interop/specs/conformance/fiat/sequenced-journeys.json](../../../services/interop/specs/conformance/fiat/sequenced-journeys.json) | <code>fiat sequenced journey vectors</code> |
| <code>interface.11.fmt-spec-ucp-sequenced-order-j47</code> | <code>file_format</code> | [services/interop/specs/conformance/ucp/sequenced-order.json](../../../services/interop/specs/conformance/ucp/sequenced-order.json) | <code>UCP sequenced order vectors</code> |
| <code>interface.11.fmt-spec-x402-sequenced-settlement-j47</code> | <code>file_format</code> | [services/interop/specs/conformance/x402/sequenced-settlement.json](../../../services/interop/specs/conformance/x402/sequenced-settlement.json) | <code>x402 sequenced settlement vectors</code> |

| Field | Rule |
|---|---|
| interface | x402 seller, facilitator and buyer settle and capture on SequencerEvidence with transaction set to the transaction hash. |
| storage | None beyond the gateway records. |
| errors | X402Error::EvidenceMismatch, X402Error::TransactionForm. |
| events | None. |
| gas | Paid by the payer-signed transaction. |
| invariants | transaction is a 32-byte hash; committed only with a verified proof. |
| pause | Not pausable. |

## fmt-specs-conformance-ap2-autonomous-001-line-items

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-specs-conformance-ap2-autonomous-001-line-items</code> | <code>file_format</code> | [services/interop/specs/conformance/ap2/autonomous/001-line-items.json](../../../services/interop/specs/conformance/ap2/autonomous/001-line-items.json) | <code>JSON object with keys agent_key_scalar, description, expected_outcome, expected_values, generator, issuer_key_scalar, merchant_key_scalar, notes, test_case_id, verification_context</code> |

## fmt-specs-conformance-ap2-constraints-001-amount-range-exceeded

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-specs-conformance-ap2-constraints-001-amount-range-exceeded</code> | <code>file_format</code> | [services/interop/specs/conformance/ap2/constraints/001-amount-range-exceeded.json](../../../services/interop/specs/conformance/ap2/constraints/001-amount-range-exceeded.json) | <code>JSON object with keys authentic_amount_minor_units, constraint_max_minor_units, description, expected_error, expected_outcome, generator, notes, test_case_id, verification_context</code> |

## fmt-specs-conformance-ap2-direct-001-minimal-valid

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-specs-conformance-ap2-direct-001-minimal-valid</code> | <code>file_format</code> | [services/interop/specs/conformance/ap2/direct/001-minimal-valid.json](../../../services/interop/specs/conformance/ap2/direct/001-minimal-valid.json) | <code>JSON object with keys description, expected_outcome, expected_values, generator, issuer_key_scalar, merchant_key_scalar, notes, test_case_id, verification_context</code> |

## fmt-specs-conformance-ap2-readme

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-specs-conformance-ap2-readme</code> | <code>file_format</code> | [services/interop/specs/conformance/ap2/README.md](../../../services/interop/specs/conformance/ap2/README.md) | <code>Markdown document: AP2 Golden Vectors</code> |

## fmt-specs-conformance-ap2-refusals-001-expired

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-specs-conformance-ap2-refusals-001-expired</code> | <code>file_format</code> | [services/interop/specs/conformance/ap2/refusals/001-expired.json](../../../services/interop/specs/conformance/ap2/refusals/001-expired.json) | <code>JSON object with keys description, expected_outcome, generator, notes, signed_checkout_exp, signed_checkout_iat, test_case_id, verification_context</code> |

## fmt-specs-conformance-ap2-refusals-002-invalid-signature

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-specs-conformance-ap2-refusals-002-invalid-signature</code> | <code>file_format</code> | [services/interop/specs/conformance/ap2/refusals/002-invalid-signature.json](../../../services/interop/specs/conformance/ap2/refusals/002-invalid-signature.json) | <code>JSON object with keys description, expected_outcome, generator, mutation, notes, test_case_id, verification_context</code> |

## fmt-specs-conformance-ap2-refusals-003-binding-mismatch

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-specs-conformance-ap2-refusals-003-binding-mismatch</code> | <code>file_format</code> | [services/interop/specs/conformance/ap2/refusals/003-binding-mismatch.json](../../../services/interop/specs/conformance/ap2/refusals/003-binding-mismatch.json) | <code>JSON object with keys description, expected_outcome, generator, mutation, notes, test_case_id, verification_context</code> |

## fmt-specs-conformance-fiat-journeys

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-specs-conformance-fiat-journeys</code> | <code>file_format</code> | [services/interop/specs/conformance/fiat/journeys.json](../../../services/interop/specs/conformance/fiat/journeys.json) | <code>JSON array of 18 items with object keys evidence, expect, name, now, plane, principal, provider_facts, replay_at, token, trace</code> |

## fmt-specs-conformance-fiat-readme

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-specs-conformance-fiat-readme</code> | <code>file_format</code> | [services/interop/specs/conformance/fiat/README.md](../../../services/interop/specs/conformance/fiat/README.md) | <code>Markdown document: Fiat Provider Callback Conformance Vectors</code> |

## fmt-specs-conformance-fiat-token-references

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-specs-conformance-fiat-token-references</code> | <code>file_format</code> | [services/interop/specs/conformance/fiat/token-references.json](../../../services/interop/specs/conformance/fiat/token-references.json) | <code>JSON array of 8 items with object keys name, redaction_probe, refusal, token, token_hex, valid</code> |

## fmt-specs-conformance-transport-a2a-readme

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-specs-conformance-transport-a2a-readme</code> | <code>file_format</code> | [services/interop/specs/conformance/transport-a2a/README.md](../../../services/interop/specs/conformance/transport-a2a/README.md) | <code>Markdown document: x402 A2A Transport Conformance Vectors</code> |

## fmt-specs-conformance-transport-a2a-role-messages

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-specs-conformance-transport-a2a-role-messages</code> | <code>file_format</code> | [services/interop/specs/conformance/transport-a2a/role-messages.json](../../../services/interop/specs/conformance/transport-a2a/role-messages.json) | <code>JSON array of 14 items with object keys accepted, document, expectations, header, name, provenance, representation, role, transport</code> |

## fmt-specs-conformance-transport-http-readme

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-specs-conformance-transport-http-readme</code> | <code>file_format</code> | [services/interop/specs/conformance/transport-http/README.md](../../../services/interop/specs/conformance/transport-http/README.md) | <code>Markdown document: x402 HTTP Transport Conformance Vectors</code> |

## fmt-specs-conformance-transport-http-role-messages

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-specs-conformance-transport-http-role-messages</code> | <code>file_format</code> | [services/interop/specs/conformance/transport-http/role-messages.json](../../../services/interop/specs/conformance/transport-http/role-messages.json) | <code>JSON array of 8 items with object keys accepted, document, header, name, representation, role, semantic, semantic_cases, transport</code> |

## fmt-specs-conformance-transport-mcp-readme

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-specs-conformance-transport-mcp-readme</code> | <code>file_format</code> | [services/interop/specs/conformance/transport-mcp/README.md](../../../services/interop/specs/conformance/transport-mcp/README.md) | <code>Markdown document: x402 MCP Transport Conformance Vectors</code> |

## fmt-specs-conformance-transport-mcp-role-messages

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-specs-conformance-transport-mcp-role-messages</code> | <code>file_format</code> | [services/interop/specs/conformance/transport-mcp/role-messages.json](../../../services/interop/specs/conformance/transport-mcp/role-messages.json) | <code>JSON array of 11 items with object keys accepted, document, header, ingress, name, producerFixture, representation, role, semanticExpectation, transport, validationScope</code> |

## fmt-specs-conformance-ucp-capabilities

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-specs-conformance-ucp-capabilities</code> | <code>file_format</code> | [services/interop/specs/conformance/ucp/capabilities.json](../../../services/interop/specs/conformance/ucp/capabilities.json) | <code>JSON array of 5 items with object keys capability, name, refusal, reverse_domain_prefix, schema, spec, valid, version</code> |

## fmt-specs-conformance-ucp-checkout-status

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-specs-conformance-ucp-checkout-status</code> | <code>file_format</code> | [services/interop/specs/conformance/ucp/checkout-status.json](../../../services/interop/specs/conformance/ucp/checkout-status.json) | <code>JSON array of 6 items with object keys meaning, name, status</code> |

## fmt-specs-conformance-ucp-idempotency-keys

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-specs-conformance-ucp-idempotency-keys</code> | <code>file_format</code> | [services/interop/specs/conformance/ucp/idempotency-keys.json](../../../services/interop/specs/conformance/ucp/idempotency-keys.json) | <code>JSON array of 6 items with object keys key, name, refusal, valid</code> |

## fmt-specs-conformance-ucp-merchant-profiles

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-specs-conformance-ucp-merchant-profiles</code> | <code>file_format</code> | [services/interop/specs/conformance/ucp/merchant-profiles.json](../../../services/interop/specs/conformance/ucp/merchant-profiles.json) | <code>JSON array of 4 items with object keys expected_capabilities, expected_version, name, payment_handler, refusal, rest_endpoint, valid</code> |

## fmt-specs-conformance-ucp-payment-handlers

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-specs-conformance-ucp-payment-handlers</code> | <code>file_format</code> | [services/interop/specs/conformance/ucp/payment-handlers.json](../../../services/interop/specs/conformance/ucp/payment-handlers.json) | <code>JSON array of 5 items with object keys digest_group, id, name, refusal, schema, spec, valid, version</code> |

## fmt-specs-conformance-ucp-readme

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-specs-conformance-ucp-readme</code> | <code>file_format</code> | [services/interop/specs/conformance/ucp/README.md](../../../services/interop/specs/conformance/ucp/README.md) | <code>Markdown document: UCP Checkout Conformance Vectors</code> |

## fmt-specs-conformance-visa-tap-credential-verification

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-specs-conformance-visa-tap-credential-verification</code> | <code>file_format</code> | [services/interop/specs/conformance/visa-tap/credential-verification.json](../../../services/interop/specs/conformance/visa-tap/credential-verification.json) | <code>JSON array of 16 items with object keys authority, clock_skew_seconds, expect, intent, key_expires_at, key_status, name, now, path, registry_agent_domain, registry_agent_id, registry_key_id, registry_layerx_agent, replays, signature_params, signed_authority, signed_path, signing_key_seed</code> |

## fmt-specs-conformance-visa-tap-readme

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-specs-conformance-visa-tap-readme</code> | <code>file_format</code> | [services/interop/specs/conformance/visa-tap/README.md](../../../services/interop/specs/conformance/visa-tap/README.md) | <code>Markdown document: Visa Trusted Agent Protocol Conformance Vectors</code> |

## fmt-specs-conformance-visa-tap-target-canonicalization

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-specs-conformance-visa-tap-target-canonicalization</code> | <code>file_format</code> | [services/interop/specs/conformance/visa-tap/target-canonicalization.json](../../../services/interop/specs/conformance/visa-tap/target-canonicalization.json) | <code>JSON array of 7 items with object keys accepted, authority, name, path, refusal, signature, signature_input</code> |

## fmt-specs-conformance-x402-payment-payload

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-specs-conformance-x402-payment-payload</code> | <code>file_format</code> | [services/interop/specs/conformance/x402/payment-payload.json](../../../services/interop/specs/conformance/x402/payment-payload.json) | <code>JSON array of 6 items with object keys document, name, valid</code> |

## fmt-specs-conformance-x402-payment-required

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-specs-conformance-x402-payment-required</code> | <code>file_format</code> | [services/interop/specs/conformance/x402/payment-required.json](../../../services/interop/specs/conformance/x402/payment-required.json) | <code>JSON array of 12 items with object keys document, name, valid</code> |

## fmt-specs-conformance-x402-readme

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-specs-conformance-x402-readme</code> | <code>file_format</code> | [services/interop/specs/conformance/x402/README.md](../../../services/interop/specs/conformance/x402/README.md) | <code>Markdown document: x402 v2 Conformance Vectors</code> |

## fmt-specs-conformance-x402-settlement-response

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-specs-conformance-x402-settlement-response</code> | <code>file_format</code> | [services/interop/specs/conformance/x402/settlement-response.json](../../../services/interop/specs/conformance/x402/settlement-response.json) | <code>JSON array of 5 items with object keys document, name, valid</code> |

## fmt-specs-vendor-ap2-provenance

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-specs-vendor-ap2-provenance</code> | <code>file_format</code> | [services/interop/specs/vendor/ap2/PROVENANCE.md](../../../services/interop/specs/vendor/ap2/PROVENANCE.md) | <code>Markdown document: AP2 vendored specification provenance</code> |

## fmt-specs-vendor-ap2-specification

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-specs-vendor-ap2-specification</code> | <code>file_format</code> | [services/interop/specs/vendor/ap2/specification.md](../../../services/interop/specs/vendor/ap2/specification.md) | <code>Markdown document: Agentic Payment Protocol (v0.2)</code> |

## fmt-specs-vendor-conformance

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-specs-vendor-conformance</code> | <code>file_format</code> | [services/interop/specs/vendor/CONFORMANCE.md](../../../services/interop/specs/vendor/CONFORMANCE.md) | <code>Markdown document: Upstream conformance suite survey</code> |

## fmt-specs-vendor-ucp-checkout-schema

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-specs-vendor-ucp-checkout-schema</code> | <code>file_format</code> | [services/interop/specs/vendor/ucp/checkout.schema.json](../../../services/interop/specs/vendor/ucp/checkout.schema.json) | <code>JSON object with keys $id, $schema, additionalProperties, description, name, properties, required, title, type, version</code> |

## fmt-specs-vendor-ucp-order-schema

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-specs-vendor-ucp-order-schema</code> | <code>file_format</code> | [services/interop/specs/vendor/ucp/order.schema.json](../../../services/interop/specs/vendor/ucp/order.schema.json) | <code>JSON object with keys $defs, $id, $schema, description, name, properties, required, title, type, version</code> |

## fmt-specs-vendor-ucp-provenance

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-specs-vendor-ucp-provenance</code> | <code>file_format</code> | [services/interop/specs/vendor/ucp/PROVENANCE.md](../../../services/interop/specs/vendor/ucp/PROVENANCE.md) | <code>Markdown document: UCP vendored specification provenance</code> |

## fmt-specs-vendor-ucp-rest-openapi

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-specs-vendor-ucp-rest-openapi</code> | <code>file_format</code> | [services/interop/specs/vendor/ucp/rest.openapi.json](../../../services/interop/specs/vendor/ucp/rest.openapi.json) | <code>JSON object with keys components, info, openapi, paths, servers, webhooks</code> |

## fmt-specs-vendor-ucp-specification-checkout

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-specs-vendor-ucp-specification-checkout</code> | <code>file_format</code> | [services/interop/specs/vendor/ucp/specification-checkout.html](../../../services/interop/specs/vendor/ucp/specification-checkout.html) | <code>HTML document (pinned specification text)</code> |

## fmt-specs-vendor-ucp-specification-order

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-specs-vendor-ucp-specification-order</code> | <code>file_format</code> | [services/interop/specs/vendor/ucp/specification-order.html](../../../services/interop/specs/vendor/ucp/specification-order.html) | <code>HTML document (pinned specification text)</code> |

## fmt-specs-vendor-visa-tap-provenance

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-specs-vendor-visa-tap-provenance</code> | <code>file_format</code> | [services/interop/specs/vendor/visa-tap/PROVENANCE.md](../../../services/interop/specs/vendor/visa-tap/PROVENANCE.md) | <code>Markdown document: Visa Trusted Agent Protocol vendored document provenance</code> |

## fmt-specs-vendor-visa-tap-readme

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-specs-vendor-visa-tap-readme</code> | <code>file_format</code> | [services/interop/specs/vendor/visa-tap/README.md](../../../services/interop/specs/vendor/visa-tap/README.md) | <code>Markdown document: Trusted Agent Protocol</code> |

## fmt-specs-vendor-x402-license

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-specs-vendor-x402-license</code> | <code>file_format</code> | [services/interop/specs/vendor/x402/LICENSE](../../../services/interop/specs/vendor/x402/LICENSE) | <code>license text for the pinned specification copy</code> |

## fmt-specs-vendor-x402-notice

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-specs-vendor-x402-notice</code> | <code>file_format</code> | [services/interop/specs/vendor/x402/NOTICE](../../../services/interop/specs/vendor/x402/NOTICE) | <code>notice text for the pinned specification copy</code> |

## fmt-specs-vendor-x402-provenance

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-specs-vendor-x402-provenance</code> | <code>file_format</code> | [services/interop/specs/vendor/x402/PROVENANCE.md](../../../services/interop/specs/vendor/x402/PROVENANCE.md) | <code>Markdown document: x402 vendored specification provenance</code> |

## fmt-specs-vendor-x402-transports-a2a

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-specs-vendor-x402-transports-a2a</code> | <code>file_format</code> | [services/interop/specs/vendor/x402/transports/a2a.md](../../../services/interop/specs/vendor/x402/transports/a2a.md) | <code>Markdown document: Transport: A2A (Agent-to-Agent Protocol)</code> |

## fmt-specs-vendor-x402-transports-http

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-specs-vendor-x402-transports-http</code> | <code>file_format</code> | [services/interop/specs/vendor/x402/transports/http.md](../../../services/interop/specs/vendor/x402/transports/http.md) | <code>Markdown document: Transport: HTTP</code> |

## fmt-specs-vendor-x402-transports-mcp

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-specs-vendor-x402-transports-mcp</code> | <code>file_format</code> | [services/interop/specs/vendor/x402/transports/mcp.md](../../../services/interop/specs/vendor/x402/transports/mcp.md) | <code>Markdown document: Transport: MCP (Model Context Protocol)</code> |

## fmt-specs-vendor-x402-x402-specification-v2

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-specs-vendor-x402-x402-specification-v2</code> | <code>file_format</code> | [services/interop/specs/vendor/x402/x402-specification-v2.md](../../../services/interop/specs/vendor/x402/x402-specification-v2.md) | <code>Markdown document: X402 Protocol Specification</code> |
