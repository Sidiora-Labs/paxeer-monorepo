# Agent errors contract

Source revision: `2a1033aa2`. This contract freezes the cited interface and acceptance blocks of the specification. Statuses include the specification overrides. A source marked planned has line 0; a positive source line is verified in the pinned revision.

The context and decision tables reproduce the rules linked by interface identifiers. An anchor without a linked context gains no additional storage, gas, activation or pause rule here. Acceptance qualification remains UNRUN until its declared gate succeeds.

## budget-authorization-required

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.err-reason-budget-authorization-required</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/human.rs:1460</code> | <code>reason budget.authorization_required</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.err-reason-budget-authorization-required</code> | notes | Human peer refusal reason (HumanRefusal::reason, services/agent/crates/layerx-agentd/src/human.rs:1447). No in-repo reference outside the defining line at this revision. |

## budget-authorization-unexpected

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.err-reason-budget-authorization-unexpected</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/human.rs:1461</code> | <code>reason budget.authorization_unexpected</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.err-reason-budget-authorization-unexpected</code> | notes | Human peer refusal reason (HumanRefusal::reason, services/agent/crates/layerx-agentd/src/human.rs:1447). No in-repo reference outside the defining line at this revision. |

## budget-codec

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.err-reason-budget-codec</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/human.rs:1455</code> | <code>reason budget.codec</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.err-reason-budget-codec</code> | notes | Human peer refusal reason (HumanRefusal::reason, services/agent/crates/layerx-agentd/src/human.rs:1447). No in-repo reference outside the defining line at this revision. |

## budget-context-mismatch

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.err-reason-budget-context-mismatch</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/human.rs:1459</code> | <code>reason budget.context_mismatch</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.err-reason-budget-context-mismatch</code> | notes | Human peer refusal reason (HumanRefusal::reason, services/agent/crates/layerx-agentd/src/human.rs:1447). No in-repo reference outside the defining line at this revision. |

## budget-daemon-limit-funding

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.err-reason-budget-daemon-limit-funding</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/human.rs:1462</code> | <code>reason budget.daemon_limit_funding</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.err-reason-budget-daemon-limit-funding</code> | notes | Human peer refusal reason (HumanRefusal::reason, services/agent/crates/layerx-agentd/src/human.rs:1447). No in-repo reference outside the defining line at this revision. |

## budget-expired

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.err-reason-budget-expired</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/human.rs:1456</code> | <code>reason budget.expired</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.err-reason-budget-expired</code> | notes | Human peer refusal reason (HumanRefusal::reason, services/agent/crates/layerx-agentd/src/human.rs:1447). No in-repo reference outside the defining line at this revision. |

## budget-limit-collision

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.err-reason-budget-limit-collision</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/human.rs:1464</code> | <code>reason budget.limit_collision</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.err-reason-budget-limit-collision</code> | notes | Human peer refusal reason (HumanRefusal::reason, services/agent/crates/layerx-agentd/src/human.rs:1447). No in-repo reference outside the defining line at this revision. |

## budget-limit-conflict

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.err-reason-budget-limit-conflict</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/human.rs:1466</code> | <code>reason budget.limit_conflict</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.err-reason-budget-limit-conflict</code> | notes | Human peer refusal reason (HumanRefusal::reason, services/agent/crates/layerx-agentd/src/human.rs:1447). No in-repo reference outside the defining line at this revision. |

## budget-limit-exceeded

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.err-reason-budget-limit-exceeded</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/human.rs:1463</code> | <code>reason budget.limit_exceeded</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.err-reason-budget-limit-exceeded</code> | notes | Human peer refusal reason (HumanRefusal::reason, services/agent/crates/layerx-agentd/src/human.rs:1447). No in-repo reference outside the defining line at this revision. |

## budget-limit-revoked

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.err-reason-budget-limit-revoked</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/human.rs:1465</code> | <code>reason budget.limit_revoked</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.err-reason-budget-limit-revoked</code> | notes | Human peer refusal reason (HumanRefusal::reason, services/agent/crates/layerx-agentd/src/human.rs:1447). No in-repo reference outside the defining line at this revision. |

## budget-not-found

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.err-reason-budget-not-found</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/schema/agent-api/v1.kvx:669</code> | <code>reason budget.not_found: BudgetRefusal Terminal. The authenticated budget lookup names no budget of this tenant and agent.</code> |

## budget-not-live

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.err-reason-budget-not-live</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/human.rs:1458</code> | <code>reason budget.not_live</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.err-reason-budget-not-live</code> | notes | Human peer refusal reason (HumanRefusal::reason, services/agent/crates/layerx-agentd/src/human.rs:1447). No in-repo reference outside the defining line at this revision. |

## budget-stale-revocation

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.err-reason-budget-stale-revocation</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/human.rs:1457</code> | <code>reason budget.stale_revocation</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.err-reason-budget-stale-revocation</code> | notes | Human peer refusal reason (HumanRefusal::reason, services/agent/crates/layerx-agentd/src/human.rs:1447). No in-repo reference outside the defining line at this revision. |

## capability-activity-type

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.err-reason-capability-activity-type</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/schema/agent-api/v1.kvx:671</code> | <code>reason capability.activity_type: CapabilityRefusal Terminal. Owner refused the typed operation: the capability does not admit the activity type.</code> |

