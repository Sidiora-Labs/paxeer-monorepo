# Interop gateway contract

The [gateway core](../../../services/interop/crates/layerx-interop-gateway/src/lib.rs) registers protocol adapters, translates requests and verifies receipt evidence before recording settlement. Its [route parser](../../../services/interop/crates/layerx-interop-gateway/src/server.rs) is used by the interop HTTP service.

## err-gw-adapter-declaration

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-gw-adapter-declaration</code> | <code>error_code</code> | [services/interop/crates/layerx-interop-gateway/src/error.rs](../../../services/interop/crates/layerx-interop-gateway/src/error.rs) | <code>GatewayError::Adapter.code() = "adapter-declaration"</code> |

## err-gw-adapter-duplicate

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-gw-adapter-duplicate</code> | <code>error_code</code> | [services/interop/crates/layerx-interop-gateway/src/error.rs](../../../services/interop/crates/layerx-interop-gateway/src/error.rs) | <code>GatewayError::DuplicateAdapter.code() = "adapter-duplicate"</code> |

## err-gw-adapter-unknown

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-gw-adapter-unknown</code> | <code>error_code</code> | [services/interop/crates/layerx-interop-gateway/src/error.rs](../../../services/interop/crates/layerx-interop-gateway/src/error.rs) | <code>GatewayError::UnknownAdapter.code() = "adapter-unknown"</code> |

## err-gw-audit-append

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-gw-audit-append</code> | <code>error_code</code> | [services/interop/crates/layerx-interop-gateway/src/error.rs](../../../services/interop/crates/layerx-interop-gateway/src/error.rs) | <code>GatewayError::Audit.code() = "audit-append"</code> |

## err-gw-idempotency-conflict

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-gw-idempotency-conflict</code> | <code>error_code</code> | [services/interop/crates/layerx-interop-gateway/src/error.rs](../../../services/interop/crates/layerx-interop-gateway/src/error.rs) | <code>GatewayError::IdempotencyConflict.code() = "idempotency-conflict"</code> |

## err-gw-principal-scope

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-gw-principal-scope</code> | <code>error_code</code> | [services/interop/crates/layerx-interop-gateway/src/error.rs](../../../services/interop/crates/layerx-interop-gateway/src/error.rs) | <code>GatewayError::Store.code() = "principal-scope"</code> |

## err-gw-receipt-rejected

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-gw-receipt-rejected</code> | <code>error_code</code> | [services/interop/crates/layerx-interop-gateway/src/error.rs](../../../services/interop/crates/layerx-interop-gateway/src/error.rs) | <code>GatewayError::ReceiptRejected.code() = "receipt-rejected"</code> |

## err-gw-receipt-required

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-gw-receipt-required</code> | <code>error_code</code> | [services/interop/crates/layerx-interop-gateway/src/error.rs](../../../services/interop/crates/layerx-interop-gateway/src/error.rs) | <code>GatewayError::ReceiptRequired.code() = "receipt-required"</code> |

## err-gw-record-corrupt

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-gw-record-corrupt</code> | <code>error_code</code> | [services/interop/crates/layerx-interop-gateway/src/error.rs](../../../services/interop/crates/layerx-interop-gateway/src/error.rs) | <code>GatewayError::Corrupt.code() = "record-corrupt"</code> |

## err-gw-translation-closed

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-gw-translation-closed</code> | <code>error_code</code> | [services/interop/crates/layerx-interop-gateway/src/error.rs](../../../services/interop/crates/layerx-interop-gateway/src/error.rs) | <code>GatewayError::TranslationClosed.code() = "translation-closed"</code> |

## err-gw-translation-invalid

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-gw-translation-invalid</code> | <code>error_code</code> | [services/interop/crates/layerx-interop-gateway/src/error.rs](../../../services/interop/crates/layerx-interop-gateway/src/error.rs) | <code>GatewayError::InvalidTranslation.code() = "translation-invalid"</code> |

## err-gw-translation-read-only

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-gw-translation-read-only</code> | <code>error_code</code> | [services/interop/crates/layerx-interop-gateway/src/error.rs](../../../services/interop/crates/layerx-interop-gateway/src/error.rs) | <code>GatewayError::NotStateChanging.code() = "translation-read-only"</code> |

## err-gw-translation-unknown

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-gw-translation-unknown</code> | <code>error_code</code> | [services/interop/crates/layerx-interop-gateway/src/error.rs](../../../services/interop/crates/layerx-interop-gateway/src/error.rs) | <code>GatewayError::UnknownTranslation.code() = "translation-unknown"</code> |

## fmt-gw-audit-entry

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-gw-audit-entry</code> | <code>file_format</code> | [services/interop/crates/layerx-interop-gateway/src/audit.rs](../../../services/interop/crates/layerx-interop-gateway/src/audit.rs) | <code>audit entry: magic "LXGA", version u8 1, AuditEventKind u8 (1 adapter-registered, 2 translation-begun, 3 translation-completed, 4 translation-refused); chain links domain-separated by "LXGG" (genesis) and "LXGL" (link)</code> |

## fmt-gw-audit-event-kind-labels

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-gw-audit-event-kind-labels</code> | <code>file_format</code> | [services/interop/crates/layerx-interop-gateway/src/audit.rs](../../../services/interop/crates/layerx-interop-gateway/src/audit.rs) | <code>AuditEventKind::label() -&gt; "adapter-registered" &#124; "translation-begun" &#124; "translation-completed" &#124; "translation-refused"</code> |

## fmt-gw-emission

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-gw-emission</code> | <code>file_format</code> | [services/interop/crates/layerx-interop-gateway/src/redaction.rs](../../../services/interop/crates/layerx-interop-gateway/src/redaction.rs) | <code>telemetry emission: magic "LXIE", version u8 1, field kinds u8 (1 digest, 2 label, 3 count, 4 duration-ms, 5 trace); schemas adapter-call, audit-append, error-response; names &lt;= 64 bytes</code> |

## fmt-gw-evidence-policy-labels

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-gw-evidence-policy-labels</code> | <code>file_format</code> | [services/interop/crates/layerx-interop-gateway/src/server.rs](../../../services/interop/crates/layerx-interop-gateway/src/server.rs) | <code>EvidencePolicy::label() -&gt; "layerx-receipt" &#124; "verified-mandate+layerx-receipt" &#124; "trusted-agent-credential" &#124; "external-settlement+layerx-receipt"</code> |

## fmt-gw-external-state-labels

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-gw-external-state-labels</code> | <code>file_format</code> | [services/interop/crates/layerx-interop-gateway/src/server.rs](../../../services/interop/crates/layerx-interop-gateway/src/server.rs) | <code>ExternalState::label() -&gt; "pending" &#124; "submitted" &#124; "reversal-pending" &#124; "refused" &#124; "receipt-verified" &#124; "reversed"</code> |

## fmt-gw-ingress-transport-labels

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-gw-ingress-transport-labels</code> | <code>file_format</code> | [services/interop/crates/layerx-interop-gateway/src/server.rs](../../../services/interop/crates/layerx-interop-gateway/src/server.rs) | <code>IngressTransport::label() -&gt; "http" &#124; "mcp" &#124; "a2a"</code> |

