# Agent errors contract

Source paths are relative to the repository root. Declaration tables identify the interfaces in the source files; runtime behavior is defined by their implementations.

Agent applications use the merged Paxeer BFT network, EVM chain ID `125` (`0x7d`). The kernel participates in block proposal and execution. Block commit establishes finality. Node JSON-RPC uses the root path; hosted methods are described by the [gateway contract](../../../services/platform/hosted/gateway/openrpc.json).

## budget-authorization-required

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.err-reason-budget-authorization-required</code> | <code>error_code</code> | <code>services/agent/crates/layerx-agentd/src/human.rs</code> | <code>reason budget.authorization_required</code> |

## budget-authorization-unexpected

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.err-reason-budget-authorization-unexpected</code> | <code>error_code</code> | <code>services/agent/crates/layerx-agentd/src/human.rs</code> | <code>reason budget.authorization_unexpected</code> |

## budget-codec

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.err-reason-budget-codec</code> | <code>error_code</code> | <code>services/agent/crates/layerx-agentd/src/human.rs</code> | <code>reason budget.codec</code> |

## budget-context-mismatch

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.err-reason-budget-context-mismatch</code> | <code>error_code</code> | <code>services/agent/crates/layerx-agentd/src/human.rs</code> | <code>reason budget.context_mismatch</code> |

## budget-daemon-limit-funding

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.err-reason-budget-daemon-limit-funding</code> | <code>error_code</code> | <code>services/agent/crates/layerx-agentd/src/human.rs</code> | <code>reason budget.daemon_limit_funding</code> |

## budget-expired

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.err-reason-budget-expired</code> | <code>error_code</code> | <code>services/agent/crates/layerx-agentd/src/human.rs</code> | <code>reason budget.expired</code> |

## budget-limit-collision

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.err-reason-budget-limit-collision</code> | <code>error_code</code> | <code>services/agent/crates/layerx-agentd/src/human.rs</code> | <code>reason budget.limit_collision</code> |

## budget-limit-conflict

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.err-reason-budget-limit-conflict</code> | <code>error_code</code> | <code>services/agent/crates/layerx-agentd/src/human.rs</code> | <code>reason budget.limit_conflict</code> |

## budget-limit-exceeded

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.err-reason-budget-limit-exceeded</code> | <code>error_code</code> | <code>services/agent/crates/layerx-agentd/src/human.rs</code> | <code>reason budget.limit_exceeded</code> |

## budget-limit-revoked

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.err-reason-budget-limit-revoked</code> | <code>error_code</code> | <code>services/agent/crates/layerx-agentd/src/human.rs</code> | <code>reason budget.limit_revoked</code> |

## budget-not-found

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.err-reason-budget-not-found</code> | <code>error_code</code> | <code>services/agent/schema/agent-api/v1.kvx</code> | <code>reason budget.not_found: BudgetRefusal Terminal. The authenticated budget lookup names no budget of this tenant and agent.</code> |

## budget-not-live

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.err-reason-budget-not-live</code> | <code>error_code</code> | <code>services/agent/crates/layerx-agentd/src/human.rs</code> | <code>reason budget.not_live</code> |

## budget-stale-revocation

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.err-reason-budget-stale-revocation</code> | <code>error_code</code> | <code>services/agent/crates/layerx-agentd/src/human.rs</code> | <code>reason budget.stale_revocation</code> |

## capability-activity-type

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.err-reason-capability-activity-type</code> | <code>error_code</code> | <code>services/agent/schema/agent-api/v1.kvx</code> | <code>reason capability.activity_type: CapabilityRefusal Terminal. Owner refused the typed operation: the capability does not admit the activity type.</code> |

## capability-amount

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.err-reason-capability-amount</code> | <code>error_code</code> | <code>services/agent/schema/agent-api/v1.kvx</code> | <code>reason capability.amount: CapabilityRefusal Terminal. Owner refused the typed operation: the amount exceeds the capability ceiling.</code> |

## capability-asset

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.err-reason-capability-asset</code> | <code>error_code</code> | <code>services/agent/schema/agent-api/v1.kvx</code> | <code>reason capability.asset: CapabilityRefusal Terminal. Owner refused the typed operation: the capability does not admit the asset.</code> |

