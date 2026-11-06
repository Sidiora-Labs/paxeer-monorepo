<!-- Generated from services/agent/schema/agent-api by docs/platform/build/build_site.py. Do not hand-edit. -->

# Agent API reference

Schema `LayerX Agent API`, contract major `2`, minor `0`, generated from `services/agent/schema/agent-api`.

The agent contract is spoken by `layerx-agentd` and by direct-node SDK deployments. Requests and responses are canonical maps carrying the contract major and minor version; consensus integers are fixed-width in Rust and decimal strings in the dynamic-language SDKs.

Within one major version a release may only add sections or keys. Removing or changing an existing declaration requires a new major version.

## Operations

| Operation | Module | Request | Response | Required request fields |
|---|---|---|---|---|
| `approval.approve` | `approval` | `ApprovalApproveRequest` | `ApprovalDecision` | `tenant`, `approval_id`, `idempotency_key` |
| `approval.get` | `approval` | `ApprovalGetRequest` | `ApprovalRecord` | `tenant`, `approval_id` |
| `approval.list` | `approval` | `ApprovalListRequest` | `ApprovalPage` | `tenant`, `cursor`, `page_limit` |
| `approval.reject` | `approval` | `ApprovalRejectRequest` | `ApprovalDecision` | `tenant`, `approval_id`, `idempotency_key`, `reason` |
| `agent.register` | `identity` | `AgentRegistration` | `AuthorityResponse<Prepared>` | `tenant`, `agent_did`, `authority_ref`, `client`, `policy_version`, `owner_address`, `binding_consent`, `fee_limit` |
| `budget.create` | `identity` | `object` | `AuthorityResponse<BudgetRecord>` | `tenant`, `agent_did`, `asset`, `limit`, `enforcement`, `expiry` |
| `budget.fund` | `identity` | `object` | `AuthorityResponse<BudgetRecord>` | `tenant`, `agent_did`, `budget_id`, `amount` |
| `budget.list` | `identity` | `object` | `AuthorityResponse<BudgetRecords>` | `tenant`, `agent_did` |
| `budget.reconciliation` | `identity` | `object` | `AuthorityResponse<BudgetReconciliation>` | `tenant`, `agent_did`, `budget_id` |
| `budget.revoke` | `identity` | `object` | `AuthorityResponse<BudgetRecord>` | `tenant`, `agent_did`, `budget_id` |
| `budget.state` | `identity` | `object` | `AuthorityResponse<BudgetState>` | `tenant`, `agent_did`, `budget_id` |
| `capability.attenuate` | `identity` | `object` | `AuthorityResponse<CapabilityRecord>` | `tenant`, `agent_did`, `parent_id`, `dimensions` |
| `capability.create` | `identity` | `object` | `AuthorityResponse<CapabilityRecord>` | `tenant`, `agent_did`, `dimensions` |
| `capability.list` | `identity` | `object` | `AuthorityResponse<CapabilityRecords>` | `tenant`, `agent_did` |
| `capability.revoke` | `identity` | `object` | `AuthorityResponse<CapabilityRecord>` | `tenant`, `agent_did`, `capability_id` |
| `mcp.invoke` | `identity` | `McpInvokeRequest` | `object` | `idempotency_key` |
| `policy.dry_run` | `identity` | `object` | `PolicyDryRunResult` | `tenant`, `agent_did`, `session_id`, `capability_id`, `activity_type`, `counterparty`, `asset`, `amount`, `purpose`, `core_sequence` |
| `session.close` | `identity` | `SessionClose` | `AuthorityResponse<SessionRecord>` | `session_id`, `context` |
| `session.list` | `identity` | `SessionList` | `AuthorityResponse<SessionRecords>` | `context` |
| `session.open` | `identity` | `SessionOpen` | `AuthorityResponse<Prepared>` | `context`, `grant_id`, `registration_payload`, `expiry_sequence`, `purpose`, `fee_limit` |
| `session.refresh` | `identity` | `SessionRefresh` | `AuthorityResponse<SessionRecord>` | `session_id`, `context` |
| `tenant.readiness` | `identity` | `TenantReadinessRequest` | `TenantReadiness` | - |
| `program.activity` | `programs` | `ProgramActivitySelector` | `ProgramSubmission` | - |
| `program.call` | `programs` | `ProgramCallRequest` | `ProgramSubmission` | `idempotency_key` |
| `program.deploy` | `programs` | `NativeProgramDeployRequest` | `ProgramLifecycleResponse` | `idempotency_key` |
| `program.discover` | `programs` | `ProgramSelector` | `VerifiedProgramDiscovery` | - |
| `program.interface` | `programs` | `ProgramSelector` | `VerifiedProgramInterface` | - |
| `program.receipt` | `programs` | `ProgramReceiptSelector` | `ProgramReceiptResponse` | - |
| `program.simulate` | `programs` | `ProgramCallRequest` | `ProgramSimulation` | - |
| `program.upgrade` | `programs` | `NativeProgramUpgradeRequest` | `ProgramLifecycleResponse` | `idempotency_key` |
| `program.wind-down` | `programs` | `NativeProgramWindDownRequest` | `ProgramLifecycleResponse` | `idempotency_key` |
| `availability.fetch` | `read` | `object` | `VerifiedRead<AvailabilityReport>` | - |
| `export.offline` | `read` | `object` | `VerifiedRead<OfflineExport>` | - |
| `project` | `read` | `FeeProjectionRequest` | `ProjectionResult` | - |
| `read.account` | `read` | `object` | `VerifiedRead<AccountValue>` | - |
| `read.balance` | `read` | `object` | `VerifiedRead<BalanceValue>` | - |
| `read.batch` | `read` | `object` | `VerifiedRead<BatchValue>` | - |
| `read.checkpoint` | `read` | `object` | `VerifiedRead<CheckpointValue>` | - |
| `read.history` | `read` | `object` | `VerifiedRead<HistoryValue>` | - |
| `read.module_state` | `read` | `object` | `VerifiedRead<ModuleStateValue>` | - |
| `read.proof_bundle` | `read` | `object` | `VerifiedRead<ProofBundle>` | - |
| `subscription.acknowledge` | `stream` | `object` | `object` | `scope`, `subscription_id`, `cursor` |
| `subscription.create` | `stream` | `object` | `SubscriptionRecord` | `scope`, `filter`, `start`, `delivery_target` |
| `subscription.delete` | `stream` | `object` | `object` | `scope`, `subscription_id` |
| `subscription.health` | `stream` | `object` | `SubscriptionHealth` | `scope`, `subscription_id` |
| `subscription.list` | `stream` | `object` | `SubscriptionRecord[]` | `scope` |
| `subscription.pause` | `stream` | `object` | `object` | `scope`, `subscription_id` |
| `subscription.resume` | `stream` | `object` | `object` | `scope`, `subscription_id` |
| `prepare` | `write` | `PrepareRequest` | `Prepared` | `actor`, `authority`, `account_sequence`, `timestamp_bound`, `idempotency_key`, `fee_limit`, `payload`, `payload_hash` |
| `sign` | `write` | `SignRequest` | `Signed` | `preparation_ref`, `signature` |
| `submit` | `write` | `SubmitRequest` | `TrackedSubmission` | `preparation_ref`, `signature` |
| `track` | `write` | `TrackRequest` | `TrackedSubmission` | `submission_ref` |
| `wait` | `write` | `WaitRequest` | `WaitResult` | `submission_ref`, `requested_rung`, `deadline` |