## fmt-gw-ledger-events

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-gw-ledger-events</code> | <code>file_format</code> | [services/interop/crates/layerx-interop-gateway/src/gateway.rs](../../../services/interop/crates/layerx-interop-gateway/src/gateway.rs) | <code>layerxledger 0x101A logs: PaymentExact(bytes32,bytes32,bytes32,bytes32,uint128) data assetId+amount (64 bytes); PaymentDrawn(bytes32,bytes32,bytes32,bytes32,uint128,bytes32,bytes32)</code> |

## fmt-gw-retriability-labels

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-gw-retriability-labels</code> | <code>file_format</code> | [services/interop/crates/layerx-interop-gateway/src/error.rs](../../../services/interop/crates/layerx-interop-gateway/src/error.rs) | <code>Retriability::label() -&gt; "retriable" &#124; "terminal"</code> |

## fmt-gw-trace-header

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-gw-trace-header</code> | <code>file_format</code> | [services/interop/crates/layerx-interop-gateway/src/trace.rs](../../../services/interop/crates/layerx-interop-gateway/src/trace.rs) | <code>HTTP header x-layerx-trace: trc_&lt;id&gt; on inbound adapter requests and every outbound call</code> |

## fmt-gw-translation-kind-labels

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-gw-translation-kind-labels</code> | <code>file_format</code> | [services/interop/crates/layerx-interop-gateway/src/gateway.rs](../../../services/interop/crates/layerx-interop-gateway/src/gateway.rs) | <code>TranslationKind::label() -&gt; "read-only" &#124; "state-changing"</code> |

## fmt-gw-translation-record

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-gw-translation-record</code> | <code>file_format</code> | [services/interop/crates/layerx-interop-gateway/src/gateway.rs](../../../services/interop/crates/layerx-interop-gateway/src/gateway.rs) | <code>translation record: magic "LXIT", version u8 1, TranslationKind u8 (1 read-only, 2 state-changing), TranslationStatus u8 (1 pending, 2 translated, 3 receipt-verified, 4 refused), ...</code> |

## fmt-gw-translation-status-labels

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-gw-translation-status-labels</code> | <code>file_format</code> | [services/interop/crates/layerx-interop-gateway/src/gateway.rs](../../../services/interop/crates/layerx-interop-gateway/src/gateway.rs) | <code>TranslationStatus::label() -&gt; "pending" &#124; "translated" &#124; "receipt-verified" &#124; "refused"</code> |

## gateway-sequencer-evidence

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-gw-receipt-pending-j47</code> | <code>error_code</code> | [services/interop/crates/layerx-interop-gateway/src/error.rs](../../../services/interop/crates/layerx-interop-gateway/src/error.rs) | <code>receipt-pending</code> |
| <code>interface.11.rs-gw-evidence-canonical-receipt-digest-j47</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/evidence.rs](../../../services/interop/crates/layerx-interop-gateway/src/evidence.rs) | <code>pub fn canonical_receipt_digest(signed_receipt: &amp;[u8]) -&gt; Result&lt;[u8; 32], GatewayError&gt;</code> |
| <code>interface.11.rs-gw-evidence-sequencer-evidence-j47</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/evidence.rs](../../../services/interop/crates/layerx-interop-gateway/src/evidence.rs) | <code>pub struct SequencerEvidence { pub transaction_hash: [u8; 32], pub signed_receipt: Vec&lt;u8&gt;, pub logs: Vec&lt;SequencerLog&gt;, pub rung: SettlementRung }</code> |
| <code>interface.11.rs-gw-evidence-settlement-rung-j47</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/evidence.rs](../../../services/interop/crates/layerx-interop-gateway/src/evidence.rs) | <code>pub enum SettlementRung { Submitted, Receipted, Committed { block_number: u64, block_hash: [u8; 32] } }</code> |

| Field | Rule |
|---|---|
| interface | GatewayCore settles a translation on SequencerEvidence and records the canonical receipt digest. |
| storage | Translation records and audit chain kept by GatewayCore. |
| errors | receipt-pending (retriable), receipt-rejected, receipt-required. |
| events | Settlement appends TranslationCompleted after receipt verification; adapter registration, translation start and refusal also append audit entries. |
| gas | None; the gateway reads receipts and logs. |
| pause | Not pausable; refusal is per translation. |

## gateway-sequencer-settlement

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-gateway-core-settle-with-sequencer-evidence-j47</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/gateway.rs](../../../services/interop/crates/layerx-interop-gateway/src/gateway.rs) | <code>pub fn settle_with_sequencer_evidence(&amp;mut self, principal: &amp;PrincipalId, idempotency_key: [u8; 32], evidence: &amp;SequencerEvidence, receipt_keys: &amp;[SequencerAuthorization], trace: &amp;TraceId, now: u64) -&gt; Result&lt;TranslationStatus, Traced&lt;SequencerSettlementError&gt;&gt;</code> |

| Field | Rule |
|---|---|
| interface | GatewayCore settles a translation on SequencerEvidence and records the canonical receipt digest. |
| storage | Translation records and audit chain kept by GatewayCore. |
| errors | receipt-pending (retriable), receipt-rejected, receipt-required. |
| events | Settlement appends TranslationCompleted after receipt verification; adapter registration, translation start and refusal also append audit entries. |
| gas | None; the gateway reads receipts and logs. |
| pause | Not pausable; refusal is per translation. |

## gw-adapter-call-schema

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-adapter-call-schema</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/redaction.rs](../../../services/interop/crates/layerx-interop-gateway/src/redaction.rs) | <code>pub const ADAPTER_CALL_SCHEMA: &amp;str</code> |

## gw-adapter-descriptor

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-adapter-descriptor</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/adapter.rs](../../../services/interop/crates/layerx-interop-gateway/src/adapter.rs) | <code>pub struct AdapterDescriptor</code> |

## gw-adapter-descriptor-conformance

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-adapter-descriptor-conformance</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/adapter.rs](../../../services/interop/crates/layerx-interop-gateway/src/adapter.rs) | <code>pub const fn conformance(&amp;self) -&gt; &amp;ConformanceSuite</code> |

## gw-adapter-descriptor-id

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-adapter-descriptor-id</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/adapter.rs](../../../services/interop/crates/layerx-interop-gateway/src/adapter.rs) | <code>pub const fn id(&amp;self) -&gt; &amp;AdapterId</code> |

## gw-adapter-descriptor-new

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-adapter-descriptor-new</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/adapter.rs](../../../services/interop/crates/layerx-interop-gateway/src/adapter.rs) | <code>pub const fn new(id: AdapterId, spec: PinnedSpec, conformance: ConformanceSuite) -&gt; Self</code> |

## gw-adapter-descriptor-spec

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-adapter-descriptor-spec</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/adapter.rs](../../../services/interop/crates/layerx-interop-gateway/src/adapter.rs) | <code>pub const fn spec(&amp;self) -&gt; &amp;PinnedSpec</code> |

## gw-adapter-error

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-adapter-error</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/adapter.rs](../../../services/interop/crates/layerx-interop-gateway/src/adapter.rs) | <code>pub enum AdapterError</code> |