## capability-counterparty

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.err-reason-capability-counterparty</code> | <code>error_code</code> | <code>services/agent/schema/agent-api/v1.kvx</code> | <code>reason capability.counterparty: CapabilityRefusal Terminal. Owner refused the typed operation: the capability does not admit the counterparty.</code> |

## capability-expiry

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.err-reason-capability-expiry</code> | <code>error_code</code> | <code>services/agent/schema/agent-api/v1.kvx</code> | <code>reason capability.expiry: CapabilityRefusal Terminal. Owner refused the typed operation: core protocol time is at or past the capability not_after.</code> |

## capability-purpose

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.err-reason-capability-purpose</code> | <code>error_code</code> | <code>services/agent/schema/agent-api/v1.kvx</code> | <code>reason capability.purpose: CapabilityRefusal Terminal. Owner refused the typed operation: the capability does not admit the purpose.</code> |

## capability-rate

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.err-reason-capability-rate</code> | <code>error_code</code> | <code>services/agent/schema/agent-api/v1.kvx</code> | <code>reason capability.rate: CapabilityRefusal Terminal. Owner refused the typed operation: the capability rate window is exhausted.</code> |

## envelope-coordinate-mismatch

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.err-reason-envelope-coordinate-mismatch</code> | <code>error_code</code> | <code>services/agent/schema/agent-api/v1.kvx</code> | <code>reason envelope.coordinate_mismatch: PolicyRefusal Terminal. A request tenant or agent coordinate differs from the authenticated principal.</code> |

## envelope-credential

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.err-reason-envelope-credential</code> | <code>error_code</code> | <code>services/agent/schema/agent-api/v1.kvx</code> | <code>reason envelope.credential: ProtocolIncompatibility Terminal. Credential missing or a credential coordinate is malformed.</code> |

## envelope-header-principal

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.err-reason-envelope-header-principal</code> | <code>error_code</code> | <code>services/agent/schema/agent-api/v1.kvx</code> | <code>reason envelope.header_principal: PolicyRefusal Terminal. A LayerX-Tenant, LayerX-Agent or Authorization header reached the daemon.</code> |

## envelope-idempotency-key

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.err-reason-envelope-idempotency-key</code> | <code>error_code</code> | <code>services/agent/schema/agent-api/v1.kvx</code> | <code>reason envelope.idempotency_key: IdempotencyConflict Terminal. Idempotency key presence violates the idempotency rule or the key is malformed.</code> |

## envelope-malformed

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.err-reason-envelope-malformed</code> | <code>error_code</code> | <code>services/agent/schema/agent-api/v1.kvx</code> | <code>reason envelope.malformed: ProtocolIncompatibility Terminal. Body is not one well-formed envelope object.</code> |

## envelope-noncanonical-integer

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.err-reason-envelope-noncanonical-integer</code> | <code>error_code</code> | <code>services/agent/schema/agent-api/v1.kvx</code> | <code>reason envelope.noncanonical_integer: ProtocolIncompatibility Terminal. An integer string is not canonical decimal.</code> |

## envelope-oversized

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.err-reason-envelope-oversized</code> | <code>error_code</code> | <code>services/agent/schema/agent-api/v1.kvx</code> | <code>reason envelope.oversized: ProtocolIncompatibility Terminal. Body above max_body_bytes.</code> |

## envelope-peer-unmapped

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.err-reason-envelope-peer-unmapped</code> | <code>error_code</code> | <code>services/agent/schema/agent-api/v1.kvx</code> | <code>reason envelope.peer_unmapped: PolicyRefusal Terminal. The authenticated principal maps to no owner peer.</code> |

## envelope-unknown-field

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.err-reason-envelope-unknown-field</code> | <code>error_code</code> | <code>services/agent/schema/agent-api/v1.kvx</code> | <code>reason envelope.unknown_field: ProtocolIncompatibility Terminal. Envelope or request object carries an unknown key.</code> |

