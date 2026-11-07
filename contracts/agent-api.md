# Agent API contract

The agent API provides authenticated preparation, signing, submission, reads,
approvals and subscriptions for Paxeer mainnet-beta. There is one BFT network:
`paxd` includes the kernel that proposes blocks, and a receipt is final when its
block commits. The EVM chain ID is **125** (`0x7d`). Sub-chains are tenant domains
in the database, rather than separately provisioned chains.

Public node JSON-RPC uses HTTPS at the root path of
`api1.mainnet-beta.paxeer.network` through `api14.mainnet-beta.paxeer.network`,
and `api-mainnet-beta.paxeer.network`. Its namespaces are `debug`, `echo`, `eth`,
`net`, `pax`, `test` and `txpool`. Hosted `px_` methods have their own
[OpenRPC contract](../services/platform/hosted/gateway/openrpc.json); do not
assume that a public node implements every hosted method. The router and unified
`/rpc`, `/rpc/ws` and `/rpc/schema` network endpoints are retired.
`node.hyperpaxeer.com` is only the hpx library mirror.

The maintained contract source is the
[agent schema](../services/agent/schema/agent-api/v1.kvx), currently **2.0**
despite its historical filename. The HTTP envelope has its own version, **1**. The executable supports contract major 2
and envelope version 1; compatibility verification explicitly refuses contract major 1.
See [configuration](agent-config.md), [formats](agent-formats.md), the
[Human peer](agent-human-peer.md) and [Rust API](agent-rust-api.md).

## Agent HTTP transport

The dedicated operation listener is enabled by `LAYERX_AGENTD_RPC_LISTEN` and
uses mutual TLS with a verified client certificate. Its private listener
implementation is `agent_rpc_exchange` in
[main.rs](../services/agent/crates/layerx-agentd/src/main.rs).
The retired public `/rpc` network endpoint must not be inferred from that private adapter's internal route.
The hosted Agent operation route is `POST /v1/agent/rpc`; this is an agent
envelope API, not an Ethereum JSON-RPC request.

Requests use UTF-8 JSON and `application/json`, with a 1,048,576-byte body bound
and 16,384-byte header bound. HTTP/1.1 framing requires one canonical decimal
`Content-Length`; transfer encoding is refused. Duplicate or unknown JSON keys,
trailing input and invalid encodings are refused. The listener refuses
`Authorization`, `LayerX-Key`, `LayerX-Tenant` and `LayerX-Agent` headers as
`PolicyRefusal`, reason `envelope.header_principal`: these cannot select an agent.

An envelope contains `version`, decimal-string `request_id`, catalogue
`operation`, its `request` object, `credential`, and, for mutations,
`idempotency_key`. The credential contains `tenant`, `session_id`, `token_id`
and decimal-string `generation`. IDs are exact lowercase hex32. The daemon
authenticates the complete credential and derives the agent from its token.
Every operation requires a credential, including registration and session opening;
neither is an unauthenticated bootstrap escape.

The idempotency key must be a nonzero lowercase hex32 for a mutating operation,
and absent or null for a read. Violations produce `IdempotencyConflict` with
`envelope.idempotency_key`. The server derives the typed canonical request
digest; callers do not provide a body digest. Repeating the same key and request
replays its recorded outcome; a different request conflicts. A transport timeout
after submission can mean `Unknown`; do not create a fresh economic request to
work around it. The owner resolves receipts and controls any exact-byte resend.

## Write lifecycle

`prepare` produces canonical bytes, a signing preimage, disclosure and a retained
preparation reference. Check the disclosure before signing. `sign` and `submit`
use that preparation; signing authority, a local capability grant and owner
consent to a preparation purpose are distinct checks. `track` observes progress;
`wait` waits within its bound and preserves pending or unknown outcomes.

Native preparation profiles are explicit discriminants on `prepare`:
`native_v1` for Programs, `native_effect_v1` for supported effects outside
Programs, `native_registration_v1` for the supported Asset Register profile,
`native_send_v1` for ordinary Asset Send, and `native_disclosure_v1` for retrieving
the retained disclosure without preparing another write. The versioned native
activity retains its protocol version, full module and ordinal. A legacy
`activity_type` is not a substitute for this record.

Program lifecycle transactions target
[`layerxprograms`](../precompiles/layerxprograms/LayerXPrograms.sol) at
`0x000000000000000000000000000000000000101d` using signed EIP-1559 bytes.
Deploy and upgrade bind the exact ABI calldata; wind-down distinguishes routing,
deprecation, tombstoning and exit. Use the versioned native call request for
EIP-1559 calls: the legacy `ProgramCallRequest` profile is typed unavailable.
Simulation and fee projection are advisory and confer no signing authority.

## Verification and errors

Write rungs are `submitted` (transaction hash), `receipted` (receipt signature
verified against the pinned receipt-key entry), and `committed` (verified Comet
commit at the receipt height). They describe available evidence on the same
network. Historical verification-level labels in internal codecs do not describe
additional networks or another finality service.

Success carries `request_id`, `value` and `verification_status`. Status is
`achieved` with a rung, `unverified` with requested/achieved rungs and a reason,
`pending` with requested/achieved rungs, or `absent` with the requested rung.
For `pending` and `unverified`, achieved must be strictly below requested;
inconsistent ordering is `VerificationFailure`, `verification.inconsistent`.

Errors preserve `class`, optional signed `protocol_result_code`, `retriability`,
`request_id` and lowercase machine-readable `reason`. A core result is not
replaced with an HTTP status. Classes are `TransportFailure`, `Deadline`,
`ProtocolIncompatibility`, `UnavailableCapability`, `CoreRejection`,
`VerificationFailure`, `PolicyRefusal`, `CapabilityRefusal`, `BudgetRefusal`,
`RateLimit`, `IdempotencyConflict` and `InternalFault`. Consult
[error.rs](../services/agent/crates/layerx-agent-api/src/error.rs) and the
[result registry](../services/agent/crates/layerx-types/src/result.rs) for exact
classification. A method in a catalogue is not proof that its dependencies are
available; typed unavailability must remain visible.

### Faucet claim

`faucet.claim` is retired. It returns `UnavailableCapability`,
`unavailable_capability.faucet.claim`, with `Terminal` retriability.

## Local read and health listeners

These local service routes are separate from public node JSON-RPC and the
session-authenticated operation envelope.

### Agent HTTP healthz