## Declared types

### Module `approval`

additive_only

| Declaration | Kind | Shape |
|---|---|---|
| `ApprovalDecisionOutcome` | type | variants: `Granted`, `Rejected`, `Expired`, `Defective`, `AlreadyDecided`, `Conflict` |
| `ApprovalLifecycleEvent` | type | variants: `Created`, `Granted`, `Rejected`, `Expired`, `Defective`<br>required: `event_id`, `tenant`, `approval_id`, `kind`, `at`, `record_digest` |
| `ApprovalRecord` | type | required: `approval_id`, `tenant`, `held_activity`, `canonical_bytes_digest`, `hold_reason`, `created_at`, `expires_at`, `state` |
| `ApprovalState` | type | variants: `Held`, `Granted`, `Rejected`, `Expired`, `Defective` |
| `HoldReason` | type | required: `code`, `message` |
| `StructuredActivityDisclosure` | type | required: `canonical_digest`, `activity_type`, `actor`, `authority`, `counterparties`, `amounts`, `asset`, `fee_limit`, `expiry`, `idempotency_key` |

### Module `errors`

additive_only

| Declaration | Kind | Shape |
|---|---|---|
| `ApiError` | type | required: `class`, `protocol_result_code`, `retriability`, `request_id`, `reason` |
| `ErrorClass` | type | variants: `TransportFailure`, `Deadline`, `ProtocolIncompatibility`, `UnavailableCapability`, `CoreRejection`, `VerificationFailure`, `PolicyRefusal`, `CapabilityRefusal`, `BudgetRefusal`, `RateLimit`, `IdempotencyConflict`, `InternalFault` |
| `IdempotentMutation` | type | required: `request_id`, `key`, `body_digest`, `operation` |
| `Retriability` | type | variants: `Terminal`, `Retriable` |
| `Rung` | type | variants: `Submitted`, `Receipted`, `Committed` |
| `VerificationStatus` | type | variants: `Achieved`, `Unverified` |