## capability-amount

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.err-reason-capability-amount</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/schema/agent-api/v1.kvx:674</code> | <code>reason capability.amount: CapabilityRefusal Terminal. Owner refused the typed operation: the amount exceeds the capability ceiling.</code> |

## capability-asset

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.err-reason-capability-asset</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/schema/agent-api/v1.kvx:673</code> | <code>reason capability.asset: CapabilityRefusal Terminal. Owner refused the typed operation: the capability does not admit the asset.</code> |

## capability-counterparty

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.err-reason-capability-counterparty</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/schema/agent-api/v1.kvx:672</code> | <code>reason capability.counterparty: CapabilityRefusal Terminal. Owner refused the typed operation: the capability does not admit the counterparty.</code> |

## capability-expiry

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.err-reason-capability-expiry</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/schema/agent-api/v1.kvx:670</code> | <code>reason capability.expiry: CapabilityRefusal Terminal. Owner refused the typed operation: core protocol time is at or past the capability not_after.</code> |

## capability-purpose

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.err-reason-capability-purpose</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/schema/agent-api/v1.kvx:676</code> | <code>reason capability.purpose: CapabilityRefusal Terminal. Owner refused the typed operation: the capability does not admit the purpose.</code> |

## capability-rate

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.err-reason-capability-rate</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/schema/agent-api/v1.kvx:675</code> | <code>reason capability.rate: CapabilityRefusal Terminal. Owner refused the typed operation: the capability rate window is exhausted.</code> |

## envelope-coordinate-mismatch

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.err-reason-envelope-coordinate-mismatch</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/schema/agent-api/v1.kvx:651</code> | <code>reason envelope.coordinate_mismatch: PolicyRefusal Terminal. A request tenant or agent coordinate differs from the authenticated principal.</code> |

## envelope-credential

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.err-reason-envelope-credential</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/schema/agent-api/v1.kvx:647</code> | <code>reason envelope.credential: ProtocolIncompatibility Terminal. Credential missing or a credential coordinate is malformed.</code> |

## envelope-header-principal

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.err-reason-envelope-header-principal</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/schema/agent-api/v1.kvx:652</code> | <code>reason envelope.header_principal: PolicyRefusal Terminal. A LayerX-Tenant, LayerX-Agent or Authorization header reached the daemon.</code> |

## envelope-idempotency-key

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.err-reason-envelope-idempotency-key</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/schema/agent-api/v1.kvx:650</code> | <code>reason envelope.idempotency_key: IdempotencyConflict Terminal. Idempotency key presence violates the idempotency rule or the key is malformed.</code> |

## envelope-malformed

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.err-reason-envelope-malformed</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/schema/agent-api/v1.kvx:642</code> | <code>reason envelope.malformed: ProtocolIncompatibility Terminal. Body is not one well-formed envelope object.</code> |

## envelope-noncanonical-integer

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.err-reason-envelope-noncanonical-integer</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/schema/agent-api/v1.kvx:646</code> | <code>reason envelope.noncanonical_integer: ProtocolIncompatibility Terminal. An integer string is not canonical decimal.</code> |

## envelope-oversized

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.err-reason-envelope-oversized</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/schema/agent-api/v1.kvx:641</code> | <code>reason envelope.oversized: ProtocolIncompatibility Terminal. Body above max_body_bytes.</code> |

## envelope-peer-unmapped

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.err-reason-envelope-peer-unmapped</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/schema/agent-api/v1.kvx:653</code> | <code>reason envelope.peer_unmapped: PolicyRefusal Terminal. The authenticated principal maps to no owner peer.</code> |

## envelope-unknown-field

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.err-reason-envelope-unknown-field</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/schema/agent-api/v1.kvx:643</code> | <code>reason envelope.unknown_field: ProtocolIncompatibility Terminal. Envelope or request object carries an unknown key.</code> |

## envelope-unknown-operation

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.err-reason-envelope-unknown-operation</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/schema/agent-api/v1.kvx:645</code> | <code>reason envelope.unknown_operation: ProtocolIncompatibility Terminal. Operation is not a catalogue operation name.</code> |

## envelope-version

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.err-reason-envelope-version</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/schema/agent-api/v1.kvx:644</code> | <code>reason envelope.version: ProtocolIncompatibility Terminal. Envelope version other than 1.</code> |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.9.2.3</code> | req | 9002.3 |
| <code>acceptance.9.2.3</code> | scenario | Given ENVELOPE_VERSION is 1 at services/agent/crates/layerx-agentd/src/agent_rpc.rs:21, when an envelope with version 2 is posted, then the response class is ProtocolIncompatibility with reason envelope.version and HTTP status 400. |
| <code>acceptance.9.2.3</code> | test | services/agent/crates/layerx-agentd/tests/rpc_surface_major_two.rs::envelope_version_other_than_one_is_refused (planned) |
| <code>acceptance.9.2.3</code> | qualification | UNRUN |
| <code>req.9002</code> | ac_3 | An envelope whose version field is 2 is refused with ProtocolIncompatibility reason envelope.version, as declared at services/agent/schema/agent-api/v1.kvx:644. |