## gw-adapter-id

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-adapter-id</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/adapter.rs](../../../services/interop/crates/layerx-interop-gateway/src/adapter.rs) | <code>pub struct AdapterId(String)</code> |

## gw-adapter-id-as-str

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-adapter-id-as-str</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/adapter.rs](../../../services/interop/crates/layerx-interop-gateway/src/adapter.rs) | <code>pub fn as_str(&amp;self) -&gt; &amp;str</code> |

## gw-adapter-id-new

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-adapter-id-new</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/adapter.rs](../../../services/interop/crates/layerx-interop-gateway/src/adapter.rs) | <code>pub fn new(value: impl Into&lt;String&gt;) -&gt; Result&lt;Self, AdapterError&gt;</code> |

## gw-audit-append-schema

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-audit-append-schema</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/redaction.rs](../../../services/interop/crates/layerx-interop-gateway/src/redaction.rs) | <code>pub const AUDIT_APPEND_SCHEMA: &amp;str</code> |

## gw-audit-chain

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-audit-chain</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/audit.rs](../../../services/interop/crates/layerx-interop-gateway/src/audit.rs) | <code>pub struct AuditChain</code> |

## gw-audit-chain-append

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-audit-chain-append</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/audit.rs](../../../services/interop/crates/layerx-interop-gateway/src/audit.rs) | <code>pub fn append(&amp;mut self, recorded_at: u64, trace: &amp;TraceId, kind: AuditEventKind, adapter: &amp;str, subject: [u8; 32]) -&gt; Result&lt;&amp;ChainEntry, AuditError&gt;</code> |

## gw-audit-chain-entries

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-audit-chain-entries</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/audit.rs](../../../services/interop/crates/layerx-interop-gateway/src/audit.rs) | <code>pub fn entries(&amp;self) -&gt; &amp;[ChainEntry]</code> |

## gw-audit-chain-export

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-audit-chain-export</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/audit.rs](../../../services/interop/crates/layerx-interop-gateway/src/audit.rs) | <code>pub fn export(&amp;self) -&gt; Vec&lt;Vec&lt;u8&gt;&gt;</code> |

## gw-audit-chain-head

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-audit-chain-head</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/audit.rs](../../../services/interop/crates/layerx-interop-gateway/src/audit.rs) | <code>pub fn head(&amp;self) -&gt; Result&lt;ChainHead, AuditError&gt;</code> |

## gw-audit-chain-name

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-audit-chain-name</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/audit.rs](../../../services/interop/crates/layerx-interop-gateway/src/audit.rs) | <code>pub fn name(&amp;self) -&gt; &amp;str</code> |

## gw-audit-chain-new

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-audit-chain-new</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/audit.rs](../../../services/interop/crates/layerx-interop-gateway/src/audit.rs) | <code>pub fn new(name: impl Into&lt;String&gt;) -&gt; Self</code> |

## gw-audit-error

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-audit-error</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/audit.rs](../../../services/interop/crates/layerx-interop-gateway/src/audit.rs) | <code>pub enum AuditError</code> |

## gw-audit-event-kind

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-audit-event-kind</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/audit.rs](../../../services/interop/crates/layerx-interop-gateway/src/audit.rs) | <code>pub enum AuditEventKind</code> |

## gw-audit-event-kind-label

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-audit-event-kind-label</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/audit.rs](../../../services/interop/crates/layerx-interop-gateway/src/audit.rs) | <code>pub const fn label(self) -&gt; &amp;'static str</code> |

## gw-banned-field-names

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-banned-field-names</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/redaction.rs](../../../services/interop/crates/layerx-interop-gateway/src/redaction.rs) | <code>pub const BANNED_FIELD_NAMES: &amp;[&amp;str]</code> |

## gw-chain-entry

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-chain-entry</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/audit.rs](../../../services/interop/crates/layerx-interop-gateway/src/audit.rs) | <code>pub struct ChainEntry</code> |

## gw-chain-entry-adapter

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-chain-entry-adapter</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/audit.rs](../../../services/interop/crates/layerx-interop-gateway/src/audit.rs) | <code>pub fn adapter(&amp;self) -&gt; &amp;str</code> |

## gw-chain-entry-bytes

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-chain-entry-bytes</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/audit.rs](../../../services/interop/crates/layerx-interop-gateway/src/audit.rs) | <code>pub fn bytes(&amp;self) -&gt; &amp;[u8]</code> |

## gw-chain-entry-kind

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-chain-entry-kind</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/audit.rs](../../../services/interop/crates/layerx-interop-gateway/src/audit.rs) | <code>pub const fn kind(&amp;self) -&gt; AuditEventKind</code> |

## gw-chain-entry-link

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-chain-entry-link</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/audit.rs](../../../services/interop/crates/layerx-interop-gateway/src/audit.rs) | <code>pub const fn link(&amp;self) -&gt; [u8; 32]</code> |

## gw-chain-entry-prev-link

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-chain-entry-prev-link</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/audit.rs](../../../services/interop/crates/layerx-interop-gateway/src/audit.rs) | <code>pub const fn prev_link(&amp;self) -&gt; [u8; 32]</code> |

## gw-chain-entry-recorded-at

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-chain-entry-recorded-at</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/audit.rs](../../../services/interop/crates/layerx-interop-gateway/src/audit.rs) | <code>pub const fn recorded_at(&amp;self) -&gt; u64</code> |

## gw-chain-entry-sequence

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-chain-entry-sequence</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/audit.rs](../../../services/interop/crates/layerx-interop-gateway/src/audit.rs) | <code>pub const fn sequence(&amp;self) -&gt; u64</code> |

## gw-chain-entry-subject

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-chain-entry-subject</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/audit.rs](../../../services/interop/crates/layerx-interop-gateway/src/audit.rs) | <code>pub const fn subject(&amp;self) -&gt; [u8; 32]</code> |

## gw-chain-entry-trace

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-chain-entry-trace</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/audit.rs](../../../services/interop/crates/layerx-interop-gateway/src/audit.rs) | <code>pub const fn trace(&amp;self) -&gt; &amp;TraceId</code> |

## gw-chain-head

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-chain-head</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/audit.rs](../../../services/interop/crates/layerx-interop-gateway/src/audit.rs) | <code>pub struct ChainHead</code> |

## gw-chain-head-length

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-chain-head-length</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/audit.rs](../../../services/interop/crates/layerx-interop-gateway/src/audit.rs) | <code>pub const fn length(&amp;self) -&gt; u64</code> |

## gw-chain-head-link

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-chain-head-link</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/audit.rs](../../../services/interop/crates/layerx-interop-gateway/src/audit.rs) | <code>pub const fn link(&amp;self) -&gt; [u8; 32]</code> |

## gw-conformance-suite

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-conformance-suite</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/adapter.rs](../../../services/interop/crates/layerx-interop-gateway/src/adapter.rs) | <code>pub struct ConformanceSuite</code> |

## gw-conformance-suite-new

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-conformance-suite-new</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/adapter.rs](../../../services/interop/crates/layerx-interop-gateway/src/adapter.rs) | <code>pub fn new(suite: AdapterId, vector_count: u64, suite_digest: [u8; 32]) -&gt; Result&lt;Self, AdapterError&gt;</code> |