### Module `identity`

additive_only

| Declaration | Kind | Shape |
|---|---|---|
| `AuthorityDescription` | type | required: `tenant`, `agent_did`, `authority_ref`, `protocol_authority` |
| `AuthorityResponse` | type | required: `authority`, `value` |
| `BudgetAuthorization` | type | required: `preparation_ref`, `signature`<br>optional: `signer_public_key` |
| `BudgetEnforcement` | type | variants: `ProtocolBudget`, `DaemonLimit` |
| `BudgetRecord` | type | required: `enforcement`, `budget_id`, `head`, `activity_id` |
| `BudgetRecords` | type | required: `budgets` |
| `BudgetState` | type | required: `record`, `balance`, `proven_head`, `activity_id` |
| `CapabilityDimensions` | type | required: `activity_types`, `counterparties`, `assets`, `amount_ceilings`, `rate_ceilings`, `purpose_constraints`, `expiry` |
| `CapabilityRecord` | type | required: `capability_id`, `parent_id`, `tenant`, `agent_did`, `dimensions`, `state`, `created_at_ms`, `created_at_sequence`, `revoked_at_ms`, `revoked_at_sequence` |
| `CapabilityRecords` | type | required: `capabilities` |
| `CapabilityState` | type | variants: `active`, `revoked`, `expired` |
| `LegacyPolicyDryRun` | type | required: `context`, `canonical_intent` |
| `McpInvokeRequest` | type | required: `tool`, `arguments` |
| `NativeRegistrationCapability` | type | - |
| `PolicyDryRunResult` | type | required: `outcome`, `policy_version`, `matched_rules`, `deciding_rule`, `reason`, `mode`, `authority_statement` |
| `PolicyIntent` | type | - |
| `SessionContext` | type | required: `tenant`, `agent_did`, `authority_ref`, `permitted_activity_types`, `expiry`, `client`, `policy_version` |
| `TenantReadiness` | type | required: `transport_ready:bool`, `verified_reads_ready:bool`, `writes_admitted:bool`, `recovery_reason:optional<TenantRecoveryReason>` |
| `TenantReadinessRequest` | type | - |
| `TenantRecoveryReason` | type | variants: `recovery_pending`, `store_unavailable`, `store_refused`, `budget_state_unverified`, `receipt_evidence_missing`, `durable_recovery_failed`, `spend_unreconciled`, `transport_unavailable`, `verified_read_unavailable` |

### Module `programs`

additive_only