## error-class

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.err-class-budgetrefusal</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/schema/agent-api/errors.kvx:10</code> | <code>ErrorClass BudgetRefusal</code> |
| <code>interface.9.err-class-capabilityrefusal</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/schema/agent-api/errors.kvx:10</code> | <code>ErrorClass CapabilityRefusal</code> |
| <code>interface.9.err-class-corerejection</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/schema/agent-api/errors.kvx:10</code> | <code>ErrorClass CoreRejection</code> |
| <code>interface.9.err-class-deadline</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/schema/agent-api/errors.kvx:10</code> | <code>ErrorClass Deadline</code> |
| <code>interface.9.err-class-idempotencyconflict</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/schema/agent-api/errors.kvx:10</code> | <code>ErrorClass IdempotencyConflict</code> |
| <code>interface.9.err-class-internalfault</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/schema/agent-api/errors.kvx:10</code> | <code>ErrorClass InternalFault</code> |
| <code>interface.9.err-class-policyrefusal</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/schema/agent-api/errors.kvx:10</code> | <code>ErrorClass PolicyRefusal</code> |
| <code>interface.9.err-class-protocolincompatibility</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/schema/agent-api/errors.kvx:10</code> | <code>ErrorClass ProtocolIncompatibility</code> |
| <code>interface.9.err-class-ratelimit</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/schema/agent-api/errors.kvx:10</code> | <code>ErrorClass RateLimit</code> |
| <code>interface.9.err-class-transportfailure</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/schema/agent-api/errors.kvx:10</code> | <code>ErrorClass TransportFailure</code> |
| <code>interface.9.err-class-unavailablecapability</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/schema/agent-api/errors.kvx:10</code> | <code>ErrorClass UnavailableCapability</code> |
| <code>interface.9.err-class-verificationfailure</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/schema/agent-api/errors.kvx:10</code> | <code>ErrorClass VerificationFailure</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.err-class-budgetrefusal</code> | notes | Rust mirror enum at services/agent/crates/layerx-agent-api/src/error.rs:40. |
| <code>interface.9.err-class-capabilityrefusal</code> | notes | Rust mirror enum at services/agent/crates/layerx-agent-api/src/error.rs:40. |
| <code>interface.9.err-class-corerejection</code> | notes | Rust mirror enum at services/agent/crates/layerx-agent-api/src/error.rs:40. |
| <code>interface.9.err-class-deadline</code> | notes | Rust mirror enum at services/agent/crates/layerx-agent-api/src/error.rs:40. |
| <code>interface.9.err-class-idempotencyconflict</code> | notes | Rust mirror enum at services/agent/crates/layerx-agent-api/src/error.rs:40. |
| <code>interface.9.err-class-internalfault</code> | notes | Rust mirror enum at services/agent/crates/layerx-agent-api/src/error.rs:40. |
| <code>interface.9.err-class-policyrefusal</code> | notes | Rust mirror enum at services/agent/crates/layerx-agent-api/src/error.rs:40. Callers list shows 58 of 63 caller files. |
| <code>interface.9.err-class-protocolincompatibility</code> | notes | Rust mirror enum at services/agent/crates/layerx-agent-api/src/error.rs:40. Callers list shows 58 of 60 caller files. |
| <code>interface.9.err-class-ratelimit</code> | notes | Rust mirror enum at services/agent/crates/layerx-agent-api/src/error.rs:40. |
| <code>interface.9.err-class-transportfailure</code> | notes | Rust mirror enum at services/agent/crates/layerx-agent-api/src/error.rs:40. |
| <code>interface.9.err-class-unavailablecapability</code> | notes | Rust mirror enum at services/agent/crates/layerx-agent-api/src/error.rs:40. A required node capability was not in the negotiated intersection. The layer does not emulate it. |
| <code>interface.9.err-class-verificationfailure</code> | notes | Rust mirror enum at services/agent/crates/layerx-agent-api/src/error.rs:40. |

## export-level-unattainable

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.err-reason-export-level-unattainable</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/human.rs:1452</code> | <code>reason export.level_unattainable</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.err-reason-export-level-unattainable</code> | notes | No in-repo reference outside the defining line at this revision. |

## export-response-too-large

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.err-reason-export-response-too-large</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/human.rs:1451</code> | <code>reason export.response_too_large</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.err-reason-export-response-too-large</code> | notes | No in-repo reference outside the defining line at this revision. |

## export-rung-unattainable

No interface block names this anchor; the citing criteria and rules below define its scope.

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.9.48.1</code> | req | 9048.1 |
| <code>acceptance.9.48.1</code> | scenario | Given a human export request naming a rung above committed, when it is served, then the refusal code is export.rung_unattainable. |
| <code>acceptance.9.48.1</code> | test | services/agent/crates/layerx-agentd/tests/human_peer_retired.rs::export_above_committed_is_rung_unattainable (planned) |
| <code>acceptance.9.48.1</code> | qualification | UNRUN |
| <code>req.9048</code> | ac_1 | An export request above Committed is refused with export.rung_unattainable and no settlement anchoring code exists. |
| <code>capability.9.13.3</code> | title | Retire the settlement anchoring refusal from the human peer |
| <code>capability.9.13.3</code> | reqs | 9048.1 |

## export-settlement-anchoring-unavailable

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.err-reason-export-settlement-anchoring-unavailable</code> | <code>error_code</code> | <code>retire</code> | <code>services/agent/crates/layerx-agentd/src/human.rs:1453</code> | <code>reason export.settlement_anchoring_unavailable</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.err-reason-export-settlement-anchoring-unavailable</code> | notes | Stale task 6.6.12 retires the settlement domain this refusal reports. |

## gateway-authentication

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.err-reason-gateway-authentication</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/crates/layerx-mcp/src/stdio.rs:649</code> | <code>reason gateway.authentication</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.err-reason-gateway-authentication</code> | notes | No in-repo reference outside the defining line at this revision. |