## gw-conformance-suite-suite

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-conformance-suite-suite</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/adapter.rs](../../../services/interop/crates/layerx-interop-gateway/src/adapter.rs) | <code>pub const fn suite(&amp;self) -&gt; &amp;AdapterId</code> |

## gw-conformance-suite-suite-digest

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-conformance-suite-suite-digest</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/adapter.rs](../../../services/interop/crates/layerx-interop-gateway/src/adapter.rs) | <code>pub const fn suite_digest(&amp;self) -&gt; [u8; 32]</code> |

## gw-conformance-suite-vector-count

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-conformance-suite-vector-count</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/adapter.rs](../../../services/interop/crates/layerx-interop-gateway/src/adapter.rs) | <code>pub const fn vector_count(&amp;self) -&gt; u64</code> |

## gw-decode

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-decode</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/redaction.rs](../../../services/interop/crates/layerx-interop-gateway/src/redaction.rs) | <code>pub fn decode(bytes: &amp;[u8]) -&gt; Result&lt;Emission, RedactionError&gt;</code> |

## gw-emission

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-emission</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/redaction.rs](../../../services/interop/crates/layerx-interop-gateway/src/redaction.rs) | <code>pub struct Emission</code> |

## gw-emission-schema

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-emission-schema</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/redaction.rs](../../../services/interop/crates/layerx-interop-gateway/src/redaction.rs) | <code>pub struct EmissionSchema</code> |

## gw-emission-values

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-emission-values</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/redaction.rs](../../../services/interop/crates/layerx-interop-gateway/src/redaction.rs) | <code>pub fn values(&amp;self) -&gt; &amp;[FieldValue]</code> |

## gw-emit

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-emit</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/redaction.rs](../../../services/interop/crates/layerx-interop-gateway/src/redaction.rs) | <code>pub fn emit(schema_name: &amp;str, values: &amp;[FieldValue]) -&gt; Result&lt;Vec&lt;u8&gt;, RedactionError&gt;</code> |

## gw-emitted-schemas

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-emitted-schemas</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/redaction.rs](../../../services/interop/crates/layerx-interop-gateway/src/redaction.rs) | <code>pub const EMITTED_SCHEMAS: &amp;[EmissionSchema]</code> |

## gw-error-emission

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-error-emission</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/error.rs](../../../services/interop/crates/layerx-interop-gateway/src/error.rs) | <code>pub fn error_emission(traced: &amp;Traced&lt;GatewayError&gt;) -&gt; Result&lt;Vec&lt;u8&gt;, RedactionError&gt;</code> |

## gw-error-response-schema

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-error-response-schema</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/redaction.rs](../../../services/interop/crates/layerx-interop-gateway/src/redaction.rs) | <code>pub const ERROR_RESPONSE_SCHEMA: &amp;str</code> |

## gw-evidence-policy

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-evidence-policy</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/server.rs](../../../services/interop/crates/layerx-interop-gateway/src/server.rs) | <code>pub enum EvidencePolicy</code> |

## gw-evidence-policy-label

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-evidence-policy-label</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/server.rs](../../../services/interop/crates/layerx-interop-gateway/src/server.rs) | <code>pub const fn label(self) -&gt; &amp;'static str</code> |

## gw-evidence-policy-parse

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-evidence-policy-parse</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/server.rs](../../../services/interop/crates/layerx-interop-gateway/src/server.rs) | <code>pub fn parse(value: &amp;str) -&gt; Option&lt;Self&gt;</code> |

## gw-external-state

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-external-state</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/server.rs](../../../services/interop/crates/layerx-interop-gateway/src/server.rs) | <code>pub enum ExternalState</code> |

## gw-external-state-label

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-external-state-label</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/server.rs](../../../services/interop/crates/layerx-interop-gateway/src/server.rs) | <code>pub const fn label(self) -&gt; &amp;'static str</code> |

## gw-field-value

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-field-value</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/redaction.rs](../../../services/interop/crates/layerx-interop-gateway/src/redaction.rs) | <code>pub enum FieldValue</code> |

## gw-field-value-class

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-field-value-class</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/redaction.rs](../../../services/interop/crates/layerx-interop-gateway/src/redaction.rs) | <code>pub const fn class(&amp;self) -&gt; SafeClass</code> |

## gw-gateway-core

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-gateway-core</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/gateway.rs](../../../services/interop/crates/layerx-interop-gateway/src/gateway.rs) | <code>pub struct GatewayCore</code> |

## gw-gateway-core-adapter

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-gateway-core-adapter</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/gateway.rs](../../../services/interop/crates/layerx-interop-gateway/src/gateway.rs) | <code>pub fn adapter(&amp;self, id: &amp;AdapterId) -&gt; Option&lt;&amp;AdapterDescriptor&gt;</code> |

## gw-gateway-core-adapter-ids

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-gateway-core-adapter-ids</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/gateway.rs](../../../services/interop/crates/layerx-interop-gateway/src/gateway.rs) | <code>pub fn adapter_ids(&amp;self) -&gt; Vec&lt;AdapterId&gt;</code> |

## gw-gateway-core-begin-translation

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-gateway-core-begin-translation</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/gateway.rs](../../../services/interop/crates/layerx-interop-gateway/src/gateway.rs) | <code>pub fn begin_translation(&amp;mut self, principal: &amp;PrincipalId, request: &amp;TranslationRequest, trace: &amp;TraceId, now: u64) -&gt; Result&lt;TranslationStatus, Traced&lt;GatewayError&gt;&gt;</code> |

## gw-gateway-core-complete-read-only

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-gateway-core-complete-read-only</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/gateway.rs](../../../services/interop/crates/layerx-interop-gateway/src/gateway.rs) | <code>pub fn complete_read_only(&amp;mut self, principal: &amp;PrincipalId, idempotency_key: [u8; 32], trace: &amp;TraceId, now: u64) -&gt; Result&lt;TranslationStatus, Traced&lt;GatewayError&gt;&gt;</code> |

## gw-gateway-core-new

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-gateway-core-new</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/gateway.rs](../../../services/interop/crates/layerx-interop-gateway/src/gateway.rs) | <code>pub fn new() -&gt; Self</code> |

## gw-gateway-core-principal-audit

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-gateway-core-principal-audit</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/gateway.rs](../../../services/interop/crates/layerx-interop-gateway/src/gateway.rs) | <code>pub fn principal_audit(&amp;self, principal: &amp;PrincipalId) -&gt; Option&lt;&amp;AuditChain&gt;</code> |

## gw-gateway-core-refuse-translation

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-gateway-core-refuse-translation</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/gateway.rs](../../../services/interop/crates/layerx-interop-gateway/src/gateway.rs) | <code>pub fn refuse_translation(&amp;mut self, principal: &amp;PrincipalId, idempotency_key: [u8; 32], trace: &amp;TraceId, now: u64) -&gt; Result&lt;TranslationStatus, Traced&lt;GatewayError&gt;&gt;</code> |