| Declaration | Kind | Shape |
|---|---|---|
| `NativeProgramCallFields` | type | required: `guest_abi`, `entrypoint`, `capabilities_hex`, `access_declaration_hex`, `response_capacity`, `resources` |
| `NativeProgramCallRequest` | type | required: `payload_encoding`, `program_id`, `calldata`, `budget`, `signed_activity`, `native_call` |
| `NativeProgramDeploy` | type | required: `program_id`, `guest_abi`, `policy`, `authority`, `new_hash`, `wasm`<br>optional: `interface` |
| `NativeProgramDeployRequest` | type | required: `signed_activity` |
| `NativeProgramUpgrade` | type | required: `program_id`, `guest_abi`, `old_hash`, `new_hash`, `migration_hook`, `clear_interface`, `wasm`<br>optional: `interface` |
| `NativeProgramUpgradeRequest` | type | required: `signed_activity` |
| `NativeProgramWindDown` | type | variants: `route`, `deprecate`, `tombstone`, `exit`<br>required: `program_id`, `operation` |
| `NativeProgramWindDownRequest` | type | required: `signed_activity` |
| `ProgramActivitySelector` | type | required: `activity_id`, `requested_rung` |
| `ProgramCallBudget` | type | required: `fuel`, `fee_limit` |
| `ProgramCallRequest` | type | - |
| `ProgramCapability` | type | variants: `storage_read`, `storage_write`, `transfer`, `emit_event`, `compose` |
| `ProgramGuestAbi` | type | variants: `ABI_V1_VERSION`, `ABI_V2_VERSION`, `ABI_V3_VERSION`, `ABI_V4_VERSION`, `ABI_V5_VERSION` |
| `ProgramLifecycleResponse` | type | variants: `ProgramSubmission` |
| `ProgramReceiptResponse` | type | variants: `ProgramSubmission` |
| `ProgramReceiptSelector` | type | required: `idempotency_key`, `expected_activity_id`, `requested_rung` |
| `ProgramSelector` | type | required: `program_id`, `requested_rung` |
| `ProgramSimulation` | type | required: `committed`, `program_id`, `terminal_result`, `terminal_kind`, `response`, `block`, `rung` |
| `ProgramSubmission` | type | required: `tx_hash`, `submission_ref`, `state`, `rung` |
| `VerifiedProgramDiscovery` | type | required: `value`, `achieved_verification_level`, `freshness` |
| `VerifiedProgramInterface` | type | required: `value`, `achieved_verification_level`, `freshness` |

### Module `read`

additive_only

| Declaration | Kind | Shape |
|---|---|---|
| `FactRef` | type | - |
| `FeeProjection` | type | required: `request`, `parameter_version`, `fee`, `gas_units`, `denom`, `canonical_schedule`, `snapshot_height`, `snapshot_state_root` |
| `FeeProjectionRequest` | type | required: `protocol_activity_type`, `canonical_bytes`, `execution_units`, `storage_units` |
| `Freshness` | type | required: `chain_head`, `value_height`, `relative_to` |
| `HistoryCursor` | type | fields: `blockHeight`, `txIndex`, `logIndex` |
| `HistoryItem` | type | required: `height`, `txIndex`, `logIndex`, `kind`, `canonical`, `proof` |
| `HistoryKind` | type | variants: `Activity`, `Receipt`, `Event` |
| `ProjectionResult` | type | required: `projected`, `rationale`, `observed_freshness` |
| `ProofBundle` | type | required: `format`, `verificationLevel`, `rawTransaction`, `signedReceipt`, `receiptDigest`, `receiptKey`<br>optional: `accountProof`<br>format: `layerx-receipt-proof-v2` |
| `ProofBundleTarget` | type | variants: `Activity`, `AccountState`, `Receipt` |
| `ReceiptKey` | type | required: `keyId`, `publicKey`, `firstHeight`, `lastHeight` |
| `VerifiedRead` | type | required: `value`, `achieved_verification_level`, `freshness` |

### Module `stream`

additive_only