## gateway-invalid

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.err-reason-gateway-invalid</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/agent_rpc_tls.rs:154</code> | <code>reason gateway.invalid</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.err-reason-gateway-invalid</code> | notes | No in-repo reference outside the defining line at this revision. |

## idempotency-body-changed

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.err-reason-idempotency-body-changed</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/schema/agent-api/v1.kvx:656</code> | <code>reason idempotency.body_changed: IdempotencyConflict Terminal. Same idempotency key with a different body digest.</code> |

## idempotency-unsupported-operation

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.err-reason-idempotency-unsupported-operation</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/schema/agent-api/v1.kvx:655</code> | <code>reason idempotency.unsupported_operation: UnavailableCapability Terminal. A keyed mutation without defined canonical request bytes.</code> |

## idempotent-replay

No interface block names this anchor; the citing criteria and rules below define its scope.

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.9.15.2</code> | req | 9015.2 |
| <code>acceptance.9.15.2</code> | scenario | Given a key whose earlier attempt the node reports through resultByIdempotency, when execute is called again with different request bytes, then it returns protocol_result_code -302 with reason idempotent_replay. |
| <code>acceptance.9.15.2</code> | test | services/agent/crates/layerx-agentd/tests/idempotency_height.rs::replay_settled_on_the_node_maps_to_idempotent_replay (planned) |
| <code>acceptance.9.15.2</code> | qualification | UNRUN |
| <code>req.9015</code> | ac_2 | A replay whose transaction the node already settled under the same idempotency key yields the LXP_IDEMPOTENT_REPLAY refusal (services/agent/schema/agent-api/errors.kvx:16). |
| <code>acceptance.9.6.3</code> | req | 9006.3 |
| <code>acceptance.9.6.3</code> | scenario | Given request id 9, when ApiError::idempotent_replay is constructed, then class is CoreRejection, protocol_result_code is -302, retriability is Terminal and reason is idempotent_replay. |
| <code>acceptance.9.6.3</code> | test | services/agent/crates/layerx-agent-api/tests/identity_rungs.rs::idempotent_replay_is_a_terminal_core_rejection (planned) |
| <code>acceptance.9.6.3</code> | qualification | UNRUN |
| <code>req.9006</code> | ac_3 | ApiError::idempotent_replay yields CoreRejection, protocol_result_code -302, Terminal and reason idempotent_replay as declared at services/agent/schema/agent-api/errors.kvx:16-21, distinct from IdempotencyConflict. |

## lxp-idempotent-replay

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.err-lxp-idempotent-replay</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/schema/agent-api/errors.kvx:16</code> | <code>error LXP_IDEMPOTENT_REPLAY class CoreRejection protocol_result_code -302 Terminal reason idempotent_replay</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.err-lxp-idempotent-replay</code> | notes | Individual LXP result codes are owned by the protocol result table (services/agent/crates/layerx-types/src/result.rs:130). |

## mcp-awaiting-approval

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.err-reason-mcp-awaiting-approval</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/main.rs:1258</code> | <code>reason mcp.awaiting_approval</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.err-reason-mcp-awaiting-approval</code> | notes | No in-repo reference outside the defining line at this revision. |

## mcp-description-invalid

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.err-reason-mcp-description-invalid</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/crates/layerx-mcp/src/stdio.rs:590</code> | <code>reason mcp.description_invalid</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.err-reason-mcp-description-invalid</code> | notes | No in-repo reference outside the defining line at this revision. |

## mcp-exchange-invalid

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.err-reason-mcp-exchange-invalid</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/crates/layerx-mcp/src/stdio.rs:654</code> | <code>reason mcp.exchange_invalid</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.err-reason-mcp-exchange-invalid</code> | notes | No in-repo reference outside the defining line at this revision. |

## mcp-external-signature

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.err-reason-mcp-external-signature</code> | <code>error_code</code> | <code>change</code> | <code>services/agent/crates/layerx-mcp/src/binding.rs:413</code> | <code>reason mcp.external_signature</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.err-reason-mcp-external-signature</code> | notes | Stale task 6.5.14 moves MCP activity tools to 65-byte external signatures. No in-repo reference outside the defining line at this revision. |

## mcp-idempotency-key-conflict

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.err-reason-mcp-idempotency-key-conflict</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/crates/layerx-mcp/src/stdio.rs:615</code> | <code>reason mcp.idempotency_key_conflict</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.err-reason-mcp-idempotency-key-conflict</code> | notes | No in-repo reference outside the defining line at this revision. |

## mcp-idempotency-key-required

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.err-reason-mcp-idempotency-key-required</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/crates/layerx-mcp/src/stdio.rs:625</code> | <code>reason mcp.idempotency_key_required</code> |

## mcp-invocation-conflict

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.err-reason-mcp-invocation-conflict</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/main.rs:1204</code> | <code>reason mcp.invocation_conflict</code> |

## mcp-invocation-unavailable

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.err-reason-mcp-invocation-unavailable</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/main.rs:1206</code> | <code>reason mcp.invocation_unavailable</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.err-reason-mcp-invocation-unavailable</code> | notes | No in-repo reference outside the defining line at this revision. |

## mcp-malformed

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.err-reason-mcp-malformed</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/main.rs:1270</code> | <code>reason mcp.malformed</code> |