## gw-gateway-core-register-adapter

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-gateway-core-register-adapter</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/gateway.rs](../../../services/interop/crates/layerx-interop-gateway/src/gateway.rs) | <code>pub fn register_adapter(&amp;mut self, descriptor: AdapterDescriptor, trace: &amp;TraceId, now: u64) -&gt; Result&lt;(), Traced&lt;GatewayError&gt;&gt;</code> |

## gw-gateway-core-registry-audit

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-gateway-core-registry-audit</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/gateway.rs](../../../services/interop/crates/layerx-interop-gateway/src/gateway.rs) | <code>pub const fn registry_audit(&amp;self) -&gt; &amp;AuditChain</code> |

## gw-gateway-core-settle-with-sequencer-receipt

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-gateway-core-settle-with-sequencer-receipt</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/gateway.rs](../../../services/interop/crates/layerx-interop-gateway/src/gateway.rs) | <code>pub fn settle_with_sequencer_receipt(&amp;mut self, principal: &amp;PrincipalId, idempotency_key: [u8; 32], signed_receipt: &amp;[u8], logs: Vec&lt;SequencerLog&gt;, receipt_keys: &amp;[SequencerAuthorization], trace: &amp;TraceId, now: u64) -&gt; Result&lt;TranslationStatus, Traced&lt;SequencerSettlementError&gt;&gt;</code> |

## gw-gateway-core-translation

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-gateway-core-translation</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/gateway.rs](../../../services/interop/crates/layerx-interop-gateway/src/gateway.rs) | <code>pub fn translation(&amp;self, principal: &amp;PrincipalId, idempotency_key: [u8; 32]) -&gt; Option&lt;TranslationStatus&gt;</code> |

## gw-gateway-error

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-gateway-error</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/error.rs](../../../services/interop/crates/layerx-interop-gateway/src/error.rs) | <code>pub enum GatewayError</code> |

## gw-gateway-error-code

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-gateway-error-code</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/error.rs](../../../services/interop/crates/layerx-interop-gateway/src/error.rs) | <code>pub const fn code(&amp;self) -&gt; &amp;'static str</code> |

## gw-gateway-error-retriability

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-gateway-error-retriability</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/error.rs](../../../services/interop/crates/layerx-interop-gateway/src/error.rs) | <code>pub const fn retriability(&amp;self) -&gt; Retriability</code> |

## gw-gateway-store

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-gateway-store</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/principal.rs](../../../services/interop/crates/layerx-interop-gateway/src/principal.rs) | <code>pub struct GatewayStore</code> |

## gw-gateway-store-new

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-gateway-store-new</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/principal.rs](../../../services/interop/crates/layerx-interop-gateway/src/principal.rs) | <code>pub fn new() -&gt; Self</code> |

## gw-gateway-store-principal

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-gateway-store-principal</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/principal.rs](../../../services/interop/crates/layerx-interop-gateway/src/principal.rs) | <code>pub fn principal(&amp;mut self, id: &amp;PrincipalId) -&gt; PrincipalScope&lt;'_&gt;</code> |

## gw-gateway-store-principal-audit

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-gateway-store-principal-audit</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/principal.rs](../../../services/interop/crates/layerx-interop-gateway/src/principal.rs) | <code>pub fn principal_audit(&amp;self, id: &amp;PrincipalId) -&gt; Option&lt;&amp;AuditChain&gt;</code> |

## gw-gateway-store-read

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-gateway-store-read</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/principal.rs](../../../services/interop/crates/layerx-interop-gateway/src/principal.rs) | <code>pub fn read(&amp;self, id: &amp;PrincipalId, table: Table, key: &amp;RowKey) -&gt; Option&lt;StoredRow&lt;'_&gt;&gt;</code> |

## gw-hosted-adapter

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-hosted-adapter</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/server.rs](../../../services/interop/crates/layerx-interop-gateway/src/server.rs) | <code>pub enum HostedAdapter</code> |

## gw-hosted-adapter-id

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-hosted-adapter-id</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/server.rs](../../../services/interop/crates/layerx-interop-gateway/src/server.rs) | <code>pub const fn id(self) -&gt; &amp;'static str</code> |

## gw-hosted-adapter-surface

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-hosted-adapter-surface</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/server.rs](../../../services/interop/crates/layerx-interop-gateway/src/server.rs) | <code>pub const fn surface(self) -&gt; &amp;'static str</code> |

## gw-ingress-transport

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-ingress-transport</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/server.rs](../../../services/interop/crates/layerx-interop-gateway/src/server.rs) | <code>pub enum IngressTransport</code> |

## gw-ingress-transport-label

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-ingress-transport-label</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/server.rs](../../../services/interop/crates/layerx-interop-gateway/src/server.rs) | <code>pub const fn label(self) -&gt; &amp;'static str</code> |

## gw-interop-gateway-core

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-interop-gateway-core</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/lib.rs](../../../services/interop/crates/layerx-interop-gateway/src/lib.rs) | <code>pub fn interop_gateway_core() -&gt; GatewayCore</code> |

## gw-interop-gateway-routes

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-interop-gateway-routes</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/server.rs](../../../services/interop/crates/layerx-interop-gateway/src/server.rs) | <code>pub fn interop_gateway_routes&lt;'a&gt;(method: &amp;str, path: &amp;'a str) -&gt; Result&lt;InteropRoute&lt;'a&gt;, RouteError&gt;</code> |

## gw-interop-route

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-interop-route</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/server.rs](../../../services/interop/crates/layerx-interop-gateway/src/server.rs) | <code>pub enum InteropRoute&lt;'a&gt;</code> |

## gw-interop-route-adapter

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-interop-route-adapter</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/server.rs](../../../services/interop/crates/layerx-interop-gateway/src/server.rs) | <code>pub const fn adapter(&amp;self) -&gt; Option&lt;HostedAdapter&gt;</code> |

## gw-interop-route-state-changing

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-interop-route-state-changing</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/server.rs](../../../services/interop/crates/layerx-interop-gateway/src/server.rs) | <code>pub const fn state_changing(&amp;self) -&gt; bool</code> |

## gw-interop-workspace-manifest

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-interop-workspace-manifest</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/lib.rs](../../../services/interop/crates/layerx-interop-gateway/src/lib.rs) | <code>pub struct InteropWorkspaceManifest</code> |

## gw-label

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-label</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/redaction.rs](../../../services/interop/crates/layerx-interop-gateway/src/redaction.rs) | <code>pub struct Label(String)</code> |

## gw-label-as-str

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-label-as-str</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/redaction.rs](../../../services/interop/crates/layerx-interop-gateway/src/redaction.rs) | <code>pub fn as_str(&amp;self) -&gt; &amp;str</code> |

## gw-label-new

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-label-new</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/redaction.rs](../../../services/interop/crates/layerx-interop-gateway/src/redaction.rs) | <code>pub fn new(value: impl Into&lt;String&gt;) -&gt; Result&lt;Self, RedactionError&gt;</code> |

## gw-pinned-spec

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-pinned-spec</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/adapter.rs](../../../services/interop/crates/layerx-interop-gateway/src/adapter.rs) | <code>pub struct PinnedSpec</code> |