## envelope-unknown-operation

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.err-reason-envelope-unknown-operation</code> | <code>error_code</code> | <code>services/agent/schema/agent-api/v1.kvx</code> | <code>reason envelope.unknown_operation: ProtocolIncompatibility Terminal. Operation is not a catalogue operation name.</code> |

## envelope-version

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.err-reason-envelope-version</code> | <code>error_code</code> | <code>services/agent/schema/agent-api/v1.kvx</code> | <code>reason envelope.version: ProtocolIncompatibility Terminal. Envelope version other than 1.</code> |

| Requirement | Behavior |
| --- | --- |
| <code>req.9002</code> | An envelope whose version field is 2 is refused with ProtocolIncompatibility reason envelope.version, as declared at services/agent/schema/agent-api/v1.kvx. |

## error-class

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.err-class-budgetrefusal</code> | <code>error_code</code> | <code>services/agent/schema/agent-api/errors.kvx</code> | <code>ErrorClass BudgetRefusal</code> |
| <code>interface.9.err-class-capabilityrefusal</code> | <code>error_code</code> | <code>services/agent/schema/agent-api/errors.kvx</code> | <code>ErrorClass CapabilityRefusal</code> |
| <code>interface.9.err-class-corerejection</code> | <code>error_code</code> | <code>services/agent/schema/agent-api/errors.kvx</code> | <code>ErrorClass CoreRejection</code> |
| <code>interface.9.err-class-deadline</code> | <code>error_code</code> | <code>services/agent/schema/agent-api/errors.kvx</code> | <code>ErrorClass Deadline</code> |
| <code>interface.9.err-class-idempotencyconflict</code> | <code>error_code</code> | <code>services/agent/schema/agent-api/errors.kvx</code> | <code>ErrorClass IdempotencyConflict</code> |
| <code>interface.9.err-class-internalfault</code> | <code>error_code</code> | <code>services/agent/schema/agent-api/errors.kvx</code> | <code>ErrorClass InternalFault</code> |
| <code>interface.9.err-class-policyrefusal</code> | <code>error_code</code> | <code>services/agent/schema/agent-api/errors.kvx</code> | <code>ErrorClass PolicyRefusal</code> |
| <code>interface.9.err-class-protocolincompatibility</code> | <code>error_code</code> | <code>services/agent/schema/agent-api/errors.kvx</code> | <code>ErrorClass ProtocolIncompatibility</code> |
| <code>interface.9.err-class-ratelimit</code> | <code>error_code</code> | <code>services/agent/schema/agent-api/errors.kvx</code> | <code>ErrorClass RateLimit</code> |
| <code>interface.9.err-class-transportfailure</code> | <code>error_code</code> | <code>services/agent/schema/agent-api/errors.kvx</code> | <code>ErrorClass TransportFailure</code> |
| <code>interface.9.err-class-unavailablecapability</code> | <code>error_code</code> | <code>services/agent/schema/agent-api/errors.kvx</code> | <code>ErrorClass UnavailableCapability</code> |
| <code>interface.9.err-class-verificationfailure</code> | <code>error_code</code> | <code>services/agent/schema/agent-api/errors.kvx</code> | <code>ErrorClass VerificationFailure</code> |

<a id="export-level-unattainable"></a>

## export-response-too-large

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.err-reason-export-response-too-large</code> | <code>error_code</code> | <code>services/agent/crates/layerx-agentd/src/human.rs</code> | <code>reason export.response_too_large</code> |

## export-rung-unattainable

| Requirement | Behavior |
| --- | --- |
| <code>req.9048</code> | An export request above Committed is refused with export.rung_unattainable and no settlement anchoring code exists. |

<a id="export-settlement-anchoring-unavailable"></a>

## gateway-authentication

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.err-reason-gateway-authentication</code> | <code>error_code</code> | <code>services/agent/crates/layerx-mcp/src/stdio.rs</code> | <code>reason gateway.authentication</code> |

## gateway-invalid

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.err-reason-gateway-invalid</code> | <code>error_code</code> | <code>services/agent/crates/layerx-agentd/src/agent_rpc_tls.rs</code> | <code>reason gateway.invalid</code> |