`GET /healthz` on the mutual-TLS operation listener reports owner/session
readiness. It uses the same TLS client boundary and refuses principal headers.

### Serve healthz

The bearer-authenticated full-mode listener serves `GET /healthz`, returning
200 or 503 with `ready`, `transport_ready`, `verified_reads_ready` and
`tenant_writes` (currently `"unknown"`). A healthy transport alone does not prove
verified reads or tenant write readiness.

### Human owner healthz

With `LAYERX_AGENT_MODE=human-owner`, the loopback bearer listener serves only
`GET /healthz`: 200 with `{"ready":true}` or 503 with `{"ready":false}`.
It authenticates `LAYERX_AGENT_PROGRAM_BEARER_TOKEN`, never the outbound
`LAYERX_AGENT_HUMAN_AUTHORITY_BEARER`. Startup refuses reuse of that credential.
It checks the owner loop and node/authority dependencies; other paths are 404.

### Native reads

Full mode routes `GET /v1/reads/{kind}/{selector}` through
[`NativeReadRoute`](../services/agent/crates/layerx-agentd/src/read/native.rs).
Kinds are `receipt` and `proof` (32-byte digest), `checkpoint` and `availability`
(canonical number), and `history` (the route's account/query cursor grammar).
History pages are bounded to 100 items and serialized results to 240 KiB.
Checkpoint verification requires protected validator trust; RPC-published
validators cannot establish their own authority.

### Program balances

`GET /v1/programs/{program_id}/balances` reads real derived accounts at a
committed block height. It preserves account, asset, amount and frozen-state
semantics. A malformed ID is `invalid_program`; unavailable or unverifiable
state is `program_state_unavailable`. A committed-height `eth_call` response
does not itself supply a kernel receipt or state-inclusion proof.

### Serve errors

The full-mode HTTP implementation is shared from
[proof_serve_errors.rs](../services/agent/crates/layerx-agentd/tests/proof_serve_errors.rs)
by the executable. Its small `{"error":"..."}` responses include
`invalid_request`, `unauthorized`, `not_found`, `invalid_program`, `invalid_read`,
`headers_too_large`, `body_too_large`, `request_timeout`, `read_too_large`,
`verified_read_unavailable` and `program_state_unavailable`. These are local
listener errors, not the Agent HTTP `ApiError` envelope.

## Operation catalogue

The catalogue below preserves registry anchors. Names and mutation flags come
from [Operation](../services/agent/crates/layerx-agent-api/src/operation_generated.rs).
The real dispatch is in
[agent_rpc_dispatch.rs](../services/agent/crates/layerx-agentd/src/agent_rpc_dispatch.rs)
and [agent_rpc_adapters.rs](../services/agent/crates/layerx-agentd/src/agent_rpc_adapters.rs).
Mutation flags specify envelope idempotency requirements; each owner still checks
session scope, policy, capability, budget and current authority as applicable.

| Operation | Mutating | Purpose |
| --- | --- | --- |
| <a id="agent-register"></a>`agent.register` | Yes | Prepare registration under the authenticated owner session. |
| <a id="approval-approve"></a>`approval.approve` | Yes | Decide a retained hold after revalidating its exact material. |
| <a id="approval-get"></a>`approval.get` | No | Fetch one owner-bound approval. |
| <a id="approval-list"></a>`approval.list` | No | List approvals visible to the authenticated owner. |
| <a id="approval-reject"></a>`approval.reject` | Yes | Reject the retained hold through its owner. |
| <a id="availability-fetch"></a>`availability.fetch` | No | Retrieve bounded availability evidence with explicit completion. |
| <a id="budget-create"></a>`budget.create` | Yes | Create the signed protocol budget or durable local limit. |
| <a id="budget-fund"></a>`budget.fund` | Yes | Apply signed funding to the selected enforcement mode. |
| <a id="budget-list"></a>`budget.list` | No | List the authenticated agent’s budget records. |
| <a id="budget-reconciliation"></a>`budget.reconciliation` | No | Read reconciliation facts without inventing a settlement. |
| <a id="budget-revoke"></a>`budget.revoke` | Yes | Apply an authorized revocation to the selected budget. |
| <a id="budget-state"></a>`budget.state` | No | Read the budget state and its producing activity. |
| <a id="capability-attenuate"></a>`capability.attenuate` | Yes | Narrow the parent capability dimensions. |
| <a id="capability-create"></a>`capability.create` | Yes | Create the agent’s bounded local capability. |
| <a id="capability-list"></a>`capability.list` | No | List the authenticated agent’s capability records. |
| <a id="capability-revoke"></a>`capability.revoke` | Yes | Revoke the selected capability. |
| <a id="export-offline"></a>`export.offline` | No | Export selected facts with independently verifiable evidence. |
| <a id="mcp-invoke"></a>`mcp.invoke` | Yes | Validate the named MCP tool and authorize its original operation. |
| <a id="policy-dry-run"></a>`policy.dry_run` | No | Evaluate local policy without granting protocol authorization. |
| <a id="prepare"></a>`prepare` | Yes | Retain exact canonical bytes, disclosure and signed-purpose binding. |
| <a id="program-activity"></a>`program.activity` | No | Look up a program activity with evidence. |
| <a id="program-call"></a>`program.call` | Yes | Submit the supported native program call profile. |
| <a id="program-deploy"></a>`program.deploy` | Yes | Submit signed EIP-1559 deployment calldata. |
| <a id="program-discover"></a>`program.discover` | No | Read verified registry discovery metadata. |
| <a id="program-interface"></a>`program.interface` | No | Read the version/code-bound program interface. |
| <a id="program-receipt"></a>`program.receipt` | No | Look up the selected program receipt; preserve pending/absent. |
| <a id="program-simulate"></a>`program.simulate` | No | Obtain an advisory result without committing a write. |
| <a id="program-upgrade"></a>`program.upgrade` | Yes | Submit signed EIP-1559 upgrade calldata. |
| <a id="program-wind-down"></a>`program.wind-down` | Yes | Submit a typed route, deprecate, tombstone or exit transaction. |
| <a id="project"></a>`project` | No | Project a fee; the projection is not protocol authority. |
| <a id="read-account"></a>`read.account` | No | Read account state at the requested evidence strength. |
| <a id="read-balance"></a>`read.balance` | No | Read the asset/account balance with freshness and evidence. |
| <a id="read-batch"></a>`read.batch` | No | Read a bounded batch observation. |
| <a id="read-checkpoint"></a>`read.checkpoint` | No | Read and verify committed checkpoint evidence. |
| <a id="read-history"></a>`read.history` | No | Read a bounded history page with its cursor. |
| <a id="read-module-state"></a>`read.module_state` | No | Read the selected module state. |
| <a id="read-proof-bundle"></a>`read.proof_bundle` | No | Retrieve and validate the selected proof bundle. |
| <a id="session-close"></a>`session.close` | Yes | Close the session and invalidate its authority. |
| <a id="session-list"></a>`session.list` | No | List sessions in the authenticated scope. |
| <a id="session-open"></a>`session.open` | Yes | Prepare session opening under owner authority. |
| <a id="session-refresh"></a>`session.refresh` | Yes | Refresh the session with current generation checks. |
| <a id="sign"></a>`sign` | Yes | Bind the signature to the retained preparation. |
| <a id="submit"></a>`submit` | Yes | Submit retained signed bytes under the owner mutation boundary. |
| <a id="subscription-acknowledge"></a>`subscription.acknowledge` | Yes | Advance the durable acknowledgement cursor. |
| <a id="subscription-create"></a>`subscription.create` | Yes | Create a bounded tenant-scoped durable subscription. |
| <a id="subscription-delete"></a>`subscription.delete` | Yes | Delete the owned subscription. |
| <a id="subscription-health"></a>`subscription.health` | No | Inspect delivery health and gaps. |
| <a id="subscription-list"></a>`subscription.list` | No | List subscriptions in the authenticated scope. |
| <a id="subscription-pause"></a>`subscription.pause` | Yes | Pause owned delivery. |
| <a id="subscription-resume"></a>`subscription.resume` | Yes | Resume owned delivery from durable state. |
| <a id="tenant-readiness"></a>`tenant.readiness` | No | Inspect per-tenant readiness and recovery reasons. |
| <a id="track"></a>`track` | No | Observe submission state and its exact evidence. |
| <a id="wait"></a>`wait` | No | Wait within the requested bound without upgrading unknown evidence. |

## Schema and Rust values

The following registry anchors identify authored schema declarations and current
Rust replacements where the schema has removed an older declaration.
Rows explicitly marked removed preserve historical links and point to the
current owning source for replacement types, rather than a live declaration
under the old name.
Fields below are compact navigation summaries; the linked schema retains exact
optional fields, encodings, versions, bounds and validation rules. Native wire
records are implemented in `layerx-agent-api::identity`; generic contract types
are reexported by their namespace in [lib.rs](../services/agent/crates/layerx-agent-api/src/lib.rs).

| Declaration | Shape | Source |
| --- | --- | --- |
| <a id="scalar-amount"></a>`Amount` | u128 | [v1.kvx](../services/agent/schema/agent-api/v1.kvx) |
| <a id="scalar-sequence"></a>`Sequence` | u64 | [v1.kvx](../services/agent/schema/agent-api/v1.kvx) |
| <a id="scalar-budgetlimit"></a>`BudgetLimit` | u128 | [v1.kvx](../services/agent/schema/agent-api/v1.kvx) |
| <a id="scalar-timestampseconds"></a>`TimestampSeconds` | u64 | [v1.kvx](../services/agent/schema/agent-api/v1.kvx) |
| <a id="record-contractversion"></a>`ContractVersion` | `major:u16`, `minor:u16` | [v1.kvx](../services/agent/schema/agent-api/v1.kvx) |
| <a id="record-versionrequest"></a>`VersionRequest` | `request_id:Sequence`, `supported:ContractVersion` | [v1.kvx](../services/agent/schema/agent-api/v1.kvx) |
| <a id="record-versionresponse"></a>`VersionResponse` | `request_id:Sequence`, `contract:ContractVersion`, `node_interface_major:u16` | [v1.kvx](../services/agent/schema/agent-api/v1.kvx) |
| <a id="record-agenthttpenvelope"></a>`AgentHttpEnvelope` | `version:u8`, `request_id:Sequence`, `operation:utf8`, `request:canonical-map`, `credential:AgentHttpCredential?`, `idempotency_key:hex32?` | [v1.kvx](../services/agent/schema/agent-api/v1.kvx) |
| <a id="record-agenthttpcredential"></a>`AgentHttpCredential` | `tenant:utf8`, `session_id:hex32`, `token_id:hex32`, `generation:Sequence` | [v1.kvx](../services/agent/schema/agent-api/v1.kvx) |
| <a id="record-agenthttpsuccess"></a>`AgentHttpSuccess` | `request_id:Sequence`, `value:canonical-map`, `verification_status:VerificationStatus` | [v1.kvx](../services/agent/schema/agent-api/v1.kvx) |
| <a id="record-agenthttperror"></a>`AgentHttpError` | `class:ErrorClass`, `protocol_result_code:i32?`, `retriability:Retriability`, `request_id:Sequence`, `reason:utf8` | [v1.kvx](../services/agent/schema/agent-api/v1.kvx) |
| <a id="record-nativeactivityv1"></a>`NativeActivityV1` | `version:decimal_string`, `module:decimal_string`, `ordinal:decimal_string` | [v1.kvx](../services/agent/schema/agent-api/v1.kvx) |
| <a id="record-nativepreparationpurposev1"></a>`NativePreparationPurposeV1` | `version:decimal_string`, `tenant:utf8`, `agent_did:utf8`, `session_id:hex32`, `generation:decimal_string`, `expires_at_ms:decimal_string`, `capability_id:hex32`, `preparation_id:hex32`, `canonical_digest:hex32`, `commitment:hex32` | [v1.kvx](../services/agent/schema/agent-api/v1.kvx) |
| <a id="record-signednativepreparationpurposev1"></a>`SignedNativePreparationPurposeV1` | `purpose:NativePreparationPurposeV1`, `owner_public_key:hex32`, `signature:hex64` | [v1.kvx](../services/agent/schema/agent-api/v1.kvx) |
| <a id="record-nativepreparerequestv1"></a>`NativePrepareRequestV1` | `variant:utf8`, `activity:NativeActivityV1`, `actor:utf8`, `authority:utf8`, `account_sequence:decimal_string`, `not_before:decimal_string`, `not_after:decimal_string`, `idempotency_key:hex32`, `fee_limit:decimal_string`, `payload:lowercase_hex`, `payload_hash:hex32`, `capability_id:hex32`, `purpose:SignedNativePreparationPurposeV1`, `local_grant:NativeLocalGrantConsentV1?` | [v1.kvx](../services/agent/schema/agent-api/v1.kvx) |
| <a id="record-nativeapprovallistv1"></a>`NativeApprovalListV1` | `variant:utf8` | [v1.kvx](../services/agent/schema/agent-api/v1.kvx) |
| <a id="record-nativeapprovalgetv1"></a>`NativeApprovalGetV1` | `variant:utf8`, `approval_id:hex32` | [v1.kvx](../services/agent/schema/agent-api/v1.kvx) |
| <a id="record-nativeapprovaldecisionv1"></a>`NativeApprovalDecisionV1` | `variant:utf8`, `approval_id:hex32`, `held_digest:hex32`, `current_sequence:decimal_string` | [v1.kvx](../services/agent/schema/agent-api/v1.kvx) |
| <a id="record-nativeprepareresultv1"></a>`NativePrepareResultV1` | `version:decimal_string`, `preparation_id:hex32`, `canonical_bytes:lowercase_hex`, `signing_preimage:hex32`, `activity:NativeActivityV1`, `approval_required:bool`, `approval_id:hex32_or_null` | [v1.kvx](../services/agent/schema/agent-api/v1.kvx) |
| <a id="record-nativeapprovalresultv1"></a>`NativeApprovalResultV1` | `version:decimal_string`, `approval_id:hex32`, `held_digest:hex32`, `activity:NativeActivityV1`, `state:utf8`, `submission_ref:hex32_or_null` | [v1.kvx](../services/agent/schema/agent-api/v1.kvx) |
| <a id="record-nativeapprovallistresultv1"></a>`NativeApprovalListResultV1` | `version:decimal_string`, `approvals:NativeApprovalResultV1[]` | [v1.kvx](../services/agent/schema/agent-api/v1.kvx) |
| <a id="record-nativelocalgrantconsentv1"></a>`NativeLocalGrantConsentV1` | `version:decimal_string`, `capability:lowercase_hex`, `session_scope:lowercase_hex`, `expires_at_ms:decimal_string`, `owner_public_key:hex32`, `signature:hex64` | [v1.kvx](../services/agent/schema/agent-api/v1.kvx) |
| <a id="record-nativeeffectpreparerequestv1"></a>`NativeEffectPrepareRequestV1` | `variant:utf8`, `activity:NativeActivityV1`, `actor:utf8`, `authority:utf8`, `account_sequence:decimal_string`, `not_before:decimal_string`, `not_after:decimal_string`, `idempotency_key:hex32`, `fee_limit:decimal_string`, `payload:lowercase_hex`, `payload_hash:hex32`, `capability_id:hex32`, `purpose:SignedNativePreparationPurposeV1`, `local_grant:NativeLocalGrantConsentV1?` | [v1.kvx](../services/agent/schema/agent-api/v1.kvx) |
| <a id="record-nativeregistrationpreparerequestv1"></a>`NativeRegistrationPrepareRequestV1` | `variant:utf8`, `activity:NativeActivityV1`, `actor:utf8`, `authority:utf8`, `account_sequence:decimal_string`, `not_before:decimal_string`, `not_after:decimal_string`, `idempotency_key:hex32`, `fee_limit:decimal_string`, `payload:lowercase_hex`, `payload_hash:hex32`, `capability_id:hex32`, `purpose:SignedNativePreparationPurposeV1`, `local_grant:NativeLocalGrantConsentV1?` | [v1.kvx](../services/agent/schema/agent-api/v1.kvx) |
| <a id="record-nativesendpurposev1"></a>`NativeSendPurposeV1` | `version:decimal_string`, `tenant:utf8`, `agent_did:utf8`, `owner_did:utf8`, `owner_public_key:hex32`, `session_id:hex32`, `generation:decimal_string`, `expires_at_ms:decimal_string`, `capability_id:hex32`, `protocol_version:decimal_string`, `network_id:decimal_string`, `activity:NativeActivityV1`, `preparation_id:hex32`, `canonical_digest:hex32`, `economic_action:hex32`, `idempotency_key:hex32`, `commitment:hex32` | [v1.kvx](../services/agent/schema/agent-api/v1.kvx) |
| <a id="record-signednativesendpurposev1"></a>`SignedNativeSendPurposeV1` | `purpose:NativeSendPurposeV1`, `owner_public_key:hex32`, `signature:hex64` | [v1.kvx](../services/agent/schema/agent-api/v1.kvx) |
| <a id="record-nativesendpreparerequestv1"></a>`NativeSendPrepareRequestV1` | `variant:utf8`, `activity:NativeActivityV1`, `actor:utf8`, `authority:utf8`, `account_sequence:decimal_string`, `not_before:decimal_string`, `not_after:decimal_string`, `idempotency_key:hex32`, `fee_limit:decimal_string`, `payload:lowercase_hex`, `payload_hash:hex32`, `capability_id:hex32`, `purpose:SignedNativeSendPurposeV1`, `local_grant:NativeLocalGrantConsentV1?` | [v1.kvx](../services/agent/schema/agent-api/v1.kvx) |
| <a id="record-nativedisclosurerequestv1"></a>`NativeDisclosureRequestV1` | `variant:utf8`, `canonical_bytes:lowercase_hex` | [v1.kvx](../services/agent/schema/agent-api/v1.kvx) |
| <a id="record-nativedisclosureresultv1"></a>`NativeDisclosureResultV1` | `version:decimal_string`, `preparation_id:hex32`, `canonical_bytes:lowercase_hex`, `disclosure_digest:hex32`, `activity_type:decimal_string`, `actor:lowercase_hex`, `authority:lowercase_hex`, `asset:hex32`, `fee_limit:decimal_string`, `not_before:decimal_string`, `not_after:decimal_string`, `payload_expires_at:decimal_string`, `idempotency_key:hex32` | [v1.kvx](../services/agent/schema/agent-api/v1.kvx) |
| <a id="type-sessioncontext"></a>`SessionContext` | `tenant`, `agent_did`, `authority_ref`, `permitted_activity_types`, `expiry`, `client`, `policy_version` | [identity.kvx](../services/agent/schema/agent-api/identity.kvx) |
| <a id="type-tenantreadinessrequest"></a>`TenantReadinessRequest` | Empty request object; tenant comes from the authenticated credential. | [identity.kvx](../services/agent/schema/agent-api/identity.kvx) |
| <a id="type-tenantreadiness"></a>`TenantReadiness` | `transport_ready:bool`, `verified_reads_ready:bool`, `writes_admitted:bool`, `recovery_reason:optional<TenantRecoveryReason>` | [identity.kvx](../services/agent/schema/agent-api/identity.kvx) |
| <a id="type-tenantrecoveryreason"></a>`TenantRecoveryReason` | `recovery_pending`, `store_unavailable`, `store_refused`, `budget_state_unverified`, `receipt_evidence_missing`, `durable_recovery_failed`, `spend_unreconciled`, `transport_unavailable`, `verified_read_unavailable` | [identity.kvx](../services/agent/schema/agent-api/identity.kvx) |
| <a id="type-capabilitydimensions"></a>`CapabilityDimensions` | `activity_types`, `counterparties`, `assets`, `amount_ceilings`, `rate_ceilings`, `purpose_constraints`, `expiry` | [identity.kvx](../services/agent/schema/agent-api/identity.kvx) |
| <a id="type-nativeregistrationcapability"></a>`NativeRegistrationCapability` | See the typed declaration and its validation rules. | [identity.kvx](../services/agent/schema/agent-api/identity.kvx) |
| <a id="type-capabilityrecord"></a>`CapabilityRecord` | `capability_id`, `parent_id`, `tenant`, `agent_did`, `dimensions`, `state`, `created_at_ms`, `created_at_sequence`, `revoked_at_ms`, `revoked_at_sequence` | [identity.kvx](../services/agent/schema/agent-api/identity.kvx) |
| <a id="type-capabilitystate"></a>`CapabilityState` | `active`, `revoked`, `expired` | [identity.kvx](../services/agent/schema/agent-api/identity.kvx) |
| <a id="type-capabilityrecords"></a>`CapabilityRecords` | `capabilities` | [identity.kvx](../services/agent/schema/agent-api/identity.kvx) |
| <a id="type-authoritydescription"></a>`AuthorityDescription` | `tenant`, `agent_did`, `authority_ref`, `protocol_authority` | [identity.kvx](../services/agent/schema/agent-api/identity.kvx) |
| <a id="type-budgetenforcement"></a>`BudgetEnforcement` | `ProtocolBudget`, `DaemonLimit` | [identity.kvx](../services/agent/schema/agent-api/identity.kvx) |
| <a id="type-authorityresponse"></a>`AuthorityResponse` | `authority`, `value` | [identity.kvx](../services/agent/schema/agent-api/identity.kvx) |
| <a id="type-mcpinvokerequest"></a>`McpInvokeRequest` | `tool`, `arguments` | [identity.kvx](../services/agent/schema/agent-api/identity.kvx) |
| <a id="type-budgetauthorization"></a>`BudgetAuthorization` | `preparation_ref`, `signature` | [identity.kvx](../services/agent/schema/agent-api/identity.kvx) |
| <a id="type-budgetrecord"></a>`BudgetRecord` | `enforcement`, `budget_id`, `head`, `activity_id` | [identity.kvx](../services/agent/schema/agent-api/identity.kvx) |
| <a id="type-budgetrecords"></a>`BudgetRecords` | `budgets` | [identity.kvx](../services/agent/schema/agent-api/identity.kvx) |
| <a id="type-budgetstate"></a>`BudgetState` | `record`, `balance`, `proven_head`, `activity_id` | [identity.kvx](../services/agent/schema/agent-api/identity.kvx) |
| <a id="type-policydryrunresult"></a>`PolicyDryRunResult` | `outcome`, `policy_version`, `matched_rules`, `deciding_rule`, `reason`, `mode`, `authority_statement` | [identity.kvx](../services/agent/schema/agent-api/identity.kvx) |
| <a id="type-legacypolicydryrun"></a>`LegacyPolicyDryRun` | `context`, `canonical_intent` | [identity.kvx](../services/agent/schema/agent-api/identity.kvx) |
| <a id="type-policyintent"></a>`PolicyIntent` | See the typed declaration and its validation rules. | [identity.kvx](../services/agent/schema/agent-api/identity.kvx) |
| <a id="type-disclosure"></a>`Disclosure` | `canonical_digest`, `activity_type`, `actor`, `authority`, `counterparties`, `amounts`, `asset`, `fee_limit`, `expiry`, `idempotency_key` | [write.kvx](../services/agent/schema/agent-api/write.kvx) |
| <a id="type-submissionstate"></a>`SubmissionState` | `Prepared`, `Signed`, `Queued`, `Submitted`, `Unknown`, `Executed`, `Committed`, `Failed`, `Expired` | [write.kvx](../services/agent/schema/agent-api/write.kvx) |
| <a id="type-transition"></a>`Transition` | `from`, `to`, `cause`, `at` | [write.kvx](../services/agent/schema/agent-api/write.kvx) |
| <a id="type-preparewithcapability"></a>`PrepareWithCapability` | `request`, `capability_id` | [write.kvx](../services/agent/schema/agent-api/write.kvx) |
| <a id="type-nativedisclosurev1"></a>`NativeDisclosureV1` | See the typed declaration and its validation rules. | [write.kvx](../services/agent/schema/agent-api/write.kvx) |
| <a id="type-mcpexternalsignaturev1"></a>`McpExternalSignatureV1` | `variant`, `preparation_ref`, `signature` | [write.kvx](../services/agent/schema/agent-api/write.kvx) |
| <a id="type-mcpnativewritev1"></a>`McpNativeWriteV1` | `variant`, `intent`, `preparation`, `canonical_bytes`, `signature`, `signer_public_key` | [write.kvx](../services/agent/schema/agent-api/write.kvx) |
| <a id="type-verifiedread"></a>`VerifiedRead<T>` | Rust: `value`, `achieved_rung`, `freshness`; do not substitute the historical schema field name for the current Rust rung. | [read.rs](../services/agent/crates/layerx-agent-api/src/read.rs) |
| <a id="type-freshness"></a>`Freshness` | `chain_head`, `value_height`, `relative_to` | [read.kvx](../services/agent/schema/agent-api/read.kvx) |
| <a id="type-historycursor"></a>`HistoryCursor` | `blockHeight`, `txIndex`, `logIndex` | [read.kvx](../services/agent/schema/agent-api/read.kvx) |
| <a id="type-proofbundletarget"></a>`ProofBundleTarget` | `Activity`, `AccountState`, `Receipt` | [read.kvx](../services/agent/schema/agent-api/read.kvx) |
| <a id="type-proofbundle"></a>`ProofBundle` | `format`, `verificationLevel`, `rawTransaction`, `signedReceipt`, `receiptDigest`, `receiptKey` | [read.kvx](../services/agent/schema/agent-api/read.kvx) |
| <a id="type-receiptkey"></a>`ReceiptKey` | `keyId`, `publicKey`, `firstHeight`, `lastHeight` | [read.kvx](../services/agent/schema/agent-api/read.kvx) |
| <a id="type-historyitem"></a>`HistoryItem` | `height`, `txIndex`, `logIndex`, `kind`, `canonical`, `proof` | [read.kvx](../services/agent/schema/agent-api/read.kvx) |
| <a id="type-historykind"></a>`HistoryKind` | `Activity`, `Receipt`, `Event` | [read.kvx](../services/agent/schema/agent-api/read.kvx) |
| <a id="type-factref"></a>`FactRef` | See the typed declaration and its validation rules. | [read.kvx](../services/agent/schema/agent-api/read.kvx) |
| <a id="type-feeprojectionrequest"></a>`FeeProjectionRequest` | `protocol_activity_type`, `canonical_bytes`, `execution_units`, `storage_units` | [read.kvx](../services/agent/schema/agent-api/read.kvx) |
| <a id="type-feeprojection"></a>`FeeProjection` | `request`, `parameter_version`, `fee`, `gas_units`, `denom`, `canonical_schedule`, `snapshot_height`, `snapshot_state_root` | [read.kvx](../services/agent/schema/agent-api/read.kvx) |
| <a id="type-projectionresult"></a>`ProjectionResult` | `projected`, `rationale`, `observed_freshness` | [read.kvx](../services/agent/schema/agent-api/read.kvx) |
| <a id="type-subscriptionscope"></a>`SubscriptionScope` | `tenant`, `agent`, `capability` | [stream.kvx](../services/agent/schema/agent-api/stream.kvx) |
| <a id="type-subscriptionfilter"></a>`SubscriptionFilter` | `agents`, `accounts`, `activity_types`, `modules`, `assets`, `counterparties`, `result_classes` | [stream.kvx](../services/agent/schema/agent-api/stream.kvx) |
| <a id="type-eventdelivery"></a>`EventDelivery` | `event_identity`, `event_bytes`, `deduplication_id`, `cursor`, `receipt_reference` | [stream.kvx](../services/agent/schema/agent-api/stream.kvx) |
| <a id="type-receiptreference"></a>`ReceiptReference` | `None`, `Verified` | [stream.kvx](../services/agent/schema/agent-api/stream.kvx) |
| <a id="type-delivery"></a>`Delivery` | `Event`, `Gap`, `Truncated` | [stream.kvx](../services/agent/schema/agent-api/stream.kvx) |
| <a id="type-gapnotice"></a>`GapNotice` | `missing_first`, `missing_last`, `backfill_cursor`, `backfill_attempted` | [stream.kvx](../services/agent/schema/agent-api/stream.kvx) |
| <a id="type-truncationnotice"></a>`TruncationNotice` | `requested_first`, `oldest_available`, `resume_cursor` | [stream.kvx](../services/agent/schema/agent-api/stream.kvx) |
| <a id="type-subscriptionrecord"></a>`SubscriptionRecord` | `subscription_id`, `scope`, `filter`, `start`, `last_acknowledged`, `delivery_target`, `paused` | [stream.kvx](../services/agent/schema/agent-api/stream.kvx) |
| <a id="type-subscriptionhealthtarget"></a>`SubscriptionHealthTarget` | `scope`, `subscription_id` | [stream.kvx](../services/agent/schema/agent-api/stream.kvx) |
| <a id="type-subscriptionhealth"></a>`SubscriptionHealth` | `target`, `last_acknowledged`, `last_delivery_at`, `pending_backfill` | [stream.kvx](../services/agent/schema/agent-api/stream.kvx) |
| <a id="type-apierror"></a>`ApiError` | `class`, `protocol_result_code`, `retriability`, `request_id`, `reason` | [errors.kvx](../services/agent/schema/agent-api/errors.kvx) |
| <a id="type-errorclass"></a>`ErrorClass` | `TransportFailure`, `Deadline`, `ProtocolIncompatibility`, `UnavailableCapability`, `CoreRejection`, `VerificationFailure`, `PolicyRefusal`, `CapabilityRefusal`, `BudgetRefusal`, `RateLimit`, `IdempotencyConflict`, `InternalFault` | [errors.kvx](../services/agent/schema/agent-api/errors.kvx) |
| <a id="type-retriability"></a>`Retriability` | `Terminal`, `Retriable` | [errors.kvx](../services/agent/schema/agent-api/errors.kvx) |
| <a id="type-rung"></a>`Rung` | `Submitted`, `Receipted`, `Committed` | [errors.kvx](../services/agent/schema/agent-api/errors.kvx) |
| <a id="type-verificationstatus"></a>`VerificationStatus` | `Achieved`, `Unverified` | [errors.kvx](../services/agent/schema/agent-api/errors.kvx) |
| <a id="type-idempotentmutation"></a>`IdempotentMutation` | `request_id`, `key`, `body_digest`, `operation` | [errors.kvx](../services/agent/schema/agent-api/errors.kvx) |
| <a id="type-structuredactivitydisclosure"></a>`StructuredActivityDisclosure` | `canonical_digest`, `activity_type`, `actor`, `authority`, `counterparties`, `amounts`, `asset`, `fee_limit`, `expiry`, `idempotency_key` | [approval.kvx](../services/agent/schema/agent-api/approval.kvx) |
| <a id="type-holdreason"></a>`HoldReason` | `code`, `message` | [approval.kvx](../services/agent/schema/agent-api/approval.kvx) |
| <a id="type-approvalrecord"></a>`ApprovalRecord` | `approval_id`, `tenant`, `held_activity`, `canonical_bytes_digest`, `hold_reason`, `created_at`, `expires_at`, `state` | [approval.kvx](../services/agent/schema/agent-api/approval.kvx) |
| <a id="type-approvalstate"></a>`ApprovalState` | `Held`, `Granted`, `Rejected`, `Expired`, `Defective` | [approval.kvx](../services/agent/schema/agent-api/approval.kvx) |
| <a id="type-approvaldecisionoutcome"></a>`ApprovalDecisionOutcome` | `Granted`, `Rejected`, `Expired`, `Defective`, `AlreadyDecided`, `Conflict` | [approval.kvx](../services/agent/schema/agent-api/approval.kvx) |
| <a id="type-approvallifecycleevent"></a>`ApprovalLifecycleEvent` | `event_id`, `tenant`, `approval_id`, `kind`, `at`, `record_digest` | [approval.kvx](../services/agent/schema/agent-api/approval.kvx) |
| <a id="type-programguestabi"></a>`ProgramGuestAbi` | `ABI_V1_VERSION`, `ABI_V2_VERSION`, `ABI_V3_VERSION`, `ABI_V4_VERSION`, `ABI_V5_VERSION` | [programs.kvx](../services/agent/schema/agent-api/programs.kvx) |
| <a id="type-nativeprogramdeployrequest"></a>`NativeProgramDeployRequest` | `signed_activity` | [programs.kvx](../services/agent/schema/agent-api/programs.kvx) |
| <a id="type-nativeprogramupgraderequest"></a>`NativeProgramUpgradeRequest` | `signed_activity` | [programs.kvx](../services/agent/schema/agent-api/programs.kvx) |
| <a id="type-nativeprogramwinddownrequest"></a>`NativeProgramWindDownRequest` | `signed_activity` | [programs.kvx](../services/agent/schema/agent-api/programs.kvx) |
| <a id="type-nativeprogramdeploy"></a>`NativeProgramDeploy` | `program_id`, `guest_abi`, `policy`, `authority`, `new_hash`, `wasm` | [programs.kvx](../services/agent/schema/agent-api/programs.kvx) |
| <a id="type-nativeprogramupgrade"></a>`NativeProgramUpgrade` | `program_id`, `guest_abi`, `old_hash`, `new_hash`, `migration_hook`, `clear_interface`, `wasm` | [programs.kvx](../services/agent/schema/agent-api/programs.kvx) |
| <a id="type-nativeprogramwinddown"></a>`NativeProgramWindDown` | `program_id`, `operation` | [programs.kvx](../services/agent/schema/agent-api/programs.kvx) |
| <a id="type-programlifecycleresponse"></a>`ProgramLifecycleResponse` | `ProgramSubmission` | [programs.kvx](../services/agent/schema/agent-api/programs.kvx) |
| <a id="type-programselector"></a>`ProgramSelector` | `program_id`, `requested_rung` | [programs.kvx](../services/agent/schema/agent-api/programs.kvx) |
| <a id="type-verifiedprogramdiscovery"></a>`VerifiedProgramDiscovery` | `value`, `achieved_verification_level`, `freshness` | [programs.kvx](../services/agent/schema/agent-api/programs.kvx) |
| <a id="type-verifiedprograminterface"></a>`VerifiedProgramInterface` | `value`, `achieved_verification_level`, `freshness` | [programs.kvx](../services/agent/schema/agent-api/programs.kvx) |
| <a id="type-programcallbudget"></a>`ProgramCallBudget` | `fuel`, `fee_limit` | [programs.kvx](../services/agent/schema/agent-api/programs.kvx) |
| <a id="type-programcapability"></a>`ProgramCapability` | `storage_read`, `storage_write`, `transfer`, `emit_event`, `compose` | [programs.kvx](../services/agent/schema/agent-api/programs.kvx) |
| <a id="type-programcallrequest"></a>`ProgramCallRequest` | See the typed declaration and its validation rules. | [programs.kvx](../services/agent/schema/agent-api/programs.kvx) |
| <a id="type-nativeprogramcallfields"></a>`NativeProgramCallFields` | `guest_abi`, `entrypoint`, `capabilities_hex`, `access_declaration_hex`, `response_capacity`, `resources` | [programs.kvx](../services/agent/schema/agent-api/programs.kvx) |
| <a id="type-nativeprogramcallrequest"></a>`NativeProgramCallRequest` | `payload_encoding`, `program_id`, `calldata`, `budget`, `signed_activity`, `native_call` | [programs.kvx](../services/agent/schema/agent-api/programs.kvx) |
| <a id="type-programreceiptselector"></a>`ProgramReceiptSelector` | `idempotency_key`, `expected_activity_id`, `requested_rung` | [programs.kvx](../services/agent/schema/agent-api/programs.kvx) |
| <a id="type-programreceiptresponse"></a>`ProgramReceiptResponse` | `ProgramSubmission` | [programs.kvx](../services/agent/schema/agent-api/programs.kvx) |
| <a id="type-programactivityselector"></a>`ProgramActivitySelector` | `activity_id`, `requested_rung` | [programs.kvx](../services/agent/schema/agent-api/programs.kvx) |
| <a id="type-programsimulation"></a>`ProgramSimulation` | `committed`, `program_id`, `terminal_result`, `terminal_kind`, `response`, `block`, `rung` | [programs.kvx](../services/agent/schema/agent-api/programs.kvx) |
| <a id="type-programsubmission"></a>`ProgramSubmission` | `tx_hash`, `submission_ref`, `state`, `rung` | [programs.kvx](../services/agent/schema/agent-api/programs.kvx) |
| <a id="type-faucetgrant"></a>`FaucetGrant` | Not in the active Agent schema; the SDK RPC carrier retains funding ID, transaction ID, amount and network. `faucet.claim` is retired. | [rpc.rs](../services/agent/crates/layerx-sdk/src/rpc.rs) |
| <a id="type-programexecutionevidence"></a>`ProgramExecutionEvidence` | SDK evidence carrier: payload hash, receipt, terminal payload, call graph, authorized batch, activity/program IDs and guest ABI version. | [programs.rs](../services/agent/crates/layerx-sdk/src/programs.rs) |
| <a id="type-programexecutionv4"></a>`ProgramExecutionV4` | Removed legacy execution wrapper; use `ProgramSubmission` and current receipt lookup. | [programs.kvx](../services/agent/schema/agent-api/programs.kvx) |
| <a id="type-programexecutionv5"></a>`ProgramExecutionV5` | Removed legacy execution wrapper; use `ProgramSubmission` and current receipt lookup. | [programs.kvx](../services/agent/schema/agent-api/programs.kvx) |
| <a id="type-programfailure"></a>`ProgramFailure` | Runtime failure carried by the verified program proof; use the authenticated failure accessor. | [program.rs](../services/agent/crates/layerx-proof/src/program.rs) |
| <a id="type-programlifecycleerror"></a>`ProgramLifecycleError` | Removed legacy lifecycle error shape; use the current lifecycle response and error envelope. | [programs.kvx](../services/agent/schema/agent-api/programs.kvx) |
| <a id="type-programlifecycleerrorenvelope"></a>`ProgramLifecycleErrorEnvelope` | Removed legacy lifecycle wrapper; preserve the current API error envelope. | [programs.kvx](../services/agent/schema/agent-api/programs.kvx) |
| <a id="type-programlifecyclereceipt"></a>`ProgramLifecycleReceipt` | Removed legacy receipt shape; use `ProgramSubmission` and the selected native receipt. | [programs.kvx](../services/agent/schema/agent-api/programs.kvx) |
| <a id="type-programlifecyclereceiptenvelope"></a>`ProgramLifecycleReceiptEnvelope` | Removed legacy receipt wrapper; use current submission/receipt responses. | [programs.kvx](../services/agent/schema/agent-api/programs.kvx) |
| <a id="type-programlifecycleresult"></a>`ProgramLifecycleResult` | Removed legacy lifecycle result; use `ProgramLifecycleResponse`. | [programs.kvx](../services/agent/schema/agent-api/programs.kvx) |
| <a id="type-programlifecycleresultenvelope"></a>`ProgramLifecycleResultEnvelope` | Removed legacy lifecycle wrapper; use the current Agent HTTP success/error envelope. | [programs.kvx](../services/agent/schema/agent-api/programs.kvx) |
| <a id="type-programlifecycleunknown"></a>`ProgramLifecycleUnknown` | Removed legacy unknown wrapper; preserve `Unknown` in `ProgramSubmission`. | [programs.kvx](../services/agent/schema/agent-api/programs.kvx) |
| <a id="type-programoutcome"></a>`ProgramOutcome` | Canonical core-receipt Programs facts, including runtime/ABI versions, exact result, metering and effect roots. | [receipt.rs](../services/agent/crates/layerx-wire/src/receipt.rs) |
| <a id="type-programsource"></a>`ProgramSource` | SDK source state: Unpublished, Verified with source/environment digests, or Mismatch with expected/reproduced artifact hashes. | [programs.rs](../services/agent/crates/layerx-sdk/src/programs.rs) |
| <a id="type-settlementdomain"></a>`SettlementDomain` | Removed from the Agent schema; retained in compatibility proof codecs, with no additional network finality requirement. | [checkpoint.rs](../services/agent/crates/layerx-proof/src/checkpoint.rs) |

## Receipt round-trip

A `program.receipt` query uses this version-1 envelope:

```json
{
  "version": 1,
  "request_id": "1533",
  "operation": "program.receipt",
  "idempotency_key": null,
  "request": {
    "idempotency_key": "<64 lowercase hex characters>",
    "expected_activity_id": "<64 lowercase hex characters>",
    "requested_rung": "committed"
  },
  "credential": {
    "tenant": "tenant-a",
    "session_id": "<64 lowercase hex characters>",
    "token_id": "<64 lowercase hex characters>",
    "generation": "1"
  }
}
```



The owner restores the exact signed EVM transaction from the durable outbox and
checks `expected_activity_id` against its transaction hash. It calls the chain's
`px_getReceiptKeys` and `pax_sequencerReceipt`, verifies the canonical signed
receipt, JSON coordinates, key identity and height interval, and binds any
landing evidence to the transaction. It persists the verified receipt and finality
metadata before producing the [Human receipt frame](agent-human-peer.md).
`agent_rpc_dispatch::dispatch_program_receipt` consumes that frame and refuses
malformed bytes or changed bindings. The SDK consumes the resulting JSON.

A found response's `value` contains `found: true` and `receipt` with these fields:

| Field | Encoding / meaning |
| --- | --- |
| `canonical_bytes` | Lowercase hex of the canonical verified receipt |
| `authorised_batch` | Five lowercase hex digests: `batch_id`, `asset`, `previous_state_root`, `resulting_state_root`, `sequencer_public_key` |
| `verification_level` | Decimal string for the Human verification code |
| `tx_hash` | Hash of the exact signed EVM transaction |
| `receipt_hash` | SHA-256 of `canonical_bytes` after hex decoding |
| `block_height` | Decimal nonzero block height |
| `rung` | `receipted` or `committed` |
| `signed_receipt` | The same canonical bytes as `canonical_bytes` |
| `receipt_key_id` | Pinned receipt signing-key identifier |
| `public_key` | Signing public key; matches the authorised batch key |
| `first_height`, `last_height` | Inclusive signing-key validity interval |
| `inclusion_height` | The receipt's height, equal to `block_height` |

The outer success object also carries `request_id` and `verification_status`.
`achieved` names the actual rung. A request for `committed` whose receipt only
achieved `receipted` produces an explicit `unverified` status (the typed Below variant), never an upgrade.
Only a verified BFT commit establishes `committed`; legacy verification labels
cannot create checkpoint or settlement authority above it.

Absent lookup produces `value: {"found":false}` and an `absent` status. Submitted
lookup adds `submitted: {"tx_hash":"...","nonce":"..."}` and a `pending` status
with achieved `submitted`. The Rust envelope SDK preserves these responses as
`EnvelopeOutcome::NotReady`, including the request ID, served value and typed
`NotReadyStatus::Absent` or `Pending`. Its recorded exchange API returns the same
material as `EnvelopeError::NotReady`; neither status proves execution at the
requested rung.

## Recovery and authentication

After restart, reopen the durable Store, restore the outbox by its transaction
hash, and retain exact signed bytes, receipt indexes and finality records. Repeating
lookup returns the same authenticated receipt. An unresolved submission remains
unresolved until authenticated evidence arrives; timeouts and process restart do
not prove failure or authorize a new transaction.

The Human-owner loopback health endpoint uses `LAYERX_AGENT_PROGRAM_BEARER_TOKEN`.
Outbound authority calls use the distinct `LAYERX_AGENT_HUMAN_AUTHORITY_BEARER`.
See [configuration](agent-config.md) for startup refusals and dependency checks.
Health authentication never substitutes for an Agent RPC session credential.

Executable proofs: `tests/repair_22_33_1533.rs` includes the production
receipt/store/codec/TLS/SDK round-trip and its malformed-frame and restart cases.