## mcp-native-intent-mismatch

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.err-reason-mcp-native-intent-mismatch</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/crates/layerx-mcp/src/tools/write.rs:556</code> | <code>reason mcp.native_intent_mismatch</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.err-reason-mcp-native-intent-mismatch</code> | notes | No in-repo reference outside the defining line at this revision. |

## mcp-native-owner-environment

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.err-reason-mcp-native-owner-environment</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/crates/layerx-mcp/src/binding.rs:403</code> | <code>reason mcp.native_owner_environment</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.err-reason-mcp-native-owner-environment</code> | notes | No in-repo reference outside the defining line at this revision. |

## mcp-native-submit-observation

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.err-reason-mcp-native-submit-observation</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/crates/layerx-mcp/src/tools/write.rs:544</code> | <code>reason mcp.native_submit_observation</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.err-reason-mcp-native-submit-observation</code> | notes | No in-repo reference outside the defining line at this revision. |

## mcp-native-write-binding

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.err-reason-mcp-native-write-binding</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/crates/layerx-mcp/src/tools/write.rs:406</code> | <code>reason mcp.native_write_binding</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.err-reason-mcp-native-write-binding</code> | notes | No in-repo reference outside the defining line at this revision. |

## mcp-operation-refused

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.err-reason-mcp-operation-refused</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/agent_rpc_dispatch.rs:1365</code> | <code>reason mcp.operation_refused</code> |

## mcp-owner-unavailable

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.err-reason-mcp-owner-unavailable</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/main.rs:1425</code> | <code>reason mcp.owner_unavailable</code> |

## mcp-request-counter-exhausted

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.err-reason-mcp-request-counter-exhausted</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/crates/layerx-mcp/src/stdio.rs:367</code> | <code>reason mcp.request_counter_exhausted</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.err-reason-mcp-request-counter-exhausted</code> | notes | No in-repo reference outside the defining line at this revision. |

## mcp-request-entropy-unavailable

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.err-reason-mcp-request-entropy-unavailable</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/crates/layerx-mcp/src/stdio.rs:597</code> | <code>reason mcp.request_entropy_unavailable</code> |

## mcp-tool-absent

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.err-reason-mcp-tool-absent</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/main.rs:1471</code> | <code>reason mcp.tool_absent</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.err-reason-mcp-tool-absent</code> | notes | No in-repo reference outside the defining line at this revision. |

## mcp-tool-not-served

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.err-reason-mcp-tool-not-served</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/main.rs:1264</code> | <code>reason mcp.tool_not_served</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.err-reason-mcp-tool-not-served</code> | notes | No in-repo reference outside the defining line at this revision. |

## mcp-transport-unavailable

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.err-reason-mcp-transport-unavailable</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/crates/layerx-mcp/src/stdio.rs:652</code> | <code>reason mcp.transport_unavailable</code> |

## native-activity-length

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.err-reason-native-activity-length</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/crates/layerx-agent-api/src/identity.rs:201</code> | <code>reason native_activity.length</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.err-reason-native-activity-length</code> | notes | ContractError field code of the native request decoders in layerx-agent-api identity.rs. No in-repo reference outside the defining line at this revision. |

## native-activity-module

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.err-reason-native-activity-module</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/crates/layerx-agent-api/src/identity.rs:186</code> | <code>reason native_activity.module</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.err-reason-native-activity-module</code> | notes | ContractError field code of the native request decoders in layerx-agent-api identity.rs. |

## native-activity-ordinal

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.err-reason-native-activity-ordinal</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/crates/layerx-agent-api/src/identity.rs:188</code> | <code>reason native_activity.ordinal</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.err-reason-native-activity-ordinal</code> | notes | ContractError field code of the native request decoders in layerx-agent-api identity.rs. |

## native-activity-version

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.err-reason-native-activity-version</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/crates/layerx-agent-api/src/identity.rs:203</code> | <code>reason native_activity.version</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.err-reason-native-activity-version</code> | notes | ContractError field code of the native request decoders in layerx-agent-api identity.rs. No in-repo reference outside the defining line at this revision. |

## native-effect-prepare-module

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.err-reason-native-effect-prepare-module</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/crates/layerx-agent-api/src/identity.rs:651</code> | <code>reason native_effect_prepare.module</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.err-reason-native-effect-prepare-module</code> | notes | ContractError field code of the native request decoders in layerx-agent-api identity.rs. No in-repo reference outside the defining line at this revision. |

## native-prepare-module

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.err-reason-native-prepare-module</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/crates/layerx-agent-api/src/identity.rs:595</code> | <code>reason native_prepare.module</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.err-reason-native-prepare-module</code> | notes | ContractError field code of the native request decoders in layerx-agent-api identity.rs. No in-repo reference outside the defining line at this revision. |

## native-purpose-canonical-bytes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.err-reason-native-purpose-canonical-bytes</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/crates/layerx-agent-api/src/identity.rs:292</code> | <code>reason native_purpose.canonical_bytes</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.err-reason-native-purpose-canonical-bytes</code> | notes | ContractError field code of the native request decoders in layerx-agent-api identity.rs. No in-repo reference outside the defining line at this revision. |

## native-purpose-domain

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.err-reason-native-purpose-domain</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/crates/layerx-agent-api/src/identity.rs:264</code> | <code>reason native_purpose.domain</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.err-reason-native-purpose-domain</code> | notes | ContractError field code of the native request decoders in layerx-agent-api identity.rs. No in-repo reference outside the defining line at this revision. |