## idempotency-body-changed

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.err-reason-idempotency-body-changed</code> | <code>error_code</code> | <code>services/agent/schema/agent-api/v1.kvx</code> | <code>reason idempotency.body_changed: IdempotencyConflict Terminal. Same idempotency key with a different body digest.</code> |

## idempotency-unsupported-operation

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.err-reason-idempotency-unsupported-operation</code> | <code>error_code</code> | <code>services/agent/schema/agent-api/v1.kvx</code> | <code>reason idempotency.unsupported_operation: UnavailableCapability Terminal. A keyed mutation without defined canonical request bytes.</code> |

## idempotent-replay

| Requirement | Behavior |
| --- | --- |
| <code>req.9015</code> | A replay whose transaction the node already settled under the same idempotency key yields the LXP_IDEMPOTENT_REPLAY refusal (services/agent/schema/agent-api/errors.kvx). |
| <code>req.9006</code> | ApiError::idempotent_replay yields CoreRejection, protocol_result_code -302, Terminal and reason idempotent_replay as declared at services/agent/schema/agent-api/errors.kvx, distinct from IdempotencyConflict. |

## lxp-idempotent-replay

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.err-lxp-idempotent-replay</code> | <code>error_code</code> | <code>services/agent/schema/agent-api/errors.kvx</code> | <code>error LXP_IDEMPOTENT_REPLAY class CoreRejection protocol_result_code -302 Terminal reason idempotent_replay</code> |

## mcp-awaiting-approval

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.err-reason-mcp-awaiting-approval</code> | <code>error_code</code> | <code>services/agent/crates/layerx-agentd/src/main.rs</code> | <code>reason mcp.awaiting_approval</code> |

## mcp-description-invalid

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.err-reason-mcp-description-invalid</code> | <code>error_code</code> | <code>services/agent/crates/layerx-mcp/src/stdio.rs</code> | <code>reason mcp.description_invalid</code> |

## mcp-exchange-invalid

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.err-reason-mcp-exchange-invalid</code> | <code>error_code</code> | <code>services/agent/crates/layerx-mcp/src/stdio.rs</code> | <code>reason mcp.exchange_invalid</code> |

## mcp-external-signature

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.err-reason-mcp-external-signature</code> | <code>error_code</code> | <code>services/agent/crates/layerx-mcp/src/binding.rs</code> | <code>reason mcp.external_signature</code> |

## mcp-idempotency-key-conflict

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.err-reason-mcp-idempotency-key-conflict</code> | <code>error_code</code> | <code>services/agent/crates/layerx-mcp/src/stdio.rs</code> | <code>reason mcp.idempotency_key_conflict</code> |

## mcp-idempotency-key-required

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.err-reason-mcp-idempotency-key-required</code> | <code>error_code</code> | <code>services/agent/crates/layerx-mcp/src/stdio.rs</code> | <code>reason mcp.idempotency_key_required</code> |

## mcp-invocation-conflict

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.err-reason-mcp-invocation-conflict</code> | <code>error_code</code> | <code>services/agent/crates/layerx-agentd/src/main.rs</code> | <code>reason mcp.invocation_conflict</code> |

## mcp-invocation-unavailable

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.err-reason-mcp-invocation-unavailable</code> | <code>error_code</code> | <code>services/agent/crates/layerx-agentd/src/main.rs</code> | <code>reason mcp.invocation_unavailable</code> |

## mcp-malformed

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.err-reason-mcp-malformed</code> | <code>error_code</code> | <code>services/agent/crates/layerx-agentd/src/main.rs</code> | <code>reason mcp.malformed</code> |

## mcp-native-intent-mismatch

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.err-reason-mcp-native-intent-mismatch</code> | <code>error_code</code> | <code>services/agent/crates/layerx-mcp/src/tools/write.rs</code> | <code>reason mcp.native_intent_mismatch</code> |

## mcp-native-owner-environment

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.err-reason-mcp-native-owner-environment</code> | <code>error_code</code> | <code>services/agent/crates/layerx-mcp/src/binding.rs</code> | <code>reason mcp.native_owner_environment</code> |