## gw-pinned-spec-document-digest

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-pinned-spec-document-digest</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/adapter.rs](../../../services/interop/crates/layerx-interop-gateway/src/adapter.rs) | <code>pub const fn document_digest(&amp;self) -&gt; [u8; 32]</code> |

## gw-pinned-spec-new

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-pinned-spec-new</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/adapter.rs](../../../services/interop/crates/layerx-interop-gateway/src/adapter.rs) | <code>pub fn new(protocol: AdapterId, version: SpecVersion, document_digest: [u8; 32]) -&gt; Result&lt;Self, AdapterError&gt;</code> |

## gw-pinned-spec-protocol

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-pinned-spec-protocol</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/adapter.rs](../../../services/interop/crates/layerx-interop-gateway/src/adapter.rs) | <code>pub const fn protocol(&amp;self) -&gt; &amp;AdapterId</code> |

## gw-pinned-spec-verify-document

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-pinned-spec-verify-document</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/adapter.rs](../../../services/interop/crates/layerx-interop-gateway/src/adapter.rs) | <code>pub fn verify_document(&amp;self, document: &amp;[u8]) -&gt; Result&lt;(), AdapterError&gt;</code> |

## gw-pinned-spec-version

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-pinned-spec-version</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/adapter.rs](../../../services/interop/crates/layerx-interop-gateway/src/adapter.rs) | <code>pub const fn version(&amp;self) -&gt; &amp;SpecVersion</code> |

## gw-principal-id

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-principal-id</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/principal.rs](../../../services/interop/crates/layerx-interop-gateway/src/principal.rs) | <code>pub struct PrincipalId(String)</code> |

## gw-principal-id-as-str

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-principal-id-as-str</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/principal.rs](../../../services/interop/crates/layerx-interop-gateway/src/principal.rs) | <code>pub fn as_str(&amp;self) -&gt; &amp;str</code> |

## gw-principal-id-new

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-principal-id-new</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/principal.rs](../../../services/interop/crates/layerx-interop-gateway/src/principal.rs) | <code>pub fn new(value: impl Into&lt;String&gt;) -&gt; Result&lt;Self, StoreError&gt;</code> |

## gw-principal-scope

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-principal-scope</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/principal.rs](../../../services/interop/crates/layerx-interop-gateway/src/principal.rs) | <code>pub struct PrincipalScope&lt;'a&gt;</code> |

## gw-principal-scope-audit

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-principal-scope-audit</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/principal.rs](../../../services/interop/crates/layerx-interop-gateway/src/principal.rs) | <code>pub fn audit(&amp;mut self) -&gt; &amp;mut AuditChain</code> |

## gw-principal-scope-audit-view

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-principal-scope-audit-view</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/principal.rs](../../../services/interop/crates/layerx-interop-gateway/src/principal.rs) | <code>pub fn audit_view(&amp;self) -&gt; &amp;AuditChain</code> |

## gw-principal-scope-get

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-principal-scope-get</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/principal.rs](../../../services/interop/crates/layerx-interop-gateway/src/principal.rs) | <code>pub fn get(&amp;self, table: Table, key: &amp;RowKey) -&gt; Option&lt;StoredRow&lt;'_&gt;&gt;</code> |

## gw-principal-scope-keys

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-principal-scope-keys</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/principal.rs](../../../services/interop/crates/layerx-interop-gateway/src/principal.rs) | <code>pub fn keys(&amp;self, table: Table) -&gt; Vec&lt;RowKey&gt;</code> |

## gw-principal-scope-principal

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-principal-scope-principal</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/principal.rs](../../../services/interop/crates/layerx-interop-gateway/src/principal.rs) | <code>pub const fn principal(&amp;self) -&gt; &amp;PrincipalId</code> |

## gw-principal-scope-put

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-principal-scope-put</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/principal.rs](../../../services/interop/crates/layerx-interop-gateway/src/principal.rs) | <code>pub fn put(&amp;mut self, table: Table, key: RowKey, written_at: u64, bytes: Vec&lt;u8&gt;)</code> |

## gw-redaction-emission-schema

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-redaction-emission-schema</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/redaction.rs](../../../services/interop/crates/layerx-interop-gateway/src/redaction.rs) | <code>pub const fn schema(&amp;self) -&gt; &amp;'static EmissionSchema</code> |

## gw-redaction-error

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-redaction-error</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/redaction.rs](../../../services/interop/crates/layerx-interop-gateway/src/redaction.rs) | <code>pub enum RedactionError</code> |

## gw-retriability

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-retriability</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/error.rs](../../../services/interop/crates/layerx-interop-gateway/src/error.rs) | <code>pub enum Retriability</code> |

## gw-retriability-label

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-retriability-label</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/error.rs](../../../services/interop/crates/layerx-interop-gateway/src/error.rs) | <code>pub const fn label(self) -&gt; &amp;'static str</code> |

## gw-root-interop-workspace-manifest

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-root-interop-workspace-manifest</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/lib.rs](../../../services/interop/crates/layerx-interop-gateway/src/lib.rs) | <code>pub const fn interop_workspace_manifest() -&gt; InteropWorkspaceManifest</code> |

## gw-route-error

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-route-error</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/server.rs](../../../services/interop/crates/layerx-interop-gateway/src/server.rs) | <code>pub enum RouteError</code> |

## gw-row-key

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-row-key</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/principal.rs](../../../services/interop/crates/layerx-interop-gateway/src/principal.rs) | <code>pub struct RowKey(String)</code> |

## gw-row-key-as-str

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-row-key-as-str</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/principal.rs](../../../services/interop/crates/layerx-interop-gateway/src/principal.rs) | <code>pub fn as_str(&amp;self) -&gt; &amp;str</code> |

## gw-row-key-new

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-row-key-new</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/principal.rs](../../../services/interop/crates/layerx-interop-gateway/src/principal.rs) | <code>pub fn new(value: impl Into&lt;String&gt;) -&gt; Result&lt;Self, StoreError&gt;</code> |

## gw-safe-class

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-safe-class</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/redaction.rs](../../../services/interop/crates/layerx-interop-gateway/src/redaction.rs) | <code>pub enum SafeClass</code> |

## gw-schema-field

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-schema-field</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/redaction.rs](../../../services/interop/crates/layerx-interop-gateway/src/redaction.rs) | <code>pub struct SchemaField</code> |

## gw-sequencer-settlement-error

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-sequencer-settlement-error</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/gateway.rs](../../../services/interop/crates/layerx-interop-gateway/src/gateway.rs) | <code>pub enum SequencerSettlementError</code> |

## gw-spec-version

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-spec-version</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/adapter.rs](../../../services/interop/crates/layerx-interop-gateway/src/adapter.rs) | <code>pub struct SpecVersion(String)</code> |

## gw-spec-version-as-str

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-spec-version-as-str</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/adapter.rs](../../../services/interop/crates/layerx-interop-gateway/src/adapter.rs) | <code>pub fn as_str(&amp;self) -&gt; &amp;str</code> |

## gw-spec-version-parse

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-spec-version-parse</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/adapter.rs](../../../services/interop/crates/layerx-interop-gateway/src/adapter.rs) | <code>pub fn parse(value: &amp;str) -&gt; Result&lt;Self, AdapterError&gt;</code> |