## native-purpose-length

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.err-reason-native-purpose-length</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/crates/layerx-agent-api/src/identity.rs:275</code> | <code>reason native_purpose.length</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.err-reason-native-purpose-length</code> | notes | ContractError field code of the native request decoders in layerx-agent-api identity.rs. |

## native-purpose-version

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.err-reason-native-purpose-version</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/crates/layerx-agent-api/src/identity.rs:267</code> | <code>reason native_purpose.version</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.err-reason-native-purpose-version</code> | notes | ContractError field code of the native request decoders in layerx-agent-api identity.rs. |

## native-registration-prepare-activity

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.err-reason-native-registration-prepare-activity</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/crates/layerx-agent-api/src/identity.rs:697</code> | <code>reason native_registration_prepare.activity</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.err-reason-native-registration-prepare-activity</code> | notes | ContractError field code of the native request decoders in layerx-agent-api identity.rs. No in-repo reference outside the defining line at this revision. |

## native-send-canonical-bytes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.err-reason-native-send-canonical-bytes</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/crates/layerx-agent-api/src/identity.rs:521</code> | <code>reason native_send.canonical_bytes</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.err-reason-native-send-canonical-bytes</code> | notes | ContractError field code of the native request decoders in layerx-agent-api identity.rs. No in-repo reference outside the defining line at this revision. |

## native-send-did

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.err-reason-native-send-did</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/crates/layerx-agent-api/src/identity.rs:410</code> | <code>reason native_send.did</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.err-reason-native-send-did</code> | notes | ContractError field code of the native request decoders in layerx-agent-api identity.rs. |

## native-send-domain

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.err-reason-native-send-domain</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/crates/layerx-agent-api/src/identity.rs:431</code> | <code>reason native_send.domain</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.err-reason-native-send-domain</code> | notes | ContractError field code of the native request decoders in layerx-agent-api identity.rs. |

## native-send-length

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.err-reason-native-send-length</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/crates/layerx-agent-api/src/identity.rs:497</code> | <code>reason native_send.length</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.err-reason-native-send-length</code> | notes | ContractError field code of the native request decoders in layerx-agent-api identity.rs. No in-repo reference outside the defining line at this revision. |

## native-send-lifetime

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.err-reason-native-send-lifetime</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/crates/layerx-agent-api/src/identity.rs:425</code> | <code>reason native_send.lifetime</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.err-reason-native-send-lifetime</code> | notes | ContractError field code of the native request decoders in layerx-agent-api identity.rs. No in-repo reference outside the defining line at this revision. |

## native-send-owner

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.err-reason-native-send-owner</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/crates/layerx-agent-api/src/identity.rs:420</code> | <code>reason native_send.owner</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.err-reason-native-send-owner</code> | notes | ContractError field code of the native request decoders in layerx-agent-api identity.rs. No in-repo reference outside the defining line at this revision. |

## native-send-prepare-activity

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.err-reason-native-send-prepare-activity</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/crates/layerx-agent-api/src/identity.rs:725</code> | <code>reason native_send_prepare.activity</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.err-reason-native-send-prepare-activity</code> | notes | ContractError field code of the native request decoders in layerx-agent-api identity.rs. No in-repo reference outside the defining line at this revision. |

## native-send-prepare-request

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.err-reason-native-send-prepare-request</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/crates/layerx-agent-api/src/identity.rs:754</code> | <code>reason native_send_prepare.request</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.err-reason-native-send-prepare-request</code> | notes | ContractError field code of the native request decoders in layerx-agent-api identity.rs. No in-repo reference outside the defining line at this revision. |

## native-send-signature-owner

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.err-reason-native-send-signature-owner</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/crates/layerx-agent-api/src/identity.rs:538</code> | <code>reason native_send.signature_owner</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.err-reason-native-send-signature-owner</code> | notes | ContractError field code of the native request decoders in layerx-agent-api identity.rs. No in-repo reference outside the defining line at this revision. |

## native-send-version

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.err-reason-native-send-version</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/crates/layerx-agent-api/src/identity.rs:487</code> | <code>reason native_send.version</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.err-reason-native-send-version</code> | notes | ContractError field code of the native request decoders in layerx-agent-api identity.rs. |

## outcome-unknown

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.err-reason-outcome-unknown</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/schema/agent-api/v1.kvx:657</code> | <code>reason outcome.unknown: TransportFailure Terminal. Mutation outcome is Unknown; never resent automatically.</code> |

## owner-degraded

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.err-reason-owner-degraded</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/schema/agent-api/v1.kvx:663</code> | <code>reason owner.degraded: UnavailableCapability Retriable. Owner is not Healthy.</code> |

## owner-refused

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.err-reason-owner-refused</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/schema/agent-api/v1.kvx:665</code> | <code>reason owner.refused: PolicyRefusal Terminal. Owner refused the typed operation.</code> |

## owner-response-malformed

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.err-reason-owner-response-malformed</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/schema/agent-api/v1.kvx:677</code> | <code>reason owner.response_malformed: InternalFault Terminal. Owner response did not decode.</code> |

## owner-unavailable

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.err-reason-owner-unavailable</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/schema/agent-api/v1.kvx:664</code> | <code>reason owner.unavailable: InternalFault Retriable. Owner unavailable at the daemon; the dispatcher maps an owner Unavailable error to UnavailableCapability Retriable with this reason.</code> |