## mcp-native-submit-observation

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.err-reason-mcp-native-submit-observation</code> | <code>error_code</code> | <code>services/agent/crates/layerx-mcp/src/tools/write.rs</code> | <code>reason mcp.native_submit_observation</code> |

## mcp-native-write-binding

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.err-reason-mcp-native-write-binding</code> | <code>error_code</code> | <code>services/agent/crates/layerx-mcp/src/tools/write.rs</code> | <code>reason mcp.native_write_binding</code> |

## mcp-operation-refused

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.err-reason-mcp-operation-refused</code> | <code>error_code</code> | <code>services/agent/crates/layerx-agentd/src/agent_rpc_dispatch.rs</code> | <code>reason mcp.operation_refused</code> |

## mcp-owner-unavailable

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.err-reason-mcp-owner-unavailable</code> | <code>error_code</code> | <code>services/agent/crates/layerx-agentd/src/main.rs</code> | <code>reason mcp.owner_unavailable</code> |

## mcp-request-counter-exhausted

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.err-reason-mcp-request-counter-exhausted</code> | <code>error_code</code> | <code>services/agent/crates/layerx-mcp/src/stdio.rs</code> | <code>reason mcp.request_counter_exhausted</code> |

## mcp-request-entropy-unavailable

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.err-reason-mcp-request-entropy-unavailable</code> | <code>error_code</code> | <code>services/agent/crates/layerx-mcp/src/stdio.rs</code> | <code>reason mcp.request_entropy_unavailable</code> |

## mcp-tool-absent

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.err-reason-mcp-tool-absent</code> | <code>error_code</code> | <code>services/agent/crates/layerx-agentd/src/main.rs</code> | <code>reason mcp.tool_absent</code> |

## mcp-tool-not-served

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.err-reason-mcp-tool-not-served</code> | <code>error_code</code> | <code>services/agent/crates/layerx-agentd/src/main.rs</code> | <code>reason mcp.tool_not_served</code> |

## mcp-transport-unavailable

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.err-reason-mcp-transport-unavailable</code> | <code>error_code</code> | <code>services/agent/crates/layerx-mcp/src/stdio.rs</code> | <code>reason mcp.transport_unavailable</code> |

## native-activity-length

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.err-reason-native-activity-length</code> | <code>error_code</code> | <code>services/agent/crates/layerx-agent-api/src/identity.rs</code> | <code>reason native_activity.length</code> |

## native-activity-module

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.err-reason-native-activity-module</code> | <code>error_code</code> | <code>services/agent/crates/layerx-agent-api/src/identity.rs</code> | <code>reason native_activity.module</code> |

## native-activity-ordinal

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.err-reason-native-activity-ordinal</code> | <code>error_code</code> | <code>services/agent/crates/layerx-agent-api/src/identity.rs</code> | <code>reason native_activity.ordinal</code> |

## native-activity-version

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.err-reason-native-activity-version</code> | <code>error_code</code> | <code>services/agent/crates/layerx-agent-api/src/identity.rs</code> | <code>reason native_activity.version</code> |

## native-effect-prepare-module

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.err-reason-native-effect-prepare-module</code> | <code>error_code</code> | <code>services/agent/crates/layerx-agent-api/src/identity.rs</code> | <code>reason native_effect_prepare.module</code> |

## native-prepare-module

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.err-reason-native-prepare-module</code> | <code>error_code</code> | <code>services/agent/crates/layerx-agent-api/src/identity.rs</code> | <code>reason native_prepare.module</code> |

## native-purpose-canonical-bytes

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.err-reason-native-purpose-canonical-bytes</code> | <code>error_code</code> | <code>services/agent/crates/layerx-agent-api/src/identity.rs</code> | <code>reason native_purpose.canonical_bytes</code> |

## native-purpose-domain

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.err-reason-native-purpose-domain</code> | <code>error_code</code> | <code>services/agent/crates/layerx-agent-api/src/identity.rs</code> | <code>reason native_purpose.domain</code> |

## native-purpose-length

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.err-reason-native-purpose-length</code> | <code>error_code</code> | <code>services/agent/crates/layerx-agent-api/src/identity.rs</code> | <code>reason native_purpose.length</code> |