| Declaration | Kind | Shape |
|---|---|---|
| `Delivery` | type | variants: `Event`, `Gap`, `Truncated` |
| `EventDelivery` | type | required: `event_identity`, `event_bytes`, `deduplication_id`, `cursor`, `receipt_reference` |
| `GapNotice` | type | required: `missing_first`, `missing_last`, `backfill_cursor`, `backfill_attempted` |
| `GapNotice.encoding` | type | - |
| `ReceiptReference` | type | variants: `None`, `Verified` |
| `SubscriptionFilter` | type | required: `agents`, `accounts`, `activity_types`, `modules`, `assets`, `counterparties`, `result_classes` |
| `SubscriptionHealth` | type | required: `target`, `last_acknowledged`, `last_delivery_at`, `pending_backfill` |
| `SubscriptionHealthTarget` | type | required: `scope`, `subscription_id` |
| `SubscriptionRecord` | type | required: `subscription_id`, `scope`, `filter`, `start`, `last_acknowledged`, `delivery_target`, `paused` |
| `SubscriptionScope` | type | required: `tenant`, `agent`, `capability` |
| `TruncationNotice` | type | required: `requested_first`, `oldest_available`, `resume_cursor` |

### Module `v1`

| Declaration | Kind | Shape |
|---|---|---|
| `AgentHttpCredential` | record | fields: `tenant:utf8`, `session_id:hex32`, `token_id:hex32`, `generation:Sequence` |
| `AgentHttpEnvelope` | record | fields: `version:u8`, `request_id:Sequence`, `operation:utf8`, `request:canonical-map`, `credential:AgentHttpCredential?`, `idempotency_key:hex32?` |
| `AgentHttpError` | record | fields: `class:ErrorClass`, `protocol_result_code:i32?`, `retriability:Retriability`, `request_id:Sequence`, `reason:utf8` |
| `AgentHttpSuccess` | record | fields: `request_id:Sequence`, `value:canonical-map`, `verification_status:VerificationStatus` |
| `ContractVersion` | record | fields: `major:u16`, `minor:u16` |
| `NativeActivityV1` | record | fields: `version:decimal_string`, `module:decimal_string`, `ordinal:decimal_string`<br>wire: `All three fields are required strings. Unknown or duplicate fields, numeric JSON values, signs, whitespace, leading zeros and overflow are refused.`<br>rust: `layerx_agent_api::identity::NativeActivity` |
| `NativeApprovalDecisionV1` | record | fields: `variant:utf8`, `approval_id:hex32`, `held_digest:hex32`, `current_sequence:decimal_string`<br>rust: `layerx_agent_api::identity::NativeApprovalDecisionV1` |
| `NativeApprovalGetV1` | record | fields: `variant:utf8`, `approval_id:hex32`<br>rust: `layerx_agent_api::identity::NativeApprovalGetV1` |
| `NativeApprovalListResultV1` | record | fields: `version:decimal_string`, `approvals:NativeApprovalResultV1[]`<br>rust: `layerx_agent_api::identity::NativeApprovalListResultV1` |
| `NativeApprovalListV1` | record | fields: `variant:utf8`<br>rust: `layerx_agent_api::identity::NativeApprovalListV1` |
| `NativeApprovalResultV1` | record | fields: `version:decimal_string`, `approval_id:hex32`, `held_digest:hex32`, `activity:NativeActivityV1`, `state:utf8`, `submission_ref:hex32_or_null`<br>rust: `layerx_agent_api::identity::NativeApprovalResultV1` |
| `NativeDisclosureRequestV1` | record | fields: `variant:utf8`, `canonical_bytes:lowercase_hex`<br>wire: `Closed request; unknown fields, noncanonical hex, missing bytes and unowned or changed preparation refuse.`<br>rust: `layerx_agent_api::identity::NativeDisclosureRequestV1` |
| `NativeDisclosureResultV1` | record | fields: `version:decimal_string`, `preparation_id:hex32`, `canonical_bytes:lowercase_hex`, `disclosure_digest:hex32`, `activity_type:decimal_string`, `actor:lowercase_hex`, `authority:lowercase_hex`, `asset:hex32`, `fee_limit:decimal_string`, `not_before:decimal_string`, `not_after:decimal_string`, `payload_expires_at:decimal_string`, `idempotency_key:hex32`<br>rust: `layerx_agent_api::identity::NativeDisclosureResultV1` |
| `NativeEffectPrepareRequestV1` | record | fields: `variant:utf8`, `activity:NativeActivityV1`, `actor:utf8`, `authority:utf8`, `account_sequence:decimal_string`, `not_before:decimal_string`, `not_after:decimal_string`, `idempotency_key:hex32`, `fee_limit:decimal_string`, `payload:lowercase_hex`, `payload_hash:hex32`, `capability_id:hex32`, `purpose:SignedNativePreparationPurposeV1`, `local_grant:NativeLocalGrantConsentV1?`<br>wire: `All fields except optional local_grant are required. Unknown fields, legacy activity_type substitution, omitted variant, wrong native version, numeric JSON integers and noncanonical decimal or hexadecimal forms are refused.`<br>rust: `layerx_agent_api::identity::NativeEffectPrepareRequestV1` |
| `NativeLocalGrantConsentV1` | record | fields: `version:decimal_string`, `capability:lowercase_hex`, `session_scope:lowercase_hex`, `expires_at_ms:decimal_string`, `owner_public_key:hex32`, `signature:hex64`<br>wire: `All fields required; unknown fields and noncanonical decimal or hexadecimal encodings refused. Native prepare canonical JSON includes this complete object or null under local_grant.`<br>rust: `layerx_agent_api::identity::NativeLocalGrantConsentV1` |
| `NativePreparationPurposeV1` | record | fields: `version:decimal_string`, `tenant:utf8`, `agent_did:utf8`, `session_id:hex32`, `generation:decimal_string`, `expires_at_ms:decimal_string`, `capability_id:hex32`, `preparation_id:hex32`, `canonical_digest:hex32`, `commitment:hex32`<br>wire: `Every field is required, unknown or duplicate fields are refused, integer fields are decimal strings and all hex fields are lowercase with no prefix. Canonical JSON is produced only after conversion through the contract validator.`<br>rust: `layerx_agent_api::identity::NativePreparationPurposeV1` |
| `NativePrepareRequestV1` | record | fields: `variant:utf8`, `activity:NativeActivityV1`, `actor:utf8`, `authority:utf8`, `account_sequence:decimal_string`, `not_before:decimal_string`, `not_after:decimal_string`, `idempotency_key:hex32`, `fee_limit:decimal_string`, `payload:lowercase_hex`, `payload_hash:hex32`, `capability_id:hex32`, `purpose:SignedNativePreparationPurposeV1`, `local_grant:NativeLocalGrantConsentV1?`<br>wire: `All fields except optional local_grant are required. Unknown fields, legacy activity_type substitution, omitted variant, wrong native version, numeric JSON integers and noncanonical decimal or hexadecimal forms are refused.`<br>rust: `layerx_agent_api::identity::NativePrepareRequestV1` |
| `NativePrepareResultV1` | record | fields: `version:decimal_string`, `preparation_id:hex32`, `canonical_bytes:lowercase_hex`, `signing_preimage:hex32`, `activity:NativeActivityV1`, `approval_required:bool`, `approval_id:hex32_or_null`<br>rust: `layerx_agent_api::identity::NativePrepareResultV1` |
| `NativeRegistrationPrepareRequestV1` | record | fields: `variant:utf8`, `activity:NativeActivityV1`, `actor:utf8`, `authority:utf8`, `account_sequence:decimal_string`, `not_before:decimal_string`, `not_after:decimal_string`, `idempotency_key:hex32`, `fee_limit:decimal_string`, `payload:lowercase_hex`, `payload_hash:hex32`, `capability_id:hex32`, `purpose:SignedNativePreparationPurposeV1`, `local_grant:NativeLocalGrantConsentV1?`<br>rust: `layerx_agent_api::identity::NativeRegistrationPrepareRequestV1` |
| `NativeSendPrepareRequestV1` | record | fields: `variant:utf8`, `activity:NativeActivityV1`, `actor:utf8`, `authority:utf8`, `account_sequence:decimal_string`, `not_before:decimal_string`, `not_after:decimal_string`, `idempotency_key:hex32`, `fee_limit:decimal_string`, `payload:lowercase_hex`, `payload_hash:hex32`, `capability_id:hex32`, `purpose:SignedNativeSendPurposeV1`, `local_grant:NativeLocalGrantConsentV1?`<br>rust: `layerx_agent_api::identity::NativeSendPrepareRequestV1` |
| `NativeSendPurposeV1` | record | fields: `version:decimal_string`, `tenant:utf8`, `agent_did:utf8`, `owner_did:utf8`, `owner_public_key:hex32`, `session_id:hex32`, `generation:decimal_string`, `expires_at_ms:decimal_string`, `capability_id:hex32`, `protocol_version:decimal_string`, `network_id:decimal_string`, `activity:NativeActivityV1`, `preparation_id:hex32`, `canonical_digest:hex32`, `economic_action:hex32`, `idempotency_key:hex32`, `commitment:hex32`<br>rust: `layerx_agent_api::identity::NativeSendPurposeV1` |
| `NativeSendPurposeV2` | record | fields: `tenant:utf8`, `agent_did:utf8`, `owner_did:utf8`, `consent_public_key:hex32`, `evm_sender:hex20`, `session_id:hex32`, `generation:decimal_string`, `expires_at_ms:decimal_string`, `capability_id:hex32`, `transaction_type:decimal_string`, `chain_id:decimal_string`, `activity:NativeActivityV1`, `preparation_id:hex32`, `canonical_digest:hex32`, `economic_action:hex32`, `idempotency_key:hex32`, `commitment:hex32`<br>rust: `layerx_agent_api::identity::NativeSendPurposeV2` |
| `SignedNativePreparationPurposeV1` | record | fields: `purpose:NativePreparationPurposeV1`, `owner_public_key:hex32`, `signature:hex64`<br>rust: `layerx_agent_api::identity::SignedNativePreparationPurposeV1` |
| `SignedNativeSendPurposeV1` | record | fields: `purpose:NativeSendPurposeV1`, `owner_public_key:hex32`, `signature:hex64`<br>rust: `layerx_agent_api::identity::SignedNativeSendPurposeV1` |
| `SignedNativeSendPurposeV2` | record | fields: `purpose:NativeSendPurposeV2`, `signature:hex64`<br>rust: `layerx_agent_api::identity::SignedNativeSendPurposeV2` |
| `VerificationStatus` | record | - |
| `VersionRequest` | record | fields: `request_id:Sequence`, `supported:ContractVersion` |
| `VersionResponse` | record | fields: `request_id:Sequence`, `contract:ContractVersion`, `node_interface_major:u16` |
| `Amount` | scalar | wire: `decimal_string`<br>rust: `u128`<br>typescript: `bigint`<br>python: `int` |
| `BudgetLimit` | scalar | wire: `decimal_string`<br>rust: `u128`<br>typescript: `bigint`<br>python: `int` |
| `Sequence` | scalar | wire: `decimal_string`<br>rust: `u64`<br>typescript: `bigint`<br>python: `int` |
| `TimestampSeconds` | scalar | wire: `decimal_string`<br>rust: `u64`<br>typescript: `bigint`<br>python: `int` |

### Module `write`

additive_only

| Declaration | Kind | Shape |
|---|---|---|
| `Disclosure` | type | required: `canonical_digest`, `activity_type`, `actor`, `authority`, `counterparties`, `amounts`, `asset`, `fee_limit`, `expiry`, `idempotency_key` |
| `McpExternalSignatureV1` | type | required: `variant`, `preparation_ref`, `signature` |
| `McpNativeWriteV1` | type | required: `variant`, `intent`, `preparation`, `canonical_bytes`, `signature`, `signer_public_key` |
| `NativeDisclosureV1` | type | - |
| `PrepareWithCapability` | type | required: `request`, `capability_id` |
| `SubmissionState` | type | variants: `Prepared`, `Signed`, `Queued`, `Submitted`, `Unknown`, `Executed`, `Committed`, `Failed`, `Expired` |
| `Transition` | type | required: `from`, `to`, `cause`, `at` |