## policy-capability-store

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.err-reason-policy-capability-store</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/policy/operation.rs:109</code> | <code>reason policy.capability_store</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.err-reason-policy-capability-store</code> | notes | No in-repo reference outside the defining line at this revision. |

## policy-context-mismatch-agent-did

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.err-reason-policy-context-mismatch-agent-did</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/human.rs:1470</code> | <code>reason policy.context_mismatch.agent_did</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.err-reason-policy-context-mismatch-agent-did</code> | notes | Human peer refusal reason (HumanRefusal::reason, services/agent/crates/layerx-agentd/src/human.rs:1447). No in-repo reference outside the defining line at this revision. |

## policy-context-mismatch-authority-ref

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.err-reason-policy-context-mismatch-authority-ref</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/human.rs:1471</code> | <code>reason policy.context_mismatch.authority_ref</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.err-reason-policy-context-mismatch-authority-ref</code> | notes | Human peer refusal reason (HumanRefusal::reason, services/agent/crates/layerx-agentd/src/human.rs:1447). No in-repo reference outside the defining line at this revision. |

## policy-context-mismatch-client

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.err-reason-policy-context-mismatch-client</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/human.rs:1476</code> | <code>reason policy.context_mismatch.client</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.err-reason-policy-context-mismatch-client</code> | notes | Human peer refusal reason (HumanRefusal::reason, services/agent/crates/layerx-agentd/src/human.rs:1447). No in-repo reference outside the defining line at this revision. |

## policy-context-mismatch-expiry

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.err-reason-policy-context-mismatch-expiry</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/human.rs:1475</code> | <code>reason policy.context_mismatch.expiry</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.err-reason-policy-context-mismatch-expiry</code> | notes | Human peer refusal reason (HumanRefusal::reason, services/agent/crates/layerx-agentd/src/human.rs:1447). No in-repo reference outside the defining line at this revision. |

## policy-context-mismatch-permitted-activity-types

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.err-reason-policy-context-mismatch-permitted-activity-types</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/human.rs:1473</code> | <code>reason policy.context_mismatch.permitted_activity_types</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.err-reason-policy-context-mismatch-permitted-activity-types</code> | notes | Human peer refusal reason (HumanRefusal::reason, services/agent/crates/layerx-agentd/src/human.rs:1447). No in-repo reference outside the defining line at this revision. |

## policy-context-mismatch-policy-version

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.err-reason-policy-context-mismatch-policy-version</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/human.rs:1477</code> | <code>reason policy.context_mismatch.policy_version</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.err-reason-policy-context-mismatch-policy-version</code> | notes | Human peer refusal reason (HumanRefusal::reason, services/agent/crates/layerx-agentd/src/human.rs:1447). No in-repo reference outside the defining line at this revision. |

## policy-context-mismatch-tenant

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.err-reason-policy-context-mismatch-tenant</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/human.rs:1469</code> | <code>reason policy.context_mismatch.tenant</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.err-reason-policy-context-mismatch-tenant</code> | notes | Human peer refusal reason (HumanRefusal::reason, services/agent/crates/layerx-agentd/src/human.rs:1447). No in-repo reference outside the defining line at this revision. |

## policy-intent-binding-missing

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.err-reason-policy-intent-binding-missing</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/schema/agent-api/v1.kvx:668</code> | <code>reason policy.intent_binding_missing: PolicyRefusal Terminal. A LegacyPolicyDryRun canonical_intent carries raw canonical unsigned activity bytes whose typed semantics or retained purpose binding does not exist.</code> |

## policy-intent-effect-underivable

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.err-reason-policy-intent-effect-underivable</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/human.rs:1484</code> | <code>reason policy.intent_effect_underivable</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.err-reason-policy-intent-effect-underivable</code> | notes | No in-repo reference outside the defining line at this revision. |

## policy-intent-malformed

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.err-reason-policy-intent-malformed</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/human.rs:1483</code> | <code>reason policy.intent_malformed</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.err-reason-policy-intent-malformed</code> | notes | No in-repo reference outside the defining line at this revision. |

## policy-legacy-project-payload

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.err-reason-policy-legacy-project-payload</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/schema/agent-api/v1.kvx:667</code> | <code>reason policy.legacy_project_payload: PolicyRefusal Terminal. A project request carries the earlier policy dry-run payload; it is never read as a fee request.</code> |

## policy-no-policy

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.err-reason-policy-no-policy</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/policy/operation.rs:106</code> | <code>reason policy.no_policy</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.err-reason-policy-no-policy</code> | notes | No in-repo reference outside the defining line at this revision. |

## policy-purpose-undisclosed

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.err-reason-policy-purpose-undisclosed</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/human.rs:1482</code> | <code>reason policy.purpose_undisclosed</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.err-reason-policy-purpose-undisclosed</code> | notes | No in-repo reference outside the defining line at this revision. |

## policy-session-binding-missing

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.err-reason-policy-session-binding-missing</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/human.rs:1479</code> | <code>reason policy.session_binding_missing</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.err-reason-policy-session-binding-missing</code> | notes | No in-repo reference outside the defining line at this revision. |

## policy-session-expired

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.err-reason-policy-session-expired</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/human.rs:1481</code> | <code>reason policy.session_expired</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.err-reason-policy-session-expired</code> | notes | No in-repo reference outside the defining line at this revision. |