## native-purpose-version

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.err-reason-native-purpose-version</code> | <code>error_code</code> | <code>services/agent/crates/layerx-agent-api/src/identity.rs</code> | <code>reason native_purpose.version</code> |

## native-registration-prepare-activity

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.err-reason-native-registration-prepare-activity</code> | <code>error_code</code> | <code>services/agent/crates/layerx-agent-api/src/identity.rs</code> | <code>reason native_registration_prepare.activity</code> |

## native-send-canonical-bytes

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.err-reason-native-send-canonical-bytes</code> | <code>error_code</code> | <code>services/agent/crates/layerx-agent-api/src/identity.rs</code> | <code>reason native_send.canonical_bytes</code> |

## native-send-did

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.err-reason-native-send-did</code> | <code>error_code</code> | <code>services/agent/crates/layerx-agent-api/src/native_send.rs</code> | <code>reason native_send.did</code> |

## native-send-domain

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.err-reason-native-send-domain</code> | <code>error_code</code> | <code>services/agent/crates/layerx-agent-api/src/identity.rs</code> | <code>reason native_send.domain</code> |

## native-send-length

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.err-reason-native-send-length</code> | <code>error_code</code> | <code>services/agent/crates/layerx-agent-api/src/identity.rs</code> | <code>reason native_send.length</code> |

## native-send-lifetime

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.err-reason-native-send-lifetime</code> | <code>error_code</code> | <code>services/agent/crates/layerx-agent-api/src/native_send.rs</code> | <code>reason native_send.lifetime</code> |

<a id="native-send-owner"></a>

## native-send-prepare-activity

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.err-reason-native-send-prepare-activity</code> | <code>error_code</code> | <code>services/agent/crates/layerx-agent-api/src/identity.rs</code> | <code>reason native_send_prepare.activity</code> |

## native-send-prepare-request

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.err-reason-native-send-prepare-request</code> | <code>error_code</code> | <code>services/agent/crates/layerx-agent-api/src/identity.rs</code> | <code>reason native_send_prepare.request</code> |

## native-send-signature-owner

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.err-reason-native-send-signature-owner</code> | <code>error_code</code> | <code>services/agent/crates/layerx-agent-api/src/identity.rs</code> | <code>reason native_send.signature_owner</code> |

## native-send-version

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.err-reason-native-send-version</code> | <code>error_code</code> | <code>services/agent/crates/layerx-agent-api/src/identity.rs</code> | <code>reason native_send.version</code> |

## outcome-unknown

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.err-reason-outcome-unknown</code> | <code>error_code</code> | <code>services/agent/schema/agent-api/v1.kvx</code> | <code>reason outcome.unknown: TransportFailure Terminal. Mutation outcome is Unknown; never resent automatically.</code> |

## owner-degraded

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.err-reason-owner-degraded</code> | <code>error_code</code> | <code>services/agent/schema/agent-api/v1.kvx</code> | <code>reason owner.degraded: UnavailableCapability Retriable. Owner is not Healthy.</code> |

## owner-refused

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.err-reason-owner-refused</code> | <code>error_code</code> | <code>services/agent/schema/agent-api/v1.kvx</code> | <code>reason owner.refused: PolicyRefusal Terminal. Owner refused the typed operation.</code> |

## owner-response-malformed

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.err-reason-owner-response-malformed</code> | <code>error_code</code> | <code>services/agent/schema/agent-api/v1.kvx</code> | <code>reason owner.response_malformed: InternalFault Terminal. Owner response did not decode.</code> |

## owner-unavailable

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.err-reason-owner-unavailable</code> | <code>error_code</code> | <code>services/agent/schema/agent-api/v1.kvx</code> | <code>reason owner.unavailable: InternalFault Retriable. Owner unavailable at the daemon; the dispatcher maps an owner Unavailable error to UnavailableCapability Retriable with this reason.</code> |

## policy-capability-store

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.err-reason-policy-capability-store</code> | <code>error_code</code> | <code>services/agent/crates/layerx-agentd/src/policy/operation.rs</code> | <code>reason policy.capability_store</code> |