## gw-store-error

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-store-error</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/principal.rs](../../../services/interop/crates/layerx-interop-gateway/src/principal.rs) | <code>pub enum StoreError</code> |

## gw-stored-row

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-stored-row</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/principal.rs](../../../services/interop/crates/layerx-interop-gateway/src/principal.rs) | <code>pub struct StoredRow&lt;'a&gt;</code> |

## gw-stored-row-bytes

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-stored-row-bytes</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/principal.rs](../../../services/interop/crates/layerx-interop-gateway/src/principal.rs) | <code>pub const fn bytes(&amp;self) -&gt; &amp;'a [u8]</code> |

## gw-stored-row-written-at

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-stored-row-written-at</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/principal.rs](../../../services/interop/crates/layerx-interop-gateway/src/principal.rs) | <code>pub const fn written_at(&amp;self) -&gt; u64</code> |

## gw-table

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-table</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/principal.rs](../../../services/interop/crates/layerx-interop-gateway/src/principal.rs) | <code>pub enum Table</code> |

## gw-trace-error

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-trace-error</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/trace.rs](../../../services/interop/crates/layerx-interop-gateway/src/trace.rs) | <code>pub enum TraceError</code> |

## gw-trace-header

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-trace-header</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/trace.rs](../../../services/interop/crates/layerx-interop-gateway/src/trace.rs) | <code>pub const TRACE_HEADER: &amp;str</code> |

## gw-trace-id

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-trace-id</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/trace.rs](../../../services/interop/crates/layerx-interop-gateway/src/trace.rs) | <code>pub struct TraceId(String)</code> |

## gw-trace-id-as-str

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-trace-id-as-str</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/trace.rs](../../../services/interop/crates/layerx-interop-gateway/src/trace.rs) | <code>pub fn as_str(&amp;self) -&gt; &amp;str</code> |

## gw-trace-id-from-inbound

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-trace-id-from-inbound</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/trace.rs](../../../services/interop/crates/layerx-interop-gateway/src/trace.rs) | <code>pub fn from_inbound(header: Option&lt;&amp;str&gt;, entropy: [u8; 16]) -&gt; Self</code> |

## gw-trace-id-mint

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-trace-id-mint</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/trace.rs](../../../services/interop/crates/layerx-interop-gateway/src/trace.rs) | <code>pub fn mint(entropy: [u8; 16]) -&gt; Self</code> |

## gw-trace-id-outbound

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-trace-id-outbound</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/trace.rs](../../../services/interop/crates/layerx-interop-gateway/src/trace.rs) | <code>pub fn outbound(&amp;self) -&gt; (&amp;'static str, &amp;str)</code> |

## gw-trace-id-parse

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-trace-id-parse</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/trace.rs](../../../services/interop/crates/layerx-interop-gateway/src/trace.rs) | <code>pub fn parse(value: &amp;str) -&gt; Result&lt;Self, TraceError&gt;</code> |

## gw-trace-id-wrap

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-trace-id-wrap</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/trace.rs](../../../services/interop/crates/layerx-interop-gateway/src/trace.rs) | <code>pub fn wrap&lt;E&gt;(&amp;self, error: E) -&gt; Traced&lt;E&gt;</code> |

## gw-traced

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-traced</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/trace.rs](../../../services/interop/crates/layerx-interop-gateway/src/trace.rs) | <code>pub struct Traced&lt;E&gt;</code> |

## gw-traced-error

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-traced-error</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/trace.rs](../../../services/interop/crates/layerx-interop-gateway/src/trace.rs) | <code>pub const fn error(&amp;self) -&gt; &amp;E</code> |

## gw-traced-into-error

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-traced-into-error</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/trace.rs](../../../services/interop/crates/layerx-interop-gateway/src/trace.rs) | <code>pub fn into_error(self) -&gt; E</code> |

## gw-traced-new

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-traced-new</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/trace.rs](../../../services/interop/crates/layerx-interop-gateway/src/trace.rs) | <code>pub fn new(trace: TraceId, error: E) -&gt; Self</code> |

## gw-traced-trace

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-traced-trace</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/trace.rs](../../../services/interop/crates/layerx-interop-gateway/src/trace.rs) | <code>pub const fn trace(&amp;self) -&gt; &amp;TraceId</code> |

## gw-translation-kind

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-translation-kind</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/gateway.rs](../../../services/interop/crates/layerx-interop-gateway/src/gateway.rs) | <code>pub enum TranslationKind</code> |

## gw-translation-kind-label

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-translation-kind-label</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/gateway.rs](../../../services/interop/crates/layerx-interop-gateway/src/gateway.rs) | <code>pub const fn label(self) -&gt; &amp;'static str</code> |

## gw-translation-request

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-translation-request</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/gateway.rs](../../../services/interop/crates/layerx-interop-gateway/src/gateway.rs) | <code>pub struct TranslationRequest</code> |

## gw-translation-request-adapter

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-translation-request-adapter</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/gateway.rs](../../../services/interop/crates/layerx-interop-gateway/src/gateway.rs) | <code>pub const fn adapter(&amp;self) -&gt; &amp;AdapterId</code> |

## gw-translation-request-idempotency-key

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-translation-request-idempotency-key</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/gateway.rs](../../../services/interop/crates/layerx-interop-gateway/src/gateway.rs) | <code>pub const fn idempotency_key(&amp;self) -&gt; [u8; 32]</code> |

## gw-translation-request-kind

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-translation-request-kind</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/gateway.rs](../../../services/interop/crates/layerx-interop-gateway/src/gateway.rs) | <code>pub const fn kind(&amp;self) -&gt; TranslationKind</code> |

## gw-translation-request-new

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-translation-request-new</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/gateway.rs](../../../services/interop/crates/layerx-interop-gateway/src/gateway.rs) | <code>pub fn new(adapter: AdapterId, kind: TranslationKind, idempotency_key: [u8; 32], request_digest: [u8; 32]) -&gt; Result&lt;Self, GatewayError&gt;</code> |

## gw-translation-request-request-digest

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-translation-request-request-digest</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/gateway.rs](../../../services/interop/crates/layerx-interop-gateway/src/gateway.rs) | <code>pub const fn request_digest(&amp;self) -&gt; [u8; 32]</code> |

## gw-translation-status

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-translation-status</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/gateway.rs](../../../services/interop/crates/layerx-interop-gateway/src/gateway.rs) | <code>pub enum TranslationStatus</code> |

## gw-translation-status-label

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-translation-status-label</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/gateway.rs](../../../services/interop/crates/layerx-interop-gateway/src/gateway.rs) | <code>pub const fn label(self) -&gt; &amp;'static str</code> |

## gw-verify-export

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-verify-export</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/audit.rs](../../../services/interop/crates/layerx-interop-gateway/src/audit.rs) | <code>pub fn verify_export(name: &amp;str, exported: &amp;[Vec&lt;u8&gt;], head: &amp;ChainHead) -&gt; Result&lt;Vec&lt;ChainEntry&gt;, AuditError&gt;</code> |