## policy-session-expiry-binding-missing

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.err-reason-policy-session-expiry-binding-missing</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/human.rs:1480</code> | <code>reason policy.session_expiry_binding_missing</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.err-reason-policy-session-expiry-binding-missing</code> | notes | No in-repo reference outside the defining line at this revision. |

## policy-unknown-capability

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.err-reason-policy-unknown-capability</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/policy/operation.rs:108</code> | <code>reason policy.unknown_capability</code> |

## policy-unknown-session

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.err-reason-policy-unknown-session</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/policy/operation.rs:107</code> | <code>reason policy.unknown_session</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.err-reason-policy-unknown-session</code> | notes | No in-repo reference outside the defining line at this revision. |

## program-stale-head

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.err-reason-program-stale-head</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/human.rs:1449</code> | <code>reason program.stale_head</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.err-reason-program-stale-head</code> | notes | No in-repo reference outside the defining line at this revision. |

## projection-fee-snapshot-skew

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.err-reason-projection-fee-snapshot-skew</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/human.rs:1450</code> | <code>reason projection.fee_snapshot_skew</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.err-reason-projection-fee-snapshot-skew</code> | notes | No in-repo reference outside the defining line at this revision. |

## receipt-key-unknown

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.err-receipt-key-unknown</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/schema/agent-api/errors.kvx:23</code> | <code>error receipt_key_unknown class VerificationFailure protocol_result_code null Terminal reason receipt_key_unknown</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.err-receipt-key-unknown</code> | notes | Raised by the sequencer receipt verifiers when the receipt key is not pinned. |

## refused-pending-bootstrap-artifact

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.err-reason-refused-pending-bootstrap-artifact</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/schema/agent-api/v1.kvx:648</code> | <code>reason refused_pending_bootstrap_artifact: PolicyRefusal Terminal. agent.register or session.open before the authenticated bootstrap artifact exists.</code> |

## rotation-unsupported

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.err-reason-rotation-unsupported-j43</code> | <code>error_code</code> | <code>new</code> | <code>services/agent/crates/layerx-agentd/src/managed_agent_rotation.rs</code> (planned) | <code>reason rotation_unsupported</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.err-reason-rotation-unsupported-j43</code> | notes | planned by task 9.2.4 per decision.905 |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.9.13.1</code> | req | 9013.1 |
| <code>acceptance.9.13.1</code> | scenario | Given a managed agent with an active session, when an owner rotation is requested, then the result is a refusal with reason rotation_unsupported and the session record is unchanged. |
| <code>acceptance.9.13.1</code> | test | services/agent/crates/layerx-agentd/tests/rotation_refused.rs::owner_rotation_is_refused_as_unsupported (planned) |
| <code>acceptance.9.13.1</code> | qualification | UNRUN |
| <code>req.9013</code> | ac_1 | A managed agent owner rotation request is refused with reason rotation_unsupported and no state changes (services/agent/crates/layerx-agentd/src/managed_agent_rotation.rs:131). |
| <code>capability.9.2.4</code> | title | Refuse agent key rotation with rotation_unsupported |
| <code>capability.9.2.4</code> | reqs | 9013.1 |
| <code>resolution.17.840</code> | title | decision.905 binds: Managed agent owner rotation is refused after the merge |
| <code>resolution.17.840</code> | applies_to | decision.905<br>task.9.2.4 |
| <code>resolution.17.840</code> | source | decision.905 in fragments/J43/decision.kvx; tasks from affects |

## session-expired

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.err-reason-session-expired</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/schema/agent-api/v1.kvx:659</code> | <code>reason session.expired: PolicyRefusal Terminal. Session expired.</code> |

## session-invalid-request

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.err-reason-session-invalid-request</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/schema/agent-api/v1.kvx:661</code> | <code>reason session.invalid_request: ProtocolIncompatibility Terminal. Session authorization request is invalid.</code> |

## session-not-authorized

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.err-reason-session-not-authorized</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/schema/agent-api/v1.kvx:662</code> | <code>reason session.not_authorized: PolicyRefusal Terminal. Session authority does not authorize the credential.</code> |

## session-revoked

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.err-reason-session-revoked</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/schema/agent-api/v1.kvx:660</code> | <code>reason session.revoked: PolicyRefusal Terminal. Session revoked.</code> |

## session-scope-denied

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.err-reason-session-scope-denied</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/schema/agent-api/v1.kvx:658</code> | <code>reason session.scope_denied: CapabilityRefusal Terminal. Session scope does not admit the operation surface.</code> |

## unavailable-capability-faucet-claim

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.err-reason-unavailable-capability-faucet-claim</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/schema/agent-api/v1.kvx:654</code> | <code>reason unavailable_capability.faucet.claim: UnavailableCapability Terminal. The retired faucet.claim operation.</code> |

## unmatched-by-ruling

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.err-reason-unmatched-by-ruling</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/schema/agent-api/v1.kvx:649</code> | <code>reason unmatched_by_ruling: UnavailableCapability Terminal. Historical program.interface refusal; retained as a reserved compatibility reason.</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.err-reason-unmatched-by-ruling</code> | notes | Declared in the schema error table only; no code emits it at this revision. No in-repo reference outside the defining line at this revision. |

## verification-inconsistent

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.err-reason-verification-inconsistent</code> | <code>error_code</code> | <code>stable</code> | <code>services/agent/schema/agent-api/v1.kvx:678</code> | <code>reason verification.inconsistent: VerificationFailure Terminal. Verification status violates the level ordering.</code> |