## policy-context-mismatch-agent-did

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.err-reason-policy-context-mismatch-agent-did</code> | <code>error_code</code> | <code>services/agent/crates/layerx-agentd/src/human.rs</code> | <code>reason policy.context_mismatch.agent_did</code> |

## policy-context-mismatch-authority-ref

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.err-reason-policy-context-mismatch-authority-ref</code> | <code>error_code</code> | <code>services/agent/crates/layerx-agentd/src/human.rs</code> | <code>reason policy.context_mismatch.authority_ref</code> |

## policy-context-mismatch-client

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.err-reason-policy-context-mismatch-client</code> | <code>error_code</code> | <code>services/agent/crates/layerx-agentd/src/human.rs</code> | <code>reason policy.context_mismatch.client</code> |

## policy-context-mismatch-expiry

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.err-reason-policy-context-mismatch-expiry</code> | <code>error_code</code> | <code>services/agent/crates/layerx-agentd/src/human.rs</code> | <code>reason policy.context_mismatch.expiry</code> |

## policy-context-mismatch-permitted-activity-types

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.err-reason-policy-context-mismatch-permitted-activity-types</code> | <code>error_code</code> | <code>services/agent/crates/layerx-agentd/src/human.rs</code> | <code>reason policy.context_mismatch.permitted_activity_types</code> |

## policy-context-mismatch-policy-version

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.err-reason-policy-context-mismatch-policy-version</code> | <code>error_code</code> | <code>services/agent/crates/layerx-agentd/src/human.rs</code> | <code>reason policy.context_mismatch.policy_version</code> |

## policy-context-mismatch-tenant

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.err-reason-policy-context-mismatch-tenant</code> | <code>error_code</code> | <code>services/agent/crates/layerx-agentd/src/human.rs</code> | <code>reason policy.context_mismatch.tenant</code> |

## policy-intent-binding-missing

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.err-reason-policy-intent-binding-missing</code> | <code>error_code</code> | <code>services/agent/schema/agent-api/v1.kvx</code> | <code>reason policy.intent_binding_missing: PolicyRefusal Terminal. A LegacyPolicyDryRun canonical_intent carries raw canonical unsigned activity bytes whose typed semantics or retained purpose binding does not exist.</code> |

## policy-intent-effect-underivable

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.err-reason-policy-intent-effect-underivable</code> | <code>error_code</code> | <code>services/agent/crates/layerx-agentd/src/human.rs</code> | <code>reason policy.intent_effect_underivable</code> |

## policy-intent-malformed

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.err-reason-policy-intent-malformed</code> | <code>error_code</code> | <code>services/agent/crates/layerx-agentd/src/human.rs</code> | <code>reason policy.intent_malformed</code> |

## policy-legacy-project-payload

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.err-reason-policy-legacy-project-payload</code> | <code>error_code</code> | <code>services/agent/schema/agent-api/v1.kvx</code> | <code>reason policy.legacy_project_payload: PolicyRefusal Terminal. A project request carries the earlier policy dry-run payload; it is never read as a fee request.</code> |

## policy-no-policy

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.err-reason-policy-no-policy</code> | <code>error_code</code> | <code>services/agent/crates/layerx-agentd/src/policy/operation.rs</code> | <code>reason policy.no_policy</code> |

## policy-purpose-undisclosed

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.err-reason-policy-purpose-undisclosed</code> | <code>error_code</code> | <code>services/agent/crates/layerx-agentd/src/human.rs</code> | <code>reason policy.purpose_undisclosed</code> |

## policy-session-binding-missing

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.err-reason-policy-session-binding-missing</code> | <code>error_code</code> | <code>services/agent/crates/layerx-agentd/src/human.rs</code> | <code>reason policy.session_binding_missing</code> |

## policy-session-expired

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.err-reason-policy-session-expired</code> | <code>error_code</code> | <code>services/agent/crates/layerx-agentd/src/human.rs</code> | <code>reason policy.session_expired</code> |

## policy-session-expiry-binding-missing

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.err-reason-policy-session-expiry-binding-missing</code> | <code>error_code</code> | <code>services/agent/crates/layerx-agentd/src/human.rs</code> | <code>reason policy.session_expiry_binding_missing</code> |