## gw-verify-registry

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-verify-registry</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/redaction.rs](../../../services/interop/crates/layerx-interop-gateway/src/redaction.rs) | <code>pub fn verify_registry() -&gt; Result&lt;(), RedactionError&gt;</code> |

## gw-verify-schema

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gw-verify-schema</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-interop-gateway/src/redaction.rs](../../../services/interop/crates/layerx-interop-gateway/src/redaction.rs) | <code>pub fn verify_schema(schema: &amp;EmissionSchema) -&gt; Result&lt;(), RedactionError&gt;</code> |

## http-gw-adapters

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.http-gw-adapters</code> | <code>http_route</code> | [services/interop/crates/layerx-interop-gateway/src/server.rs](../../../services/interop/crates/layerx-interop-gateway/src/server.rs) | <code>GET /v1/adapters</code> |

## http-gw-ap2-execute

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.http-gw-ap2-execute</code> | <code>http_route</code> | [services/interop/crates/layerx-interop-gateway/src/server.rs](../../../services/interop/crates/layerx-interop-gateway/src/server.rs) | <code>POST /v1/http/ap2/execute</code> |

## http-gw-ap2-mandates-verify

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.http-gw-ap2-mandates-verify</code> | <code>http_route</code> | [services/interop/crates/layerx-interop-gateway/src/server.rs](../../../services/interop/crates/layerx-interop-gateway/src/server.rs) | <code>POST /v1/http/ap2/mandates/verify</code> |

## http-gw-fiat-bank-callbacks

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.http-gw-fiat-bank-callbacks</code> | <code>http_route</code> | [services/interop/crates/layerx-interop-gateway/src/server.rs](../../../services/interop/crates/layerx-interop-gateway/src/server.rs) | <code>POST /v1/http/fiat/bank/callbacks</code> |

## http-gw-fiat-card-callbacks

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.http-gw-fiat-card-callbacks</code> | <code>http_route</code> | [services/interop/crates/layerx-interop-gateway/src/server.rs](../../../services/interop/crates/layerx-interop-gateway/src/server.rs) | <code>POST /v1/http/fiat/card/callbacks</code> |

## http-gw-fiat-rtp-callbacks

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.http-gw-fiat-rtp-callbacks</code> | <code>http_route</code> | [services/interop/crates/layerx-interop-gateway/src/server.rs](../../../services/interop/crates/layerx-interop-gateway/src/server.rs) | <code>POST /v1/http/fiat/rtp/callbacks</code> |

## http-gw-livez

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.http-gw-livez</code> | <code>http_route</code> | [services/interop/crates/layerx-interop-gateway/src/server.rs](../../../services/interop/crates/layerx-interop-gateway/src/server.rs) | <code>GET /livez</code> |

## http-gw-migration-accounts

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.http-gw-migration-accounts</code> | <code>http_route</code> | [services/interop/crates/layerx-interop-gateway/src/server.rs](../../../services/interop/crates/layerx-interop-gateway/src/server.rs) | <code>POST /v2/migration/accounts</code> |

## http-gw-migration-assets

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.http-gw-migration-assets</code> | <code>http_route</code> | [services/interop/crates/layerx-interop-gateway/src/server.rs](../../../services/interop/crates/layerx-interop-gateway/src/server.rs) | <code>POST /v2/migration/assets</code> |

## http-gw-migration-history

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.http-gw-migration-history</code> | <code>http_route</code> | [services/interop/crates/layerx-interop-gateway/src/server.rs](../../../services/interop/crates/layerx-interop-gateway/src/server.rs) | <code>POST /v2/migration/history</code> |

## http-gw-migration-history-read

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.http-gw-migration-history-read</code> | <code>http_route</code> | [services/interop/crates/layerx-interop-gateway/src/server.rs](../../../services/interop/crates/layerx-interop-gateway/src/server.rs) | <code>POST /v2/migration/history/read</code> |

## http-gw-operations

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.http-gw-operations</code> | <code>http_route</code> | [services/interop/crates/layerx-interop-gateway/src/server.rs](../../../services/interop/crates/layerx-interop-gateway/src/server.rs) | <code>GET /v1/operations/{operation: 64 hex}</code> |

## http-gw-readyz

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.http-gw-readyz</code> | <code>http_route</code> | [services/interop/crates/layerx-interop-gateway/src/server.rs](../../../services/interop/crates/layerx-interop-gateway/src/server.rs) | <code>GET /readyz</code> |

## http-gw-ucp-checkouts-complete

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.http-gw-ucp-checkouts-complete</code> | <code>http_route</code> | [services/interop/crates/layerx-interop-gateway/src/server.rs](../../../services/interop/crates/layerx-interop-gateway/src/server.rs) | <code>POST /v1/http/ucp/checkouts/complete</code> |

## http-gw-visa-tap-intents-execute

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.http-gw-visa-tap-intents-execute</code> | <code>http_route</code> | [services/interop/crates/layerx-interop-gateway/src/server.rs](../../../services/interop/crates/layerx-interop-gateway/src/server.rs) | <code>POST /v1/http/visa-tap/intents/execute</code> |

## http-gw-visa-tap-intents-verify

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.http-gw-visa-tap-intents-verify</code> | <code>http_route</code> | [services/interop/crates/layerx-interop-gateway/src/server.rs](../../../services/interop/crates/layerx-interop-gateway/src/server.rs) | <code>POST /v1/http/visa-tap/intents/verify</code> |

## http-gw-x402-buyer-build

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.http-gw-x402-buyer-build</code> | <code>http_route</code> | [services/interop/crates/layerx-interop-gateway/src/server.rs](../../../services/interop/crates/layerx-interop-gateway/src/server.rs) | <code>POST /v1/{http&#124;mcp&#124;a2a}/x402/buyer/build</code> |

## http-gw-x402-seller-offer

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.http-gw-x402-seller-offer</code> | <code>http_route</code> | [services/interop/crates/layerx-interop-gateway/src/server.rs](../../../services/interop/crates/layerx-interop-gateway/src/server.rs) | <code>POST /v1/{http&#124;mcp&#124;a2a}/x402/seller/offer</code> |

## http-gw-x402-settle

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.http-gw-x402-settle</code> | <code>http_route</code> | [services/interop/crates/layerx-interop-gateway/src/server.rs](../../../services/interop/crates/layerx-interop-gateway/src/server.rs) | <code>POST /v1/{http&#124;mcp&#124;a2a}/x402/settle, /seller/settle and /facilitator/settle</code> |

## http-gw-x402-supported

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.http-gw-x402-supported</code> | <code>http_route</code> | [services/interop/crates/layerx-interop-gateway/src/server.rs](../../../services/interop/crates/layerx-interop-gateway/src/server.rs) | <code>GET /v1/{http&#124;mcp&#124;a2a}/x402/supported and /v1/{transport}/x402/facilitator/supported</code> |

## http-gw-x402-verify

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.http-gw-x402-verify</code> | <code>http_route</code> | [services/interop/crates/layerx-interop-gateway/src/server.rs](../../../services/interop/crates/layerx-interop-gateway/src/server.rs) | <code>POST /v1/{http&#124;mcp&#124;a2a}/x402/verify and /v1/{transport}/x402/facilitator/verify</code> |