## policy-unknown-capability

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.err-reason-policy-unknown-capability</code> | <code>error_code</code> | <code>services/agent/crates/layerx-agentd/src/policy/operation.rs</code> | <code>reason policy.unknown_capability</code> |

## policy-unknown-session

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.err-reason-policy-unknown-session</code> | <code>error_code</code> | <code>services/agent/crates/layerx-agentd/src/policy/operation.rs</code> | <code>reason policy.unknown_session</code> |

## program-stale-head

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.err-reason-program-stale-head</code> | <code>error_code</code> | <code>services/agent/crates/layerx-agentd/src/human.rs</code> | <code>reason program.stale_head</code> |

## projection-fee-snapshot-skew

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.err-reason-projection-fee-snapshot-skew</code> | <code>error_code</code> | <code>services/agent/crates/layerx-agentd/src/human.rs</code> | <code>reason projection.fee_snapshot_skew</code> |

## receipt-key-unknown

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.err-receipt-key-unknown</code> | <code>error_code</code> | <code>services/agent/schema/agent-api/errors.kvx</code> | <code>error receipt_key_unknown class VerificationFailure protocol_result_code null Terminal reason receipt_key_unknown</code> |

<a id="refused-pending-bootstrap-artifact"></a>

## rotation-unsupported

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.err-reason-rotation-unsupported-j43</code> | <code>error_code</code> | <code>services/agent/crates/layerx-agentd/src/managed_agent_rotation.rs</code> | <code>reason rotation_unsupported</code> |

| Requirement | Behavior |
| --- | --- |
| <code>req.9013</code> | A managed agent owner rotation request is refused with reason rotation_unsupported and no state changes (services/agent/crates/layerx-agentd/src/managed_agent_rotation.rs). |

## session-expired

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.err-reason-session-expired</code> | <code>error_code</code> | <code>services/agent/schema/agent-api/v1.kvx</code> | <code>reason session.expired: PolicyRefusal Terminal. Session expired.</code> |

## session-invalid-request

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.err-reason-session-invalid-request</code> | <code>error_code</code> | <code>services/agent/schema/agent-api/v1.kvx</code> | <code>reason session.invalid_request: ProtocolIncompatibility Terminal. Session authorization request is invalid.</code> |

## session-not-authorized

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.err-reason-session-not-authorized</code> | <code>error_code</code> | <code>services/agent/schema/agent-api/v1.kvx</code> | <code>reason session.not_authorized: PolicyRefusal Terminal. Session authority does not authorize the credential.</code> |

## session-revoked

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.err-reason-session-revoked</code> | <code>error_code</code> | <code>services/agent/schema/agent-api/v1.kvx</code> | <code>reason session.revoked: PolicyRefusal Terminal. Session revoked.</code> |

## session-scope-denied

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.err-reason-session-scope-denied</code> | <code>error_code</code> | <code>services/agent/schema/agent-api/v1.kvx</code> | <code>reason session.scope_denied: CapabilityRefusal Terminal. Session scope does not admit the operation surface.</code> |

## unavailable-capability-faucet-claim

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.err-reason-unavailable-capability-faucet-claim</code> | <code>error_code</code> | <code>services/agent/schema/agent-api/v1.kvx</code> | <code>reason unavailable_capability.faucet.claim: UnavailableCapability Terminal. The retired faucet.claim operation.</code> |

## unmatched-by-ruling

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.err-reason-unmatched-by-ruling</code> | <code>error_code</code> | <code>services/agent/schema/agent-api/v1.kvx</code> | <code>reason unmatched_by_ruling: UnavailableCapability Terminal. Historical program.interface refusal; retained as a reserved compatibility reason.</code> |

## verification-inconsistent

| Interface | Kind | Source | Declaration |
| --- | --- | --- | --- |
| <code>interface.9.err-reason-verification-inconsistent</code> | <code>error_code</code> | <code>services/agent/schema/agent-api/v1.kvx</code> | <code>reason verification.inconsistent: VerificationFailure Terminal. Verification status violates the level ordering.</code> |
